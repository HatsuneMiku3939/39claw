# Implement `task` mode task workflow and command orchestration

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, `task` mode should support explicit, durable work streams. A user should be able to create a task, switch tasks, list open tasks, close a task, and continue a task across days without losing context. Normal messages in `task` mode must not reach Codex unless the user has an active task, and the missing-context response must point the user toward the `/task ...` workflow.

## Progress

- [x] (2026-04-04 15:27Z) Defined the `task` mode plan and its acceptance targets.
- [x] (2026-04-04 17:12Z) Confirmed the required starting state with `make test` and `make lint`.
- [x] (2026-04-04 17:12Z) Verified SQLite already provided persistent task records and active-task state, then extended tests to prove reopen and close behavior.
- [x] (2026-04-04 17:12Z) Implemented `TaskCommandService` for `/task`, `/task list`, `/task new <name>`, `/task switch <id>`, and `/task close <id>` in the app layer.
- [x] (2026-04-04 17:12Z) Kept missing-active-task guidance on the normal mention path and ensured task-mode bindings persist the active `task_id`.
- [x] (2026-04-04 17:12Z) Added task-mode routing tests that prove logical keys stay stable across days and task switches.
- [x] (2026-04-04 17:12Z) Added task lifecycle, active-task, reopen, and guidance coverage, then reran `make test` and `make lint`.

## Surprises & Discoveries

- Observation: `task` mode shares the same Codex gateway and thread-binding machinery as `daily` mode, but it introduces extra user-scoped state that must remain correct under closure and switching.
  Evidence: `docs/design-docs/implementation-spec.md`

- Observation: Most of the required SQLite behavior already existed before this plan started, so the main missing slice was application orchestration plus stronger proof around restart and inactive-close cases.
  Evidence: `internal/store/sqlite/store.go`, `internal/store/sqlite/store_test.go`

- Observation: The normal-message service needed one extra task-mode read after logical-key resolution so persisted task bindings also record `task_id` rather than only the derived `logical_thread_key`.
  Evidence: `internal/app/message_service_impl.go`

## Decision Log

- Decision: Keep active-task state scoped to one Discord user within one bot instance.
  Rationale: This is the fixed v1 behavior in the implementation spec and product specs.
  Date/Author: 2026-04-04 / Codex

- Decision: Use ULID strings for `task_id`.
  Rationale: The implementation spec fixes ULID as the v1 identifier format and it keeps task IDs sortable and copyable.
  Date/Author: 2026-04-04 / Codex

- Decision: Make the app-layer task command responses ephemeral by default.
  Rationale: The implementation spec fixes task-control command responses as ephemeral, and returning that hint from the service keeps the runtime thin later.
  Date/Author: 2026-04-04 / Codex

- Decision: Keep user-facing task command failures inside `MessageResponse` for expected cases such as missing IDs, unknown tasks, and closed tasks.
  Rationale: These cases are product-level workflow guidance, not infrastructure failures, so the app layer should return actionable command text instead of surfacing internal errors.
  Date/Author: 2026-04-04 / Codex

## Outcomes & Retrospective

This plan now lands the durable `task` workflow in the application and persistence layers. The repository can create, inspect, list, switch, and close user-scoped tasks; task-mode normal mentions refuse to route without an active task; and task thread bindings stay stable across day boundaries and process restart. The remaining Discord slash-command wiring still belongs to the next runtime plan, but that runtime work can now call into finished app-layer services instead of inventing task behavior itself.

## Context and Orientation

The relevant behavior definitions live in `docs/product-specs/task-mode-user-flow.md` and `docs/product-specs/discord-command-behavior.md`.

In `task` mode, the logical thread key is `discord_user_id + task_id`. The user-facing task identity is the ULID `task_id` plus the human-friendly `task_name`. The active task must persist in SQLite so a later normal message can continue the same work stream without the user reselecting it after every restart.

Task command responses should be written in application terms here. Whether those responses are presented as Discord ephemeral messages belongs to the later runtime plan.

## Starting State

Start this plan only after confirming the repository provides all of the following capabilities:

- a real startup path in `cmd/39claw`
- stable app-layer request and response contracts
- SQLite schema creation and thread-binding persistence
- a Codex gateway wrapper that can create or resume turns
- `daily` mode normal-message orchestration and tests

Verify that state with:

    make test
    make lint

If the repository is missing the foundation items, implement them first. If the repository is missing only the `daily` mode behavior, complete that behavior before adding `task` mode. This plan assumes those pieces exist so it can focus on explicit task context instead of rebuilding the first normal-message slice.

## Preconditions

This document is self-contained. The facts you need are repeated here:

- task identity is user-scoped within one bot instance
- task IDs are ULID strings
- open and closed are the only v1 task statuses
- closing an active task must remove the active-task mapping
- normal messages in `task` mode must not route unless an active task exists

