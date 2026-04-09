package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"slices"
	"testing"
)

func TestMigrateFreshDatabase(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "39claw.db")
	db, err := OpenDB(context.Background(), path)
	if err != nil {
		t.Fatalf("OpenDB() error = %v", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Fatalf("db.Close() error = %v", closeErr)
		}
	}()

	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	versions := appliedVersionsForTest(t, db)
	if !slices.Equal(versions, []int{1, 2}) {
		t.Fatalf("applied migration versions = %v, want %v", versions, []int{1, 2})
	}

	for _, tableName := range []string{"schema_migrations", "thread_bindings", "tasks", "active_tasks"} {
		exists, err := tableExists(context.Background(), db, tableName)
		if err != nil {
			t.Fatalf("tableExists(%q) error = %v", tableName, err)
		}
		if !exists {
			t.Fatalf("table %q exists = false, want true", tableName)
		}
	}

	taskColumns, err := tableColumnNames(context.Background(), db, "tasks")
	if err != nil {
		t.Fatalf("tableColumnNames(tasks) error = %v", err)
	}

	if !hasAllColumns(taskColumns, legacyTaskWorktreeColumns) {
		t.Fatalf("tasks columns = %v, want worktree metadata columns present", taskColumns)
	}
}

func TestMigrateLegacyDatabaseBootstrapRecognizesLatestSchema(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "39claw.db")
	db, err := sql.Open(driverName, path)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `CREATE TABLE thread_bindings (
		mode TEXT NOT NULL,
		logical_thread_key TEXT NOT NULL,
		codex_thread_id TEXT NOT NULL,
		task_id TEXT NULL,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY (mode, logical_thread_key)
	);`); err != nil {
		t.Fatalf("create thread_bindings table error = %v", err)
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE tasks (
		task_id TEXT PRIMARY KEY,
		discord_user_id TEXT NOT NULL,
		task_name TEXT NOT NULL,
		status TEXT NOT NULL,
		branch_name TEXT NOT NULL DEFAULT '',
		base_ref TEXT NULL,
		worktree_path TEXT NULL,
		worktree_status TEXT NOT NULL DEFAULT 'pending',
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		closed_at TEXT NULL,
		worktree_created_at TEXT NULL,
		worktree_pruned_at TEXT NULL,
		last_used_at TEXT NULL
	);`); err != nil {
		t.Fatalf("create tasks table error = %v", err)
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE active_tasks (
		discord_user_id TEXT PRIMARY KEY,
		task_id TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);`); err != nil {
		t.Fatalf("create active_tasks table error = %v", err)
	}

	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO tasks (
			task_id, discord_user_id, task_name, status, branch_name, base_ref, worktree_path, worktree_status,
			created_at, updated_at, closed_at, worktree_created_at, worktree_pruned_at, last_used_at
		) VALUES (?, ?, ?, ?, ?, NULL, NULL, ?, ?, ?, NULL, NULL, NULL, NULL)`,
		"task-1",
		"user-1",
		"Already migrated task",
		"open",
		"",
		"pending",
		"2026-04-05T15:04:00Z",
		"2026-04-05T15:04:00Z",
	); err != nil {
		t.Fatalf("insert latest legacy task error = %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("db.Close() error = %v", err)
	}

	reopened, err := OpenDB(context.Background(), path)
	if err != nil {
		t.Fatalf("OpenDB() reopen error = %v", err)
	}
	defer func() {
		if closeErr := reopened.Close(); closeErr != nil {
			t.Fatalf("reopened.Close() error = %v", closeErr)
		}
	}()

	if err := Migrate(ctx, reopened); err != nil {
		t.Fatalf("Migrate() reopen error = %v", err)
	}

	versions := appliedVersionsForTest(t, reopened)
	if !slices.Equal(versions, []int{1, 2}) {
		t.Fatalf("applied migration versions = %v, want %v", versions, []int{1, 2})
	}

	store := New(reopened)
	task, ok, err := store.GetTask(ctx, "user-1", "task-1")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if !ok {
		t.Fatal("GetTask() ok = false, want true")
	}
	if task.BranchName != "task/task-1" {
		t.Fatalf("BranchName = %q, want %q", task.BranchName, "task/task-1")
	}
}

func appliedVersionsForTest(t *testing.T, db *sql.DB) []int {
	t.Helper()

	rows, err := db.QueryContext(context.Background(), `SELECT version FROM schema_migrations ORDER BY version ASC`)
	if err != nil {
		t.Fatalf("query schema_migrations versions error = %v", err)
	}
	defer rows.Close()

	versions := make([]int, 0)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			t.Fatalf("scan schema_migrations version error = %v", err)
		}
		versions = append(versions, version)
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterate schema_migrations versions error = %v", err)
	}

	return versions
}
