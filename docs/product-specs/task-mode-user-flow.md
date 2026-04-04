# Task Mode User Flow

Status: Draft

## Purpose

This document defines the intended user-facing behavior of 39bot when the bot instance is configured to use `task` mode.

The goal of `task` mode is to support durable, explicit work streams that can continue across multiple days without relying on date-based buckets.

## Product Goal

A user should be able to attach conversation continuity to an explicit task identity and switch work contexts intentionally rather than implicitly.
At a product level, this mode should feel like driving real repository work through Discord rather than using a general chat thread.

## User Promise

In `task` mode, the bot should feel like:

- a tool for focused ongoing work
- a system with explicit context boundaries
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

If the user tries to work without an active task, the response should tell them what to do next in simple language and point them toward the `/task ...` command flow.

## Primary Flow

### Scenario: User has no active task

Expected flow:

1. The user sends a normal message in a supported channel.
2. 39bot determines that `task` mode requires an active task context.
3. 39bot detects that no active task exists for the user in the current bot instance.
4. 39bot does not route the message into an arbitrary thread.
5. The bot responds with a clear explanation and a next step.

Expected user perception:

- “The bot is waiting for me to choose a task.”
- “The requirement is explicit, not random.”

### Scenario: User creates or selects a task

Expected flow:

1. The user uses `/task new <name>`, `/task switch <id>`, or `/task`-related controls to establish the desired task context.
2. 39bot records that task as the active context for the user within the current bot instance.
3. The next normal message routes to the thread associated with that task.
4. If the task has no bound thread yet, 39bot creates one.

Expected user perception:

- “I chose the work context.”
- “The bot now knows what I mean by continuing this task.”

### Scenario: User continues an active task on a later day

Expected flow:

1. The user returns later and continues talking while the same task is active.
2. 39bot routes the message to the thread associated with that task.
3. The response reflects ongoing continuity for that work stream.

Expected user perception:

- “This task keeps its context over time.”
- “I do not need to start over just because the date changed.”

### Scenario: User switches to another task

Expected flow:

1. The user explicitly changes the active task.
2. 39bot updates the current task selection for the user within the current bot instance.
3. The next normal message routes to the newly selected task thread.

Expected user perception:

- “I intentionally switched work contexts.”
- “The bot should now respond in terms of the new task.”

## Minimum UX Requirements

For `task` mode to feel usable, v1 should support at least:

- `/task`
  - show the current task name and ID
- `/task list`
  - show task names and IDs
- `/task new <name>`
  - create a new task and switch the active task to it
- `/task switch <id>`
  - switch the active task to the specified task
- `/task close <id>`
  - close the specified task

The command family should stay explicit and stable enough that users can learn it as the standard task-control surface for `task` mode.

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
That is especially important for `/task`, `/task list`, and `/task switch <id>`.

### Task scope

Active task state should be scoped to the user within the current bot instance.
Shared-task state across multiple users is out of scope for v1.

## Failure and Edge Cases

### Missing active task

When no active task exists, the bot should:

- say that an active task is required
- explain the next action clearly
- direct the user toward `/task new <name>`, `/task switch <id>`, or `/task list` as appropriate
- avoid pretending the user message was processed normally

If a user closes a task and then immediately sends a normal message, the bot should treat that as a missing-active-task case.
In `task` mode, normal messages should not trigger task creation or implicit recovery on their own.

### Stale or invalid task binding

If a task exists but its thread binding cannot be resumed, the bot should explain that continuity could not be restored and indicate whether retrying or re-establishing the task is needed.

### Incorrect task risk

If the product is not certain which task should receive a message, it should prefer asking for explicit user action rather than guessing and contaminating the wrong context.

## Non-Goals

`task` mode is not intended to optimize for:

- frictionless casual daily chat
- invisible context management
- implicit switching between unrelated work streams

## Decisions

- Active task state is always user-scoped within a bot instance in v1.
- The active task should remain active until the user explicitly closes it or switches to another task.
- If a task is closed and the user then sends a normal message, the bot should respond with missing-active-task guidance rather than routing the message normally.
