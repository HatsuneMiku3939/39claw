# Scheduled Tasks User Flow

Status: Active

## Purpose

This document defines the intended user-facing behavior of 39claw for scheduled Codex tasks.

Its job is to answer questions such as:

- what scheduled tasks are in product terms
- how users create and manage them
- how Codex interacts with 39claw to manage task definitions
- what users should expect from scheduled execution and Discord delivery

This document is product-facing.
It describes user experience and stable product rules rather than internal package structure.

## Product Goal

39claw should let users define repeatable Codex work and have the bot execute that work on schedule without turning 39claw into a general-purpose job runner.

At a product level, scheduled tasks should feel like:

- Codex-native automation
- a natural extension of normal chat-driven interaction
- a small, understandable product surface
- managed automation rather than repository-authored infrastructure

## Scope

This document covers:

- the identity and boundary of scheduled tasks
- the user-facing authoring and management flow
- schedule-triggered execution expectations
- reporting expectations in Discord
- failure and recovery expectations visible to users

This document does not define:

- SQLite schema details
- scheduler implementation details
- internal runtime shutdown coordination
- Codex SDK integration mechanics

## Product Identity

Scheduled tasks are scheduled Codex runs.

They are not:

- arbitrary shell jobs
- a generic cron-hosting feature
- a multi-runtime job orchestration product

The execution target is always Codex running against the bot instance's configured working directory.

## User Promise

When users adopt scheduled tasks, the product should feel like:

- “I can manage scheduled work by talking to the bot normally.”
- “Codex uses built-in management tools for schedules instead of me editing hidden database state directly.”
- “39claw stores and owns the canonical scheduled-task definitions.”
- “Scheduled execution uses the same Codex capability the bot already exposes elsewhere.”

## Core Experience Rules

### 1. Scheduled-task definitions are owned by 39claw

The source of truth for scheduled-task definitions is 39claw-managed state under the bot data directory.

Users should not need to know or manipulate that storage layout directly.

### 2. Management happens through Codex-mediated tools

Users should be able to create, inspect, update, enable, disable, and delete scheduled tasks through normal message interaction with 39claw.

The user experience should remain conversational, but Codex should perform schedule management through MCP tools exposed by 39claw rather than by directly editing repository files.

### 3. Canonical state must be independent from interactive task workspaces

Scheduled-task definitions should not depend on which interactive task worktree or task context Codex is currently using.

From a product perspective, there is one canonical scheduled-task set for the bot instance.

### 4. Execution always targets Codex

Every scheduled run should execute as a fresh Codex thread in the same configured working directory used for normal message-driven Codex interaction.

Scheduled tasks do not get their own separate runtime model or special permission tier.

When the bot instance runs in `journal` mode, the scheduled run should use `CLAW_CODEX_WORKDIR` directly and should not create a temporary worktree.
When the bot instance runs in `thread` mode, the scheduled run should use its own fresh temporary worktree rather than borrowing a user's interactive task worktree.

### 5. Task definitions should stay small and understandable

Users should be able to understand a scheduled task in terms of its name, schedule, prompt, enabled state, and optional report target.

The product should avoid expanding the definition surface into a large policy system in v1.

### 6. Reporting is bridged, not interpreted

39claw should deliver scheduled-task output into Discord, but it should not impose its own success-classification policy beyond delivery behavior and system-level retry rules.

If a task wants to say “success,” “warning,” or “failure,” that judgment should come from the Codex-produced content rather than from a separate 39claw policy engine.

## Definition Model

### Canonical definition ownership

Scheduled-task definitions are stored in 39claw-owned persistent state under the bot instance data directory.

This state should be treated as canonical for the bot instance.

### User-facing management model

Users manage scheduled tasks by asking Codex through normal message interaction.

Codex uses MCP tools exposed by 39claw to:

- list scheduled tasks
- inspect one scheduled task
- create a scheduled task
- update a scheduled task
- enable or disable a scheduled task
- delete a scheduled task

