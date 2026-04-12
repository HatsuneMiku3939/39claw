# Scheduled Tasks User Flow

Status: Active

## Purpose

This document defines the intended user-facing behavior of 39claw for repository-defined scheduled Codex tasks.

Its job is to answer questions such as:

- what scheduled tasks are in product terms
- how users create and update them
- how 39claw discovers and applies schedule-definition changes
- what users should expect from scheduled execution and Discord delivery

This document is product-facing.
It describes user experience and stable product rules rather than internal package structure.

## Product Goal

39claw should let users define repeatable Codex work inside the repository and have the bot execute that work on schedule without turning 39claw into a general-purpose job runner.

At a product level, scheduled tasks should feel like:

- repository-native automation
- Codex-native execution
- easy-to-audit recurring work
- a small extension of the existing chat-driven workflow rather than a separate operations product

## Scope

This document covers:

- the identity and boundary of scheduled tasks
- the authoring and update flow for job definitions
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

- “I define the task in the repository, so the repository remains the source of truth.”
- “The bot executes the same kind of Codex work it already performs for normal messages.”
- “The schedule is understandable from the file itself.”
- “Discord acts as a delivery surface for results, not as the canonical configuration database.”

## Core Experience Rules

### 1. Repository definitions are canonical

The source of truth for scheduled tasks is the repository, not Discord state.

If users want to add, remove, enable, disable, or change a scheduled task, they should do so by editing the repository files that define those tasks.

### 2. Scheduled tasks are managed through normal Codex interaction

Users should be able to ask 39claw through normal message interaction to inspect or edit the scheduled-task definition files in the repository.

The product should treat that as ordinary Codex-mediated repository work rather than as a separate built-in CRUD command surface.

### 3. Discovery is file-driven

39claw should periodically scan the scheduled-task definition directory and reconcile the active task set against those files.

From a user perspective, the product rule is simple:

- if a valid definition file appears, the task becomes scheduled
- if a definition file changes, the scheduled task updates
- if a definition file is removed, the scheduled task disappears

### 4. Execution always targets Codex

Every scheduled run should execute as a fresh Codex thread in the same configured working directory used for normal message-driven Codex interaction.

Scheduled tasks do not get their own separate runtime model or special permission tier.

### 5. Task definitions should stay small and readable

Users should be able to understand the purpose, timing, and prompt of a scheduled task from one repository file without reading internal implementation docs.

### 6. Reporting is bridged, not interpreted

39claw should deliver scheduled-task output into Discord, but it should not impose its own success-classification policy beyond delivery behavior and system-level retry rules.

If a task wants to say “success,” “warning,” or “failure,” that judgment should come from the Codex-produced content rather than from a separate 39claw policy engine.

## Definition Model

### Definition location

Scheduled-task definitions live under:

- `.agents/schedules/`

That directory should be treated as the stable, product-visible home for scheduled-task definitions.

### Definition format

Each scheduled task is defined as a repository file that uses frontmatter plus plain-text body content.

The frontmatter carries the structured job metadata.
The body carries the task prompt as plain text.

### Minimum schema

The minimum product-visible fields are:

- `name`
- `schedule`
- `enabled`

Optional field:

- `report_channel_id`

The body content is the prompt and should be treated as required for a meaningful task definition.

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
2. Codex reads or writes files under `.agents/schedules/` in the repository.
3. A new valid definition file is created with frontmatter metadata and a plain-text prompt body.
4. 39claw later discovers the new file through its periodic scan.
5. The task becomes active if the definition is valid and `enabled` is true.

Expected user perception:

- “I created this task by changing repository files.”
- “The repository now describes the automation.”
- “The bot picked it up from the repository rather than from hidden state.”

### Scenario: User updates an existing scheduled task

Expected flow:

1. The user asks 39claw to inspect or modify an existing schedule-definition file.
2. Codex edits the file in `.agents/schedules/`.
3. 39claw discovers the changed file on a later scan.
4. The active scheduled-task behavior updates to match the new file contents.

Expected user perception:

- “Changing the file changes the scheduled behavior.”
- “There is one obvious place to review the task definition.”

### Scenario: User disables a scheduled task

Expected flow:

1. The user changes the definition so `enabled` is false.
2. 39claw discovers the updated file.
3. The task remains defined but no longer schedules future runs.

Expected user perception:

- “The task still exists in the repository.”
- “It is intentionally paused rather than deleted.”

