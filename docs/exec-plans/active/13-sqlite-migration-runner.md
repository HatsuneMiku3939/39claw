# Build a versioned SQLite migration runner and bootstrap path

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, 39claw should start against its local SQLite database by opening the connection, applying SQLite pragmas, and running embedded, versioned, up-only migrations before any store method touches application tables. Contributors should be able to add a new migration file such as `migrations/sqlite/0003_example.sql`, restart the bot, and observe that the change is applied exactly once and recorded in `schema_migrations`.

This change matters because the repository currently creates and mutates schema inline inside store code. That approach was small enough for the first milestones, but it makes future schema and data changes harder to reason about, harder to test in isolation, and harder to apply safely to databases that were created by earlier versions of the bot. The outcome of this plan is a stable migration foundation that later work, including `daily_sessions`, can build on without mixing feature logic into startup schema repair.

## Progress

- [x] (2026-04-06 00:00Z) Reviewed the current SQLite startup path, the inline schema initialization in `internal/store/sqlite/store.go`, the new design note in `docs/design-docs/sqlite-migrations.md`, and the active `daily` rotation plan to confirm that the migration runner should land as a separate prerequisite.
- [ ] Create embedded migration assets under `migrations/sqlite` plus `migrations/embed.go` and move the current baseline schema into versioned SQL files.
- [ ] Add `internal/store/sqlite/db.go` and `internal/store/sqlite/migrate.go` so startup can open the database, apply pragmas, create `schema_migrations`, and run pending migrations transactionally.
- [ ] Add a bootstrap reconciliation path for legacy databases that were created before `schema_migrations` existed.
- [ ] Update `cmd/39claw/main.go` and the store package so schema setup happens through `Migrate()` before store construction, not through inline schema mutation inside CRUD-oriented code.
- [ ] Replace store tests that depend on `InitSchema()` with migration-aware coverage and keep reopen-oriented persistence tests passing.
- [ ] Run `make test` and `make lint`, then update this plan with evidence and any follow-up work that should be deferred to a later plan.

## Surprises & Discoveries

- Observation: The current production startup path still constructs a store and then calls `InitSchema(ctx)` from `cmd/39claw/main.go`, so the migration refactor needs to change both startup ordering and test helpers rather than only adding new files.
  Evidence: `cmd/39claw/main.go`, `internal/store/sqlite/store.go`

- Observation: The current store already contains one additive migration pattern for task worktree columns and a branch-name backfill, which means the repository already has real legacy-state behavior that must be preserved during bootstrap reconciliation.
  Evidence: `internal/store/sqlite/store.go`

- Observation: The active `daily` generation plan currently assumes it can add `daily_sessions` directly in store initialization. That assumption should be revised only after this plan lands so the later feature can target the new migration foundation rather than the old inline path.
  Evidence: `docs/exec-plans/active/12-daily-clear-generation.md`

## Decision Log

- Decision: Scope this plan to the migration runner and bootstrap path only, and intentionally defer the `daily_sessions` schema plus data rewrite to a later revision of the `daily` generation plan.
  Rationale: The migration runner is a cross-cutting infrastructure change. Landing it first keeps the risky refactor small enough to validate on its own and prevents the first versioned migration set from being tangled together with new user-facing `daily` behavior.
  Date/Author: 2026-04-06 / Codex

- Decision: Keep the migration system repository-local and dependency-light instead of introducing an external migration framework.
  Rationale: 39claw is a small local SQLite application. Embedded SQL files plus a small Go runner are sufficient, easier to audit, and already match the repository's architecture direction.
  Date/Author: 2026-04-06 / Codex

- Decision: Require an explicit bootstrap reconciliation path for legacy databases that lack `schema_migrations`.
  Rationale: Early users may already have local databases created by the inline `InitSchema()` path. A migration runner that only works for fresh databases would strand those users and would not be a complete replacement.
  Date/Author: 2026-04-06 / Codex

## Outcomes & Retrospective

Implementation has not started yet. The intended outcome is a repository where SQLite schema evolution is versioned, repeatable, and observable through dedicated migration history, while existing local databases still upgrade safely. This plan will be complete when fresh databases migrate from embedded SQL, legacy databases bootstrap into the same latest shape, and the startup path no longer relies on CRUD code to alter schema.

## Context and Orientation

39claw is a Go-based Discord bot that stores local routing state in SQLite. Today, the database is opened in `internal/store/sqlite/store.go`, and schema creation happens through `(*Store).InitSchema(ctx)`. That function currently creates the `thread_bindings`, `tasks`, and `active_tasks` tables directly and also performs one manual additive migration by inspecting the `tasks` table and issuing `ALTER TABLE` statements for missing worktree columns. It also backfills empty `branch_name` values to `task/<task_id>`.

