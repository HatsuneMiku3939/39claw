# Rename conversation modes to journal and thread

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, a 39claw operator should configure bot instances with `CLAW_MODE=journal` for the shared day-based assistant flow and `CLAW_MODE=thread` for the explicit task-oriented repository flow. Users should see those names consistently in help text, examples, product documentation, and runtime errors. This is intentionally a breaking rename: deployments that still set `CLAW_MODE=daily` or `CLAW_MODE=task` should fail fast with the normal unsupported-mode configuration error until operators update their environment files.

The user-visible proof is concrete. A contributor should be able to load configuration with `CLAW_MODE=journal` and see help output report `Mode: journal`; load configuration with `CLAW_MODE=thread` and see task actions exposed; and load configuration with `CLAW_MODE=daily` or `CLAW_MODE=task` and observe an explicit unsupported-mode error. Existing SQLite rows keyed by the old mode values are not migrated by this plan; a bot started under the new names creates or uses bindings under the new canonical mode names.

## Progress

- [x] (2026-04-29 22:53Z) Reset the previous unplanned implementation attempt back to `HEAD` and confirmed the working tree was clean before writing this plan.
- [x] (2026-04-29 22:53Z) Reviewed `.agents/PLANS.md`, the current ExecPlan index, and the current code/documentation references for `daily` and `task` mode.
- [ ] Implement canonical `journal` and `thread` mode values and reject `daily` and `task` as unsupported configuration values.
- [ ] Confirm no SQLite compatibility migration is added for old `thread_bindings.mode` values.
- [ ] Update user-facing runtime text, generated journal-memory guidance, documentation, and examples to use `journal` and `thread`.
- [ ] Run the repository validation commands and record the exact results in this plan.

## Surprises & Discoveries

- Observation: The word `task` has two meanings in the repository: it is currently a mode name, and it is also the durable work item inside the repository-oriented flow.
  Evidence: `internal/config/config.go` defines `ModeTask`, while `internal/app/task_service.go`, `internal/app/task_workspace.go`, the `tasks` SQLite table, `action:task-*` commands, and the `task:<name>` override all model user-created work items.

- Observation: The word `daily` is used both as a mode name and as the implementation name for date-based generation state.
  Evidence: `migrations/sqlite/0003_daily_sessions.sql`, `internal/app/daily_sessions.go`, and `internal/store/sqlite/daily_sessions.go` store and manipulate date-scoped generation metadata.

- Observation: Completed ExecPlans and historical migration documents intentionally describe features by their original names.
  Evidence: `docs/exec-plans/index.md` links completed plans such as `02-daily-mode-routing.md` and `03-task-mode-workflow.md`.

## Decision Log

- Decision: Make `journal` and `thread` the only accepted mode configuration values.
  Rationale: The user explicitly chose to drop backward compatibility. Rejecting `daily` and `task` avoids carrying alias logic and makes misconfigured deployments fail clearly instead of silently running under renamed behavior.
  Date/Author: 2026-04-30 / Codex

- Decision: Keep the task entity, task commands, task table, task branch prefix, and `task:<name>` one-shot override named `task`.
  Rationale: In the new naming, `thread` is the mode and `task` remains the work item selected inside that mode. Renaming both at once would make Discord commands and database state churn far beyond the user's requested mode-name change.
  Date/Author: 2026-04-29 / Codex

- Decision: Keep the existing `daily_sessions` SQLite table and closely related internal data shape, while updating user-facing language to say journal where the mode is being described.
  Rationale: The table stores local-date generation state and does not expose the configured mode value directly. Renaming the table would require a heavier schema migration without improving user-visible clarity.
  Date/Author: 2026-04-29 / Codex

- Decision: Do not migrate old `thread_bindings.mode` values from `daily` or `task`.
  Rationale: Dropping backward compatibility means old conversation continuity can be abandoned. Avoiding a compatibility migration keeps the implementation smaller and makes the new mode names the only persisted values created after the rename.
  Date/Author: 2026-04-30 / Codex

- Decision: Leave completed ExecPlan files historically named unless an active or current index entry must describe current behavior.
  Rationale: Completed plans are historical records. Current docs, active plans, examples, and runtime behavior should use the new names; old completed plan titles can remain accurate descriptions of the past implementation.
  Date/Author: 2026-04-29 / Codex

