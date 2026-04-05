package app_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
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

func runGit(t *testing.T, workdir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = workdir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v error = %v\n%s", args, err, output)
	}
}
