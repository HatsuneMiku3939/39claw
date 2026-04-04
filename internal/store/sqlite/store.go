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
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			closed_at TEXT NULL
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

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO tasks (
			task_id, discord_user_id, task_name, status, created_at, updated_at, closed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		task.TaskID,
		task.DiscordUserID,
		task.TaskName,
		string(task.Status),
		task.CreatedAt.Format(time.RFC3339Nano),
		task.UpdatedAt.Format(time.RFC3339Nano),
		nullableTime(task.ClosedAt),
	)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}

	return nil
}

func (s *Store) GetTask(ctx context.Context, discordUserID string, taskID string) (app.Task, bool, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT task_id, discord_user_id, task_name, status, created_at, updated_at, closed_at
		FROM tasks WHERE discord_user_id = ? AND task_id = ?`,
		discordUserID,
		taskID,
	)

	return scanTask(row)
}

func (s *Store) ListOpenTasks(ctx context.Context, discordUserID string) ([]app.Task, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT task_id, discord_user_id, task_name, status, created_at, updated_at, closed_at
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
	var createdAt string
	var updatedAt string
	var closedAt sql.NullString

	err := scanner.Scan(
		&task.TaskID,
		&task.DiscordUserID,
		&task.TaskName,
		&status,
		&createdAt,
		&updatedAt,
		&closedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return app.Task{}, false, nil
	}

	if err != nil {
		return app.Task{}, false, fmt.Errorf("scan task: %w", err)
	}

	task.Status = app.TaskStatus(status)

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

	return task, true, nil
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
