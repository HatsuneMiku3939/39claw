package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"

	// Register the pure-Go SQLite driver used by the store package.
	_ "modernc.org/sqlite"
)

const (
	driverName               = "sqlite"
	sqliteDirectoryPermsMode = 0o755
)

var taskColumnDefinitions = []struct {
	name       string
	definition string
}{
	{name: "branch_name", definition: `TEXT NOT NULL DEFAULT ''`},
	{name: "base_ref", definition: `TEXT NULL`},
	{name: "worktree_path", definition: `TEXT NULL`},
	{name: "worktree_status", definition: `TEXT NOT NULL DEFAULT 'pending'`},
	{name: "worktree_created_at", definition: `TEXT NULL`},
	{name: "worktree_pruned_at", definition: `TEXT NULL`},
	{name: "last_used_at", definition: `TEXT NULL`},
}

type Store struct {
	db    *sql.DB
	clock func() time.Time
}

func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("sqlite path must not be empty")
	}

	if path != ":memory:" {
		dir := filepath.Dir(path)
		if dir != "." {
			if err := os.MkdirAll(dir, sqliteDirectoryPermsMode); err != nil {
				return nil, fmt.Errorf("create sqlite directory: %w", err)
			}
		}
	}

	db, err := sql.Open(driverName, path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	db.SetMaxOpenConns(1)
	return &Store{
		db:    db,
		clock: time.Now().UTC,
	}, nil
}

func New(db *sql.DB) *Store {
	return &Store{
		db:    db,
		clock: time.Now().UTC,
	}
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) InitSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS thread_bindings (
			mode TEXT NOT NULL,
			logical_thread_key TEXT NOT NULL,
			codex_thread_id TEXT NOT NULL,
			task_id TEXT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY (mode, logical_thread_key)
		);`,
		`CREATE TABLE IF NOT EXISTS tasks (
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
		);`,
		`CREATE TABLE IF NOT EXISTS active_tasks (
			discord_user_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
	}

	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("exec schema statement: %w", err)
		}
	}

	if err := s.ensureTaskColumns(ctx); err != nil {
		return err
	}

	if _, err := s.db.ExecContext(
		ctx,
		`UPDATE tasks SET branch_name = 'task/' || task_id WHERE branch_name = ''`,
	); err != nil {
		return fmt.Errorf("backfill task branch names: %w", err)
	}

	return nil
}

func (s *Store) GetThreadBinding(ctx context.Context, mode string, logicalThreadKey string) (app.ThreadBinding, bool, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT mode, logical_thread_key, codex_thread_id, COALESCE(task_id, ''), created_at, updated_at
		FROM thread_bindings WHERE mode = ? AND logical_thread_key = ?`,
		mode,
		logicalThreadKey,
	)

	var binding app.ThreadBinding
	var createdAt string
	var updatedAt string

	err := row.Scan(
		&binding.Mode,
		&binding.LogicalThreadKey,
		&binding.CodexThreadID,
		&binding.TaskID,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return app.ThreadBinding{}, false, nil
	}

	if err != nil {
		return app.ThreadBinding{}, false, fmt.Errorf("scan thread binding: %w", err)
	}

	binding.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return app.ThreadBinding{}, false, fmt.Errorf("parse thread binding created_at: %w", err)
	}

	binding.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return app.ThreadBinding{}, false, fmt.Errorf("parse thread binding updated_at: %w", err)
	}

	return binding, true, nil
}

func (s *Store) UpsertThreadBinding(ctx context.Context, binding app.ThreadBinding) error {
	now := s.clock()
	if binding.CreatedAt.IsZero() {
		binding.CreatedAt = now
	}
	binding.UpdatedAt = now

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO thread_bindings (
			mode, logical_thread_key, codex_thread_id, task_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(mode, logical_thread_key) DO UPDATE SET
			codex_thread_id = excluded.codex_thread_id,
			task_id = excluded.task_id,
			updated_at = excluded.updated_at`,
		binding.Mode,
		binding.LogicalThreadKey,
		binding.CodexThreadID,
		nullableString(binding.TaskID),
		binding.CreatedAt.Format(time.RFC3339Nano),
		binding.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert thread binding: %w", err)
	}

	return nil
}

func (s *Store) CreateTask(ctx context.Context, task app.Task) error {
	now := s.clock()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = now
	}
	if task.Status == "" {
		task.Status = app.TaskStatusOpen
	}

	if task.BranchName == "" {
		task.BranchName = app.DefaultTaskBranchName(task.TaskID)
	}

	if task.WorktreeStatus == "" {
		task.WorktreeStatus = app.TaskWorktreeStatusPending
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO tasks (
			task_id, discord_user_id, task_name, status, branch_name, base_ref, worktree_path,
			worktree_status, created_at, updated_at, closed_at, worktree_created_at, worktree_pruned_at, last_used_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.TaskID,
		task.DiscordUserID,
		task.TaskName,
		string(task.Status),
		task.BranchName,
		nullableString(task.BaseRef),
		nullableString(task.WorktreePath),
		string(task.WorktreeStatus),
		task.CreatedAt.Format(time.RFC3339Nano),
		task.UpdatedAt.Format(time.RFC3339Nano),
		nullableTime(task.ClosedAt),
		nullableTime(task.WorktreeCreatedAt),
		nullableTime(task.WorktreePrunedAt),
		nullableTime(task.LastUsedAt),
	)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	return nil
}

