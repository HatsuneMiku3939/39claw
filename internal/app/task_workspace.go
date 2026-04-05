package app

import (
	"bytes"
	"context"
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
	defaultGitExecutablePath   = "git"
	defaultClosedTaskRetention = 15
	worktreesDirectoryName     = "worktrees"
	worktreeDirectoryPerms     = 0o755
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

func NewTaskWorkspaceManager(deps TaskWorkspaceManagerDependencies) (*GitTaskWorkspaceManager, error) {
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

	return &GitTaskWorkspaceManager{
		store:            deps.Store,
		sourceRepository: sourceRepository,
		dataDir:          dataDir,
		gitExecutable:    gitExecutable,
		closedRetention:  closedRetention,
		logger:           logger,
		clock:            clock,
	}, nil
}

func DefaultTaskBranchName(taskID string) string {
	return "task/" + strings.TrimSpace(taskID)
}

func (m *GitTaskWorkspaceManager) EnsureReady(ctx context.Context, task Task) (Task, error) {
	if strings.TrimSpace(task.TaskID) == "" {
		return Task{}, errors.New("task id must not be empty")
	}

	if task.BranchName == "" {
		task.BranchName = DefaultTaskBranchName(task.TaskID)
	}

	if task.WorktreeStatus == "" {
		task.WorktreeStatus = TaskWorktreeStatusPending
	}

	if task.WorktreeStatus == TaskWorktreeStatusReady && strings.TrimSpace(task.WorktreePath) != "" {
		return task, nil
	}

	baseRef := task.BaseRef
	if baseRef == "" {
		detectedBaseRef, err := m.detectBaseRef(ctx)
		if err != nil {
			return Task{}, m.markTaskWorktreeFailed(ctx, task, "", err)
		}
		baseRef = detectedBaseRef
	}

	worktreePath := task.WorktreePath
	if worktreePath == "" {
		worktreePath = m.worktreePath(task.TaskID)
	}

	if err := m.prepareWorktreePath(ctx, worktreePath); err != nil {
		return Task{}, m.markTaskWorktreeFailed(ctx, task, worktreePath, err)
	}

	branchExists, err := m.branchExists(ctx, task.BranchName)
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

	if _, err := m.runGit(ctx, args...); err != nil {
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

	var errs []error
	for _, task := range tasks[m.closedRetention:] {
		if strings.TrimSpace(task.WorktreePath) == "" {
			continue
		}

		if _, err := m.runGit(ctx, "worktree", "remove", "--force", task.WorktreePath); err != nil {
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

func (m *GitTaskWorkspaceManager) detectBaseRef(ctx context.Context) (string, error) {
	for _, ref := range []string{"main", "master"} {
		if _, err := m.runGit(ctx, "rev-parse", "--verify", "--quiet", ref+"^{commit}"); err == nil {
			return ref, nil
		}
	}

	return "", errors.New("detect task worktree base ref: expected local branch main or master")
}

func (m *GitTaskWorkspaceManager) branchExists(ctx context.Context, branchName string) (bool, error) {
	_, err := m.runGit(ctx, "show-ref", "--verify", "--quiet", "refs/heads/"+branchName)
	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}

	return false, fmt.Errorf("check branch existence: %w", err)
}

func (m *GitTaskWorkspaceManager) prepareWorktreePath(ctx context.Context, worktreePath string) error {
	if err := os.MkdirAll(filepath.Dir(worktreePath), worktreeDirectoryPerms); err != nil {
		return fmt.Errorf("create worktree parent directory: %w", err)
	}

	if _, statErr := os.Stat(worktreePath); statErr == nil {
		if _, err := m.runGit(ctx, "worktree", "remove", "--force", worktreePath); err != nil {
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

func (m *GitTaskWorkspaceManager) runGit(ctx context.Context, args ...string) (string, error) {
	commandArgs := append([]string{"-C", m.sourceRepository}, args...)

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
