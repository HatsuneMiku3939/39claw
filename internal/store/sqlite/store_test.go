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

func TestMigrateIsIdempotent(t *testing.T) {
	t.Parallel()

	db, err := OpenDB(context.Background(), filepath.Join(t.TempDir(), "39claw.db"))
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Fatalf("db.Close() error = %v", closeErr)
		}
	}()

	for i := 0; i < 2; i++ {
		if err := Migrate(context.Background(), db); err != nil {
			t.Fatalf("Migrate() error = %v", err)
		}
	}

	var count int
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("query schema_migrations count error = %v", err)
	}

	if count != 6 {
		t.Fatalf("schema_migrations count = %d, want %d", count, 6)
	}
}

func TestStoreCreateDailySessionCreatesGenerationOne(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()

	session, err := store.CreateDailySession(ctx, app.DailySession{
		LocalDate:        "2026-04-05",
		Generation:       1,
		LogicalThreadKey: "2026-04-05#1",
		ActivationReason: app.DailySessionActivationAutomatic,
		IsActive:         true,
	})
	if err != nil {
		t.Fatalf("CreateDailySession() error = %v", err)
	}

	if session.LogicalThreadKey != "2026-04-05#1" {
		t.Fatalf("LogicalThreadKey = %q, want %q", session.LogicalThreadKey, "2026-04-05#1")
	}

	active, ok, err := store.GetActiveDailySession(ctx, "2026-04-05")
	if err != nil {
		t.Fatalf("GetActiveDailySession() error = %v", err)
	}
	if !ok {
		t.Fatal("GetActiveDailySession() ok = false, want true")
	}
	if active.Generation != 1 {
		t.Fatalf("Generation = %d, want %d", active.Generation, 1)
	}
}

func TestStoreRotateDailySessionCreatesNextGeneration(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()

	if _, err := store.CreateDailySession(ctx, app.DailySession{
		LocalDate:        "2026-04-05",
		Generation:       1,
		LogicalThreadKey: "2026-04-05#1",
		ActivationReason: app.DailySessionActivationAutomatic,
		IsActive:         true,
	}); err != nil {
		t.Fatalf("CreateDailySession() error = %v", err)
	}

	next, err := store.RotateDailySession(ctx, "2026-04-05", app.DailySessionActivationClear)
	if err != nil {
		t.Fatalf("RotateDailySession() error = %v", err)
	}

	if next.Generation != 2 {
		t.Fatalf("Generation = %d, want %d", next.Generation, 2)
	}
	if next.PreviousLogicalThreadKey != "2026-04-05#1" {
		t.Fatalf("PreviousLogicalThreadKey = %q, want %q", next.PreviousLogicalThreadKey, "2026-04-05#1")
	}

	active, ok, err := store.GetActiveDailySession(ctx, "2026-04-05")
	if err != nil {
		t.Fatalf("GetActiveDailySession() error = %v", err)
	}
	if !ok {
		t.Fatal("GetActiveDailySession() ok = false, want true")
	}
	if active.LogicalThreadKey != "2026-04-05#2" {
		t.Fatalf("active logical key = %q, want %q", active.LogicalThreadKey, "2026-04-05#2")
	}
}

