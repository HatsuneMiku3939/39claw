package app_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
)

func TestGitTaskWorkspaceManagerEnsureReadyCreatesManagedBareWorktree(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree integration tests")
	}

	sourceRepo := createRemoteBackedGitRepository(t, "main")
	dataDir := t.TempDir()
	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:         "task-1",
				DiscordUserID:  "user-1",
				TaskName:       "Release work",
				Status:         app.TaskStatusOpen,
				BranchName:     app.DefaultTaskBranchName("Release work", "task-1"),
				WorktreeStatus: app.TaskWorktreeStatusPending,
				CreatedAt:      time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	manager, err := app.NewTaskWorkspaceManager(context.Background(), app.TaskWorkspaceManagerDependencies{
		Store:            store,
		SourceRepository: sourceRepo,
		DataDir:          dataDir,
		GitExecutable:    "git",
		Clock: func() time.Time {
			return time.Date(2026, time.April, 5, 15, 4, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewTaskWorkspaceManager() error = %v", err)
	}

	task, ok, err := store.GetTask(context.Background(), "user-1", "task-1")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if !ok {
		t.Fatal("GetTask() ok = false, want true")
	}

	readyTask, err := manager.EnsureReady(context.Background(), task)
	if err != nil {
		t.Fatalf("EnsureReady() error = %v", err)
	}

	wantPath := filepath.Join(dataDir, "worktrees", "task-1")
	if readyTask.WorktreePath != wantPath {
		t.Fatalf("WorktreePath = %q, want %q", readyTask.WorktreePath, wantPath)
	}

	if readyTask.WorktreeStatus != app.TaskWorktreeStatusReady {
		t.Fatalf("WorktreeStatus = %q, want %q", readyTask.WorktreeStatus, app.TaskWorktreeStatusReady)
	}

	if readyTask.BaseRef != "origin/main" {
		t.Fatalf("BaseRef = %q, want %q", readyTask.BaseRef, "origin/main")
	}

	if _, err := os.Stat(filepath.Join(wantPath, ".git")); err != nil {
		t.Fatalf("worktree .git stat error = %v", err)
	}

	managedRepoPath := managedRepoPathFromDataDir(t, dataDir)
	originURL := gitOutput(t, sourceRepo, "remote", "get-url", "origin")
	if got := gitOutput(t, readyTask.WorktreePath, "remote", "get-url", "origin"); got != originURL {
		t.Fatalf("worktree origin URL = %q, want %q", got, originURL)
	}

	if got := gitOutput(t, managedRepoPath, "rev-parse", "--is-bare-repository"); got != "true" {
		t.Fatalf("managed repository bare flag = %q, want %q", got, "true")
	}

	if _, err := gitOutputWithError(sourceRepo, "show-ref", "--verify", "refs/heads/"+readyTask.BranchName); err == nil {
		t.Fatalf("source repository unexpectedly contains task branch %q", readyTask.BranchName)
	}

	runGit(t, readyTask.WorktreePath, "switch", "main")
	if got := gitOutput(t, readyTask.WorktreePath, "branch", "--show-current"); got != "main" {
		t.Fatalf("current branch after switch = %q, want %q", got, "main")
	}
}

func TestGitTaskWorkspaceManagerCurrentBranchReadsWorktreeState(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree integration tests")
	}

	sourceRepo := createRemoteBackedGitRepository(t, "main")
	dataDir := t.TempDir()
	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:         "task-1",
				DiscordUserID:  "user-1",
				TaskName:       "Release work",
				Status:         app.TaskStatusOpen,
				BranchName:     app.DefaultTaskBranchName("Release work", "task-1"),
				WorktreeStatus: app.TaskWorktreeStatusPending,
				CreatedAt:      time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	manager, err := app.NewTaskWorkspaceManager(context.Background(), app.TaskWorkspaceManagerDependencies{
		Store:            store,
		SourceRepository: sourceRepo,
		DataDir:          dataDir,
		GitExecutable:    "git",
		Clock: func() time.Time {
			return time.Date(2026, time.April, 5, 15, 4, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewTaskWorkspaceManager() error = %v", err)
	}

	task, ok, err := store.GetTask(context.Background(), "user-1", "task-1")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if !ok {
		t.Fatal("GetTask() ok = false, want true")
	}

	readyTask, err := manager.EnsureReady(context.Background(), task)
	if err != nil {
		t.Fatalf("EnsureReady() error = %v", err)
	}

	runGit(t, readyTask.WorktreePath, "switch", "main")

	branch, branchOK, err := manager.CurrentBranch(context.Background(), readyTask)
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}
	if !branchOK {
		t.Fatal("CurrentBranch() ok = false, want true")
	}
	if branch != "main" {
		t.Fatalf("CurrentBranch() = %q, want %q", branch, "main")
	}
}

func TestGitTaskWorkspaceManagerEnsureReadyPrefersRemoteDefaultBranch(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree integration tests")
	}

	sourceRepo := createRemoteBackedGitRepository(t, "master")
	writeFile(t, filepath.Join(sourceRepo, "local-only.txt"), "local only\n")
	runGit(t, sourceRepo, "add", "local-only.txt")
	runGit(t, sourceRepo, "commit", "-m", "local only commit")

	localHead := gitOutput(t, sourceRepo, "rev-parse", "HEAD")
	remoteHead := gitOutput(t, sourceRepo, "rev-parse", "origin/master")
	if localHead == remoteHead {
		t.Fatal("local HEAD unexpectedly matches origin/master")
	}

	dataDir := t.TempDir()
	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-remote": {
				TaskID:         "task-remote",
				DiscordUserID:  "user-1",
				TaskName:       "Remote base",
				Status:         app.TaskStatusOpen,
				BranchName:     app.DefaultTaskBranchName("Remote base", "task-remote"),
				WorktreeStatus: app.TaskWorktreeStatusPending,
				CreatedAt:      time.Date(2026, time.April, 6, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	manager, err := app.NewTaskWorkspaceManager(context.Background(), app.TaskWorkspaceManagerDependencies{
		Store:            store,
		SourceRepository: sourceRepo,
		DataDir:          dataDir,
		GitExecutable:    "git",
		Clock: func() time.Time {
			return time.Date(2026, time.April, 6, 1, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewTaskWorkspaceManager() error = %v", err)
	}

	task, ok, err := store.GetTask(context.Background(), "user-1", "task-remote")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if !ok {
		t.Fatal("GetTask() ok = false, want true")
	}

	readyTask, err := manager.EnsureReady(context.Background(), task)
	if err != nil {
		t.Fatalf("EnsureReady() error = %v", err)
	}

	if readyTask.BaseRef != "origin/master" {
		t.Fatalf("BaseRef = %q, want %q", readyTask.BaseRef, "origin/master")
	}

	worktreeHead := gitOutput(t, readyTask.WorktreePath, "rev-parse", "HEAD")
	if worktreeHead != remoteHead {
		t.Fatalf("worktree HEAD = %q, want %q", worktreeHead, remoteHead)
	}
	if worktreeHead == localHead {
		t.Fatal("worktree HEAD unexpectedly matched local-only source HEAD")
	}
}

func TestGitTaskWorkspaceManagerEnsureReadyUsesCachedRemoteRefsWhenFetchFails(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree integration tests")
	}

	sourceRepo, remoteRepo := createRemoteBackedGitRepositoryWithRemote(t, "master")

	dataDir := t.TempDir()
	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-cached-remote": {
				TaskID:         "task-cached-remote",
				DiscordUserID:  "user-1",
				TaskName:       "Cached remote",
				Status:         app.TaskStatusOpen,
				BranchName:     app.DefaultTaskBranchName("Cached remote", "task-cached-remote"),
				WorktreeStatus: app.TaskWorktreeStatusPending,
				CreatedAt:      time.Date(2026, time.April, 6, 0, 0, 0, 0, time.UTC),
			},
			"user-1:task-fetch-failure": {
				TaskID:         "task-fetch-failure",
				DiscordUserID:  "user-1",
				TaskName:       "Fetch failure",
				Status:         app.TaskStatusOpen,
				BranchName:     app.DefaultTaskBranchName("Fetch failure", "task-fetch-failure"),
				WorktreeStatus: app.TaskWorktreeStatusPending,
				CreatedAt:      time.Date(2026, time.April, 6, 1, 0, 0, 0, time.UTC),
			},
		},
	}

	manager, err := app.NewTaskWorkspaceManager(context.Background(), app.TaskWorkspaceManagerDependencies{
		Store:            store,
		SourceRepository: sourceRepo,
		DataDir:          dataDir,
		GitExecutable:    "git",
		Clock: func() time.Time {
			return time.Date(2026, time.April, 6, 1, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewTaskWorkspaceManager() error = %v", err)
	}

	task, ok, err := store.GetTask(context.Background(), "user-1", "task-cached-remote")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if !ok {
		t.Fatal("GetTask() ok = false, want true")
	}

	readyTask, err := manager.EnsureReady(context.Background(), task)
	if err != nil {
		t.Fatalf("EnsureReady() error = %v", err)
	}

	if readyTask.BaseRef != "origin/master" {
		t.Fatalf("BaseRef = %q, want %q", readyTask.BaseRef, "origin/master")
	}

	runGit(t, sourceRepo, "remote", "set-url", "origin", filepath.Join(filepath.Dir(remoteRepo), "missing-remote.git"))

	secondTask, ok, err := store.GetTask(context.Background(), "user-1", "task-fetch-failure")
	if err != nil {
		t.Fatalf("GetTask(second) error = %v", err)
	}
	if !ok {
		t.Fatal("GetTask(second) ok = false, want true")
	}

	secondReadyTask, err := manager.EnsureReady(context.Background(), secondTask)
	if err != nil {
		t.Fatalf("EnsureReady(second) error = %v", err)
	}

	if secondReadyTask.BaseRef != "origin/master" {
		t.Fatalf("second BaseRef = %q, want %q", secondReadyTask.BaseRef, "origin/master")
	}
}

func TestGitTaskWorkspaceManagerEnsureReadySerializesManagedRepositoryMutation(t *testing.T) {
	t.Parallel()

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is required for worktree integration tests")
	}

	sourceRepo := createRemoteBackedGitRepository(t, "main")
	dataDir := t.TempDir()
	traceDir := t.TempDir()

	wrapperPath := filepath.Join(t.TempDir(), "git-wrapper.sh")
	wrapperScript := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail

real_git=%q
trace_dir=%q
managed_root=%q
original_args=("$@")
parsed_args=("$@")
instrument=0

if [[ ${#parsed_args[@]} -ge 3 && "${parsed_args[0]}" == "-C" ]]; then
	repository_path="${parsed_args[1]}"
	parsed_args=("${parsed_args[@]:2}")

	if [[ "${repository_path}" == "${managed_root}"/* ]]; then
		case "${parsed_args[0]:-}" in
			config|fetch|worktree)
				instrument=1
				;;
			remote)
				case "${parsed_args[1]:-}" in
					add|set-url|set-head)
						instrument=1
						;;
				esac
				;;
		esac
	fi
elif [[ ${#parsed_args[@]} -ge 3 && "${parsed_args[0]}" == "init" && "${parsed_args[1]}" == "--bare" ]]; then
	if [[ "${parsed_args[2]}" == "${managed_root}"/* ]]; then
		instrument=1
	fi
fi

acquire_state_lock() {
	while ! mkdir "${trace_dir}/state.lock" 2>/dev/null; do
		sleep 0.01
	done
}

release_state_lock() {
	rmdir "${trace_dir}/state.lock"
}

enter_critical() {
	acquire_state_lock

	local current=0
	if [[ -f "${trace_dir}/current" ]]; then
		current=$(cat "${trace_dir}/current")
	fi
	current=$((current + 1))
	printf '%%s\n' "${current}" > "${trace_dir}/current"

	local max_seen=0
	if [[ -f "${trace_dir}/max" ]]; then
		max_seen=$(cat "${trace_dir}/max")
	fi
	if (( current > max_seen )); then
		printf '%%s\n' "${current}" > "${trace_dir}/max"
	fi

	release_state_lock
}

leave_critical() {
	acquire_state_lock

	local current=0
	if [[ -f "${trace_dir}/current" ]]; then
		current=$(cat "${trace_dir}/current")
	fi
	if (( current > 0 )); then
		current=$((current - 1))
	fi
	printf '%%s\n' "${current}" > "${trace_dir}/current"

	release_state_lock
}

if (( instrument == 1 )); then
	enter_critical
	sleep 0.2
fi

set +e
"${real_git}" "${original_args[@]}"
status=$?
set -e

if (( instrument == 1 )); then
	leave_critical
fi

exit "${status}"
`, realGit, traceDir, filepath.Join(dataDir, "repos"))
	if err := os.WriteFile(wrapperPath, []byte(wrapperScript), 0o755); err != nil {
		t.Fatalf("WriteFile(git wrapper) error = %v", err)
	}

	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:         "task-1",
				DiscordUserID:  "user-1",
				TaskName:       "Task 1",
				Status:         app.TaskStatusOpen,
				BranchName:     app.DefaultTaskBranchName("Task 1", "task-1"),
				WorktreeStatus: app.TaskWorktreeStatusPending,
				CreatedAt:      time.Date(2026, time.April, 7, 0, 0, 0, 0, time.UTC),
			},
			"user-1:task-2": {
				TaskID:         "task-2",
				DiscordUserID:  "user-1",
				TaskName:       "Task 2",
				Status:         app.TaskStatusOpen,
				BranchName:     app.DefaultTaskBranchName("Task 2", "task-2"),
				WorktreeStatus: app.TaskWorktreeStatusPending,
				CreatedAt:      time.Date(2026, time.April, 7, 0, 1, 0, 0, time.UTC),
			},
		},
	}

	manager, err := app.NewTaskWorkspaceManager(context.Background(), app.TaskWorkspaceManagerDependencies{
		Store:            store,
		SourceRepository: sourceRepo,
		DataDir:          dataDir,
		GitExecutable:    wrapperPath,
		Clock: func() time.Time {
			return time.Date(2026, time.April, 7, 1, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewTaskWorkspaceManager() error = %v", err)
	}

	type ensureResult struct {
		task app.Task
		err  error
	}

	start := make(chan struct{})
	results := make(chan ensureResult, 2)
	var wg sync.WaitGroup

	for _, taskID := range []string{"task-1", "task-2"} {
		taskID := taskID
		wg.Add(1)
		go func() {
			defer wg.Done()

			task, ok, err := store.GetTask(context.Background(), "user-1", taskID)
			if err != nil {
				results <- ensureResult{err: err}
				return
			}
			if !ok {
				results <- ensureResult{err: context.Canceled}
				return
			}

			<-start
			readyTask, ensureErr := manager.EnsureReady(context.Background(), task)
			results <- ensureResult{task: readyTask, err: ensureErr}
		}()
	}

	close(start)
	wg.Wait()
	close(results)

	for result := range results {
		if result.err != nil {
			t.Fatalf("EnsureReady() error = %v", result.err)
		}
		if result.task.WorktreeStatus != app.TaskWorktreeStatusReady {
			t.Fatalf("WorktreeStatus = %q, want %q", result.task.WorktreeStatus, app.TaskWorktreeStatusReady)
		}
	}

	maxActiveBytes, err := os.ReadFile(filepath.Join(traceDir, "max"))
	if err != nil {
		t.Fatalf("ReadFile(max) error = %v", err)
	}

	maxActive, err := strconv.Atoi(strings.TrimSpace(string(maxActiveBytes)))
	if err != nil {
		t.Fatalf("Atoi(max) error = %v", err)
	}

	if maxActive != 1 {
		t.Fatalf("max concurrent managed repository mutations = %d, want %d", maxActive, 1)
	}
}

func TestGitTaskWorkspaceManagerPruneClosedRemovesOldReadyWorktrees(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree integration tests")
	}

	sourceRepo := createRemoteBackedGitRepository(t, "main")
	dataDir := t.TempDir()
	store := &memoryThreadStore{}
	clock := time.Date(2026, time.April, 5, 15, 4, 0, 0, time.UTC)

	manager, err := app.NewTaskWorkspaceManager(context.Background(), app.TaskWorkspaceManagerDependencies{
		Store:            store,
		SourceRepository: sourceRepo,
		DataDir:          dataDir,
		GitExecutable:    "git",
		ClosedRetention:  2,
		Clock: func() time.Time {
			return clock
		},
	})
	if err != nil {
		t.Fatalf("NewTaskWorkspaceManager() error = %v", err)
	}

	for index := 1; index <= 3; index++ {
		taskID := "task-" + string(rune('0'+index))
		task := app.Task{
			TaskID:         taskID,
			DiscordUserID:  "user-1",
			TaskName:       taskID,
			Status:         app.TaskStatusOpen,
			BranchName:     app.DefaultTaskBranchName(taskID, taskID),
			WorktreeStatus: app.TaskWorktreeStatusPending,
			CreatedAt:      clock.Add(-time.Duration(index) * time.Hour),
		}
		if err := store.CreateTask(context.Background(), task); err != nil {
			t.Fatalf("CreateTask(%s) error = %v", taskID, err)
		}

		readyTask, err := manager.EnsureReady(context.Background(), task)
		if err != nil {
			t.Fatalf("EnsureReady(%s) error = %v", taskID, err)
		}

		closedAt := clock.Add(-time.Duration(index) * time.Hour)
		readyTask.Status = app.TaskStatusClosed
		readyTask.ClosedAt = &closedAt
		if err := store.UpdateTask(context.Background(), readyTask); err != nil {
			t.Fatalf("UpdateTask(%s) error = %v", taskID, err)
		}
	}

	if err := manager.PruneClosed(context.Background()); err != nil {
		t.Fatalf("PruneClosed() error = %v", err)
	}

	oldestTask, ok, err := store.GetTask(context.Background(), "user-1", "task-3")
	if err != nil {
		t.Fatalf("GetTask(task-3) error = %v", err)
	}
	if !ok {
		t.Fatal("GetTask(task-3) ok = false, want true")
	}

	if oldestTask.WorktreeStatus != app.TaskWorktreeStatusPruned {
		t.Fatalf("oldest task WorktreeStatus = %q, want %q", oldestTask.WorktreeStatus, app.TaskWorktreeStatusPruned)
	}

	if oldestTask.WorktreePrunedAt == nil {
		t.Fatal("oldest task WorktreePrunedAt = nil, want non-nil")
	}

	if _, err := os.Stat(filepath.Join(dataDir, "worktrees", "task-3")); !os.IsNotExist(err) {
		t.Fatalf("oldest worktree path stat error = %v, want not-exist", err)
	}

	newestTask, ok, err := store.GetTask(context.Background(), "user-1", "task-1")
	if err != nil {
		t.Fatalf("GetTask(task-1) error = %v", err)
	}
	if !ok {
		t.Fatal("GetTask(task-1) ok = false, want true")
	}

	if newestTask.WorktreeStatus != app.TaskWorktreeStatusReady {
		t.Fatalf("newest task WorktreeStatus = %q, want %q", newestTask.WorktreeStatus, app.TaskWorktreeStatusReady)
	}

	managedRepoPath := managedRepoPathFromDataDir(t, dataDir)
	if got := gitOutput(t, managedRepoPath, "show-ref", "--verify", "refs/heads/task/task-3"); got == "" {
		t.Fatal("managed repository is missing retained branch refs/heads/task/task-3")
	}

	if _, err := gitOutputWithError(sourceRepo, "show-ref", "--verify", "refs/heads/task/task-3"); err == nil {
		t.Fatal("source repository unexpectedly contains pruned task branch")
	}
}

func TestGitTaskWorkspaceManagerRejectsSourceRepositoryWithoutOriginRemote(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree integration tests")
	}

	sourceRepo := createGitRepository(t, "main")

	_, err := app.NewTaskWorkspaceManager(context.Background(), app.TaskWorkspaceManagerDependencies{
		Store:            &memoryThreadStore{},
		SourceRepository: sourceRepo,
		DataDir:          t.TempDir(),
		GitExecutable:    "git",
	})
	if err == nil {
		t.Fatal("NewTaskWorkspaceManager() error = nil, want missing origin remote")
	}

	if !strings.Contains(err.Error(), "task mode requires CLAW_CODEX_WORKDIR to have an origin remote") {
		t.Fatalf("NewTaskWorkspaceManager() error = %q, want missing origin remote guidance", err.Error())
	}
}

func createRemoteBackedGitRepository(t *testing.T, branch string) string {
	t.Helper()

	source, _ := createRemoteBackedGitRepositoryWithRemote(t, branch)
	return source
}

func createRemoteBackedGitRepositoryWithRemote(t *testing.T, branch string) (string, string) {
	t.Helper()

	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	source := filepath.Join(root, "source")

	runGit(t, root, "init", "--bare", "-b", branch, remote)
	runGit(t, root, "clone", remote, source)
	runGit(t, source, "config", "user.email", "codex@example.com")
	runGit(t, source, "config", "user.name", "Codex")

	writeFile(t, filepath.Join(source, "README.md"), "hello\n")
	runGit(t, source, "add", "README.md")
	runGit(t, source, "commit", "-m", "initial commit")
	runGit(t, source, "push", "-u", "origin", branch)

	return source, remote
}

func createGitRepository(t *testing.T, branch string) string {
	t.Helper()

	repo := t.TempDir()

	runGit(t, repo, "init", "-b", branch)
	runGit(t, repo, "config", "user.email", "codex@example.com")
	runGit(t, repo, "config", "user.name", "Codex")

	filePath := filepath.Join(repo, "README.md")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.md) error = %v", err)
	}

	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial commit")

	return repo
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func gitOutput(t *testing.T, workdir string, args ...string) string {
	t.Helper()

	output, err := gitOutputWithError(workdir, args...)
	if err != nil {
		t.Fatalf("git %v error = %v\n%s", args, err, output)
	}

	return output
}

func gitOutputWithError(workdir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(output)), err
}

func runGit(t *testing.T, workdir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = workdir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v error = %v\n%s", args, err, output)
	}
}

func managedRepoPathFromDataDir(t *testing.T, dataDir string) string {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(dataDir, "repos"))
	if err != nil {
		t.Fatalf("ReadDir(repos) error = %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("managed repository count = %d, want %d", len(entries), 1)
	}

	return filepath.Join(dataDir, "repos", entries[0].Name())
}
