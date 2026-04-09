package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	driverName               = "sqlite"
	sqliteDirectoryPermsMode = 0o755
)

var sqlitePragmas = []string{
	`PRAGMA foreign_keys = ON`,
	`PRAGMA journal_mode = WAL`,
	`PRAGMA synchronous = NORMAL`,
	`PRAGMA busy_timeout = 5000`,
}

func OpenDB(ctx context.Context, path string) (*sql.DB, error) {
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

	if err := applyPragmas(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func Open(path string) (*Store, error) {
	db, err := OpenDB(context.Background(), path)
	if err != nil {
		return nil, err
	}

	return New(db), nil
}

func applyPragmas(ctx context.Context, db *sql.DB) error {
	for _, pragma := range sqlitePragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			return fmt.Errorf("apply sqlite pragma %q: %w", pragma, err)
		}
	}

	return nil
}
