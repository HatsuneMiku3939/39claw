package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultGitExecutablePath      = "git"
	defaultClosedTaskRetention    = 15
	worktreesDirectoryName        = "worktrees"
	managedRepositoriesDirectory  = "repos"
	worktreeDirectoryPerms        = 0o755
	managedOriginFetchRefspec     = "+refs/heads/*:refs/remotes/origin/*"
	managedRepositoryHashByteSize = 6
)

type TaskWorkspaceManagerDependencies struct {
	Store            ThreadStore
	SourceRepository string
	DataDir          string
	GitExecutable    string
	ClosedRetention  int
	Logger           *slog.Logger
	Clock            func() time.Time
}

type GitTaskWorkspaceManager struct {
	store            ThreadStore
	sourceRepository string
	dataDir          string
	gitExecutable    string
	closedRetention  int
	logger           *slog.Logger
	clock            func() time.Time
}

type gitRemoteConfig struct {
	url     string
	pushURL string
}

func NewTaskWorkspaceManager(ctx context.Context, deps TaskWorkspaceManagerDependencies) (*GitTaskWorkspaceManager, error) {
	if deps.Store == nil {
		return nil, errors.New("thread store must not be nil")
	}

	sourceRepository := strings.TrimSpace(deps.SourceRepository)
	if sourceRepository == "" {
		return nil, errors.New("source repository must not be empty")
	}

	dataDir := strings.TrimSpace(deps.DataDir)
	if dataDir == "" {
		return nil, errors.New("data dir must not be empty")
	}

	gitExecutable := strings.TrimSpace(deps.GitExecutable)
	if gitExecutable == "" {
		gitExecutable = defaultGitExecutablePath
	}

	closedRetention := deps.ClosedRetention
	if closedRetention <= 0 {
		closedRetention = defaultClosedTaskRetention
	}

	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	clock := deps.Clock
	if clock == nil {
		clock = time.Now().UTC
	}

	manager := &GitTaskWorkspaceManager{
		store:            deps.Store,
		sourceRepository: sourceRepository,
		dataDir:          dataDir,
		gitExecutable:    gitExecutable,
		closedRetention:  closedRetention,
		logger:           logger,
		clock:            clock,
	}

	if _, err := manager.sourceOriginConfig(ctx); err != nil {
		return nil, err
	}

	return manager, nil
}

func (m *GitTaskWorkspaceManager) EnsureReady(ctx context.Context, task Task) (Task, error) {
	if strings.TrimSpace(task.TaskID) == "" {
		return Task{}, errors.New("task id must not be empty")
	}

	if task.BranchName == "" {
		task.BranchName = DefaultTaskBranchName(task.TaskName, task.TaskID)
	}

	if task.WorktreeStatus == "" {
		task.WorktreeStatus = TaskWorktreeStatusPending
	}

	if task.WorktreeStatus == TaskWorktreeStatusReady && strings.TrimSpace(task.WorktreePath) != "" {
		return task, nil
	}

	managedRepositoryPath, err := m.ensureManagedRepository(ctx)
	if err != nil {
		return Task{}, m.markTaskWorktreeFailed(ctx, task, "", err)
	}

	baseRef := task.BaseRef
	if baseRef == "" {
		detectedBaseRef, err := m.detectBaseRef(ctx, managedRepositoryPath)
		if err != nil {
			return Task{}, m.markTaskWorktreeFailed(ctx, task, "", err)
		}
		baseRef = detectedBaseRef
	}

	worktreePath := task.WorktreePath
	if worktreePath == "" {
		worktreePath = m.worktreePath(task.TaskID)
	}

	if err := m.prepareWorktreePath(ctx, managedRepositoryPath, worktreePath); err != nil {
		return Task{}, m.markTaskWorktreeFailed(ctx, task, worktreePath, err)
	}

	branchExists, err := m.branchExists(ctx, managedRepositoryPath, task.BranchName)
	if err != nil {
		return Task{}, m.markTaskWorktreeFailed(ctx, task, worktreePath, err)
	}

	args := []string{"worktree", "add"}
	if !branchExists {
		args = append(args, "-b", task.BranchName)
	}
	args = append(args, worktreePath)
	if branchExists {
		args = append(args, task.BranchName)
	} else {
		args = append(args, baseRef)
	}

	if _, err := m.runGitIn(ctx, managedRepositoryPath, args...); err != nil {
		return Task{}, m.markTaskWorktreeFailed(ctx, task, worktreePath, err)
	}

	now := m.clock()
	task.BaseRef = baseRef
	task.WorktreePath = worktreePath
	task.WorktreeStatus = TaskWorktreeStatusReady
	task.WorktreePrunedAt = nil
	task.UpdatedAt = now
	if task.WorktreeCreatedAt == nil {
		task.WorktreeCreatedAt = &now
	}

	if err := m.store.UpdateTask(ctx, task); err != nil {
		return Task{}, fmt.Errorf("persist ready task worktree: %w", err)
	}

	return task, nil
}

