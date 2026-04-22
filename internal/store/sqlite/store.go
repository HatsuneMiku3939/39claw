package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"

	// Register the pure-Go SQLite driver used by the store package.
	_ "modernc.org/sqlite"
)

type Store struct {
	db    *sql.DB
	clock func() time.Time
}

func New(db *sql.DB) *Store {
	return &Store{
		db: db,
		clock: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) InitSchema(ctx context.Context) error {
	return Migrate(ctx, s.db)
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

func (s *Store) DeleteThreadBinding(ctx context.Context, mode string, logicalThreadKey string) error {
	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM thread_bindings WHERE mode = ? AND logical_thread_key = ?`,
		mode,
		logicalThreadKey,
	)
	if err != nil {
		return fmt.Errorf("delete thread binding: %w", err)
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
		task.BranchName = app.DefaultTaskBranchName(task.TaskName, task.TaskID)
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

func (s *Store) HasClosedTaskWithName(ctx context.Context, discordUserID string, taskName string) (bool, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT 1
		FROM tasks
		WHERE discord_user_id = ? AND task_name = ? AND status = ?
		LIMIT 1`,
		discordUserID,
		taskName,
		string(app.TaskStatusClosed),
	)

	var matched int
	if err := row.Scan(&matched); errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("query closed task by name: %w", err)
	}

	return true, nil
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
		task.BranchName = app.DefaultTaskBranchName(task.TaskName, task.TaskID)
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

func (s *Store) ListScheduledTasks(ctx context.Context) ([]app.ScheduledTask, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT scheduled_task_id, name, schedule_kind, schedule_expr, prompt, enabled, report_target,
			created_at, updated_at, disabled_at
		FROM scheduled_tasks
		ORDER BY name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query scheduled tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]app.ScheduledTask, 0)
	for rows.Next() {
		task, _, err := scanScheduledTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scheduled tasks: %w", err)
	}

	return tasks, nil
}

func (s *Store) ListEnabledScheduledTasks(ctx context.Context) ([]app.ScheduledTask, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT scheduled_task_id, name, schedule_kind, schedule_expr, prompt, enabled, report_target,
			created_at, updated_at, disabled_at
		FROM scheduled_tasks
		WHERE enabled = 1
		ORDER BY name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query enabled scheduled tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]app.ScheduledTask, 0)
	for rows.Next() {
		task, _, err := scanScheduledTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate enabled scheduled tasks: %w", err)
	}

	return tasks, nil
}

func (s *Store) GetScheduledTaskByID(ctx context.Context, scheduledTaskID string) (app.ScheduledTask, bool, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT scheduled_task_id, name, schedule_kind, schedule_expr, prompt, enabled, report_target,
			created_at, updated_at, disabled_at
		FROM scheduled_tasks
		WHERE scheduled_task_id = ?`,
		scheduledTaskID,
	)

	return scanScheduledTask(row)
}

func (s *Store) GetScheduledTaskByName(ctx context.Context, name string) (app.ScheduledTask, bool, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT scheduled_task_id, name, schedule_kind, schedule_expr, prompt, enabled, report_target,
			created_at, updated_at, disabled_at
		FROM scheduled_tasks
		WHERE name = ?`,
		name,
	)

	return scanScheduledTask(row)
}

func (s *Store) CreateScheduledTask(ctx context.Context, task app.ScheduledTask) error {
	now := s.clock()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = now
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO scheduled_tasks (
			scheduled_task_id, name, schedule_kind, schedule_expr, prompt, enabled, report_target,
			created_at, updated_at, disabled_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ScheduledTaskID,
		task.Name,
		string(task.ScheduleKind),
		task.ScheduleExpr,
		task.Prompt,
		boolToSQLite(task.Enabled),
		nullableString(task.ReportTarget),
		task.CreatedAt.Format(time.RFC3339Nano),
		task.UpdatedAt.Format(time.RFC3339Nano),
		nullableTime(task.DisabledAt),
	)
	if err != nil {
		return fmt.Errorf("create scheduled task: %w", err)
	}

	return nil
}

