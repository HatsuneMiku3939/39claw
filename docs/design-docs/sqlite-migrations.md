# SQLite Migrations

This document describes the migration structure now used for 39claw's local SQLite schema.

The repository now replaces the old "create tables on startup and patch selected columns inline" approach with a versioned, testable, and explicit schema-evolution path.

## Why This Change Is Needed

The original store implementation initialized schema directly inside `internal/store/sqlite/store.go`.
That kept the first implementation small, but it became harder to reason about as the schema evolved.

The current approach has three limits:

- schema shape is mixed into CRUD-oriented store code
- additive column migration and data backfill logic are not version-tracked explicitly
- upcoming daily-session and legacy-key rewrites need deterministic, repeatable data migrations

39claw now has enough persistent state that schema evolution should be treated as a first-class concern.

## Design Goals

- keep the runtime small and dependency-light
- keep migrations embedded in the binary
- make startup idempotent and safe to rerun
- support both schema changes and data backfills
- preserve compatibility with already-created local databases
- keep migration logic separate from query and store logic

## Package Shape

The package boundary remains `internal/store/sqlite`, but responsibilities are now split explicitly:

- `internal/store/sqlite/db.go`
  - open the SQLite database
  - create parent directories
  - apply connection pragmas
- `internal/store/sqlite/migrate.go`
  - load embedded migration files
  - ensure the migration history table exists
  - execute pending migrations in version order
- `internal/store/sqlite/store.go`
  - expose store methods only
  - assume schema has already been migrated
- `migrations/sqlite/*.sql`
  - hold versioned SQL files for schema creation and additive schema updates
- `migrations/embed.go`
  - embed migration files into the Go binary

This keeps the store package small without introducing an external migration dependency.

## Startup Flow

The current startup flow is:

```text
OpenDB -> apply pragmas -> Migrate -> New(store) -> serve requests
```

The migration step should happen exactly once per opened database handle during startup.
Store methods should not contain hidden schema-altering behavior after this refactor.

## Migration History Table

The database contains a dedicated history table:

```sql
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);
```

Rules:

- `version` is an integer derived from the SQL filename prefix
- `applied_at` uses UTC RFC3339 or RFC3339Nano text
- rows are append-only in normal operation
- migrations are up-only for v1

Down migrations are intentionally out of scope because 39claw is a local single-binary application and additive forward repair is the safer default.

## Migration File Naming

Migration files should use zero-padded integer prefixes:

```text
0001_initial_schema.sql
0002_task_worktree_metadata.sql
0003_daily_sessions.sql
0004_daily_legacy_key_backfill.sql
```

Rules:

- one increasing numeric version per file
- a short, descriptive suffix after the first underscore
- no gaps are required, but versions must stay unique and monotonic
- each file should be rerunnable only through migration history, not by assuming repeated execution is harmless

## Migration Boundaries

One migration file may contain:

- one cohesive schema change
- one tightly related schema-plus-backfill step

Avoid combining unrelated features into a single migration file.
For example, a `daily_sessions` table introduction and a legacy daily-key rewrite may live in separate versions if they can be reasoned about independently.

## Transaction Strategy

Each migration version should run inside its own transaction:

```text
begin transaction -> execute SQL statements -> record version -> commit
```

If any statement fails:

- roll back the transaction
- stop startup with a clear error
- leave the database at the previous fully applied version

This behavior is especially important for data backfills such as legacy daily-key rewrites.

## SQL Versus Go Logic

The default preference is:

- schema creation and straightforward backfills belong in SQL migration files
- conditional inspection, SQLite capability probing, or content-aware repair may live in Go migration steps when SQL alone would be too brittle

The first implementation can stay SQL-only if it covers current needs cleanly.
If a later migration truly needs procedural branching, `migrate.go` may support a small Go-side post-step for a specific version, but this should be the exception rather than the default.

## Initial Version Mapping

The first migration set reflects the schema that users may already have in the field.

Current initial sequence:

1. `0001_initial_schema.sql`
   - create `schema_migrations`
   - create `thread_bindings`
   - create `tasks`
   - create `active_tasks`
2. `0002_task_worktree_metadata.sql`
   - add task worktree metadata columns introduced after the original task table
   - backfill empty `branch_name` values to `task/<task_id>`
3. `0003_daily_sessions.sql`
   - create `daily_sessions`
   - add constraints and indexes needed for one active generation per local date
4. `0004_daily_legacy_key_backfill.sql`
   - rewrite legacy `daily` logical thread keys from `YYYY-MM-DD` to `YYYY-MM-DD#1`
   - insert or backfill matching `daily_sessions` rows

This sequence mirrors the repository's shipped legacy states without pretending the original schema always existed in one step.

## Compatibility With Existing Databases

Existing user databases created by the old inline startup schema path need a bootstrap rule.

Recommended bootstrap behavior:

- if `schema_migrations` is missing and no application tables exist, apply migrations from version `0001`
- if `schema_migrations` is missing but legacy application tables already exist, run a one-time bootstrap reconciliation that:
  - creates `schema_migrations`
  - inspects which legacy tables and columns are already present
  - inserts the highest fully satisfied migration version
  - runs any remaining later migrations normally

This bootstrap path is the trickiest part of the design.
It should stay narrow and heavily tested so the repository does not strand early adopters on pre-migration databases.

## Pragmas

The database-opening path should also centralize SQLite pragmas.

Recommended defaults:

- `PRAGMA foreign_keys = ON`
- `PRAGMA journal_mode = WAL`
- `PRAGMA synchronous = NORMAL`
- `PRAGMA busy_timeout = 5000`

These settings match the current scale and single-process local deployment model well.

## Testing Strategy

Migration coverage should move from incidental store tests to explicit migration tests.

At minimum, add tests for:

- fresh empty database migrates to latest version
- running migrations twice is safe
- legacy pre-migration database bootstraps to the correct version
- task worktree metadata backfill sets `branch_name` and default worktree state
- legacy daily keys migrate to `#1` and create matching `daily_sessions`
- failed migration rolls back and does not record the failed version

The current reopen-oriented store tests remain useful and should continue proving that persisted state survives process restart.

## Rollout Strategy

The migration refactor should land in small steps:

1. introduce `db.go`, embedded migrations, and `schema_migrations`
2. move current schema creation into versioned SQL files
3. add a bootstrap path for already-existing databases
4. switch `cmd/39claw` startup to call `Migrate()` before constructing stores
5. remove inline schema mutation from store CRUD code

This order keeps the risk concentrated in the migration layer and avoids mixing feature work with infrastructure churn.

## Non-Goals

This design does not currently propose:

- external migration tooling
- down migrations
- checksum validation for migration files
- multi-process distributed migration locking
- a generalized plugin-style migration framework

39claw should keep a minimal, repository-local migration runner unless a future requirement proves that this is too small.

## Recommendation

Keep future SQLite schema work on top of this migration foundation.
In particular, the planned `daily_sessions` feature and legacy daily-key rewrite should land as new migration versions instead of reopening ad hoc startup schema mutation.