func (m *GitTaskWorkspaceManager) PruneClosed(ctx context.Context) error {
	tasks, err := m.store.ListClosedReadyTasks(ctx)
	if err != nil {
		return fmt.Errorf("list closed ready tasks: %w", err)
	}

	if len(tasks) <= m.closedRetention {
		return nil
	}

	managedRepositoryPath, err := m.ensureManagedRepository(ctx)
	if err != nil {
		return fmt.Errorf("ensure managed task repository: %w", err)
	}

	var errs []error
	for _, task := range tasks[m.closedRetention:] {
		if strings.TrimSpace(task.WorktreePath) == "" {
			continue
		}

		if _, err := m.runGitIn(ctx, managedRepositoryPath, "worktree", "remove", "--force", task.WorktreePath); err != nil {
			m.logger.Error(
				"prune closed task worktree",
				"task_id", task.TaskID,
				"worktree_path", task.WorktreePath,
				"error", err,
			)
			errs = append(errs, fmt.Errorf("prune task %s: %w", task.TaskID, err))
			continue
		}

		now := m.clock()
		task.WorktreeStatus = TaskWorktreeStatusPruned
		task.WorktreePrunedAt = &now
		task.UpdatedAt = now
		if err := m.store.UpdateTask(ctx, task); err != nil {
			m.logger.Error(
				"persist pruned task worktree",
				"task_id", task.TaskID,
				"worktree_path", task.WorktreePath,
				"error", err,
			)
			errs = append(errs, fmt.Errorf("persist pruned task %s: %w", task.TaskID, err))
		}
	}

	return errors.Join(errs...)
}

func (m *GitTaskWorkspaceManager) CurrentBranch(ctx context.Context, task Task) (string, bool, error) {
	if task.WorktreeStatus != TaskWorktreeStatusReady {
		return "", false, nil
	}

	worktreePath := strings.TrimSpace(task.WorktreePath)
	if worktreePath == "" {
		return "", false, nil
	}

	branch, err := m.runGitIn(ctx, worktreePath, "branch", "--show-current")
	if err != nil {
		return "", false, fmt.Errorf("read current worktree branch: %w", err)
	}

	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "", false, nil
	}

	return branch, true, nil
}

func (m *GitTaskWorkspaceManager) detectBaseRef(ctx context.Context, repositoryPath string) (string, error) {
	if ref, ok := m.originHeadRef(ctx, repositoryPath); ok {
		return ref, nil
	}

	for _, ref := range []string{"origin/main", "origin/master", "main", "master"} {
		exists, err := m.refExists(ctx, repositoryPath, ref)
		if err != nil {
			return "", err
		}
		if exists {
			return ref, nil
		}
	}

	return "", errors.New("detect task worktree base ref: expected origin/HEAD, origin/main, origin/master, main, or master")
}

func (m *GitTaskWorkspaceManager) branchExists(ctx context.Context, repositoryPath string, branchName string) (bool, error) {
	_, err := m.runGitIn(ctx, repositoryPath, "show-ref", "--verify", "--quiet", "refs/heads/"+branchName)
	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}

	return false, fmt.Errorf("check branch existence: %w", err)
}

func (m *GitTaskWorkspaceManager) refreshOrigin(ctx context.Context, repositoryPath string) {
	exists, err := m.remoteExistsIn(ctx, repositoryPath, "origin")
	if err != nil {
		m.logger.Warn("check git remote before task worktree fetch", "remote", "origin", "error", err)
		return
	}
	if !exists {
		return
	}

	if _, err := m.runGitIn(ctx, repositoryPath, "fetch", "origin", "--prune"); err != nil {
		m.logger.Warn("refresh git remote before task worktree base ref detection", "remote", "origin", "error", err)
		return
	}

	if _, err := m.runGitIn(ctx, repositoryPath, "remote", "set-head", "origin", "--auto"); err != nil {
		m.logger.Warn("refresh git remote head for task worktree base ref detection", "remote", "origin", "error", err)
	}
}

func (m *GitTaskWorkspaceManager) remoteExistsIn(ctx context.Context, repositoryPath string, remoteName string) (bool, error) {
	_, err := m.runGitIn(ctx, repositoryPath, "remote", "get-url", remoteName)
	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
		return false, nil
	}

	return false, fmt.Errorf("check remote existence: %w", err)
}