func (s *Store) GetTask(ctx context.Context, discordUserID string, taskID string) (app.Task, bool, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT task_id, discord_user_id, task_name, status, branch_name, base_ref, worktree_path, worktree_status,
			created_at, updated_at, closed_at, worktree_created_at, worktree_pruned_at, last_used_at
		FROM tasks WHERE discord_user_id = ? AND task_id = ?`,
		discordUserID,
		taskID,
	)

	return scanTask(row)
}

func (s *Store) ListOpenTasks(ctx context.Context, discordUserID string) ([]app.Task, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT task_id, discord_user_id, task_name, status, branch_name, base_ref, worktree_path, worktree_status,
			created_at, updated_at, closed_at, worktree_created_at, worktree_pruned_at, last_used_at
		FROM tasks WHERE discord_user_id = ? AND status = ? ORDER BY created_at ASC`,
		discordUserID,
		string(app.TaskStatusOpen),
	)
	if err != nil {
		return nil, fmt.Errorf("query open tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]app.Task, 0)
	for rows.Next() {
		task, _, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate open tasks: %w", err)
	}

	return tasks, nil
}

func (s *Store) UpdateTask(ctx context.Context, task app.Task) error {
	now := s.clock()
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = now
	}

	if task.Status == "" {
		task.Status = app.TaskStatusOpen
	}

	if task.BranchName == "" {
		task.BranchName = app.DefaultTaskBranchName(task.TaskID)
	}

	if task.WorktreeStatus == "" {
		task.WorktreeStatus = app.TaskWorktreeStatusPending
	}

	result, err := s.db.ExecContext(
		ctx,
		`UPDATE tasks
		SET task_name = ?, status = ?, branch_name = ?, base_ref = ?, worktree_path = ?, worktree_status = ?,
			updated_at = ?, closed_at = ?, worktree_created_at = ?, worktree_pruned_at = ?, last_used_at = ?
		WHERE discord_user_id = ? AND task_id = ?`,
		task.TaskName,
		string(task.Status),
		task.BranchName,
		nullableString(task.BaseRef),
		nullableString(task.WorktreePath),
		string(task.WorktreeStatus),
		task.UpdatedAt.Format(time.RFC3339Nano),
		nullableTime(task.ClosedAt),
		nullableTime(task.WorktreeCreatedAt),
		nullableTime(task.WorktreePrunedAt),
		nullableTime(task.LastUsedAt),
		task.DiscordUserID,
		task.TaskID,
	)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update task rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *Store) ListClosedReadyTasks(ctx context.Context) ([]app.Task, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT task_id, discord_user_id, task_name, status, branch_name, base_ref, worktree_path, worktree_status,
			created_at, updated_at, closed_at, worktree_created_at, worktree_pruned_at, last_used_at
		FROM tasks
		WHERE status = ? AND worktree_status = ? AND closed_at IS NOT NULL
		ORDER BY closed_at DESC, task_id ASC`,
		string(app.TaskStatusClosed),
		string(app.TaskWorktreeStatusReady),
	)
	if err != nil {
		return nil, fmt.Errorf("query closed ready tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]app.Task, 0)
	for rows.Next() {
		task, _, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate closed ready tasks: %w", err)
	}

	return tasks, nil
}

func (s *Store) SetActiveTask(ctx context.Context, activeTask app.ActiveTask) error {
	now := s.clock()
	if activeTask.UpdatedAt.IsZero() {
		activeTask.UpdatedAt = now
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO active_tasks (discord_user_id, task_id, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(discord_user_id) DO UPDATE SET
			task_id = excluded.task_id,
			updated_at = excluded.updated_at`,
		activeTask.DiscordUserID,
		activeTask.TaskID,
		activeTask.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("set active task: %w", err)
	}

	return nil
}

func (s *Store) GetActiveTask(ctx context.Context, discordUserID string) (app.ActiveTask, bool, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT discord_user_id, task_id, updated_at FROM active_tasks WHERE discord_user_id = ?`,
		discordUserID,
	)

	var activeTask app.ActiveTask
	var updatedAt string

	err := row.Scan(&activeTask.DiscordUserID, &activeTask.TaskID, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return app.ActiveTask{}, false, nil
	}

	if err != nil {
		return app.ActiveTask{}, false, fmt.Errorf("scan active task: %w", err)
	}

	activeTask.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return app.ActiveTask{}, false, fmt.Errorf("parse active task updated_at: %w", err)
	}

	return activeTask, true, nil
}

func (s *Store) ClearActiveTask(ctx context.Context, discordUserID string) error {
	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM active_tasks WHERE discord_user_id = ?`,
		discordUserID,
	)
	if err != nil {
		return fmt.Errorf("clear active task: %w", err)
	}

	return nil
}

