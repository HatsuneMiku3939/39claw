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

func TestGitTaskWorkspaceManagerPrepareTemporaryWorktreeCreatesAndCleansUp(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required for worktree integration tests")
	}

	sourceRepo := createRemoteBackedGitRepository(t, "main")
	dataDir := t.TempDir()
	store := &memoryThreadStore{}

	manager, err := app.NewTaskWorkspaceManager(context.Background(), app.TaskWorkspaceManagerDependencies{
		Store:            store,
		SourceRepository: sourceRepo,
		DataDir:          dataDir,
		GitExecutable:    "git",
		Clock: func() time.Time {
			return time.Date(2026, time.April, 12, 9, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("NewTaskWorkspaceManager() error = %v", err)
	}

	worktreePath, cleanup, err := manager.PrepareTemporaryWorktree(context.Background(), "run-1")
	if err != nil {
		t.Fatalf("PrepareTemporaryWorktree() error = %v", err)
	}

	wantPath := filepath.Join(dataDir, "scheduled-worktrees", "run-1")
	if worktreePath != wantPath {
		t.Fatalf("worktreePath = %q, want %q", worktreePath, wantPath)
	}

	if got := gitOutput(t, worktreePath, "branch", "--show-current"); got != "" {
		t.Fatalf("scheduled worktree branch = %q, want detached HEAD", got)
	}

	if err := cleanup(context.Background()); err != nil {
		t.Fatalf("cleanup() error = %v", err)
	}

	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Fatalf("scheduled worktree path still exists after cleanup: %v", err)
	}
}
