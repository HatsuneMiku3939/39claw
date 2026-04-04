# Daily Mode User Flow

Status: Active

## Purpose

This document defines the intended user-facing behavior of 39claw when the bot instance is configured to use `daily` mode.

The goal of `daily` mode is to make the bot feel natural and low-friction for ongoing daily conversation and lightweight work.

## Product Goal

A user should be able to send a normal message and continue the conversation naturally throughout the day without needing to manage thread state explicitly.
At a product level, this mode should feel like talking to a living knowledge base backed by repository instructions and documentation.

## User Promise

In `daily` mode, the bot should feel like:

- a conversation that continues during the current day
- a fresh start on a new day
- a tool that does not require explicit setup for normal use

## Core Experience Rules

### 1. Normal conversation should be the default

The user should be able to send a message without first creating a task, selecting a thread, or learning a control command.

### 2. Daily continuity should be automatic

Messages from the same user on the same local date should feel like they continue the same line of work unless the product explicitly says otherwise.

### 3. Day boundaries should reset context cleanly

When the relevant local date changes, the user should experience a fresh conversation context.
This should feel predictable rather than surprising.

### 4. The bot should avoid over-explaining thread mechanics

The user does not need internal detail about logical keys or Codex thread IDs during normal operation.

## Primary Flow

### Scenario: First message of the day

Expected flow:

1. The user sends a normal message in a supported channel.
2. 39claw determines that the message should be handled.
3. 39claw resolves the current daily thread bucket for that user.
4. If no thread exists for that bucket, 39claw creates a new one automatically.
5. The user receives a normal response.

Expected user perception:

- “I can just talk to the bot.”
- “The bot is ready without setup.”

### Scenario: Follow-up message on the same day

Expected flow:

1. The user sends another message on the same local date.
2. 39claw routes the message to the already bound daily thread.
3. The response reflects same-day continuity.

Expected user perception:

- “The bot remembers today’s context.”
- “I do not need to restate everything.”

### Scenario: First message on a new day

Expected flow:

1. The user sends a message after the local date has changed.
2. 39claw resolves a new daily bucket automatically.
3. If no thread exists for the new bucket, 39claw creates a new one.
4. The response begins from a fresh context unless the user supplies previous context again.

Expected user perception:

- “Today feels like a fresh session.”
- “The reset is expected and understandable.”

## UX Requirements

### Message routing

The user should not have to manually select a thread for normal use in `daily` mode.

### Continuity boundaries

Continuity should be preserved:

- for the same user
- on the same configured local date

Continuity should not be assumed across different days unless the product later adds an explicit bridging workflow.
Changing channels within the same bot instance should not reset the daily context by itself.

### Response tone

The bot should behave as if continuity is normal, not as if the user is performing a session-management action.

### Reset clarity

If a date-boundary reset causes confusion, the product may need a lightweight explanation, but this should not be the default for every first message of the day.

## Failure and Edge Cases

### Backend or storage failure

If the bot cannot resolve or create the required thread, it should:

- explain that it could not continue the conversation
- avoid leaking unnecessary internal detail
- tell the user whether retrying is likely to help

### Timezone confusion

If the product exposes date-boundary behavior to users, it should do so in terms that map to the configured local timezone for the bot instance.

### Channel changes

If the same user speaks in a different channel within the same bot instance, the product should preserve daily continuity rather than silently resetting it.
If that creates confusion for a specific deployment, the preferred solution is to separate bot instances by purpose rather than keying continuity by channel.

## Non-Goals

`daily` mode is not intended to optimize for:

- long-lived project management
- explicit task switching
- multi-day durable work context without re-grounding

## Decisions

- The bot should not proactively mention that a new day created a fresh context.
- There should not be an explicit command for inspecting the current daily context in v1.
- The configured timezone should not be surfaced proactively to end users in normal daily-mode flow.
- The bot should not provide lightweight guidance when channel changes preserve continuity.
