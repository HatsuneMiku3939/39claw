package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
)

func TestStoreInitSchemaIsIdempotent(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	for i := 0; i < 2; i++ {
		if err := store.InitSchema(context.Background()); err != nil {
			t.Fatalf("InitSchema() error = %v", err)
		}
	}
}

func TestStoreThreadBindingLifecycle(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	if err := store.UpsertThreadBinding(context.Background(), app.ThreadBinding{
		Mode:             "daily",
		LogicalThreadKey: "2026-04-05",
		CodexThreadID:    "thread-1",
	}); err != nil {
		t.Fatalf("UpsertThreadBinding() error = %v", err)
	}

	binding, ok, err := store.GetThreadBinding(context.Background(), "daily", "2026-04-05")
	if err != nil {
		t.Fatalf("GetThreadBinding() error = %v", err)
	}

	if !ok {
		t.Fatal("GetThreadBinding() ok = false, want true")
	}

	if binding.CodexThreadID != "thread-1" {
		t.Fatalf("CodexThreadID = %q, want %q", binding.CodexThreadID, "thread-1")
	}

	if binding.CreatedAt.IsZero() {
		t.Fatal("CreatedAt = zero, want populated timestamp")
	}

	if err := store.UpsertThreadBinding(context.Background(), app.ThreadBinding{
		Mode:             "daily",
		LogicalThreadKey: "2026-04-05",
		CodexThreadID:    "thread-2",
	}); err != nil {
		t.Fatalf("UpsertThreadBinding() second error = %v", err)
	}

	binding, ok, err = store.GetThreadBinding(context.Background(), "daily", "2026-04-05")
	if err != nil {
		t.Fatalf("GetThreadBinding() second error = %v", err)
	}

	if !ok {
		t.Fatal("GetThreadBinding() second ok = false, want true")
	}

	if binding.CodexThreadID != "thread-2" {
		t.Fatalf("CodexThreadID = %q, want %q", binding.CodexThreadID, "thread-2")
	}
}

func TestStoreThreadBindingPersistsAcrossReopen(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "39claw.db")

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	store.clock = func() time.Time {
		return time.Date(2026, time.April, 5, 15, 4, 0, 0, time.UTC)
	}

	if err := store.InitSchema(context.Background()); err != nil {
		t.Fatalf("InitSchema() error = %v", err)
	}

	if err := store.UpsertThreadBinding(context.Background(), app.ThreadBinding{
		Mode:             "daily",
		LogicalThreadKey: "2026-04-05",
		CodexThreadID:    "thread-1",
	}); err != nil {
		t.Fatalf("UpsertThreadBinding() error = %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatalf("Open() reopen error = %v", err)
	}
	defer func() {
		if closeErr := reopened.Close(); closeErr != nil {
			t.Fatalf("Close() reopen error = %v", closeErr)
		}
	}()

	if err := reopened.InitSchema(context.Background()); err != nil {
		t.Fatalf("InitSchema() reopen error = %v", err)
	}

	binding, ok, err := reopened.GetThreadBinding(context.Background(), "daily", "2026-04-05")
	if err != nil {
		t.Fatalf("GetThreadBinding() reopen error = %v", err)
	}

	if !ok {
		t.Fatal("GetThreadBinding() reopen ok = false, want true")
	}

	if binding.CodexThreadID != "thread-1" {
		t.Fatalf("CodexThreadID after reopen = %q, want %q", binding.CodexThreadID, "thread-1")
	}
}

func TestStoreTaskLifecycle(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	ctx := context.Background()
	if err := store.CreateTask(ctx, app.Task{
		TaskID:        "task-1",
		DiscordUserID: "user-1",
		TaskName:      "Foundation work",
	}); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	if err := store.SetActiveTask(ctx, app.ActiveTask{
		DiscordUserID: "user-1",
		TaskID:        "task-1",
	}); err != nil {
		t.Fatalf("SetActiveTask() error = %v", err)
	}

	task, ok, err := store.GetTask(ctx, "user-1", "task-1")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}

	if !ok {
		t.Fatal("GetTask() ok = false, want true")
	}

	if task.Status != app.TaskStatusOpen {
		t.Fatalf("Status = %q, want %q", task.Status, app.TaskStatusOpen)
	}

	tasks, err := store.ListOpenTasks(ctx, "user-1")
	if err != nil {
		t.Fatalf("ListOpenTasks() error = %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("ListOpenTasks() len = %d, want %d", len(tasks), 1)
	}

	activeTask, ok, err := store.GetActiveTask(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetActiveTask() error = %v", err)
	}

	if !ok {
		t.Fatal("GetActiveTask() ok = false, want true")
	}

	if activeTask.TaskID != "task-1" {
		t.Fatalf("TaskID = %q, want %q", activeTask.TaskID, "task-1")
	}

	if err := store.CloseTask(ctx, "user-1", "task-1"); err != nil {
		t.Fatalf("CloseTask() error = %v", err)
	}

	task, ok, err = store.GetTask(ctx, "user-1", "task-1")
	if err != nil {
		t.Fatalf("GetTask() after close error = %v", err)
	}

	if !ok {
		t.Fatal("GetTask() after close ok = false, want true")
	}

	if task.Status != app.TaskStatusClosed {
		t.Fatalf("Status = %q, want %q", task.Status, app.TaskStatusClosed)
	}

	if task.ClosedAt == nil {
		t.Fatal("ClosedAt = nil, want non-nil")
	}

	_, ok, err = store.GetActiveTask(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetActiveTask() after close error = %v", err)
	}

	if ok {
		t.Fatal("GetActiveTask() after close ok = true, want false")
	}
}

func TestStoreCloseTaskRejectsUnknownTask(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)

	err := store.CloseTask(context.Background(), "user-1", "missing")
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("CloseTask() error = %v, want %v", err, sql.ErrNoRows)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	store, err := Open(filepath.Join(t.TempDir(), "39claw.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	store.clock = func() time.Time {
		return time.Date(2026, time.April, 5, 15, 4, 0, 0, time.UTC)
	}

	if err := store.InitSchema(context.Background()); err != nil {
		t.Fatalf("InitSchema() error = %v", err)
	}

	t.Cleanup(func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	})

	return store
}