func (m *GitTaskWorkspaceManager) originHeadRef(ctx context.Context, repositoryPath string) (string, bool) {
	ref, err := m.runGitIn(ctx, repositoryPath, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err != nil || strings.TrimSpace(ref) == "" {
		return "", false
	}

	exists, verifyErr := m.refExists(ctx, repositoryPath, ref)
	if verifyErr != nil || !exists {
		return "", false
	}

	return ref, true
}

func (m *GitTaskWorkspaceManager) refExists(ctx context.Context, repositoryPath string, ref string) (bool, error) {
	_, err := m.runGitIn(ctx, repositoryPath, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}

	return false, fmt.Errorf("check ref existence %q: %w", ref, err)
}

func (m *GitTaskWorkspaceManager) prepareWorktreePath(ctx context.Context, repositoryPath string, worktreePath string) error {
	if err := os.MkdirAll(filepath.Dir(worktreePath), worktreeDirectoryPerms); err != nil {
		return fmt.Errorf("create worktree parent directory: %w", err)
	}

	if _, statErr := os.Stat(worktreePath); statErr == nil {
		if _, err := m.runGitIn(ctx, repositoryPath, "worktree", "remove", "--force", worktreePath); err != nil {
			m.logger.Debug("ignore stale worktree removal failure before retry", "worktree_path", worktreePath, "error", err)
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("stat existing worktree path: %w", statErr)
	}

	if err := os.RemoveAll(worktreePath); err != nil {
		return fmt.Errorf("clear worktree path: %w", err)
	}

	return nil
}

func (m *GitTaskWorkspaceManager) markTaskWorktreeFailed(
	ctx context.Context,
	task Task,
	worktreePath string,
	cause error,
) error {
	now := m.clock()
	task.WorktreeStatus = TaskWorktreeStatusFailed
	task.UpdatedAt = now
	if task.WorktreePath == "" {
		task.WorktreePath = worktreePath
	}

	if err := m.store.UpdateTask(ctx, task); err != nil {
		return fmt.Errorf("mark task worktree failed: %w: %v", err, cause)
	}

	return cause
}

func (m *GitTaskWorkspaceManager) worktreePath(taskID string) string {
	return filepath.Join(m.dataDir, worktreesDirectoryName, taskID)
}

func (m *GitTaskWorkspaceManager) managedRepositoryPath() string {
	baseName := sanitizeTaskRepositoryName(filepath.Base(filepath.Clean(m.sourceRepository)))
	if baseName == "" {
		baseName = "source-repository"
	}

	sourceHash := sha256.Sum256([]byte(filepath.Clean(m.sourceRepository)))
	suffix := hex.EncodeToString(sourceHash[:managedRepositoryHashByteSize])

	return filepath.Join(m.dataDir, managedRepositoriesDirectory, baseName+"-"+suffix+".git")
}

func (m *GitTaskWorkspaceManager) ensureManagedRepository(ctx context.Context) (string, error) {
	remoteConfig, err := m.sourceOriginConfig(ctx)
	if err != nil {
		return "", err
	}

	managedRepositoryPath := m.managedRepositoryPath()
	if err := os.MkdirAll(filepath.Dir(managedRepositoryPath), worktreeDirectoryPerms); err != nil {
		return "", fmt.Errorf("create managed repository parent directory: %w", err)
	}

	_, statErr := os.Stat(managedRepositoryPath)
	switch {
	case statErr == nil:
		isBare, bareErr := m.isBareRepository(ctx, managedRepositoryPath)
		if bareErr != nil {
			return "", bareErr
		}
		if !isBare {
			return "", fmt.Errorf("managed task repository is not bare: %s", managedRepositoryPath)
		}
	case errors.Is(statErr, os.ErrNotExist):
		if err := m.initBareRepository(ctx, managedRepositoryPath); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("stat managed task repository: %w", statErr)
	}

	if err := m.syncManagedOriginConfig(ctx, managedRepositoryPath, remoteConfig); err != nil {
		return "", err
	}

	m.refreshOrigin(ctx, managedRepositoryPath)
	return managedRepositoryPath, nil
}

func (m *GitTaskWorkspaceManager) sourceOriginConfig(ctx context.Context) (gitRemoteConfig, error) {
	exists, err := m.remoteExistsIn(ctx, m.sourceRepository, "origin")
	if err != nil {
		return gitRemoteConfig{}, fmt.Errorf("check source repository origin remote: %w", err)
	}
	if !exists {
		return gitRemoteConfig{}, fmt.Errorf(
			"task mode requires CLAW_CODEX_WORKDIR to have an origin remote: %s",
			m.sourceRepository,
		)
	}

	url, err := m.runGitIn(ctx, m.sourceRepository, "remote", "get-url", "origin")
	if err != nil {
		return gitRemoteConfig{}, fmt.Errorf("load source repository origin remote URL: %w", err)
	}

	pushURL, err := m.optionalGitConfigValue(ctx, m.sourceRepository, "remote.origin.pushurl")
	if err != nil {
		return gitRemoteConfig{}, fmt.Errorf("load source repository origin push URL: %w", err)
	}

	return gitRemoteConfig{
		url:     strings.TrimSpace(url),
		pushURL: strings.TrimSpace(pushURL),
	}, nil
}

func (m *GitTaskWorkspaceManager) isBareRepository(ctx context.Context, repositoryPath string) (bool, error) {
	value, err := m.runGitIn(ctx, repositoryPath, "rev-parse", "--is-bare-repository")
	if err != nil {
		return false, fmt.Errorf("check managed task repository type: %w", err)
	}

	return strings.EqualFold(strings.TrimSpace(value), "true"), nil
}

func (m *GitTaskWorkspaceManager) initBareRepository(ctx context.Context, repositoryPath string) error {
	//nolint:gosec // The git executable path is intentionally configurable for tests.
	cmd := exec.CommandContext(ctx, m.gitExecutable, "init", "--bare", repositoryPath)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message == "" {
			return fmt.Errorf("initialize managed task repository: %w", err)
		}
		return fmt.Errorf("initialize managed task repository: %w: %s", err, message)
	}

	return nil
}

func (m *GitTaskWorkspaceManager) syncManagedOriginConfig(
	ctx context.Context,
	repositoryPath string,
	remoteConfig gitRemoteConfig,
) error {
	exists, err := m.remoteExistsIn(ctx, repositoryPath, "origin")
	if err != nil {
		return fmt.Errorf("check managed task repository origin remote: %w", err)
	}

	if exists {
		if _, err := m.runGitIn(ctx, repositoryPath, "remote", "set-url", "origin", remoteConfig.url); err != nil {
			return fmt.Errorf("update managed task repository origin remote URL: %w", err)
		}
	} else {
		if _, err := m.runGitIn(ctx, repositoryPath, "remote", "add", "origin", remoteConfig.url); err != nil {
			return fmt.Errorf("add managed task repository origin remote: %w", err)
		}
	}

	if _, err := m.runGitIn(ctx, repositoryPath, "config", "--replace-all", "remote.origin.fetch", managedOriginFetchRefspec); err != nil {
		return fmt.Errorf("configure managed task repository origin fetch refspec: %w", err)
	}

	if err := m.syncManagedOriginPushURL(ctx, repositoryPath, remoteConfig.pushURL); err != nil {
		return err
	}

	return nil
}

func (m *GitTaskWorkspaceManager) syncManagedOriginPushURL(
	ctx context.Context,
	repositoryPath string,
	pushURL string,
) error {
	existingPushURL, err := m.optionalGitConfigValue(ctx, repositoryPath, "remote.origin.pushurl")
	if err != nil {
		return fmt.Errorf("check managed task repository origin push URL: %w", err)
	}

	if existingPushURL != "" {
		if _, err := m.runGitIn(ctx, repositoryPath, "config", "--unset-all", "remote.origin.pushurl"); err != nil {
			return fmt.Errorf("clear managed task repository origin push URL: %w", err)
		}
	}

	if strings.TrimSpace(pushURL) == "" {
		return nil
	}

	if _, err := m.runGitIn(ctx, repositoryPath, "config", "remote.origin.pushurl", pushURL); err != nil {
		return fmt.Errorf("configure managed task repository origin push URL: %w", err)
	}

	return nil
}

func (m *GitTaskWorkspaceManager) optionalGitConfigValue(
	ctx context.Context,
	repositoryPath string,
	key string,
) (string, error) {
	value, err := m.runGitIn(ctx, repositoryPath, "config", "--get", key)
	if err == nil {
		return strings.TrimSpace(value), nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return "", nil
	}

	return "", fmt.Errorf("load git config %s: %w", key, err)
}

func (m *GitTaskWorkspaceManager) runGitIn(ctx context.Context, repositoryPath string, args ...string) (string, error) {
	commandArgs := append([]string{"-C", repositoryPath}, args...)

	//nolint:gosec // The git executable path is intentionally configurable for tests.
	cmd := exec.CommandContext(ctx, m.gitExecutable, commandArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message == "" {
			return "", fmt.Errorf("run git %s: %w", strings.Join(args, " "), err)
		}
		return "", fmt.Errorf("run git %s: %w: %s", strings.Join(args, " "), err, message)
	}

	return strings.TrimSpace(stdout.String()), nil
}

func sanitizeTaskRepositoryName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}

	var builder strings.Builder
	builder.Grow(len(trimmed))
	for _, r := range trimmed {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r + ('a' - 'A'))
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-', r == '_', r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteByte('-')
		}
	}

	return strings.Trim(builder.String(), "-.")
}
