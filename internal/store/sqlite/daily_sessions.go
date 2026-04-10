package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/HatsuneMiku3939/39claw/internal/app"
)

func (s *Store) GetActiveDailySession(ctx context.Context, localDate string) (app.DailySession, bool, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT local_date, generation, logical_thread_key, COALESCE(previous_logical_thread_key, ''),
			activation_reason, is_active, created_at, updated_at
		FROM daily_sessions
		WHERE local_date = ? AND is_active = 1`,
		localDate,
	)

	return scanDailySession(row)
}

func (s *Store) GetLatestDailySessionBefore(ctx context.Context, localDate string) (app.DailySession, bool, error) {
	row := s.db.QueryRowContext(
		ctx,
		`SELECT local_date, generation, logical_thread_key, COALESCE(previous_logical_thread_key, ''),
			activation_reason, is_active, created_at, updated_at
		FROM daily_sessions
		WHERE local_date < ? AND is_active = 1
		ORDER BY local_date DESC, generation DESC
		LIMIT 1`,
		localDate,
	)

	return scanDailySession(row)
}

func (s *Store) CreateDailySession(ctx context.Context, session app.DailySession) (app.DailySession, error) {
	now := s.clock()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = session.CreatedAt
	}
	session.IsActive = true

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return app.DailySession{}, fmt.Errorf("begin create daily session transaction: %w", err)
	}
	defer func() {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return
		}
	}()

	active, ok, err := getActiveDailySessionTx(ctx, tx, session.LocalDate)
	if err != nil {
		return app.DailySession{}, fmt.Errorf("load active daily session in transaction: %w", err)
	}

	if ok {
		return active, nil
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO daily_sessions (
			local_date, generation, logical_thread_key, previous_logical_thread_key,
			activation_reason, is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		session.LocalDate,
		session.Generation,
		session.LogicalThreadKey,
		nullableString(session.PreviousLogicalThreadKey),
		session.ActivationReason,
		boolToSQLiteInt(session.IsActive),
		session.CreatedAt.Format(time.RFC3339Nano),
		session.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return app.DailySession{}, fmt.Errorf("insert daily session: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return app.DailySession{}, fmt.Errorf("commit create daily session transaction: %w", err)
	}

	return session, nil
}

func (s *Store) RotateDailySession(ctx context.Context, localDate string, activationReason string) (app.DailySession, error) {
	now := s.clock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return app.DailySession{}, fmt.Errorf("begin rotate daily session transaction: %w", err)
	}
	defer func() {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return
		}
	}()

	active, ok, err := getActiveDailySessionTx(ctx, tx, localDate)
	if err != nil {
		return app.DailySession{}, fmt.Errorf("load active daily session in transaction: %w", err)
	}
	if !ok {
		return app.DailySession{}, sql.ErrNoRows
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE daily_sessions
		SET is_active = 0, updated_at = ?
		WHERE local_date = ? AND generation = ?`,
		now.Format(time.RFC3339Nano),
		active.LocalDate,
		active.Generation,
	); err != nil {
		return app.DailySession{}, fmt.Errorf("deactivate current daily session: %w", err)
	}

	next := app.DailySession{
		LocalDate:                localDate,
		Generation:               active.Generation + 1,
		LogicalThreadKey:         app.BuildDailyLogicalKey(localDate, active.Generation+1),
		PreviousLogicalThreadKey: active.LogicalThreadKey,
		ActivationReason:         activationReason,
		IsActive:                 true,
		CreatedAt:                now,
		UpdatedAt:                now,
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO daily_sessions (
			local_date, generation, logical_thread_key, previous_logical_thread_key,
			activation_reason, is_active, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		next.LocalDate,
		next.Generation,
		next.LogicalThreadKey,
		next.PreviousLogicalThreadKey,
		next.ActivationReason,
		boolToSQLiteInt(next.IsActive),
		next.CreatedAt.Format(time.RFC3339Nano),
		next.UpdatedAt.Format(time.RFC3339Nano),
	); err != nil {
		return app.DailySession{}, fmt.Errorf("insert rotated daily session: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return app.DailySession{}, fmt.Errorf("commit rotate daily session transaction: %w", err)
	}

	return next, nil
}

func getActiveDailySessionTx(ctx context.Context, tx *sql.Tx, localDate string) (app.DailySession, bool, error) {
	row := tx.QueryRowContext(
		ctx,
		`SELECT local_date, generation, logical_thread_key, COALESCE(previous_logical_thread_key, ''),
			activation_reason, is_active, created_at, updated_at
		FROM daily_sessions
		WHERE local_date = ? AND is_active = 1`,
		localDate,
	)

	return scanDailySession(row)
}

func scanDailySession(scanner interface{ Scan(dest ...any) error }) (app.DailySession, bool, error) {
	var session app.DailySession
	var previousLogicalThreadKey string
	var isActive int
	var createdAt string
	var updatedAt string

	err := scanner.Scan(
		&session.LocalDate,
		&session.Generation,
		&session.LogicalThreadKey,
		&previousLogicalThreadKey,
		&session.ActivationReason,
		&isActive,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return app.DailySession{}, false, nil
	}
	if err != nil {
		return app.DailySession{}, false, fmt.Errorf("scan daily session: %w", err)
	}

	session.PreviousLogicalThreadKey = previousLogicalThreadKey
	session.IsActive = isActive != 0

	session.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return app.DailySession{}, false, fmt.Errorf("parse daily session created_at: %w", err)
	}

	session.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return app.DailySession{}, false, fmt.Errorf("parse daily session updated_at: %w", err)
	}

	return session, true, nil
}

func boolToSQLiteInt(value bool) int {
	if value {
		return 1
	}

	return 0
}
