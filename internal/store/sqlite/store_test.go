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

func TestStoreTaskStatePersistsAcrossReopen(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "39claw.db")

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	store.clock = func() time.Time {
		return time.Date(2026, time.April, 5, 15, 4, 0, 0, time.UTC)
	}

	ctx := context.Background()
	if err := store.InitSchema(ctx); err != nil {
		t.Fatalf("InitSchema() error = %v", err)
	}

	if err := store.CreateTask(ctx, app.Task{
		TaskID:        "task-1",
		DiscordUserID: "user-1",
		TaskName:      "Release work",
	}); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	if err := store.SetActiveTask(ctx, app.ActiveTask{
		DiscordUserID: "user-1",
		TaskID:        "task-1",
	}); err != nil {
		t.Fatalf("SetActiveTask() error = %v", err)
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

	if err := reopened.InitSchema(ctx); err != nil {
		t.Fatalf("InitSchema() reopen error = %v", err)
	}

	task, ok, err := reopened.GetTask(ctx, "user-1", "task-1")
	if err != nil {
		t.Fatalf("GetTask() reopen error = %v", err)
	}

	if !ok {
		t.Fatal("GetTask() reopen ok = false, want true")
	}

	if task.TaskName != "Release work" {
		t.Fatalf("TaskName = %q, want %q", task.TaskName, "Release work")
	}

	if task.BranchName != "task/task-1" {
		t.Fatalf("BranchName = %q, want %q", task.BranchName, "task/task-1")
	}

	if task.WorktreeStatus != app.TaskWorktreeStatusPending {
		t.Fatalf("WorktreeStatus = %q, want %q", task.WorktreeStatus, app.TaskWorktreeStatusPending)
	}

	activeTask, ok, err := reopened.GetActiveTask(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetActiveTask() reopen error = %v", err)
	}

	if !ok {
		t.Fatal("GetActiveTask() reopen ok = false, want true")
	}

	if activeTask.TaskID != "task-1" {
		t.Fatalf("TaskID = %q, want %q", activeTask.TaskID, "task-1")
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

	if task.BranchName != "task/task-1" {
		t.Fatalf("BranchName = %q, want %q", task.BranchName, "task/task-1")
	}

	if task.WorktreeStatus != app.TaskWorktreeStatusPending {
		t.Fatalf("WorktreeStatus = %q, want %q", task.WorktreeStatus, app.TaskWorktreeStatusPending)
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

func TestStoreCloseTaskKeepsDifferentActiveTask(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()

	for _, task := range []app.Task{
		{
			TaskID:        "task-1",
			DiscordUserID: "user-1",
			TaskName:      "Release work",
		},
		{
			TaskID:        "task-2",
			DiscordUserID: "user-1",
			TaskName:      "Docs update",
		},
	} {
		if err := store.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask(%s) error = %v", task.TaskID, err)
		}
	}

	if err := store.SetActiveTask(ctx, app.ActiveTask{
		DiscordUserID: "user-1",
		TaskID:        "task-2",
	}); err != nil {
		t.Fatalf("SetActiveTask() error = %v", err)
	}

	if err := store.CloseTask(ctx, "user-1", "task-1"); err != nil {
		t.Fatalf("CloseTask() error = %v", err)
	}

	activeTask, ok, err := store.GetActiveTask(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetActiveTask() error = %v", err)
	}

	if !ok {
		t.Fatal("GetActiveTask() ok = false, want true")
	}

	if activeTask.TaskID != "task-2" {
		t.Fatalf("TaskID = %q, want %q", activeTask.TaskID, "task-2")
	}
}

func TestStoreInitSchemaMigratesExistingTaskTable(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "39claw.db")

	db, err := sql.Open(driverName, path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}

	if _, err := db.ExecContext(context.Background(), `CREATE TABLE tasks (
		task_id TEXT PRIMARY KEY,
		discord_user_id TEXT NOT NULL,
		task_name TEXT NOT NULL,
		status TEXT NOT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		closed_at TEXT NULL
	);`); err != nil {
		t.Fatalf("create legacy tasks table error = %v", err)
	}

	createdAt := time.Date(2026, time.April, 5, 15, 4, 0, 0, time.UTC).Format(time.RFC3339Nano)
	if _, err := db.ExecContext(
		context.Background(),
		`INSERT INTO tasks (task_id, discord_user_id, task_name, status, created_at, updated_at, closed_at)
		VALUES (?, ?, ?, ?, ?, ?, NULL)`,
		"task-1",
		"user-1",
		"Legacy task",
		string(app.TaskStatusOpen),
		createdAt,
		createdAt,
	); err != nil {
		t.Fatalf("insert legacy task error = %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}

	store, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	}()

	if err := store.InitSchema(context.Background()); err != nil {
		t.Fatalf("InitSchema() error = %v", err)
	}

	task, ok, err := store.GetTask(context.Background(), "user-1", "task-1")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}

	if !ok {
		t.Fatal("GetTask() ok = false, want true")
	}

	if task.BranchName != "task/task-1" {
		t.Fatalf("BranchName = %q, want %q", task.BranchName, "task/task-1")
	}

	if task.WorktreeStatus != app.TaskWorktreeStatusPending {
		t.Fatalf("WorktreeStatus = %q, want %q", task.WorktreeStatus, app.TaskWorktreeStatusPending)
	}
}

func TestStoreUpdateTaskAndListClosedReadyTasks(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()

	tasks := []app.Task{
		{
			TaskID:         "task-1",
			DiscordUserID:  "user-1",
			TaskName:       "Older closed task",
			Status:         app.TaskStatusClosed,
			BranchName:     "task/task-1",
			WorktreePath:   "/tmp/worktrees/task-1",
			WorktreeStatus: app.TaskWorktreeStatusReady,
			ClosedAt:       timePtr(time.Date(2026, time.April, 5, 10, 0, 0, 0, time.UTC)),
		},
		{
			TaskID:         "task-2",
			DiscordUserID:  "user-1",
			TaskName:       "Newest closed task",
			Status:         app.TaskStatusClosed,
			BranchName:     "task/task-2",
			WorktreePath:   "/tmp/worktrees/task-2",
			WorktreeStatus: app.TaskWorktreeStatusReady,
			ClosedAt:       timePtr(time.Date(2026, time.April, 5, 12, 0, 0, 0, time.UTC)),
		},
	}

	for _, task := range tasks {
		if err := store.CreateTask(ctx, task); err != nil {
			t.Fatalf("CreateTask(%s) error = %v", task.TaskID, err)
		}
		if err := store.UpdateTask(ctx, task); err != nil {
			t.Fatalf("UpdateTask(%s) error = %v", task.TaskID, err)
		}
	}

	closedReady, err := store.ListClosedReadyTasks(ctx)
	if err != nil {
		t.Fatalf("ListClosedReadyTasks() error = %v", err)
	}

	if len(closedReady) != 2 {
		t.Fatalf("ListClosedReadyTasks() len = %d, want %d", len(closedReady), 2)
	}

	if closedReady[0].TaskID != "task-2" {
		t.Fatalf("first closed ready task = %q, want %q", closedReady[0].TaskID, "task-2")
	}

	if closedReady[1].TaskID != "task-1" {
		t.Fatalf("second closed ready task = %q, want %q", closedReady[1].TaskID, "task-1")
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

func timePtr(value time.Time) *time.Time {
	return &value
}
