# Task Mode User Flow

Status: Active

## Purpose

This document defines the intended user-facing behavior of 39claw when the bot instance is configured to use `task` mode.

The goal of `task` mode is to support durable, explicit work streams that can continue across multiple days without relying on date-based buckets.

## Product Goal

A user should be able to attach conversation continuity to an explicit task identity and switch work contexts intentionally rather than implicitly.
At a product level, this mode should feel like driving real repository work through Discord rather than using a general chat thread.

## User Promise

In `task` mode, the bot should feel like:

- a tool for focused ongoing work
- a system with explicit context boundaries
- a system that gives each task its own isolated repository workspace
- a bot that does not guess the wrong long-lived context silently

## Core Experience Rules

### 1. Explicit task context is required

The bot should not silently invent or guess a task when normal conversation depends on one.

### 2. Task continuity should be durable

Once a task is active, the user should be able to continue that line of work over time without losing context because of date boundaries.

### 3. Task switching should be intentional

Changing the active task should be treated as a meaningful context change.
The product should make that behavior understandable to the user.

### 4. Missing context should produce actionable guidance

If the user tries to work without an active task, the response should tell them what to do next in simple language and point them toward the `/<instance-command> action:task-*` command flow.

### 5. Task work should be isolated

Each task should run inside its own repository workspace rather than sharing one mutable checkout with other tasks.
That isolation should make task switching feel like switching work streams, not only switching chat memory.

## Primary Flow

### Scenario: User has no active task

Expected flow:

1. The user sends a normal message in a supported channel.
2. 39claw determines that `task` mode requires an active task context.
3. 39claw detects that no active task exists for the user in the current bot instance.
4. 39claw does not route the message into an arbitrary thread.
5. The bot responds with a clear explanation and a next step.

Expected user perception:

- “The bot is waiting for me to choose a task.”
- “The requirement is explicit, not random.”

### Scenario: User creates or selects a task

Expected flow:

1. The user uses `/<instance-command> action:task-new task_name:<name>`, `/<instance-command> action:task-switch task_name:<name>`, or `/<instance-command> action:task-current` plus other task actions to establish the desired task context.
2. 39claw records that task as the active context for the user within the current bot instance.
3. A newly created task reserves its own task branch identity even before worktree creation.
4. The next normal message routes to the thread associated with that task.
5. If the task has no ready worktree yet, 39claw prepares the task workspace lazily before running Codex.
6. If the task has no bound thread yet, 39claw creates one.

Expected user perception:

- “I chose the work context.”
- “The bot now knows what I mean by continuing this task.”
- “This task has its own workspace.”

### Scenario: User continues an active task on a later day

Expected flow:

1. The user returns later and continues talking while the same task is active.
2. 39claw routes the message to the thread associated with that task.
3. The response reflects ongoing continuity for that work stream.

Expected user perception:

- “This task keeps its context over time.”
- “I do not need to start over just because the date changed.”
- “My task workspace is still separate from other tasks.”

### Scenario: User switches to another task

Expected flow:

1. The user explicitly changes the active task.
2. 39claw updates the current task selection for the user within the current bot instance.
3. The next normal message routes to the newly selected task thread.

Expected user perception:

- “I intentionally switched work contexts.”
- “The bot should now respond in terms of the new task.”
- “The switch changes both the Codex context and the task workspace used for later work.”

### Scenario: The first normal message for a task needs workspace preparation

Expected flow:

1. The user creates or switches to a task that has no ready task worktree yet.
2. The user sends a normal message to continue that task.
3. 39claw detects that the task worktree is still pending or previously failed.
4. 39claw creates or refreshes the managed bare parent under `${CLAW_DATADIR}/repos`, best-effort fetches `origin`, and then prepares the task-specific Git worktree under the bot data directory from the shared remote default branch when available.
5. After workspace preparation succeeds, 39claw runs the Codex turn in that task worktree.

Expected user perception:

- “The bot prepared this task's workspace when I first used it.”
- “New tasks start from the shared team baseline instead of some random local-only commit.”
- “Later turns for this task should continue in the same isolated workspace.”