The product should present this as one integrated conversational workflow rather than as a separate scheduling console.

### Minimum schema

The minimum product-visible fields are:

- `name`
- `schedule`
- `prompt`
- `enabled`

Optional field:

- `report_target`

There is no separate per-task policy surface in v1.
Scheduled tasks inherit the same effective permission model as normal message-driven Codex execution.

### Schedule syntax

v1 supports:

- `cron`
- one-shot `at` using the bot instance's configured local time zone

The product should present these as the only supported schedule forms in v1.
Users should not expect `every <duration>`, calendar-rule variants, or arbitrary recurrence syntax.

## Primary Flow

### Scenario: User wants to create a new scheduled task

Expected flow:

1. The user asks 39claw through normal message interaction to create a scheduled task.
2. Codex uses a 39claw schedule-management tool to create the task definition.
3. 39claw persists the new definition in its canonical scheduled-task store.
4. The task becomes active if the new definition is valid and `enabled` is true.

Expected user perception:

- “I created this task by talking to the bot normally.”
- “Codex handled the task creation for me.”
- “39claw now owns that scheduled definition.”

### Scenario: User updates an existing scheduled task

Expected flow:

1. The user asks 39claw to inspect or modify an existing scheduled task.
2. Codex reads the existing definition through a 39claw schedule-management tool.
3. Codex updates the task through the appropriate 39claw tool.
4. 39claw persists the new canonical definition.

Expected user perception:

- “Changing the task through conversation changes the real scheduled behavior.”
- “There is one canonical definition for this bot instance.”

### Scenario: User disables a scheduled task

Expected flow:

1. The user asks 39claw to disable a scheduled task.
2. Codex uses a 39claw schedule-management tool to update `enabled`.
3. 39claw persists the new disabled state.
4. The task remains defined but no longer schedules future runs.

Expected user perception:

- “The task still exists.”
- “It is intentionally paused rather than deleted.”

### Scenario: User deletes a scheduled task

Expected flow:

1. The user asks 39claw to delete a scheduled task.
2. Codex uses a 39claw schedule-management tool to remove it.
3. 39claw deletes the canonical definition from its persistent store.
4. Future scheduled runs for that task no longer occur.

Expected user perception:

- “Deleting the task removes its future scheduled behavior.”
- “The bot no longer considers that task part of the active schedule set.”

### Scenario: A cron-based task runs on schedule

Expected flow:

1. A scheduled time is reached according to the bot instance's local time zone.
2. 39claw starts a new scheduled run for that task.
3. The run executes as a fresh Codex thread in the configured working directory.
4. If the bot instance runs in `thread` mode, 39claw first creates a fresh temporary worktree for that scheduled run.
5. After the run finishes, 39claw removes that temporary worktree.
6. The stored task prompt is sent as the run input.
7. 39claw bridges the resulting output to Discord delivery behavior.

Expected user perception:

- “This was a new execution, not a continuation of an old task thread.”
- “The task ran against the same repository context as normal bot work.”
- “In thread mode, the scheduled run used its own temporary workspace instead of reusing an interactive task workspace.”

If the bot instance was offline long enough to miss recurring cron boundaries, 39claw should not replay that old backlog for personal-instance use.
When the scheduler process starts again, recurring cron boundaries that happened before startup should be skipped rather than replayed.

### Scenario: A one-shot `at` task runs once

Expected flow:

1. A task definition uses the one-shot local-time `at` schedule form.
2. 39claw stores the task before that time arrives.
3. At the scheduled local time, 39claw runs the task once.
4. After execution, the task is not treated as a recurring schedule.

Expected user perception:

- “This task was scheduled for one specific local-time moment.”
- “It did not turn into a repeating cron job.”

### Scenario: A task sends output to a specific report channel

Expected flow:

