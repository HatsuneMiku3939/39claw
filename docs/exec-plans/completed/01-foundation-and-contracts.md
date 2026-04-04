# Build the foundation, contracts, and bootstrap path

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, the repository should have a real application skeleton instead of a dummy `hello world` executable. A maintainer should be able to start `39claw`, have it load configuration from environment variables, initialize logging, open SQLite, construct the Codex integration boundary, and start a runtime shell. This plan does not need to deliver complete `daily` or `task` behavior yet. Its job is to freeze the seams that the later plans will build on.

## Progress

- [x] (2026-04-04 15:27Z) Defined this plan from `ARCHITECTURE.md`, `docs/design-docs/implementation-spec.md`, and the current repository state.
- [x] (2026-04-05 03:58Z) Confirmed the baseline still matched the placeholder startup state and reran `make test` plus `make lint`.
- [x] (2026-04-05 04:22Z) Replaced the placeholder `cmd/39claw` executable with real startup wiring.
- [x] (2026-04-05 04:22Z) Added `internal/config` with environment parsing and validation tests.
- [x] (2026-04-05 04:22Z) Added `internal/observe` with `slog` logger construction.
- [x] (2026-04-05 04:22Z) Added the app-layer request and response contracts in `internal/app`.
- [x] (2026-04-05 04:22Z) Added the initial thread-policy and execution-guard seams in `internal/thread`.
- [x] (2026-04-05 04:22Z) Added `internal/store/sqlite` with schema creation and repository-facing interfaces.
- [x] (2026-04-05 04:22Z) Added a higher-level Codex gateway wrapper around the existing `internal/codex` client.
- [x] (2026-04-05 04:28Z) Updated startup-oriented docs once the new bootstrap path existed.

## Surprises & Discoveries

- Observation: The repository already has a tested low-level Codex adapter, so the missing foundation work is around orchestration rather than around raw Codex execution.
  Evidence: `internal/codex/codex_test.go`

- Observation: There is no current package layout for application orchestration, storage, or runtime concerns.
  Evidence: `internal/` currently contains only `internal/codex`

- Observation: The low-level Codex adapter does not expose a separate "create empty remote thread" operation today, so the first remote thread ID still appears during the first turn execution.
  Evidence: `internal/codex/client.go`, `internal/codex/thread.go`

- Observation: A minimal Discord runtime shell is enough to validate dependency injection and graceful shutdown without prematurely pulling Discord SDK details into the app layer.
  Evidence: `internal/runtime/discord/shell.go`

## Decision Log

- Decision: Keep the first plan focused on frozen seams and startup wiring instead of partial end-user features.
  Rationale: Later plans will move faster and break less often if they build on stable contracts.
  Date/Author: 2026-04-04 / Codex

- Decision: Prefer `database/sql` with a small SQLite driver hidden behind `internal/store/sqlite`.
  Rationale: The app layer should not know which SQLite driver was chosen, and the standard library surface is enough for v1.
  Date/Author: 2026-04-04 / Codex

- Decision: Normalize Codex execution around `RunTurn(threadID, prompt)` instead of adding a separate thread-creation call to the first gateway contract.
  Rationale: The current low-level Codex client creates remote thread identity as part of the first executed turn, so the higher-level seam should match the real behavior instead of pretending an empty-thread API already exists.
  Date/Author: 2026-04-05 / Codex

## Outcomes & Retrospective

The outcome of this plan should be a repository that finally has a real startup spine. Even if the bot does not yet answer useful messages, the code should now express where configuration, runtime, storage, application orchestration, and Codex boundaries live.

This outcome is now present in the repository. `cmd/39claw` loads environment configuration, builds a `slog` logger, opens SQLite, initializes schema, constructs the Codex gateway, and runs a minimal Discord runtime shell that starts cleanly and exits cleanly on context cancellation.

## Context and Orientation

The current `cmd/39claw/main.go` returns a greeting string and prints it. That file must be replaced. The design target from `docs/design-docs/implementation-spec.md` requires these packages:

