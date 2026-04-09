package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/HatsuneMiku3939/39claw/migrations"
)

const schemaMigrationsTable = "schema_migrations"

const taskWorktreeMetadataVersion = 2

var legacyTaskWorktreeColumns = []string{
	"branch_name",
	"base_ref",
	"worktree_path",
	"worktree_status",
	"worktree_created_at",
	"worktree_pruned_at",
	"last_used_at",
}

type migrationFile struct {
	version int
	name    string
	sql     string
}

func Migrate(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("sqlite db must not be nil")
	}

	migrationFiles, err := loadMigrationFiles()
	if err != nil {
		return err
	}

	schemaExists, err := tableExists(ctx, db, schemaMigrationsTable)
	if err != nil {
		return err
	}

	legacyAppTablesExist, err := applicationTablesExist(ctx, db)
	if err != nil {
		return err
	}

	if !schemaExists && legacyAppTablesExist {
		if err := bootstrapLegacySchemaMigrations(ctx, db); err != nil {
			return err
		}
	}

	if err := ensureSchemaMigrationsTable(ctx, db); err != nil {
		return err
	}

	appliedVersions, err := appliedMigrationVersions(ctx, db)
	if err != nil {
		return err
	}

	for _, migrationFile := range migrationFiles {
		if appliedVersions[migrationFile.version] {
			continue
		}

		if err := applyMigration(ctx, db, migrationFile); err != nil {
			return err
		}
	}

	return nil
}

func loadMigrationFiles() ([]migrationFile, error) {
	entries, err := fs.ReadDir(migrations.SQLite, "sqlite")
	if err != nil {
		return nil, fmt.Errorf("read sqlite migrations: %w", err)
	}

	migrationFiles := make([]migrationFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) != ".sql" {
			continue
		}

		version, err := parseMigrationVersion(name)
		if err != nil {
			return nil, err
		}

		sqlBytes, err := fs.ReadFile(migrations.SQLite, filepath.Join("sqlite", name))
		if err != nil {
			return nil, fmt.Errorf("read sqlite migration %q: %w", name, err)
		}

		migrationFiles = append(migrationFiles, migrationFile{
			version: version,
			name:    name,
			sql:     string(sqlBytes),
		})
	}

	slices.SortFunc(migrationFiles, func(left, right migrationFile) int {
		return left.version - right.version
	})

	return migrationFiles, nil
}

func parseMigrationVersion(name string) (int, error) {
	prefix, _, ok := strings.Cut(name, "_")
	if !ok {
		return 0, fmt.Errorf("parse migration version from %q: missing underscore separator", name)
	}

	version, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("parse migration version from %q: %w", name, err)
	}

	return version, nil
}

func ensureSchemaMigrationsTable(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	return nil
}

func appliedMigrationVersions(ctx context.Context, db *sql.DB) (map[int]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY version ASC`)
	if err != nil {
		return nil, fmt.Errorf("query applied schema migrations: %w", err)
	}
	defer rows.Close()

	appliedVersions := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied schema migration: %w", err)
		}
		appliedVersions[version] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied schema migrations: %w", err)
	}

	return appliedVersions, nil
}

func applyMigration(ctx context.Context, db *sql.DB, migrationFile migrationFile) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sqlite migration %s: %w", migrationFile.name, err)
	}
	defer func() {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			return
		}
	}()

	if _, err := tx.ExecContext(ctx, migrationFile.sql); err != nil {
		return fmt.Errorf("execute sqlite migration %s: %w", migrationFile.name, err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
		migrationFile.version,
		time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("record sqlite migration %s: %w", migrationFile.name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sqlite migration %s: %w", migrationFile.name, err)
	}

	return nil
}

func bootstrapLegacySchemaMigrations(ctx context.Context, db *sql.DB) error {
	if err := ensureSchemaMigrationsTable(ctx, db); err != nil {
		return err
	}

	satisfiedVersions, err := detectLegacySatisfiedVersions(ctx, db)
	if err != nil {
		return err
	}

	for _, version := range satisfiedVersions {
		if _, err := db.ExecContext(
			ctx,
			`INSERT OR IGNORE INTO schema_migrations (version, applied_at) VALUES (?, ?)`,
			version,
			time.Now().UTC().Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("record legacy sqlite migration version %d: %w", version, err)
		}
	}

	return nil
}

func detectLegacySatisfiedVersions(ctx context.Context, db *sql.DB) ([]int, error) {
	threadBindingsExists, err := tableExists(ctx, db, "thread_bindings")
	if err != nil {
		return nil, err
	}

	tasksExists, err := tableExists(ctx, db, "tasks")
	if err != nil {
		return nil, err
	}

	activeTasksExists, err := tableExists(ctx, db, "active_tasks")
	if err != nil {
		return nil, err
	}

	if !threadBindingsExists || !tasksExists || !activeTasksExists {
		return nil, fmt.Errorf("bootstrap legacy sqlite schema: unsupported legacy table set")
	}

	satisfiedVersions := []int{1}

	taskColumns, err := tableColumnNames(ctx, db, "tasks")
	if err != nil {
		return nil, err
	}

	if hasAllColumns(taskColumns, legacyTaskWorktreeColumns) {
		if _, err := db.ExecContext(
			ctx,
			`UPDATE tasks SET branch_name = 'task/' || task_id WHERE branch_name = ''`,
		); err != nil {
			return nil, fmt.Errorf("backfill legacy task branch names during bootstrap: %w", err)
		}

		satisfiedVersions = append(satisfiedVersions, taskWorktreeMetadataVersion)
	}

	return satisfiedVersions, nil
}

func applicationTablesExist(ctx context.Context, db *sql.DB) (bool, error) {
	for _, tableName := range []string{"thread_bindings", "tasks", "active_tasks"} {
		exists, err := tableExists(ctx, db, tableName)
		if err != nil {
			return false, err
		}

		if exists {
			return true, nil
		}
	}

	return false, nil
}

func tableExists(ctx context.Context, db *sql.DB, tableName string) (bool, error) {
	row := db.QueryRowContext(
		ctx,
		`SELECT EXISTS(
			SELECT 1
			FROM sqlite_master
			WHERE type = 'table' AND name = ?
		)`,
		tableName,
	)

	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("query sqlite table %q existence: %w", tableName, err)
	}

	return exists, nil
}

func tableColumnNames(ctx context.Context, db *sql.DB, tableName string) ([]string, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s)`, tableName))
	if err != nil {
		return nil, fmt.Errorf("query sqlite table %q info: %w", tableName, err)
	}
	defer rows.Close()

	columnNames := make([]string, 0)
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, fmt.Errorf("scan sqlite table %q info: %w", tableName, err)
		}
		columnNames = append(columnNames, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sqlite table %q info: %w", tableName, err)
	}

	return columnNames, nil
}

func hasAllColumns(existingColumns []string, requiredColumns []string) bool {
	existing := make(map[string]struct{}, len(existingColumns))
	for _, columnName := range existingColumns {
		existing[columnName] = struct{}{}
	}

	for _, requiredColumn := range requiredColumns {
		if _, ok := existing[requiredColumn]; !ok {
			return false
		}
	}

	return true
}
