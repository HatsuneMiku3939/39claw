# Implement `task` mode task workflow and command orchestration

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, `task` mode should support explicit, durable work streams. A user should be able to create a task, switch tasks, list open tasks, close a task, and continue a task across days without losing context. Normal messages in `task` mode must not reach Codex unless the user has an active task, and the missing-context response must point the user toward the `/task ...` workflow.

## Progress

- [x] (2026-04-04 15:27Z) Defined the `task` mode plan and its acceptance targets.
- [ ] Confirm that the repository provides the capabilities listed in `Starting State`.
- [ ] Implement persistent task records and active-task state in SQLite.
- [ ] Implement `/task`, `/task list`, `/task new <name>`, `/task switch <id>`, and `/task close <id>` in the app layer.
- [ ] Implement missing-active-task guidance for normal mentions in `task` mode.
- [ ] Ensure task-based logical thread keys are stable across days and process restart.
- [ ] Add tests for task lifecycle transitions, user-scoped active task behavior, and missing-context guidance.

## Surprises & Discoveries

- Observation: `task` mode shares the same Codex gateway and thread-binding machinery as `daily` mode, but it introduces extra user-scoped state that must remain correct under closure and switching.
  Evidence: `docs/design-docs/implementation-spec.md`

## Decision Log

- Decision: Keep active-task state scoped to one Discord user within one bot instance.
  Rationale: This is the fixed v1 behavior in the implementation spec and product specs.
  Date/Author: 2026-04-04 / Codex

- Decision: Use ULID strings for `task_id`.
  Rationale: The implementation spec fixes ULID as the v1 identifier format and it keeps task IDs sortable and copyable.
  Date/Author: 2026-04-04 / Codex

## Outcomes & Retrospective

The outcome of this plan should be a repository that can support durable work context intentionally instead of guessing. Success means the user can see and control which work stream the bot is continuing.

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

Extend `internal/store/sqlite/store.go` with the concrete task operations required here. Add methods for creating a task, listing open tasks for a user, reading the current active task, setting the active task, and closing a task. Closing a task should mark its status as `closed`, set `closed_at`, and remove the `active_tasks` mapping if that task was active.

Implement `TaskCommandService` in `internal/app/task_service.go`. The service should provide separate methods for showing the current task, listing open tasks, creating a task, switching tasks, and closing a task. The response text should clearly describe what changed and what the active task is now when relevant.

Update the normal-message service so that in `task` mode it checks for an active task before routing to Codex. When no active task exists, it must return actionable guidance that points the user toward `/task new <name>`, `/task list`, or `/task switch <id>`. It must not create tasks implicitly and must not route the message anyway.

Reuse the same thread-binding table for task threads. The binding should include the logical key built from user ID and task ID. Add tests that show the same task routes to the same Codex thread across multiple days because task mode is not date-bound.

If you discover that the current normal-message service is too narrow to support both `daily` and `task` behavior cleanly, refactor it here as part of the plan. Do not create a separate parallel orchestration path just for tasks.

## Concrete Steps

Run all commands from `/home/filepang/playground/39claw`.

1. Confirm the repository matches the required starting state.

    make test
    make lint

2. Implement task storage operations and command orchestration.

3. Run focused tests while iterating.

    go test ./internal/app ./internal/store/sqlite -run 'TestTask|TestActiveTask|TestCloseTask|TestMissingActiveTask'

4. Run the full repository checks after the plan lands.

    make test
    make lint

5. Record a short proof artifact for the next contributor:

    go test ./internal/app ./internal/store/sqlite -run 'TestTask|TestActiveTask|TestCloseTask|TestMissingActiveTask' -v

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

## Interfaces and Dependencies

This plan should rely on store methods shaped like these examples:

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
        CreateTask(ctx context.Context, params CreateTaskParams) (Task, error)
        ListOpenTasks(ctx context.Context, userID string) ([]Task, error)
        GetActiveTask(ctx context.Context, userID string) (Task, bool, error)
        SetActiveTask(ctx context.Context, userID string, taskID string) error
        CloseTask(ctx context.Context, userID string, taskID string, closedAt time.Time) error
    }

Any later Discord runtime work should call into this service rather than rebuilding task logic itself.

Revision Note: 2026-04-04 / Codex - Created this smaller child ExecPlan during the split of the original all-in-one runtime plan.
Revision Note: 2026-04-04 / Codex - Removed the parent-plan dependency and added explicit starting-state and recovery guidance so the document can stand alone.