The current startup path lives in `cmd/39claw/main.go`. It opens the store with `sqlitestore.Open(cfg.SQLitePath)`, then calls `store.InitSchema(ctx)`, and only then constructs the rest of the application wiring. Tests under `internal/store/sqlite/store_test.go` and some app tests rely on this behavior directly or indirectly.

For this plan, the following terms are important:

A “migration runner” is the small Go entrypoint that loads embedded SQL files, determines which versions are already applied, executes pending versions in order, and records success in a dedicated history table.

A “versioned migration” is one SQL file whose filename begins with a zero-padded integer prefix such as `0001_initial_schema.sql`. The numeric prefix is the migration version. The file is applied once, in order, and then remembered in `schema_migrations`.

A “bootstrap reconciliation” is the one-time upgrade path for a legacy database that already has application tables but does not yet have `schema_migrations`. The runner must inspect the real database shape, create the migration history table, mark already-satisfied versions, and then apply only the missing later versions.

The key repository files for this plan are:

- `cmd/39claw/main.go`
  - the current startup sequence that opens the SQLite store and calls `InitSchema`
- `internal/store/sqlite/store.go`
  - the current database-opening path, schema creation, inline additive migration logic, and CRUD methods
- `internal/store/sqlite/store_test.go`
  - current schema and persistence tests that will need migration-aware updates
- `docs/design-docs/sqlite-migrations.md`
  - the design note that already describes the target migration structure
- `docs/exec-plans/active/12-daily-clear-generation.md`
  - the later feature plan that should be revised only after this migration foundation exists

## Starting State

Begin this plan only after confirming the repository still matches these assumptions:

- `cmd/39claw/main.go` still opens the SQLite store and then calls `InitSchema(ctx)`
- `internal/store/sqlite/store.go` still contains both database opening and inline schema setup
- there is not yet a repository-level `migrations/sqlite` directory with the production schema in versioned SQL files
- there is not yet a production `schema_migrations` table or `Migrate(ctx, db)` function in `internal/store/sqlite`
- `make test` and `make lint` pass before implementation begins

Verify that state with:

    cd /home/filepang/playground/39claw
    make test
    make lint

If the repository has drifted away from that shape, update this ExecPlan first so it remains self-contained and truthful.

## Preconditions

This plan fixes the following implementation choices:

- migrations are embedded in the Go binary under `migrations/sqlite/*.sql`
- migrations are up-only for v1
- each migration version runs in its own transaction
- applied versions are tracked in `schema_migrations(version, applied_at)`
- the SQLite connection pragmas live in a dedicated database-opening helper rather than in ad hoc startup code
- startup must run migrations before the application constructs store-backed services
- existing pre-runner databases must remain upgradeable through a bootstrap reconciliation path
- this plan does not add `daily_sessions` or any new user-facing behavior

## Milestone 1: Introduce embedded migration assets and a fresh-database runner

At the end of this milestone, a fresh empty database should reach the current latest schema entirely through embedded versioned SQL files rather than through inline table creation in `store.go`.

Create a new top-level package `migrations` with `migrations/embed.go` and a new directory `migrations/sqlite`. Add the first migration files that represent the schema already supported by the repository today. The initial sequence should cover:

1. a baseline application schema containing `thread_bindings`, `tasks`, and `active_tasks`
2. the later task worktree metadata additions and the branch-name backfill that older databases need

Implement `internal/store/sqlite/migrate.go` with a `Migrate(ctx context.Context, db *sql.DB) error` function that:

- loads embedded `.sql` files from `migrations/sqlite`
- parses integer versions from the filename prefix
- sorts migrations in ascending version order
- ensures `schema_migrations` exists
- queries applied versions
- runs each pending migration in its own transaction
- records `version` and `applied_at` after the SQL succeeds

Keep this milestone focused on fresh databases first. A newly created SQLite file should migrate all the way to the latest version without relying on any legacy bootstrap special cases yet.

## Milestone 2: Separate database opening from store CRUD and switch startup to `Migrate()`

At the end of this milestone, the startup path should open the database, apply pragmas, run migrations, and only then construct a store that assumes schema already exists.

Add `internal/store/sqlite/db.go` to own:

- path validation
- parent directory creation
- `sql.Open`
- `db.SetMaxOpenConns(1)`
- SQLite pragmas:
  - `PRAGMA foreign_keys = ON`
  - `PRAGMA journal_mode = WAL`
  - `PRAGMA synchronous = NORMAL`
  - `PRAGMA busy_timeout = 5000`