- `internal/config`
- `internal/observe`
- `internal/runtime/discord`
- `internal/app`
- `internal/thread`
- `internal/store/sqlite`
- `internal/codex`

This plan should create the first six boundaries and extend the seventh with a higher-level gateway wrapper. The application contracts that must exist by the end of this plan are:

- `MessageRequest`
- `MessageResponse`
- `ThreadPolicy`
- `ThreadStore`
- `CodexGateway`
- `TaskCommandService`

In this repository, "frozen contracts" means the later plans can depend on these responsibilities without rewriting them every time. Exact Go type names may evolve, but the responsibilities must stay stable.

## Starting State

Begin this plan only after confirming the repository still matches this baseline:

- `cmd/39claw/main.go` is still a placeholder executable
- `internal/` contains only `internal/codex`
- `make test` passes
- `make lint` passes

If the repository has already moved beyond that baseline, do not discard the newer code. Instead, compare the existing implementation against the acceptance section in this plan and treat any already-complete step as done. Update the `Progress` section before making new edits so the plan remains truthful.

## Preconditions

This plan has no earlier ExecPlan prerequisite. It is the first implementation plan in the sequence.

The only required repository knowledge is embedded here:

- `ARCHITECTURE.md` defines the system boundary: Discord integration, thread routing, SQLite persistence, and Codex integration
- `docs/design-docs/implementation-spec.md` fixes the v1 package direction and storage defaults
- `docs/product-specs/discord-command-behavior.md` explains that normal conversation is mention-only and slash commands are explicit control surfaces

If those documents change while this plan is in flight, update this plan to capture the changed assumptions before continuing.

## Plan of Work

Replace `cmd/39claw/main.go` with a real bootstrap path. Add a small `run` function that reads environment variables through `internal/config`, builds a logger through `internal/observe`, opens the SQLite store, ensures the schema exists, constructs the Codex gateway, and initializes a Discord runtime shell. If the runtime shell is not fully implemented yet, it may return a clear not-implemented startup error, but the bootstrap path itself must be real and testable.

Create `internal/config/config.go` and `internal/config/config_test.go`. Parse the environment variables from `docs/design-docs/implementation-spec.md`: `CLAW_MODE`, `CLAW_TIMEZONE`, `CLAW_DISCORD_TOKEN`, `CLAW_CODEX_WORKDIR`, `CLAW_SQLITE_PATH`, and `CLAW_CODEX_EXECUTABLE` as required values, with optional support for `CLAW_CODEX_BASE_URL`, `CLAW_CODEX_API_KEY`, and `CLAW_LOG_LEVEL`. Validate that mode is `daily` or `task`, timezone loads successfully, and required paths are not empty.

Create `internal/observe/logger.go` with a function that returns `*slog.Logger` using the configured log level. Keep the logging package minimal and avoid introducing a broad observability framework.

Create `internal/app/types.go` and define request and response structs that contain application-level information only. Create service-oriented files such as `internal/app/message_service.go` and `internal/app/task_service.go` for the interfaces that later plans will implement. Do not import `discordgo` into this package.

Create `internal/thread/policy.go` and `internal/thread/guard.go`. The policy file should define the interface for resolving logical thread keys. The guard file should define an in-memory per-key execution guard that can later reject concurrent turns for the same logical key.

Create `internal/store/sqlite/store.go` and `internal/store/sqlite/store_test.go`. Use SQLite as required, create the schema idempotently, and define repository-facing methods for thread bindings, tasks, and active tasks even if some task methods are not fully exercised until the next plan.

Create `internal/codex/gateway.go` with a higher-level wrapper that uses the existing `Client` and `Thread` types. The gateway should normalize the result of a Codex turn into application-friendly fields such as final response text and thread ID.

Do not implement full Discord behavior in this plan. The runtime shell only needs enough structure to prove startup wiring, dependency injection, and graceful shutdown. A small interface plus a no-op or minimal runtime implementation is acceptable if it allows the process to start cleanly and keeps the later Discord-specific work isolated.