func (s *Store) UpdateScheduledTask(ctx context.Context, task app.ScheduledTask) error {
	now := s.clock()
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = now
	}

	result, err := s.db.ExecContext(
		ctx,
		`UPDATE scheduled_tasks
		SET name = ?, schedule_kind = ?, schedule_expr = ?, prompt = ?, enabled = ?, report_target = ?,
			updated_at = ?, disabled_at = ?
		WHERE scheduled_task_id = ?`,
		task.Name,
		string(task.ScheduleKind),
		task.ScheduleExpr,
		task.Prompt,
		boolToSQLite(task.Enabled),
		nullableString(task.ReportTarget),
		task.UpdatedAt.Format(time.RFC3339Nano),
		nullableTime(task.DisabledAt),
		task.ScheduledTaskID,
	)
	if err != nil {
		return fmt.Errorf("update scheduled task: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("scheduled task rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *Store) DeleteScheduledTask(ctx context.Context, scheduledTaskID string) error {
	_, err := s.db.ExecContext(
		ctx,
		`DELETE FROM scheduled_tasks WHERE scheduled_task_id = ?`,
		scheduledTaskID,
	)
	if err != nil {
		return fmt.Errorf("delete scheduled task: %w", err)
	}

	return nil
}

func (s *Store) GetLatestScheduledTaskRunForTask(ctx context.Context, scheduledTaskID string) (app.ScheduledTaskRun, bool, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT scheduled_run_id, scheduled_task_id, mode, scheduled_for, attempt, status, codex_thread_id,
			workdir_path, temp_worktree_path, started_at, finished_at, error_code, error_message, response_text,
			created_at, updated_at
		FROM scheduled_task_runs
		WHERE scheduled_task_id = ?
		ORDER BY scheduled_for DESC, attempt DESC
		LIMIT 1`,
		scheduledTaskID,
	)

	return scanScheduledTaskRun(row)
}

func (s *Store) AdmitScheduledTaskRun(ctx context.Context, run app.ScheduledTaskRun) (app.ScheduledTaskRun, bool, error) {
	now := s.clock()
	if run.CreatedAt.IsZero() {
		run.CreatedAt = now
	}
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = now
	}
	if run.Status == "" {
		run.Status = app.ScheduledTaskRunStatusPending
	}
	if run.Attempt <= 0 {
		run.Attempt = 1
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO scheduled_task_runs (
			scheduled_run_id, scheduled_task_id, mode, scheduled_for, attempt, status, codex_thread_id,
			workdir_path, temp_worktree_path, started_at, finished_at, error_code, error_message, response_text,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ScheduledRunID,
		run.ScheduledTaskID,
		run.Mode,
		run.ScheduledFor.Format(time.RFC3339Nano),
		run.Attempt,
		string(run.Status),
		nullableString(run.CodexThreadID),
		nullableString(run.WorkdirPath),
		nullableString(run.TempWorktreePath),
		nullableTime(run.StartedAt),
		nullableTime(run.FinishedAt),
		nullableString(run.ErrorCode),
		nullableString(run.ErrorMessage),
		nullableString(run.ResponseText),
		run.CreatedAt.Format(time.RFC3339Nano),
		run.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		if isUniqueConstraintError(err) {
			return app.ScheduledTaskRun{}, false, nil
		}

		return app.ScheduledTaskRun{}, false, fmt.Errorf("create scheduled task run: %w", err)
	}

	return run, true, nil
}

func (s *Store) UpdateScheduledTaskRun(ctx context.Context, run app.ScheduledTaskRun) error {
	now := s.clock()
	if run.UpdatedAt.IsZero() {
		run.UpdatedAt = now
	}

	result, err := s.db.ExecContext(
		ctx,
		`UPDATE scheduled_task_runs
		SET mode = ?, scheduled_for = ?, attempt = ?, status = ?, codex_thread_id = ?, workdir_path = ?,
			temp_worktree_path = ?, started_at = ?, finished_at = ?, error_code = ?, error_message = ?,
			response_text = ?, updated_at = ?
		WHERE scheduled_run_id = ?`,
		run.Mode,
		run.ScheduledFor.Format(time.RFC3339Nano),
		run.Attempt,
		string(run.Status),
		nullableString(run.CodexThreadID),
		nullableString(run.WorkdirPath),
		nullableString(run.TempWorktreePath),
		nullableTime(run.StartedAt),
		nullableTime(run.FinishedAt),
		nullableString(run.ErrorCode),
		nullableString(run.ErrorMessage),
		nullableString(run.ResponseText),
		run.UpdatedAt.Format(time.RFC3339Nano),
		run.ScheduledRunID,
	)
	if err != nil {
		return fmt.Errorf("update scheduled task run: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("scheduled task run rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *Store) ListScheduledTaskRunsForDueTime(ctx context.Context, scheduledTaskID string, scheduledFor time.Time) ([]app.ScheduledTaskRun, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT scheduled_run_id, scheduled_task_id, mode, scheduled_for, attempt, status, codex_thread_id,
			workdir_path, temp_worktree_path, started_at, finished_at, error_code, error_message, response_text,
			created_at, updated_at
		FROM scheduled_task_runs
		WHERE scheduled_task_id = ? AND scheduled_for = ?
		ORDER BY attempt ASC`,
		scheduledTaskID,
		scheduledFor.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("query scheduled task runs: %w", err)
	}
	defer rows.Close()

	runs := make([]app.ScheduledTaskRun, 0)
	for rows.Next() {
		run, _, err := scanScheduledTaskRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate scheduled task runs: %w", err)
	}

	return runs, nil
}