## Plan of Work

Use the existing SQLite task and active-task operations in `internal/store/sqlite/store.go` as the persistence base for this plan, then strengthen their proof with reopen and inactive-close tests. Implement the missing app-layer orchestration in `internal/app/task_service.go` so task commands can return normalized `MessageResponse` values without Discord SDK types. Update `internal/app/message_service_impl.go` so `task` mode still shares the same orchestration path as `daily`, while also persisting the active `task_id` onto thread bindings. Add message-service and store tests that prove the same task keeps its Codex thread across multiple days, that task switches create distinct logical bindings, and that close behavior preserves or clears the active mapping correctly.

## Concrete Steps

Run all commands from `/home/filepang/playground/39claw`.

1. Confirm the repository matches the required starting state.

    make test
    make lint

    Observed result:

        all Go tests passed
        lint passed with 0 issues

2. Implement task storage proof and command orchestration in:

    - `internal/app/task_service.go`
    - `internal/app/message_service_impl.go`
    - `internal/app/task_service_test.go`
    - `internal/app/message_service_test.go`
    - `internal/store/sqlite/store_test.go`
    - `README.md`

3. Run focused tests while iterating.

    go test ./internal/app ./internal/store/sqlite -run 'Test(TaskCommandService|MessageServiceHandleMessageTask|StoreTask|StoreCloseTask)'

    Observed result:

        ok   github.com/HatsuneMiku3939/39claw/internal/app
        ok   github.com/HatsuneMiku3939/39claw/internal/store/sqlite

4. Run the full repository checks after the plan lands.

    make test
    make lint

    Observed result:

        all Go tests passed
        lint passed with 0 issues

5. Record a short proof artifact for the next contributor:

    go test ./internal/app ./internal/store/sqlite -run 'Test(TaskCommandService|MessageServiceHandleMessageTask|StoreTask|StoreCloseTask)' -v

## Validation and Acceptance

This plan is complete when:

- `/task new <name>` creates a task and makes it active for the requesting user
- `/task` shows the current active task name and ID when one exists
- `/task list` shows open tasks and clearly marks the active task
- `/task switch <id>` changes the active task for the requesting user only
- `/task close <id>` marks the task closed and clears active state if that task was active
- in `task` mode, a normal mention without an active task returns actionable guidance instead of reaching Codex
- task-based thread bindings survive process restart and are not reset by a date boundary
- `make test` passes
- `make lint` passes

The next plan should be able to assume these repository facts:

- task records and active-task state are persisted in SQLite
- the app layer can answer `/task` command requests without Discord SDK types
- normal mentions in `task` mode produce guidance when no active task exists

## Idempotence and Recovery

Task creation tests should use isolated SQLite files or temporary databases so reruns remain safe. If the output wording of task guidance changes, update the tests intentionally rather than weakening them to vague string matches that stop proving useful behavior.

If you open this plan and the repository is missing a required starting-state capability, add the smallest necessary missing piece and record that decision in `Decision Log`. The recovery rule is to preserve one orchestration path, not to fork a second one.

## Artifacts and Notes

Keep this user-flow reminder visible:

    no active task -> normal mention -> guidance only
    /task new release -> task created and active
    next normal mention -> routes to release task thread
    /task close <id> -> task closed, active mapping cleared if applicable

Key proof points captured during implementation:

    task command responses are normalized in the app layer and marked ephemeral
    task bindings persist both logical_thread_key and task_id
    the same task binding survives later-day messages and SQLite reopen
    closing a non-active task does not clear a different active task

## Interfaces and Dependencies

This plan now relies on store and service methods shaped like these repository interfaces:

    type Task struct {
        TaskID        string
        DiscordUserID string
        TaskName      string
        Status        string
        CreatedAt     time.Time
        UpdatedAt     time.Time
        ClosedAt      *time.Time
    }

    type ThreadStore interface {
        CreateTask(ctx context.Context, task Task) error
        GetTask(ctx context.Context, discordUserID string, taskID string) (Task, bool, error)
        ListOpenTasks(ctx context.Context, userID string) ([]Task, error)
        SetActiveTask(ctx context.Context, activeTask ActiveTask) error
        GetActiveTask(ctx context.Context, userID string) (ActiveTask, bool, error)
        ClearActiveTask(ctx context.Context, userID string) error
        CloseTask(ctx context.Context, userID string, taskID string) error
    }

Any later Discord runtime work should call into this service rather than rebuilding task logic itself.

Revision Note: 2026-04-04 / Codex - Created this smaller child ExecPlan during the split of the original all-in-one runtime plan.
Revision Note: 2026-04-04 / Codex - Removed the parent-plan dependency and added explicit starting-state and recovery guidance so the document can stand alone.
Revision Note: 2026-04-04 / Codex - Updated progress, decisions, proof commands, and outcomes after completing the task-mode workflow implementation and validation.
