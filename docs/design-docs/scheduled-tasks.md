# Scheduled Tasks

This document defines the current design direction for scheduled Codex tasks in 39claw.

It exists to support `docs/product-specs/scheduled-tasks-user-flow.md` with implementation-facing boundaries and state rules while keeping `ARCHITECTURE.md` authoritative for overall system shape.

## Why This Exists

The product spec defines scheduled tasks as conversationally managed, Codex-native automation.
That leaves several design questions that should be fixed before implementation work starts:

- where canonical scheduled-task definitions live
- which runtime component owns schedule evaluation
- how Codex manages tasks without directly editing repository files
- how scheduled runs relate to normal Discord-driven Codex turns
- how delivery, retries, and run records should be separated

This document answers those questions at the concept level without turning 39claw into a general-purpose job platform.

## Core Decisions

- Scheduled-task definitions are canonical bot-managed state stored under `CLAW_DATADIR`, not repository-authored files.
- Users manage scheduled tasks through normal Codex conversation, but Codex must use 39claw-owned MCP tools for all create, read, update, enable, disable, and delete operations.
- There is one scheduled-task set per bot instance, independent from `daily` or `task` thread bindings and independent from whichever user task worktree is currently active.
- Every scheduled execution starts a fresh Codex thread and never reuses a prior scheduled-run thread ID.
- Scheduled execution uses the same Codex runtime configuration as normal turns, including the same effective repository root for the current bot mode.
- When the bot instance runs in `task` mode, a scheduled run executes in its own fresh temporary worktree created for that scheduled run and removed after the run finishes.
- Scheduled tasks are defined by a deliberately small schema: name, schedule, prompt, enabled state, and optional report-channel override.
- Infrastructure-level execution failure may trigger at most one automatic retry for the due run; content-level failure is reported by Codex output rather than by a separate 39claw policy engine.
- Recurring cron schedules do not backfill missed occurrences from before the current scheduler process started; those older overdue boundaries are skipped instead of replayed after downtime.
- Delivery to Discord is a separate step from Codex execution and should be recorded separately.

## Architectural Placement

Scheduled tasks should fit into the existing thin-runtime shape instead of introducing a parallel orchestration stack.

Conceptually:

```text
Discord runtime -> application services -> thread/store boundaries -> Codex gateway
Scheduler loop -> scheduled-task service -> scheduled-task store -> Codex gateway -> Discord presenter
```

This adds one new runtime path:

- a scheduler loop that notices due tasks
- a scheduled-task service that resolves execution and persistence rules

It should not add:

- a second agent loop
- generic workflow graphs
- arbitrary shell-job execution
- repository-file-based schedule discovery

## Ownership Boundaries

### 39claw owns

- canonical scheduled-task definitions
- schedule parsing and due-time evaluation
- durable run records
- bounded infrastructure retry policy
- Discord delivery orchestration
- MCP tools for schedule management

### Codex owns

- interpreting user intent during conversational management
- calling the exposed schedule-management MCP tools
- executing the scheduled prompt when a due run starts
- deciding how task output describes success, warning, or failure in content terms

### Repositories do not own

- the canonical schedule set
- hidden database-editing conventions
- cron configuration files
- scheduled-run continuity state

This keeps scheduled tasks aligned with the product promise that 39claw owns the managed automation surface.

## Definition Model

The scheduled-task definition should stay intentionally small.

### Canonical fields

- `task_id`
  - stable internal identifier
- `name`
  - human-meaningful unique name within the bot instance
- `schedule_kind`
  - `cron` or `at`
- `schedule_expr`
  - the source expression to evaluate in the bot instance timezone
- `prompt`
  - the Codex instruction body for the scheduled run
- `enabled`
  - whether future due times should be admitted
- `report_channel_id`
  - optional Discord channel override for delivery
- `created_at`
- `updated_at`

### Scheduling semantics

- `cron` means recurring evaluation in the configured instance timezone.
- `at` means one future local-time execution only.
- After a one-shot `at` task fires successfully or reaches a terminal failed state after the bounded retry rule, it should no longer be due again.

### Naming direction

Scheduled-task names should be unique at the bot-instance level.
Unlike `task` mode task names, these names do not participate in message routing keys, so they can be more presentation-oriented as long as they remain stable enough for conversational reference.

## Persistence Direction

Scheduled-task state belongs in SQLite alongside the other bot-managed state.

The concept-level storage direction is:

- `scheduled_tasks`
  - canonical task definitions
- `scheduled_task_runs`
  - one row per admitted due run
- `scheduled_task_deliveries`
  - one row per Discord delivery attempt or final delivery outcome

The exact schema can evolve later, but the design should preserve these separations:

- definition state is not the same as execution state
- execution state is not the same as Discord delivery state
- deleting a task definition should stop future admissions without rewriting historical runs

## MCP Management Surface

Scheduled-task management should be exposed to Codex as explicit 39claw-owned tools.

The concept-level tool surface should cover:

- list scheduled tasks
- inspect one scheduled task
- create a scheduled task
- update a scheduled task
- enable a scheduled task
- disable a scheduled task
- delete a scheduled task

These tools should validate input and mutate the canonical store directly.
Codex should not be expected to invent hidden file formats or write directly into SQLite-managed state.

This preserves the product behavior that scheduled tasks feel conversational while remaining 39claw-owned automation.