## Outcomes & Retrospective

No implementation has landed yet. The immediate outcome is a clean working tree plus this active plan, so the rename can be implemented deliberately instead of as an unplanned broad search-and-replace. The plan has now been reframed as an intentional breaking rename with no legacy aliases or persisted binding migration. Update this section after each major milestone with what changed, what was validated, and any remaining gaps.

## Context and Orientation

39claw is a Go Discord bot that routes Discord messages into Codex conversation threads. A "mode" is the global routing policy chosen by `CLAW_MODE` for one bot process. Today there are two modes:

- `daily`, a shared day-based flow where messages route to one active generation for the local date and `action:clear` can rotate that same-day generation.
- `task`, a repository work flow where a user selects a durable task, and each task can have its own Codex thread and Git worktree.

The requested rename changes those mode names to:

- `journal`, replacing `daily`.
- `thread`, replacing `task`.

The task-oriented mode still contains "tasks" as user-created work items. A task is a durable work stream stored in SQLite and controlled through `action:task-current`, `action:task-list`, `action:task-new`, `action:task-switch`, `action:task-close`, and `action:task-reset-context`. This plan does not rename those task concepts.

The key code locations are:

- `internal/config/config.go`, which defines `type Mode`, the current `ModeDaily` and `ModeTask` constants, `parseMode`, and `ValidateRuntimePaths`.
- `internal/thread/policy.go`, which resolves logical thread keys differently for daily and task mode.
- `internal/app/message_service_impl.go`, which branches on mode to parse `task:<name>` overrides, resolve daily sessions, prepare task worktrees, refresh memory, load thread bindings, and run Codex.
- `internal/app/daily_command_service.go`, which implements `action:clear` for the date-based mode.
- `internal/app/task_service.go`, which implements task commands and uses the task-mode thread binding key for reset.
- `internal/app/scheduled_task_service.go`, which uses the mode to decide whether scheduled runs use `CLAW_CODEX_WORKDIR` directly or a temporary task worktree.
- `internal/runtime/discord/commands.go` and `internal/runtime/discord/runtime.go`, which expose mode-specific slash-command choices and help/error text.
- `internal/dailymemory/bootstrap.go` and `internal/dailymemory/service.go`, which generate and invoke the memory-refresh instructions used by the date-based mode.
- `internal/store/sqlite/store.go` and `migrations/sqlite/*.sql`, which persist thread bindings and migration state.

The key current documentation and example locations are:

- `ARCHITECTURE.md`
- `README.md`
- `AGENTS.md`
- `docs/design-docs/thread-modes.md`
- `docs/design-docs/architecture-overview.md`
- `docs/design-docs/implementation-spec.md`
- `docs/design-docs/state-and-storage.md`
- `docs/design-docs/task-mode-worktrees.md`
- `docs/product-specs/daily-mode-user-flow.md`
- `docs/product-specs/task-mode-user-flow.md`
- `docs/product-specs/discord-command-behavior.md`
- `docs/product-specs/scheduled-tasks-user-flow.md`
- `example/daily-obsidian-vault.md`
- `example/task-repository.md`
- the matching `.env.local.sample` files under `example/`

## Plan of Work

First, change the canonical mode values in `internal/config/config.go`. Replace `ModeDaily` with `ModeJournal` whose value is `journal`, and replace `ModeTask` with `ModeThread` whose value is `thread`. Update `parseMode` so only `journal` returns `ModeJournal` and only `thread` returns `ModeThread`; `daily`, `task`, and any other value should return `unsupported CLAW_MODE`. Update runtime validation errors to say "thread mode" when requiring a Git repository workdir.

Next, update all mode branches in Go code to use the new constant names. This includes `cmd/39claw/main.go`, `internal/thread/policy.go`, `internal/app/message_service_impl.go`, `internal/app/daily_command_service.go`, `internal/app/task_service.go`, `internal/app/scheduled_task_service.go`, `internal/runtime/discord/commands.go`, and `internal/runtime/discord/runtime.go`. User-facing strings should say journal mode or thread mode where they refer to the configured mode. Strings that refer to user-created tasks should keep saying task.

Do not add a SQLite migration for old `thread_bindings.mode` values. Existing rows with `mode = 'daily'` or `mode = 'task'` may remain in the database as historical rows, but the renamed runtime should read and write only `journal` or `thread` bindings. Do not rename the `daily_sessions`, `tasks`, or `active_tasks` tables.