### Scenario: User deletes a scheduled task

Expected flow:

1. The user removes the scheduled-task definition file from `.agents/schedules/`.
2. 39claw discovers that the file no longer exists.
3. The previously registered task is removed from the active schedule set.

Expected user perception:

- “Deleting the file removes the scheduled behavior.”
- “The repository state still explains why the task is gone.”

### Scenario: A cron-based task runs on schedule

Expected flow:

1. A scheduled time is reached according to the bot instance's local time zone.
2. 39claw starts a new scheduled run for that task.
3. The run executes as a fresh Codex thread in the configured working directory.
4. The task prompt body is sent as the run input.
5. 39claw bridges the resulting output to Discord delivery behavior.

Expected user perception:

- “This was a new execution, not a continuation of an old task thread.”
- “The task ran against the same repository context as normal bot work.”

### Scenario: A one-shot `at` task runs once

Expected flow:

1. A task definition uses the one-shot local-time `at` schedule form.
2. 39claw discovers and registers the task before that time arrives.
3. At the scheduled local time, 39claw runs the task once.
4. After execution, the task is not treated as a recurring schedule.

Expected user perception:

- “This task was scheduled for one specific local-time moment.”
- “It did not turn into a repeating cron job.”

### Scenario: A task sends output to a specific report channel

Expected flow:

1. The task definition includes `report_channel_id`.
2. The task runs on schedule.
3. 39claw delivers the output to the configured Discord report target for that task.

Expected user perception:

- “This task can report somewhere other than the default report destination.”
- “The destination is part of the repository-defined task.”

### Scenario: A task has no per-task report override

Expected flow:

1. The task definition omits `report_channel_id`.
2. The task runs on schedule.
3. 39claw delivers the output using the instance-level reporting behavior.

Expected user perception:

- “Tasks inherit the instance default unless the file says otherwise.”

## Definition Expectations

For scheduled tasks to feel product-ready, v1 should keep the authoring contract simple and stable.

Users should be able to rely on these expectations:

- one repository directory for all scheduled definitions
- one definition file per scheduled task
- frontmatter for metadata
- plain-text body for the prompt
- no separate per-task permission policy surface
- no special scheduled-task-only command language beyond supported schedule syntax

## UX Requirements

### Auditability

A user should be able to inspect the repository and understand what scheduled work exists without querying a hidden control plane.

### Explainability

If a task does not run, runs once, or stops recurring, the user should be able to explain that behavior through the file definition and documented product rules.

### Consistency with normal Codex behavior

Scheduled tasks should feel like the same Codex capability the bot exposes through normal messages, not like a separate execution engine.

### Fresh-run expectation

Users should be able to assume that each scheduled run starts from a fresh Codex thread.
Continuity should come from the repository state and prompt design rather than from hidden thread reuse.

### Small product surface

The product should avoid expanding scheduled tasks into a full job-management suite.
If a proposed feature starts requiring task-specific permission models, generic shell execution, or heavy non-Codex orchestration semantics, it should be treated as out of scope for this product surface.

## Failure and Edge Cases

### Invalid definition file

If a schedule-definition file is malformed or missing required fields, the task should not become active.

Expected user perception:

- “The repository file is invalid.”
- “The bot did not silently invent defaults for a broken task.”

### Scan delay after repository change

Scheduled-task changes do not need to appear instantly after a file edit.

Expected user perception:

- “The bot discovers changes by scanning.”
- “A small delay between editing the file and seeing the new schedule is expected.”

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

### Task removed while previously scheduled

If a definition file is deleted after a task was previously active, 39claw should stop scheduling future runs for that task after reconciliation.

The product does not need to promise that already-started runs disappear retroactively.

## Out of Scope

The following are out of scope for this product surface in v1:

- arbitrary shell-job scheduling
- per-task permission policies
- generic workflow graphs or dependency chains
- built-in repository-external job editors
- recurring schedules beyond `cron`
- duration-based schedule syntax such as `every 6h`
- reuse of prior Codex threads for scheduled-run continuity

## Open Presentation Questions

The following details remain product-adjacent and may need a separate Discord-facing spec once the reporting UX is implemented:

- the exact message shape used for scheduled-task delivery
- whether scheduled deliveries include task-name headers or metadata blocks
- how invalid schedule definitions are surfaced to operators in Discord, if at all
- whether operators get a built-in summary view of currently discovered scheduled tasks