func (s *Store) CreateScheduledTaskDelivery(ctx context.Context, delivery app.ScheduledTaskDelivery) error {
	now := s.clock()
	if delivery.CreatedAt.IsZero() {
		delivery.CreatedAt = now
	}
	if delivery.UpdatedAt.IsZero() {
		delivery.UpdatedAt = now
	}
	if delivery.Status == "" {
		delivery.Status = app.ScheduledTaskDeliveryStatusPending
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO scheduled_task_deliveries (
			scheduled_delivery_id, scheduled_run_id, report_target, discord_message_id, status, delivered_at,
			error_code, error_message, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		delivery.ScheduledDeliveryID,
		delivery.ScheduledRunID,
		delivery.ReportTarget,
		nullableString(delivery.DiscordMessageID),
		string(delivery.Status),
		nullableTime(delivery.DeliveredAt),
		nullableString(delivery.ErrorCode),
		nullableString(delivery.ErrorMessage),
		delivery.CreatedAt.Format(time.RFC3339Nano),
		delivery.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("create scheduled task delivery: %w", err)
	}

	return nil
}

func (s *Store) UpdateScheduledTaskDelivery(ctx context.Context, delivery app.ScheduledTaskDelivery) error {
	now := s.clock()
	if delivery.UpdatedAt.IsZero() {
		delivery.UpdatedAt = now
	}

	result, err := s.db.ExecContext(
		ctx,
		`UPDATE scheduled_task_deliveries
		SET report_target = ?, discord_message_id = ?, status = ?, delivered_at = ?, error_code = ?,
			error_message = ?, updated_at = ?
		WHERE scheduled_delivery_id = ?`,
		delivery.ReportTarget,
		nullableString(delivery.DiscordMessageID),
		string(delivery.Status),
		nullableTime(delivery.DeliveredAt),
		nullableString(delivery.ErrorCode),
		nullableString(delivery.ErrorMessage),
		delivery.UpdatedAt.Format(time.RFC3339Nano),
		delivery.ScheduledDeliveryID,
	)
	if err != nil {
		return fmt.Errorf("update scheduled task delivery: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("scheduled task delivery rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func scanScheduledTask(scanner interface{ Scan(dest ...any) error }) (app.ScheduledTask, bool, error) {
	var task app.ScheduledTask
	var scheduleKind string
	var enabled int
	var reportTarget sql.NullString
	var createdAt string
	var updatedAt string
	var disabledAt sql.NullString

	err := scanner.Scan(
		&task.ScheduledTaskID,
		&task.Name,
		&scheduleKind,
		&task.ScheduleExpr,
		&task.Prompt,
		&enabled,
		&reportTarget,
		&createdAt,
		&updatedAt,
		&disabledAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return app.ScheduledTask{}, false, nil
	}
	if err != nil {
		return app.ScheduledTask{}, false, fmt.Errorf("scan scheduled task: %w", err)
	}

	task.ScheduleKind = app.ScheduledTaskScheduleKind(scheduleKind)
	task.Enabled = enabled != 0
	task.ReportTarget = nullableStringValue(reportTarget)

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return app.ScheduledTask{}, false, fmt.Errorf("parse scheduled task created_at: %w", err)
	}
	task.CreatedAt = parsedCreatedAt

	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return app.ScheduledTask{}, false, fmt.Errorf("parse scheduled task updated_at: %w", err)
	}
	task.UpdatedAt = parsedUpdatedAt

	if task.DisabledAt, err = parseNullableTime(disabledAt); err != nil {
		return app.ScheduledTask{}, false, fmt.Errorf("parse scheduled task disabled_at: %w", err)
	}

	return task, true, nil
}

func scanScheduledTaskRun(scanner interface{ Scan(dest ...any) error }) (app.ScheduledTaskRun, bool, error) {
	var run app.ScheduledTaskRun
	var status string
	var scheduledFor string
	var codexThreadID sql.NullString
	var workdirPath sql.NullString
	var tempWorktreePath sql.NullString
	var startedAt sql.NullString
	var finishedAt sql.NullString
	var errorCode sql.NullString
	var errorMessage sql.NullString
	var responseText sql.NullString
	var createdAt string
	var updatedAt string

	err := scanner.Scan(
		&run.ScheduledRunID,
		&run.ScheduledTaskID,
		&run.Mode,
		&scheduledFor,
		&run.Attempt,
		&status,
		&codexThreadID,
		&workdirPath,
		&tempWorktreePath,
		&startedAt,
		&finishedAt,
		&errorCode,
		&errorMessage,
		&responseText,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return app.ScheduledTaskRun{}, false, nil
	}
	if err != nil {
		return app.ScheduledTaskRun{}, false, fmt.Errorf("scan scheduled task run: %w", err)
	}

	run.Status = app.ScheduledTaskRunStatus(status)
	run.CodexThreadID = nullableStringValue(codexThreadID)
	run.WorkdirPath = nullableStringValue(workdirPath)
	run.TempWorktreePath = nullableStringValue(tempWorktreePath)
	run.ErrorCode = nullableStringValue(errorCode)
	run.ErrorMessage = nullableStringValue(errorMessage)
	run.ResponseText = nullableStringValue(responseText)

	parsedScheduledFor, err := time.Parse(time.RFC3339Nano, scheduledFor)
	if err != nil {
		return app.ScheduledTaskRun{}, false, fmt.Errorf("parse scheduled task run scheduled_for: %w", err)
	}
	run.ScheduledFor = parsedScheduledFor

	if run.StartedAt, err = parseNullableTime(startedAt); err != nil {
		return app.ScheduledTaskRun{}, false, fmt.Errorf("parse scheduled task run started_at: %w", err)
	}
	if run.FinishedAt, err = parseNullableTime(finishedAt); err != nil {
		return app.ScheduledTaskRun{}, false, fmt.Errorf("parse scheduled task run finished_at: %w", err)
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return app.ScheduledTaskRun{}, false, fmt.Errorf("parse scheduled task run created_at: %w", err)
	}
	run.CreatedAt = parsedCreatedAt

	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return app.ScheduledTaskRun{}, false, fmt.Errorf("parse scheduled task run updated_at: %w", err)
	}
	run.UpdatedAt = parsedUpdatedAt

	return run, true, nil
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
		task.BranchName = app.DefaultTaskBranchName(task.TaskName, task.TaskID)
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

func nullableString(value string) any {
	if value == "" {
		return nil
	}

	return value
}

func boolToSQLite(value bool) int {
	if value {
		return 1
	}

	return 0
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

func isUniqueConstraintError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint failed")
}