Then update `cmd/39claw/main.go` so startup follows this sequence:

    db, err := sqlitestore.OpenDB(cfg.SQLitePath)
    if err != nil { ... }
    if err := sqlitestore.Migrate(ctx, db); err != nil { ... }
    store := sqlitestore.New(db)

The exact helper names may differ, but the final shape must make the ordering explicit and remove schema mutation responsibilities from startup callers that only want a ready store.

During this milestone, either delete `InitSchema()` or reduce it to a thin compatibility shim used only by tests while the refactor is in progress. By the end of the plan, production code should no longer depend on `InitSchema()`.

## Milestone 3: Add bootstrap reconciliation for legacy databases without `schema_migrations`

At the end of this milestone, a database created by the older inline `InitSchema()` path should upgrade safely into the versioned migration world without losing data and without re-running already-satisfied schema steps in a harmful way.

Implement a narrow bootstrap reconciliation path in `internal/store/sqlite/migrate.go` that runs only when:

- application tables already exist, and
- `schema_migrations` does not yet exist

That reconciliation should:

- create `schema_migrations`
- inspect the legacy schema shape, especially the `tasks` columns
- determine which migration versions are already fully satisfied
- insert those versions into `schema_migrations`
- continue with ordinary pending migration execution for any later versions

Keep this bootstrap logic tightly scoped to the repository's known legacy states. Do not build a generic schema-diff engine. The runner only needs to recognize the database shapes that 39claw itself previously produced.

The bootstrap path must preserve the current task worktree backfill behavior. A legacy database with a `tasks` row whose `branch_name` is empty after the worktree-column version lands should still end up with `task/<task_id>` as its branch name.

## Milestone 4: Replace inline schema tests with migration-focused coverage

At the end of this milestone, test coverage should prove that migrations, bootstrap reconciliation, and persistence all work through the new runner rather than through `InitSchema()`.

Update `internal/store/sqlite/store_test.go` and add any new migration-focused test files needed to cover these scenarios:

- a fresh database migrates to the latest version and has the expected tables
- running `Migrate()` twice is safe
- a legacy pre-runner database bootstraps into the latest schema
- a legacy `tasks` table receives worktree columns and the `branch_name` backfill
- persisted thread bindings and tasks still survive reopening the database after migration

Keep tests concrete. Do not weaken them to “migration ran” assertions only. Query the database or call the store APIs to prove the real persisted shape.

## Milestone 5: Final cleanup and follow-up handoff

At the end of this milestone, the repository should have a clean migration foundation, passing checks, and a clear note that the `daily` generation plan must be revised to target the new runner.

Remove dead inline schema-mutation helpers from `internal/store/sqlite/store.go` once tests no longer need them. Keep store code focused on CRUD and row mapping. If any compatibility shim remains temporarily, document why in this plan's `Decision Log` and `Outcomes & Retrospective`.

When the code is complete, update the active `daily` generation plan in a follow-up change so it no longer instructs contributors to add `daily_sessions` directly through inline startup schema creation. That later revision is outside the scope of this plan, but this plan is not truly successful unless it leaves the repository ready for that next planning step.

## Plan of Work

Start by extracting the current schema knowledge from `internal/store/sqlite/store.go` into explicit migration versions. The first version should capture the original baseline tables. The second version should represent the already-shipped task worktree column additions and the branch-name backfill. Keep the SQL in embedded files and keep the Go runner small and explicit.

Next, split database-opening responsibilities from store CRUD responsibilities. Add a database-opening helper that applies pragmas and returns `*sql.DB`, then add a migration runner that operates on that handle. Update `cmd/39claw/main.go` so it opens the database, migrates it, and then constructs the store from the already-prepared handle. This makes startup ordering obvious and prepares the codebase for later schema changes.

After that, implement the legacy bootstrap path. Reproduce the old database shapes in tests by creating SQLite files manually with the pre-runner schema, then run the new migration entrypoint and assert that the resulting schema and data match the latest expectations. Preserve the existing branch-name backfill behavior because it is the most important previously shipped data repair step.

Finally, clean up the now-redundant inline schema code, rerun the full checks, and record the concrete proof in this plan. Leave the repository in a state where a later plan can add `daily_sessions` as new migration files instead of reopening this infrastructure decision.

## Concrete Steps

Run all commands from `/home/filepang/playground/39claw`.

1. Confirm the baseline repository state before refactoring.

    make test
    make lint

2. Implement the migration asset package and runner, then run focused SQLite tests while iterating.

    go test ./internal/store/sqlite -run 'TestStore|TestMigrate' -v