func TestStoreGetLatestDailySessionBefore(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()

	for _, session := range []app.DailySession{
		{
			LocalDate:        "2026-04-05",
			Generation:       1,
			LogicalThreadKey: "2026-04-05#1",
			ActivationReason: app.DailySessionActivationAutomatic,
			IsActive:         true,
		},
		{
			LocalDate:        "2026-04-06",
			Generation:       1,
			LogicalThreadKey: "2026-04-06#1",
			ActivationReason: app.DailySessionActivationAutomatic,
			IsActive:         true,
		},
	} {
		if _, err := store.CreateDailySession(ctx, session); err != nil {
			t.Fatalf("CreateDailySession(%s) error = %v", session.LogicalThreadKey, err)
		}
	}

	latest, ok, err := store.GetLatestDailySessionBefore(ctx, "2026-04-07")
	if err != nil {
		t.Fatalf("GetLatestDailySessionBefore() error = %v", err)
	}
	if !ok {
		t.Fatal("GetLatestDailySessionBefore() ok = false, want true")
	}
	if latest.LogicalThreadKey != "2026-04-06#1" {
		t.Fatalf("LogicalThreadKey = %q, want %q", latest.LogicalThreadKey, "2026-04-06#1")
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

func TestStoreDeleteThreadBinding(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UpsertThreadBinding(ctx, app.ThreadBinding{
		Mode:             "task",
		LogicalThreadKey: "user-1:task-1",
		CodexThreadID:    "thread-1",
		TaskID:           "task-1",
	}); err != nil {
		t.Fatalf("UpsertThreadBinding() error = %v", err)
	}

	if err := store.DeleteThreadBinding(ctx, "task", "user-1:task-1"); err != nil {
		t.Fatalf("DeleteThreadBinding() error = %v", err)
	}

	if _, ok, err := store.GetThreadBinding(ctx, "task", "user-1:task-1"); err != nil {
		t.Fatalf("GetThreadBinding() error = %v", err)
	} else if ok {
		t.Fatal("GetThreadBinding() ok = true, want false")
	}

	if err := store.DeleteThreadBinding(ctx, "task", "user-1:task-1"); err != nil {
		t.Fatalf("DeleteThreadBinding() second error = %v", err)
	}
}

func TestStoreThreadBindingPersistsAcrossReopen(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "39claw.db")

	store := newMigratedStoreAtPath(t, path)

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

	reopened := newMigratedStoreAtPath(t, path)
	defer func() {
		if closeErr := reopened.Close(); closeErr != nil {
			t.Fatalf("Close() reopen error = %v", closeErr)
		}
	}()

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

	store := newMigratedStoreAtPath(t, path)
	ctx := context.Background()

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

	reopened := newMigratedStoreAtPath(t, path)
	defer func() {
		if closeErr := reopened.Close(); closeErr != nil {
			t.Fatalf("Close() reopen error = %v", closeErr)
		}
	}()

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

	if task.BranchName != "task/release-work" {
		t.Fatalf("BranchName = %q, want %q", task.BranchName, "task/release-work")
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

	if task.BranchName != "task/foundation-work" {
		t.Fatalf("BranchName = %q, want %q", task.BranchName, "task/foundation-work")
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

func TestMigrateLegacyDatabaseBootstrapAddsTaskWorktreeColumns(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "39claw.db")

	db, err := sql.Open(driverName, path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}

	if _, err := db.ExecContext(context.Background(), `CREATE TABLE thread_bindings (
		mode TEXT NOT NULL,
		logical_thread_key TEXT NOT NULL,
		codex_thread_id TEXT NOT NULL,
		task_id TEXT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (mode, logical_thread_key)
	);`); err != nil {
		t.Fatalf("create legacy thread_bindings table error = %v", err)
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

	if _, err := db.ExecContext(context.Background(), `CREATE TABLE active_tasks (
		discord_user_id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);`); err != nil {
		t.Fatalf("create legacy active_tasks table error = %v", err)
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

	store := newMigratedStoreAtPath(t, path)

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

func TestStoreAdmitScheduledTaskRunRejectsDuplicateAttempt(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()
	createScheduledTaskForTest(t, ctx, store, app.ScheduledTask{
		ScheduledTaskID: "scheduled-task-1",
		Name:            "daily-report",
		ScheduleKind:    app.ScheduledTaskScheduleKindCron,
		ScheduleExpr:    "0 9 * * *",
		Prompt:          "Write the report.",
		Enabled:         true,
		ReportTarget:    "channel:channel-1",
	})

	dueTime := time.Date(2026, time.April, 12, 0, 0, 0, 0, time.UTC)
	run := app.ScheduledTaskRun{
		ScheduledRunID:  "run-1",
		ScheduledTaskID: "scheduled-task-1",
		Mode:            "daily",
		ScheduledFor:    dueTime,
		Attempt:         1,
		Status:          app.ScheduledTaskRunStatusPending,
	}

	admittedRun, admitted, err := store.AdmitScheduledTaskRun(ctx, run)
	if err != nil {
		t.Fatalf("AdmitScheduledTaskRun() error = %v", err)
	}
	if !admitted {
		t.Fatal("AdmitScheduledTaskRun() admitted = false, want true")
	}
	if admittedRun.ScheduledRunID != "run-1" {
		t.Fatalf("admitted run ID = %q, want %q", admittedRun.ScheduledRunID, "run-1")
	}

	if _, admitted, err := store.AdmitScheduledTaskRun(ctx, run); err != nil {
		t.Fatalf("AdmitScheduledTaskRun() duplicate error = %v", err)
	} else if admitted {
		t.Fatal("AdmitScheduledTaskRun() duplicate admitted = true, want false")
	}

	retryRun, admitted, err := store.AdmitScheduledTaskRun(ctx, app.ScheduledTaskRun{
		ScheduledRunID:  "run-2",
		ScheduledTaskID: "scheduled-task-1",
		Mode:            "daily",
		ScheduledFor:    dueTime,
		Attempt:         2,
		Status:          app.ScheduledTaskRunStatusPending,
	})
	if err != nil {
		t.Fatalf("AdmitScheduledTaskRun() retry error = %v", err)
	}
	if !admitted {
		t.Fatal("AdmitScheduledTaskRun() retry admitted = false, want true")
	}
	if retryRun.Attempt != 2 {
		t.Fatalf("retry attempt = %d, want %d", retryRun.Attempt, 2)
	}
}

func TestStoreListScheduledTaskRunsForDueTimeOrdersAttempts(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()
	createScheduledTaskForTest(t, ctx, store, app.ScheduledTask{
		ScheduledTaskID: "scheduled-task-1",
		Name:            "daily-report",
		ScheduleKind:    app.ScheduledTaskScheduleKindCron,
		ScheduleExpr:    "0 9 * * *",
		Prompt:          "Write the report.",
		Enabled:         true,
		ReportTarget:    "channel:channel-1",
	})

	dueTime := time.Date(2026, time.April, 12, 0, 0, 0, 0, time.UTC)
	runs := []app.ScheduledTaskRun{
		{
			ScheduledRunID:  "run-2",
			ScheduledTaskID: "scheduled-task-1",
			Mode:            "daily",
			ScheduledFor:    dueTime,
			Attempt:         2,
			Status:          app.ScheduledTaskRunStatusFailed,
		},
		{
			ScheduledRunID:  "run-1",
			ScheduledTaskID: "scheduled-task-1",
			Mode:            "daily",
			ScheduledFor:    dueTime,
			Attempt:         1,
			Status:          app.ScheduledTaskRunStatusSucceeded,
		},
	}

	for _, run := range runs {
		if _, admitted, err := store.AdmitScheduledTaskRun(ctx, run); err != nil {
			t.Fatalf("AdmitScheduledTaskRun(%s) error = %v", run.ScheduledRunID, err)
		} else if !admitted {
			t.Fatalf("AdmitScheduledTaskRun(%s) admitted = false, want true", run.ScheduledRunID)
		}
	}

	listedRuns, err := store.ListScheduledTaskRunsForDueTime(ctx, "scheduled-task-1", dueTime)
	if err != nil {
		t.Fatalf("ListScheduledTaskRunsForDueTime() error = %v", err)
	}
	if len(listedRuns) != 2 {
		t.Fatalf("listed run count = %d, want %d", len(listedRuns), 2)
	}
	if listedRuns[0].ScheduledRunID != "run-1" {
		t.Fatalf("listedRuns[0].ScheduledRunID = %q, want %q", listedRuns[0].ScheduledRunID, "run-1")
	}
	if listedRuns[1].ScheduledRunID != "run-2" {
		t.Fatalf("listedRuns[1].ScheduledRunID = %q, want %q", listedRuns[1].ScheduledRunID, "run-2")
	}
}

func TestStoreScheduledTaskAndDeliveryPersistReportTarget(t *testing.T) {
	t.Parallel()

	store := newTestStore(t)
	ctx := context.Background()

	task := app.ScheduledTask{
		ScheduledTaskID: "scheduled-task-1",
		Name:            "daily-report",
		ScheduleKind:    app.ScheduledTaskScheduleKindCron,
		ScheduleExpr:    "0 9 * * *",
		Prompt:          "Write the report.",
		Enabled:         true,
		ReportTarget:    "dm:user-1",
	}
	createScheduledTaskForTest(t, ctx, store, task)

	storedTask, ok, err := store.GetScheduledTaskByName(ctx, task.Name)
	if err != nil {
		t.Fatalf("GetScheduledTaskByName() error = %v", err)
	}
	if !ok {
		t.Fatal("GetScheduledTaskByName() ok = false, want true")
	}
	if storedTask.ReportTarget != task.ReportTarget {
		t.Fatalf("stored task report target = %q, want %q", storedTask.ReportTarget, task.ReportTarget)
	}

	run, admitted, err := store.AdmitScheduledTaskRun(ctx, app.ScheduledTaskRun{
		ScheduledRunID:  "run-1",
		ScheduledTaskID: task.ScheduledTaskID,
		Mode:            "daily",
		ScheduledFor:    time.Date(2026, time.April, 12, 0, 0, 0, 0, time.UTC),
		Attempt:         1,
		Status:          app.ScheduledTaskRunStatusSucceeded,
	})
	if err != nil {
		t.Fatalf("AdmitScheduledTaskRun() error = %v", err)
	}
	if !admitted {
		t.Fatal("AdmitScheduledTaskRun() admitted = false, want true")
	}

	delivery := app.ScheduledTaskDelivery{
		ScheduledDeliveryID: "delivery-1",
		ScheduledRunID:      run.ScheduledRunID,
		ReportTarget:        "dm:user-1",
		Status:              app.ScheduledTaskDeliveryStatusPending,
	}
	if err := store.CreateScheduledTaskDelivery(ctx, delivery); err != nil {
		t.Fatalf("CreateScheduledTaskDelivery() error = %v", err)
	}

	if err := store.UpdateScheduledTaskDelivery(ctx, app.ScheduledTaskDelivery{
		ScheduledDeliveryID: delivery.ScheduledDeliveryID,
		ScheduledRunID:      delivery.ScheduledRunID,
		ReportTarget:        delivery.ReportTarget,
		DiscordMessageID:    "discord-message-1",
		Status:              app.ScheduledTaskDeliveryStatusSucceeded,
		DeliveredAt:         timePtr(time.Date(2026, time.April, 12, 0, 1, 0, 0, time.UTC)),
	}); err != nil {
		t.Fatalf("UpdateScheduledTaskDelivery() error = %v", err)
	}

	var persistedReportTarget string
	var persistedMessageID sql.NullString
	if err := store.db.QueryRowContext(
		ctx,
		`SELECT report_target, discord_message_id
		FROM scheduled_task_deliveries
		WHERE scheduled_delivery_id = ?`,
		delivery.ScheduledDeliveryID,
	).Scan(&persistedReportTarget, &persistedMessageID); err != nil {
		t.Fatalf("query scheduled_task_deliveries error = %v", err)
	}
	if persistedReportTarget != delivery.ReportTarget {
		t.Fatalf("persisted report target = %q, want %q", persistedReportTarget, delivery.ReportTarget)
	}
	if !persistedMessageID.Valid || persistedMessageID.String != "discord-message-1" {
		t.Fatalf("persisted message ID = %+v, want %q", persistedMessageID, "discord-message-1")
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()

	store := newMigratedStoreAtPath(t, filepath.Join(t.TempDir(), "39claw.db"))
	t.Cleanup(func() {
		if closeErr := store.Close(); closeErr != nil {
			t.Fatalf("Close() error = %v", closeErr)
		}
	})

	return store
}

func newMigratedStoreAtPath(t *testing.T, path string) *Store {
	t.Helper()

	db, err := OpenDB(context.Background(), path)
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	store := New(db)
	store.clock = func() time.Time {
		return time.Date(2026, time.April, 5, 15, 4, 0, 0, time.UTC)
	}

	return store
}

func timePtr(value time.Time) *time.Time {
	return &value
}

func createScheduledTaskForTest(t *testing.T, ctx context.Context, store *Store, task app.ScheduledTask) {
	t.Helper()

	if err := store.CreateScheduledTask(ctx, task); err != nil {
		t.Fatalf("CreateScheduledTask(%s) error = %v", task.ScheduledTaskID, err)
	}
}