1. The task definition includes `report_target`.
2. The task runs on schedule.
3. 39claw delivers the output to the configured Discord report target for that task.

Expected user perception:

- “This task can report somewhere other than the default report destination.”
- “That destination is part of the managed task definition.”

### Scenario: A task has no per-task report override

Expected flow:

1. The task definition omits `report_target`.
2. The task runs on schedule.
3. 39claw delivers the output using the instance-level reporting behavior.

Expected user perception:

- “Tasks inherit the instance default unless they define a specific report destination.”

## Definition Expectations

For scheduled tasks to feel product-ready, v1 should keep the management contract simple and stable.

Users should be able to rely on these expectations:

- one canonical scheduled-task set per bot instance
- conversational management through Codex
- MCP-backed create, read, update, enable, disable, and delete behavior
- a small task schema
- no separate per-task permission policy surface
- no special scheduled-task-only language beyond supported schedule syntax

## UX Requirements

### Canonical clarity

A user should be able to assume that scheduled-task definitions live in one canonical bot-managed store for the instance.

### Explainability

If a task does not run, runs once, or stops recurring, the user should be able to explain that behavior through the task definition and documented product rules.

### Consistency with normal Codex behavior

Scheduled tasks should feel like the same Codex capability the bot exposes through normal messages, not like a separate execution engine.

### Fresh-run expectation

Users should be able to assume that each scheduled run starts from a fresh Codex thread.
Continuity should come from repository state and prompt design rather than from hidden thread reuse.

### Workspace independence

Interactive task-worktree isolation should not create multiple competing scheduled-task definition sets.
The scheduled-task management surface should remain stable regardless of which task context the user is currently in.

### Small product surface

The product should avoid expanding scheduled tasks into a full job-management suite.
If a proposed feature starts requiring task-specific permission models, generic shell execution, or heavy non-Codex orchestration semantics, it should be treated as out of scope for this product surface.

## Failure and Edge Cases

### Invalid task definition input

If a requested task definition is malformed or missing required fields, the create or update operation should not succeed.

Expected user perception:

- “My requested task definition was invalid.”
- “The bot did not silently invent defaults for a broken task.”

### Management-tool failure

If Codex cannot complete a schedule-management operation because the 39claw management tool call fails, the user should receive a clear failure outcome rather than a fake success.

Expected user perception:

- “The requested change was not applied.”
- “The failure happened in the management path, not silently later.”

### System failure during a scheduled run

If the system fails in a way that prevents the run from completing normally, 39claw may retry that scheduled run once.

Expected user perception:

- “The system attempted a bounded recovery for an infrastructure-level failure.”
- “This was not an open-ended retry loop.”

The product should treat this retry as a system reliability rule, not as a task-author-controlled policy knob in v1.

### Discord delivery failure

If a task completes but Discord delivery fails, the product should preserve the run record and the delivery outcome separately.

Expected user perception:

- “The task run and the report delivery are related but not the same thing.”
- “A delivery failure does not necessarily mean the Codex run itself failed.”

### Task deleted while an execution is already in progress

If a task is deleted after a run has already started, the product does not need to promise that the in-flight execution disappears retroactively.

The deletion should stop future scheduled runs for that task.

## Out of Scope

The following are out of scope for this product surface in v1:

- arbitrary shell-job scheduling
- per-task permission policies
- generic workflow graphs or dependency chains
- repository-file-based canonical schedule definitions
- recurring schedules beyond `cron`
- duration-based schedule syntax such as `every 6h`
- reuse of prior Codex threads for scheduled-run continuity

## Open Presentation Questions

The following details remain product-adjacent and may need a separate Discord-facing spec once the reporting UX is implemented:

- the exact message shape used for scheduled-task delivery
- whether scheduled deliveries include task-name headers or metadata blocks
- how create, update, enable, disable, and delete confirmations are phrased in Discord
- whether operators get a built-in summary view of currently registered scheduled tasks