3. Update the startup path and run the main package plus SQLite-focused coverage.

    go test ./cmd/39claw ./internal/store/sqlite -v

4. Run the full required checks before considering the plan complete.

    make test
    make lint

5. If the user also wants a commit after implementation, stage only the intended files and create an English Conventional Commit message after all checks pass.

Expected command outcomes:

    $ go test ./internal/store/sqlite -run 'TestStore|TestMigrate' -v
    === RUN   TestMigrateFreshDatabase
    --- PASS: TestMigrateFreshDatabase (0.00s)
    === RUN   TestMigrateLegacyDatabaseBootstrap
    --- PASS: TestMigrateLegacyDatabaseBootstrap (0.00s)
    PASS

    $ make lint
    <repository lint command exits with status 0 and no reported violations>

## Validation and Acceptance

Acceptance is behavioral, not merely structural.

For a fresh database:

- starting from a non-existent SQLite path should create the database file
- `Migrate()` should create `schema_migrations`
- the latest application tables should exist after migration
- running `Migrate()` again should not create duplicate `schema_migrations` rows or fail

For a legacy database:

- manually create a database that matches the old inline schema without `schema_migrations`
- insert at least one legacy task row
- run the new migration entrypoint
- observe that the database reaches the latest schema
- observe that task worktree columns exist and legacy data is backfilled as expected

For startup integration:

- `cmd/39claw/main.go` should no longer rely on schema mutation hidden inside store CRUD code
- store-backed tests and reopen tests should still pass through the migrated database path

The required automated proof is:

    cd /home/filepang/playground/39claw
    make test
    make lint

This plan is complete only when both commands pass and the plan's living sections are updated with the actual results.

## Idempotence and Recovery

The migration runner must be safe to call more than once against the same database. Applied versions are tracked in `schema_migrations`, so rerunning startup should skip previously applied versions.

The bootstrap reconciliation path must also be idempotent. If implementation fails midway during development, the safe retry path is:

- delete the temporary test database created by the failing test, or create a fresh temporary one
- fix the migration logic
- rerun the failing focused test

Do not attempt destructive resets on any real user database during implementation. Use temporary SQLite files created inside Go tests to model fresh and legacy states safely.

If a migration version fails in production or during tests, the transaction for that version must roll back and the version must not be inserted into `schema_migrations`. The next retry should resume from the last committed version.

## Artifacts and Notes

Important files that should exist by the end of implementation:

- `migrations/embed.go`
- `migrations/sqlite/0001_initial_schema.sql`
- `migrations/sqlite/0002_task_worktree_metadata.sql`
- `internal/store/sqlite/db.go`
- `internal/store/sqlite/migrate.go`

Important functions or entrypoints that should exist by the end of implementation:

- a database-opening helper in `internal/store/sqlite/db.go`
- `func Migrate(ctx context.Context, db *sql.DB) error` in `internal/store/sqlite/migrate.go`
- a startup path in `cmd/39claw/main.go` that calls the migration runner before constructing the store

Short example of the intended startup shape:

    db, err := sqlitestore.OpenDB(cfg.SQLitePath)
    if err != nil {
        return fmt.Errorf("open sqlite database: %w", err)
    }
    defer db.Close()

    if err := sqlitestore.Migrate(ctx, db); err != nil {
        return fmt.Errorf("migrate sqlite database: %w", err)
    }

    store := sqlitestore.New(db)

## Interfaces and Dependencies

Use only the existing repository dependencies needed for `database/sql`, the pure-Go `modernc.org/sqlite` driver, standard-library `embed`, and standard-library filesystem helpers. Do not add a third-party migration library.

In `migrations/embed.go`, define:

    package migrations

    import "embed"

    //go:embed sqlite/*.sql
    var Files embed.FS

In `internal/store/sqlite/migrate.go`, define a migration record type that carries:

- integer version
- filename
- SQL text

The migration runner must provide or internally use helpers equivalent to:

- `ensureSchemaMigrationsTable`
- `loadAppliedVersions`
- `loadMigrations`
- `applyMigration`
- a legacy bootstrap reconciliation helper for pre-runner databases

In `internal/store/sqlite/store.go`, keep:

- `type Store struct`
- `func New(db *sql.DB) *Store`
- CRUD methods for thread bindings and tasks

By the end of this plan, `store.go` should no longer be the place that defines the repository's schema evolution policy.

Revision note (2026-04-06): Created this plan after narrowing scope away from `daily_sessions`. The user chose to land the migration runner first and to revise the active `daily` generation plan only after that infrastructure exists.