### Scenario: A message is queued for the active task and the user switches tasks before it runs

Expected flow:

1. The user sends a normal message while the current task already has a running turn.
2. 39claw accepts the message into the waiting queue for that `user + task_id` context and immediately acknowledges that it was queued.
3. Before the queued turn runs, the user switches the active task to a different task.
4. The queued turn still executes against the task that was active when the message was accepted.
5. The real answer arrives later as a reply to the queued message.

Expected user perception:

- “My queued message stayed attached to the task I meant.”
- “Switching tasks later did not silently reroute earlier work.”

## Minimum UX Requirements

For `task` mode to feel usable, v1 should support at least:

- `/<instance-command> action:task-current`
  - show the current task name and ID
- `/<instance-command> action:task-list`
  - show task names and IDs
- `/<instance-command> action:task-new task_name:<name>`
  - create a new task and switch the active task to it
- `/<instance-command> action:task-switch task_name:<name>`
  - switch the active task to the uniquely named open task, with `task_id` used only when the name is ambiguous
- `/<instance-command> action:task-close task_name:<name>`
  - close the uniquely named open task, with `task_id` used only when the name is ambiguous

The root-command action surface should stay explicit and stable enough that users can learn it as the standard task-control surface for `task` mode.

## UX Requirements

### Context safety

The bot should favor correctness of task routing over convenience when the active task is ambiguous or missing.

### Explainability

Task-related blocking behavior should be easy to understand without knowledge of internal storage or Codex thread mechanics.

### Persistence expectation

Users should be able to assume that task continuity is stable over multiple sessions unless the bot explicitly reports that something went wrong.
The active task should remain active until the user explicitly closes it or switches to another task.

### Task identity clarity

If task names, IDs, or labels are exposed, the product should make it obvious which task is currently active.
That is especially important for `action:task-current`, `action:task-list`, and `action:task-switch`.

### Task scope

Active task state should be scoped to the user within the current bot instance.
Shared-task state across multiple users is out of scope for v1.
Task worktree isolation is also user-scoped through the task identity.

## Failure and Edge Cases

### Missing active task

When no active task exists, the bot should:

- say that an active task is required
- explain the next action clearly
- direct the user toward `/<instance-command> action:task-new task_name:<name>`, `/<instance-command> action:task-switch task_name:<name>`, or `/<instance-command> action:task-list` as appropriate
- avoid pretending the user message was processed normally

If a user closes a task and then immediately sends a normal message, the bot should treat that as a missing-active-task case.
In `task` mode, normal messages should not trigger task creation or implicit recovery on their own.

### Stale or invalid task binding

If a task exists but its thread binding cannot be resumed, the bot should explain that continuity could not be restored and indicate whether retrying or re-establishing the task is needed.

### Workspace preparation failure

If the task workspace cannot be prepared, the bot should explain that the task workspace is not ready and tell the user to retry.
The bot should not pretend that Codex processed the message normally.

### Incorrect task risk

If the product is not certain which task should receive a message, it should prefer asking for explicit user action rather than guessing and contaminating the wrong context.
That same safety rule applies to queued work: the task context must be frozen at queue-admission time rather than re-read later.

### Closed-task workspace retention

Closing a task should not imply immediate deletion of its task branch.
However, the product may prune older closed-task worktrees to keep local disk usage bounded.
The most recent closed task workspaces should remain available longer than older ones.

## Non-Goals

`task` mode is not intended to optimize for:

- frictionless casual daily chat
- invisible context management
- implicit switching between unrelated work streams

## Decisions

- Active task state is always user-scoped within a bot instance in v1.
- The active task should remain active until the user explicitly closes it or switches to another task.
- If a task is closed and the user then sends a normal message, the bot should respond with missing-active-task guidance rather than routing the message normally.
- `task` mode assumes the configured workdir is a Git repository with an `origin` remote.
- New tasks create metadata first and prepare their task worktree lazily on the first normal message.
- Closed-task retention keeps only the fifteen most recently closed ready task worktrees.
- Task branches are retained in the managed bare parent even when older closed-task worktrees are pruned.