## Concrete Steps

Run all commands from `/home/filepang/playground/39claw`.

1. Confirm the baseline before starting.

    make test
    make lint

2. Implement the new foundation packages and bootstrap wiring.

3. Run focused tests while iterating.

    go test ./cmd/39claw ./internal/config ./internal/observe ./internal/app ./internal/thread ./internal/store/sqlite ./internal/codex

4. Run the full repository checks after the plan lands.

    make test
    make lint

5. Perform one startup validation with missing environment variables and expect a clear error instead of a panic.

6. Record what exists at the end of the plan so later contributors can quickly verify the expected starting state for the next plan:

    find internal -maxdepth 2 -type f | sort

## Validation and Acceptance

This plan is complete when:

- `cmd/39claw` no longer prints a placeholder greeting
- `internal/config` validates required environment variables and timezone loading with tests
- `internal/observe` builds a `slog` logger from the configured level
- `internal/app` exposes stable request, response, and service contracts without Discord SDK types
- `internal/store/sqlite` creates the required tables idempotently
- `internal/codex/gateway.go` exists and wraps the low-level Codex adapter in application-friendly behavior
- `make test` passes
- `make lint` passes

The next plan should be able to assume these repository facts without inventing them:

- `cmd/39claw` has a real `run` path
- configuration parsing and logger construction exist
- SQLite schema initialization exists
- app-layer request and response contracts exist
- a Codex gateway interface and implementation exist

## Idempotence and Recovery

Schema creation must use `CREATE TABLE IF NOT EXISTS` statements so the initialization path can be rerun safely. If startup wiring fails midway, recovery should only require fixing the code and rerunning `make test`; no manual state cleanup should be required for a fresh SQLite file used during development.

If you open this plan and the repository is missing one of the acceptance items, implement the missing piece here rather than jumping ahead to a later plan. This plan is the recovery point for all missing foundation work.

## Artifacts and Notes

Keep this schema excerpt aligned with the implementation:

    CREATE TABLE IF NOT EXISTS thread_bindings (
        mode TEXT NOT NULL,
        logical_thread_key TEXT NOT NULL,
        codex_thread_id TEXT NOT NULL,
        task_id TEXT NULL,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        PRIMARY KEY (mode, logical_thread_key)
    );

    CREATE TABLE IF NOT EXISTS tasks (
        task_id TEXT PRIMARY KEY,
        discord_user_id TEXT NOT NULL,
        task_name TEXT NOT NULL,
        status TEXT NOT NULL,
        created_at TEXT NOT NULL,
        updated_at TEXT NOT NULL,
        closed_at TEXT NULL
    );

    CREATE TABLE IF NOT EXISTS active_tasks (
        discord_user_id TEXT PRIMARY KEY,
        task_id TEXT NOT NULL,
        updated_at TEXT NOT NULL
    );

## Interfaces and Dependencies

At the end of this plan, the repository should contain interfaces shaped like these examples:

    type MessageRequest struct {
        UserID      string
        ChannelID   string
        MessageID   string
        Content     string
        Mentioned   bool
        CommandName string
        CommandArgs []string
        ReceivedAt  time.Time
    }

    type MessageResponse struct {
        Text      string
        ReplyToID string
        Ephemeral bool
        Ignore    bool
    }

    type ThreadPolicy interface {
        ResolveMessageKey(ctx context.Context, request MessageRequest) (string, error)
    }

    type CodexGateway interface {
        RunTurn(ctx context.Context, threadID string, prompt string) (RunTurnResult, error)
    }

Revision Note: 2026-04-04 / Codex - Created this smaller child ExecPlan during the split of the original all-in-one runtime plan.
Revision Note: 2026-04-04 / Codex - Removed the master-plan dependency and expanded this document with explicit starting-state and recovery guidance so it can stand alone.
Revision Note: 2026-04-05 / Codex - Moved this fully completed ExecPlan from `active/` to `completed/` during ExecPlan index cleanup.
