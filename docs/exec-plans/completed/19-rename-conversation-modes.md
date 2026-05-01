# Rename conversation modes to journal and thread

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, a 39claw operator configures bot instances with `CLAW_MODE=journal` for the shared day-based assistant flow and `CLAW_MODE=thread` for the explicit task-oriented repository flow. Users see those names consistently in help text, examples, product documentation, and runtime errors.

This is intentionally a breaking rename. Deployments that still set `CLAW_MODE=daily` or `CLAW_MODE=task` fail fast with the normal unsupported-mode configuration error until operators update their environment files. Existing SQLite rows keyed by old mode values are not migrated; the renamed runtime reads and writes bindings under the new canonical mode names.

## Progress

- [x] (2026-04-29 22:53Z) Reset the previous unplanned implementation attempt back to `HEAD` and confirmed the working tree was clean before writing this plan.
- [x] (2026-04-29 22:53Z) Reviewed `.agents/PLANS.md`, the current ExecPlan index, and the current code/documentation references for `daily` and `task` mode.
- [x] (2026-05-01 08:40Z) Implemented canonical `journal` and `thread` mode values and added config coverage proving legacy `daily` and `task` values are rejected.
- [x] (2026-05-01 08:40Z) Confirmed no SQLite compatibility migration was added for old `thread_bindings.mode` values.
- [x] (2026-05-01 08:40Z) Updated user-facing runtime text, generated journal-memory guidance, documentation, and examples to use `journal` and `thread`.
- [x] (2026-05-01 08:40Z) Ran repository validation with `make test`; it passed.
- [x] (2026-05-01 08:42Z) Ran repository validation with `make lint`; it passed.
- [x] (2026-05-01 08:40Z) Confirmed the repository has no separate `e2e` Makefile target.

## Surprises & Discoveries

- Observation: The word `task` has two meanings in the repository: it was previously a mode name, and it is also the durable work item inside the repository-oriented flow.
  Evidence: `internal/app/task_service.go`, `internal/app/task_workspace.go`, the `tasks` SQLite table, `action:task-*` commands, and the `task:<name>` override all model user-created work items.

- Observation: The word `daily` is used both as the old mode name and as the implementation name for date-based generation state.
  Evidence: `migrations/sqlite/0003_daily_sessions.sql`, `internal/app/daily_sessions.go`, and `internal/store/sqlite/daily_sessions.go` store and manipulate date-scoped generation metadata.

- Observation: Completed ExecPlans and historical migration documents intentionally describe features by their original names.
  Evidence: completed plans such as `02-daily-mode-routing.md`, `03-task-mode-workflow.md`, and migration notes about legacy `daily` logical keys remain historical records.

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

## Outcomes & Retrospective

The rename landed as a breaking change. Runtime configuration now accepts only `journal` and `thread`; `daily` and `task` are rejected. Thread bindings, scheduler run mode fields, Discord help output, startup validation errors, generated memory-refresh skill paths, examples, and current documentation now use the new canonical mode names.

No compatibility migration was added. Historical `daily` or `task` rows in `thread_bindings` remain untouched and unused by the renamed runtime. The implementation also keeps the task work-item vocabulary unchanged, including `action:task-*`, `task:<name>`, task branch names, and task storage tables.

## Context and Orientation

39claw is a Go Discord bot that routes Discord messages into Codex conversation threads. A "mode" is the global routing policy chosen by `CLAW_MODE` for one bot process. The two modes are now:

- `journal`, a shared day-based flow where messages route to one active generation for the local date and `action:clear` can rotate that same-day generation.
- `thread`, a repository work flow where a user selects a durable task, and each task can have its own Codex thread and Git worktree.

The thread-oriented mode still contains "tasks" as user-created work items. A task is a durable work stream stored in SQLite and controlled through `action:task-current`, `action:task-list`, `action:task-new`, `action:task-switch`, `action:task-close`, and `action:task-reset-context`.

## Completed Work

Changed `internal/config/config.go` so `ModeJournal` has value `journal`, `ModeThread` has value `thread`, and `parseMode` accepts only those values. `ValidateRuntimePaths` now reports thread-mode Git workdir requirements.