Update tests. Add table-driven tests in `internal/config/config_test.go` showing that `journal` and `thread` load as canonical modes and that `daily` and `task` are rejected with the unsupported-mode error. Update runtime, app, policy, scheduler, and store tests to expect `journal` and `thread` in persisted `ThreadBinding.Mode`, scheduled run mode strings, help output, and error messages. Update migration-count tests only if another migration is needed for a different reason; this plan should not increase the migration version just to preserve old mode names.

Then, update the date-based memory bridge language. In `internal/dailymemory/bootstrap.go`, rename the generated managed skill path from `.agents/skills/39claw-daily-memory-refresh/SKILL.md` to `.agents/skills/39claw-journal-memory-refresh/SKILL.md`, update the generated skill frontmatter and headings to say journal, and update `internal/dailymemory/service.go` so the refresh prompt points at the new skill path and looks up thread bindings under mode `journal`. Existing old generated skill files in user workdirs can remain harmlessly unused.

Finally, update documentation and examples. Rename current user-facing docs and examples from daily/task mode names to journal/thread mode names where appropriate:

- `docs/product-specs/daily-mode-user-flow.md` to `docs/product-specs/journal-mode-user-flow.md`
- `docs/product-specs/task-mode-user-flow.md` to `docs/product-specs/thread-mode-user-flow.md`
- `docs/design-docs/task-mode-worktrees.md` to `docs/design-docs/thread-mode-worktrees.md`
- `example/daily-obsidian-vault.md` to `example/journal-obsidian-vault.md`
- `example/task-repository.md` to `example/thread-repository.md`
- the matching example `.env.local.sample` files

Update `README.md`, `AGENTS.md`, `ARCHITECTURE.md`, `docs/index.md`, `docs/design-docs/index.md`, `docs/product-specs/index.md`, and `example/README.md` so links point to the new file names and the documented `CLAW_MODE` values are `journal` and `thread`. Current active ExecPlans should describe current mode names if they mention runtime behavior. Completed ExecPlans may remain historically named.

## Concrete Steps

Work from the repository root:

    cd /home/filepang/.local/share/39claw/39claw/worktrees/01KQDP6C0QHEWBP0098TVPJ5Q3

Before editing, confirm the tree is clean or only contains intentional plan edits:

    git status --short

Perform code changes with small patches or clearly scoped mechanical renames. Use `rg` to inspect remaining references after each major step:

    rg -n 'ModeDaily|ModeTask|CLAW_MODE=(daily|task)|daily mode|task mode|daily-mode|task-mode|Mode: daily|Mode: task|39claw-daily-memory-refresh' --glob '!docs/exec-plans/completed/**'

Expected output after implementation should include only intentional breaking-change tests that assert old config values are rejected, task-domain concepts such as `task:<name>`, date-based storage implementation names, or historical completed-plan links. It should not show current runtime text, current example setup values, current product spec links, compatibility alias tests, migration tests for old mode values, or generated journal-memory skill paths using old mode names.

When renaming tracked documentation files, use `git mv` so Git records the rename clearly:

    git mv docs/product-specs/daily-mode-user-flow.md docs/product-specs/journal-mode-user-flow.md
    git mv docs/product-specs/task-mode-user-flow.md docs/product-specs/thread-mode-user-flow.md
    git mv docs/design-docs/task-mode-worktrees.md docs/design-docs/thread-mode-worktrees.md
    git mv example/daily-obsidian-vault.md example/journal-obsidian-vault.md
    git mv example/task-repository.md example/thread-repository.md

Run Go formatting before validation:

    gofmt -w $(rg --files -g '*.go')

Run the repository checks:

    make test
    make lint

The expected successful test transcript is shaped like:

    ok   github.com/HatsuneMiku3939/39claw/cmd/39claw
    ok   github.com/HatsuneMiku3939/39claw/internal/app
    ok   github.com/HatsuneMiku3939/39claw/internal/config
    ok   github.com/HatsuneMiku3939/39claw/internal/store/sqlite
    ...
    Linting passed

If `make lint` installs `golangci-lint` into `.tools`, that is expected and `.tools` should remain ignored.