## Execution Model

When a task becomes due:

1. the scheduler loop detects the due definition in instance-local time
2. 39claw attempts to atomically admit one run record for that due occurrence
3. the scheduled-task service resolves the effective report target
4. 39claw starts a fresh Codex thread for the run
5. Codex executes the scheduled prompt against the bot instance's effective working directory
6. 39claw stores the run outcome
7. 39claw attempts Discord delivery and records that delivery outcome separately

### Working-directory rules

Scheduled tasks should use the same mode-aware repository target as normal interaction:

- in `daily` mode, the configured `CLAW_CODEX_WORKDIR`
- in `task` mode, a fresh temporary scheduled-run worktree created from the managed bare parent for the configured source checkout rather than from an arbitrary user task worktree

This keeps scheduled tasks independent from whichever task context a user last selected.
The feature is instance-scoped automation, not per-user active-task automation.

### Task-mode scheduled-run workspace rule

When the bot instance runs in `task` mode, scheduled execution should not borrow a user's existing task worktree.
Instead, each scheduled run should:

1. create a fresh temporary worktree for the scheduled run
2. run Codex inside that temporary workspace
3. remove the temporary worktree after the run reaches a terminal result

This keeps scheduled automation isolated from interactive task state while avoiding hidden cross-run workspace drift.
The tradeoff is that uncommitted filesystem changes from a scheduled run are disposable unless the run commits or otherwise exports them intentionally before cleanup.

### Threading rules

Each scheduled run starts from no saved thread binding.
39claw should not persist or reuse a long-lived scheduled-thread mapping for later recurrences.

This preserves the product rule that scheduled continuity comes from repository state and prompt design rather than from hidden thread reuse.

## Scheduler Behavior

The scheduler should remain intentionally small.

### Responsibilities

- parse supported schedule forms
- compute next due times in the configured timezone
- detect eligible tasks that are both valid and enabled
- admit each due occurrence once
- dispatch execution without blocking normal Discord intake

### Non-responsibilities

- general calendar-rule authoring beyond `cron` and `at`
- dependency graphs between tasks
- arbitrary concurrency policies exposed to users
- user-defined retry counts

### Admission rule

The scheduler must admit a due occurrence at most once, even across process restarts, using durable run-state checks rather than in-memory timers alone.

This is the key requirement that keeps scheduled tasks reliable enough without introducing distributed coordination as a v1 requirement.

### Overdue recurring runs

For personal-instance cron automation, missed recurring occurrences should not produce an unbounded backlog after downtime.
When the scheduler process starts, overdue recurring cron boundaries that happened before that startup moment should be skipped instead of replayed.
One-shot `at` schedules still admit their single due occurrence normally.

## Failure Model

The design should distinguish clearly between three failure classes.

### 1. Management failure

This is a failure while Codex tries to create or update a definition through MCP tools.

Result:

- no fake success
- no partial silent mutation
- the user sees that the requested management action failed

### 2. Execution failure

This is a system-level failure before the Codex run reaches a terminal result, such as process interruption or a Codex invocation failure.

Result:

- 39claw may retry the due run once
- the retry rule is infrastructure-owned, not task-authored
- the final run record should show whether the retry happened

### 3. Delivery failure

This is a failure after Codex has already produced a run result but Discord delivery could not be completed.

Result:

- the run remains recorded as executed
- delivery outcome is recorded separately
- future schedule evaluation is unaffected unless the task definition itself changes

## Interaction With Existing Modes

Scheduled tasks are instance-scoped automation and should not inherit user-scoped thread routing behavior.

### In `daily` mode

- scheduled runs share the same repository root as normal daily interaction
- scheduled runs do not join or mutate the active daily thread binding
- scheduled runs do not participate in daily-generation rotation semantics

### In `task` mode

- scheduled runs remain independent from per-user active-task selection
- scheduled runs do not target user task worktrees
- scheduled-task definitions remain canonical for the instance even while users switch tasks

This separation is important because otherwise scheduled automation would become ambiguous whenever user task context changes.

## Discord Reporting Direction

Scheduled output should be delivered through the existing presentation layer rather than through a special raw transport.

The report-target rule is:

- use `report_channel_id` when the task defines one
- otherwise use the instance default scheduled-task reporting behavior

The exact message template can stay product-facing, but the design should preserve a few invariants:

- users can tell which scheduled task produced the report
- delivery metadata should not leak internal storage details
- Discord formatting and chunking should reuse the normal presenter path where practical

## Operational Consequences

This design introduces new long-lived runtime responsibilities:

- scheduler startup and shutdown lifecycle
- due-run scanning after restart
- validation for schedule syntax and report-channel references
- durable run history for debugging and operator visibility

The tradeoff is intentional.
39claw grows a small amount of runtime state and timing logic so users can treat scheduled Codex work as a managed product capability rather than a repository hack.

## Implementation Notes

This design will affect at least:

- application-service boundaries for non-Discord-triggered execution
- SQLite schema and migrations
- a new scheduled-task store and service boundary
- Codex gateway invocation for fresh scheduled runs
- Discord delivery orchestration for bot-initiated reports

If later implementation needs per-user task-worktree scheduling, repository-file-backed definitions, or richer recurrence policies, that should be treated as a deliberate scope expansion and documented separately instead of being folded implicitly into this v1 design.