Updated mode branches across startup, thread policy, message execution, scheduled task execution, task context reset, and Discord runtime command registration. User-facing runtime strings now say journal mode or thread mode where they refer to the configured mode.

Updated the date-based memory bridge path from `.agents/skills/39claw-daily-memory-refresh/SKILL.md` to `.agents/skills/39claw-journal-memory-refresh/SKILL.md`, plus generated skill frontmatter, headings, prompts, bridge-note headings, and thread binding lookup mode.

Renamed current user-facing docs and examples:

- `docs/product-specs/daily-mode-user-flow.md` to `docs/product-specs/journal-mode-user-flow.md`
- `docs/product-specs/task-mode-user-flow.md` to `docs/product-specs/thread-mode-user-flow.md`
- `docs/design-docs/task-mode-worktrees.md` to `docs/design-docs/thread-mode-worktrees.md`
- `example/daily-obsidian-vault.md` to `example/journal-obsidian-vault.md`
- `example/task-repository.md` to `example/thread-repository.md`
- matching example `.env.local.sample` files

Updated `README.md`, `AGENTS.md`, `ARCHITECTURE.md`, documentation indexes, current active ExecPlans, and product/design docs so current behavior points to `journal` and `thread`.

## Validation and Acceptance

Automated validation:

    make test
    make lint

Result:

    ok   github.com/HatsuneMiku3939/39claw/cmd/39claw
    ok   github.com/HatsuneMiku3939/39claw/internal/app
    ok   github.com/HatsuneMiku3939/39claw/internal/config
    ok   github.com/HatsuneMiku3939/39claw/internal/dailymemory
    ok   github.com/HatsuneMiku3939/39claw/internal/runtime/discord
    ok   github.com/HatsuneMiku3939/39claw/internal/store/sqlite
    ok   github.com/HatsuneMiku3939/39claw/internal/thread
    0 issues.
    Linting passed

The repository `Makefile` exposes `test`, `lint`, `release-check`, and `release-snapshot`; it does not expose a separate e2e target.

Acceptance coverage now proves:

- `CLAW_MODE=journal` loads `config.ModeJournal`.
- `CLAW_MODE=thread` loads `config.ModeThread`.
- `CLAW_MODE=daily` is rejected with `unsupported CLAW_MODE "daily"`.
- `CLAW_MODE=task` is rejected with `unsupported CLAW_MODE "task"`.
- thread-mode path validation errors say "thread mode".
- no migration was added solely to rewrite old `thread_bindings.mode` values.
- journal-mode help output says `Mode: journal` and exposes `action:clear`.
- thread-mode help output says `Mode: thread` and exposes task actions.
- `action:task-reset-context` deletes the `thread`-mode binding without changing task metadata.

## Idempotence and Recovery

Because this is a breaking rename, there is no mode-name recovery path for old environment values. If an operator upgrades the binary but leaves `CLAW_MODE=daily` or `CLAW_MODE=task`, startup fails with the unsupported-mode error. The recovery is to edit the environment file to `CLAW_MODE=journal` or `CLAW_MODE=thread` and restart.

Because this plan does not migrate old mode values in `thread_bindings`, an upgraded bot may start fresh Codex conversation continuity under the new canonical mode names. That is acceptable for this plan. If preserving old thread IDs later becomes important again, write a separate explicit migration plan instead of reintroducing compatibility silently.

## Artifacts and Notes

Final search command:

    rg -n 'ModeDaily|ModeTask|CLAW_MODE=(daily|task)|daily mode|task mode|daily-mode|task-mode|Mode: daily|Mode: task|39claw-daily-memory-refresh' --glob '!docs/exec-plans/completed/**'

Expected remaining hits outside this completed plan are intentional legacy rejection tests, task-domain concepts, date-based storage implementation names, or historical notes.

Revision Note: 2026-04-29 22:53Z / Codex - Created this active ExecPlan after resetting the earlier direct implementation attempt.

Revision Note: 2026-04-30 04:58Z / Codex - Reframed the plan after the user decided backward compatibility can be dropped.

Revision Note: 2026-05-01 08:40Z / Codex - Implemented the breaking rename, recorded validation, and prepared the plan for completion.