## Validation and Acceptance

Automated validation must include `make test` and `make lint` from the repository root. This repository's `Makefile` currently exposes `test`, `lint`, `release-check`, and `release-snapshot`; it does not expose a separate e2e target. If no e2e target exists at implementation time, record that fact in this plan and rely on the focused runtime tests plus the manual smoke steps below.

The config tests must prove:

- `CLAW_MODE=journal` loads `config.ModeJournal`.
- `CLAW_MODE=thread` loads `config.ModeThread`.
- `CLAW_MODE=daily` is rejected with `unsupported CLAW_MODE "daily"`.
- `CLAW_MODE=task` is rejected with `unsupported CLAW_MODE "task"`.
- thread-mode path validation errors say "thread mode".

The migration tests must prove:

- A fresh database still applies the expected current migration versions.
- Re-running `Migrate` is idempotent.
- No compatibility migration is added solely to rewrite `thread_bindings.mode` values from `daily` or `task`.

The Discord/runtime tests must prove:

- In journal mode, help output says `Mode: journal` and exposes `action:clear`.
- In thread mode, help output says `Mode: thread` and exposes task actions.
- Task actions are unavailable in journal mode with a journal-mode message.
- `action:task-reset-context` still uses the active task and deletes the `thread`-mode binding without changing task metadata.

Manual smoke validation, if a Discord test environment is available, should use two separate bot instances:

1. Start one instance with `CLAW_MODE=journal` and a writable non-Git knowledge directory. Run `/<command> action:help` and observe `Mode: journal`. Mention the bot and observe a normal reply. Run `/<command> action:clear` while idle and observe a fresh shared journal generation confirmation.
2. Start one instance with `CLAW_MODE=thread` and a Git repository workdir with an `origin` remote. Run `/<command> action:help` and observe `Mode: thread` plus task actions. Run `/<command> action:task-new task_name:smoke-test`, mention the bot, and observe that the first task message prepares a task worktree and replies.

## Idempotence and Recovery

Because this is a breaking rename, there is no mode-name recovery path for old environment values. If an operator upgrades the binary but leaves `CLAW_MODE=daily` or `CLAW_MODE=task`, startup should fail with the unsupported-mode error. The recovery is to edit the environment file to `CLAW_MODE=journal` or `CLAW_MODE=thread` and restart.

Because this plan does not migrate old mode values in `thread_bindings`, an upgraded bot may start fresh Codex conversation continuity under the new canonical mode names. That is acceptable for this plan. If preserving old thread IDs later becomes important again, write a separate explicit migration plan instead of reintroducing compatibility silently.

If implementation introduces too much churn from renaming internal structs like `DailySession`, stop and update this plan before continuing. The accepted scope is mode naming, persisted mode values, user-facing text, docs, examples, and tests. Internal names that describe date-based generation storage may remain when renaming them would add schema or API churn without user-visible benefit.

To undo an incomplete implementation before it is committed, inspect `git status --short`, then use targeted `git restore <path>` for files touched by this plan. Use `git clean -fd <path>` only for untracked files created by this plan, such as an abandoned migration file, and only after confirming they are not user work.

## Artifacts and Notes

The initial repository scan before this plan found current mode references in these areas:

    internal/config/config.go: ModeDaily = "daily", ModeTask = "task"
    cmd/39claw/main.go: startup branches on config.ModeDaily and config.ModeTask
    internal/runtime/discord/commands.go: help text says daily-mode and task-mode
    migrations/sqlite/0003_daily_sessions.sql: backfills legacy daily keys
    README.md and example/*.sample: document CLAW_MODE=daily and CLAW_MODE=task

After implementation, a short final search transcript should be added here showing that current docs and runtime code use journal/thread and that remaining daily/task hits are either rejection tests for old config values, task-domain concepts, date-based storage implementation names, or completed historical records.

Revision Note: 2026-04-29 22:53Z / Codex - Created this active ExecPlan after resetting the earlier direct implementation attempt. The plan intentionally separates the mode rename from the task entity so the next implementation can be smaller, safer, and easier to review.

Revision Note: 2026-04-30 04:58Z / Codex - Reframed the plan after the user decided backward compatibility can be dropped. The implementation should reject old `daily` and `task` mode values and should not add a compatibility migration for old `thread_bindings.mode` rows.