func (s *Store) CloseTask(ctx context.Context, discordUserID string, taskID string) error {
	now := s.clock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin close task transaction: %w", err)
	}
	defer func() {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return
		}
	}()

	result, err := tx.ExecContext(
		ctx,
		`UPDATE tasks
		SET status = ?, updated_at = ?, closed_at = ?
		WHERE discord_user_id = ? AND task_id = ?`,
		string(app.TaskStatusClosed),
		now.Format(time.RFC3339Nano),
		now.Format(time.RFC3339Nano),
		discordUserID,
		taskID,
	)
	if err != nil {
		return fmt.Errorf("update task status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("task status rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM active_tasks WHERE discord_user_id = ? AND task_id = ?`,
		discordUserID,
		taskID,
	); err != nil {
		return fmt.Errorf("delete active task: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit close task transaction: %w", err)
	}

	return nil
}

func scanTask(scanner interface{ Scan(dest ...any) error }) (app.Task, bool, error) {
	var task app.Task
	var status string
	var worktreeStatus string
	var branchName string
	var baseRef sql.NullString
	var worktreePath sql.NullString
	var createdAt string
	var updatedAt string
	var closedAt sql.NullString
	var worktreeCreatedAt sql.NullString
	var worktreePrunedAt sql.NullString
	var lastUsedAt sql.NullString

	err := scanner.Scan(
		&task.TaskID,
		&task.DiscordUserID,
		&task.TaskName,
		&status,
		&branchName,
		&baseRef,
		&worktreePath,
		&worktreeStatus,
		&createdAt,
		&updatedAt,
		&closedAt,
		&worktreeCreatedAt,
		&worktreePrunedAt,
		&lastUsedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return app.Task{}, false, nil
	}

	if err != nil {
		return app.Task{}, false, fmt.Errorf("scan task: %w", err)
	}

	task.Status = app.TaskStatus(status)
	task.BranchName = branchName
	if task.BranchName == "" {
		task.BranchName = app.DefaultTaskBranchName(task.TaskID)
	}
	task.BaseRef = nullableStringValue(baseRef)
	task.WorktreePath = nullableStringValue(worktreePath)
	task.WorktreeStatus = app.TaskWorktreeStatus(worktreeStatus)
	if task.WorktreeStatus == "" {
		task.WorktreeStatus = app.TaskWorktreeStatusPending
	}

	task.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return app.Task{}, false, fmt.Errorf("parse task created_at: %w", err)
	}

	task.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return app.Task{}, false, fmt.Errorf("parse task updated_at: %w", err)
	}

	if closedAt.Valid {
		parsedClosedAt, parseErr := time.Parse(time.RFC3339Nano, closedAt.String)
		if parseErr != nil {
			return app.Task{}, false, fmt.Errorf("parse task closed_at: %w", parseErr)
		}
		task.ClosedAt = &parsedClosedAt
	}

	if task.WorktreeCreatedAt, err = parseNullableTime(worktreeCreatedAt); err != nil {
		return app.Task{}, false, fmt.Errorf("parse task worktree_created_at: %w", err)
	}

	if task.WorktreePrunedAt, err = parseNullableTime(worktreePrunedAt); err != nil {
		return app.Task{}, false, fmt.Errorf("parse task worktree_pruned_at: %w", err)
	}

	if task.LastUsedAt, err = parseNullableTime(lastUsedAt); err != nil {
		return app.Task{}, false, fmt.Errorf("parse task last_used_at: %w", err)
	}

	return task, true, nil
}

func (s *Store) ensureTaskColumns(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(tasks)`)
	if err != nil {
		return fmt.Errorf("query task table info: %w", err)
	}
	defer rows.Close()

	existingColumns := make(map[string]struct{})
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("scan task table info: %w", err)
		}
		existingColumns[name] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate task table info: %w", err)
	}

	for _, column := range taskColumnDefinitions {
		if _, ok := existingColumns[column.name]; ok {
			continue
		}

		statement := fmt.Sprintf(`ALTER TABLE tasks ADD COLUMN %s %s`, column.name, column.definition)
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("add tasks.%s column: %w", column.name, err)
		}
	}

	return nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}

	return value
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}

	return value.Format(time.RFC3339Nano)
}

func nullableStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}

	return value.String
}

func parseNullableTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}

	parsed, err := time.Parse(time.RFC3339Nano, value.String)
	if err != nil {
		return nil, err
	}

	return &parsed, nil
}
