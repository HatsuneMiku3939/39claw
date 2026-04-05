package app_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
)

func TestGitTaskWorkspaceManagerEnsureReadyCreatesWorktree(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree integration tests")
	}

	sourceRepo := createGitRepository(t, "main")
	dataDir := t.TempDir()
	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-1": {
				TaskID:         "task-1",
				DiscordUserID:  "user-1",
				TaskName:       "Release work",
				Status:         app.TaskStatusOpen,
				BranchName:     app.DefaultTaskBranchName("task-1"),
				WorktreeStatus: app.TaskWorktreeStatusPending,
				CreatedAt:      time.Date(2026, time.April, 5, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	manager, err := app.NewTaskWorkspaceManager(app.TaskWorkspaceManagerDependencies{
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

	if readyTask.BaseRef != "main" {
		t.Fatalf("BaseRef = %q, want %q", readyTask.BaseRef, "main")
	}

	if _, err := os.Stat(filepath.Join(wantPath, ".git")); err != nil {
		t.Fatalf("worktree .git stat error = %v", err)
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
				BranchName:     app.DefaultTaskBranchName("task-remote"),
				WorktreeStatus: app.TaskWorktreeStatusPending,
				CreatedAt:      time.Date(2026, time.April, 6, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	manager, err := app.NewTaskWorkspaceManager(app.TaskWorkspaceManagerDependencies{
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

func TestGitTaskWorkspaceManagerEnsureReadyFallsBackToLocalBranchWhenFetchFails(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree integration tests")
	}

	sourceRepo := createGitRepository(t, "master")
	runGit(t, sourceRepo, "remote", "add", "origin", filepath.Join(t.TempDir(), "missing-remote.git"))

	dataDir := t.TempDir()
	store := &memoryThreadStore{
		tasks: map[string]app.Task{
			"user-1:task-local-fallback": {
				TaskID:         "task-local-fallback",
				DiscordUserID:  "user-1",
				TaskName:       "Local fallback",
				Status:         app.TaskStatusOpen,
				BranchName:     app.DefaultTaskBranchName("task-local-fallback"),
				WorktreeStatus: app.TaskWorktreeStatusPending,
				CreatedAt:      time.Date(2026, time.April, 6, 0, 0, 0, 0, time.UTC),
			},
		},
	}

	manager, err := app.NewTaskWorkspaceManager(app.TaskWorkspaceManagerDependencies{
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

	task, ok, err := store.GetTask(context.Background(), "user-1", "task-local-fallback")
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

	if readyTask.BaseRef != "master" {
		t.Fatalf("BaseRef = %q, want %q", readyTask.BaseRef, "master")
	}
}

func TestGitTaskWorkspaceManagerPruneClosedRemovesOldReadyWorktrees(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree integration tests")
	}

	sourceRepo := createGitRepository(t, "main")
	dataDir := t.TempDir()
	store := &memoryThreadStore{}
	clock := time.Date(2026, time.April, 5, 15, 4, 0, 0, time.UTC)

	manager, err := app.NewTaskWorkspaceManager(app.TaskWorkspaceManagerDependencies{
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
			BranchName:     app.DefaultTaskBranchName(taskID),
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
}

func createRemoteBackedGitRepository(t *testing.T, branch string) string {
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

	return source
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

	cmd := exec.Command("git", args...)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v error = %v\n%s", args, err, output)
	}

	return strings.TrimSpace(string(output))
}

func runGit(t *testing.T, workdir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = workdir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v error = %v\n%s", args, err, output)
	}
}
