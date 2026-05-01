# Journal Mode User Flow

Status: Active

## Purpose

This document defines the intended user-facing behavior of 39claw when the bot instance is configured to use `journal` mode.

The goal of `journal` mode is to make the bot feel natural and low-friction for ongoing journal conversation and lightweight work.

## Product Goal

A user should be able to send a normal message and continue the shared conversation naturally throughout the day without needing to manage thread state explicitly.
At a product level, this mode should feel like talking to a living knowledge base backed by repository instructions and documentation.

## User Promise

In `journal` mode, the bot should feel like:

- a shared conversation that continues during the current day
- a tool that can intentionally start a fresh shared same-day thread when users need to cut down context growth
- a fresh remote thread on a new day without losing durable preferences or other long-lived context
- a tool that does not require explicit setup for normal use

## Core Experience Rules

### 1. Normal conversation should be the default

The user should be able to send a message without first creating a task, selecting a thread, or learning a control command.

### 2. Daily continuity should be automatic

Messages handled on the same local date should feel like they continue the same line of work unless the product explicitly says otherwise.

### 3. Same-day resets should be explicit and shared

If a user invokes `/<instance-command> action:clear`, the bot should start a fresh shared same-day generation for the whole bot instance.
That reset should apply to the shared journal context, not only to the user who issued the command.
If the current shared generation is still busy, the bot should reject the clear request and ask the user to retry after the queued work finishes.

### 4. Day boundaries should reset the thread cleanly without discarding durable memory

When the relevant local date changes, the user should experience a fresh conversation thread.
That reset should feel predictable rather than surprising, while durable facts that matter on future days may still carry forward through the runtime-managed memory bridge.

### 5. The bot should avoid over-explaining thread mechanics

The user does not need internal detail about logical keys or Codex thread IDs during normal operation.

## Primary Flow

### Scenario: First message of the day

Expected flow:

1. The user sends a normal message in a supported channel.
2. 39claw determines that the message should be handled.
3. 39claw resolves the current daily thread bucket.
4. If no active generation exists for that bucket yet, 39claw creates generation `#1` automatically.
5. If no thread exists for that active generation, 39claw creates a new one automatically.
6. The user receives a normal response.

Expected user perception:

- “I can just talk to the bot.”
- “The bot is ready without setup.”

### Scenario: Follow-up message on the same day

Expected flow:

1. The user sends another message on the same local date.
2. 39claw routes the message to the current active same-day generation.
3. The response reflects same-day continuity.

Expected user perception:

- “The bot remembers today’s shared context.”
- “I do not need to restate everything.”

### Scenario: A same-day message arrives while another turn is still running

Expected flow:

1. The user sends a message for the current journal conversation while an earlier same-day turn is still executing.
2. 39claw accepts the later message into the active-generation waiting queue if capacity remains.
3. The bot immediately posts a short queued acknowledgment as a reply to the later message.
4. After the earlier turn completes, 39claw executes the queued message in the same daily thread.
5. The user receives the real answer later as a reply to the queued message.

Expected user perception:

- “The bot accepted my message instead of making me retry manually.”
- “The later answer still belongs to the message I actually sent.”

### Scenario: A user intentionally clears today's shared context

Expected flow:

1. A user invokes `/<instance-command> action:clear`.
2. 39claw checks whether the current active journal generation is idle.
3. If the generation is idle, 39claw rotates the shared same-day session to the next generation and confirms the reset.
4. The next normal message on the same local date targets that fresh generation.
5. Before the first visible reply on the new generation, 39claw may run a hidden memory-refresh preflight against the previous generation and update `AGENT_MEMORY/MEMORY.md` plus `AGENT_MEMORY/YYYY-MM-DD.<generation>.md`.

Expected user perception:

- “We intentionally started fresh for today.”
- “The next message uses a new thread without losing durable memory.”

### Scenario: First message on a new day

Expected flow:

1. The user sends a message after the local date has changed.
2. 39claw resolves a new journal bucket automatically.
3. If no active generation exists for the new bucket yet, 39claw creates generation `#1` automatically.
4. If no thread exists for the new generation and a previous recorded journal generation exists, 39claw first runs a hidden memory-refresh preflight against that previous thread.
5. The preflight updates `AGENT_MEMORY/MEMORY.md` plus today's `AGENT_MEMORY/YYYY-MM-DD.<generation>.md` note.
6. 39claw creates the new day's visible Codex thread.
7. The response begins from a fresh thread and may reflect durable remembered preferences or long-lived context when the deployment's own instructions tell Codex to consult the projected memory files.

Expected user perception:

- “Today feels like a fresh session.”
- “I still do not have to restate durable preferences every morning.”

## UX Requirements

### Message routing

The user should not have to manually select a thread for normal use in `journal` mode.

### Continuity boundaries

Continuity should be preserved:

- within the active generation on the same configured local date

Across different days, same-thread continuity should not be assumed, but durable memory may still be projected forward through the runtime-managed Markdown bridge.
Whether that projected memory affects normal visible turns depends on the deployment's own instructions rather than on 39claw rewriting user-owned instruction files.
Changing channels within the same bot instance should not reset the journal context by itself.

### Response tone

The bot should behave as if continuity is normal, not as if the user is performing a session-management action.

### Reset clarity

If a date-boundary reset or explicit `action:clear` reset causes confusion, the product may need a lightweight explanation, but this should not be the default for every fresh generation.

## Failure and Edge Cases

### Backend or storage failure

If the bot cannot resolve or create the required thread, it should:

- explain that it could not continue the conversation
- avoid leaking unnecessary internal detail
- tell the user whether retrying is likely to help

If the hidden new-day memory refresh fails, the bot should still continue with the visible reply instead of failing the whole user request.
If `action:clear` is requested while the active generation is still busy or has queued work, the bot should reject the clear request and tell the user to retry later.

### Timezone confusion

If the product exposes date-boundary behavior to users, it should do so in terms that map to the configured local timezone for the bot instance.

### Channel changes

If conversation continues in a different channel within the same bot instance, the product should preserve journal continuity rather than silently resetting it.
If that creates confusion for a specific deployment, the preferred solution is to separate bot instances by purpose rather than keying continuity by channel.

### Concurrent unrelated topics

If multiple unrelated discussions happen on the same day, `journal` mode may expose some shared context across those turns.
That tradeoff is acceptable when the product is intentionally operating as a shared assistant for a bounded group.
If multiple same-day requests stack up while one turn is already running, up to five waiting messages may queue before the bot falls back to a retry-later response.

## Non-Goals

`journal` mode is not intended to optimize for:

- long-lived project management
- explicit task switching
- reusing the same remote Codex thread across multiple local days

## Decisions

- The bot should not proactively mention that a new day created a fresh context.
- The bot may expose `/<instance-command> action:clear` so users can intentionally rotate the shared same-day generation.
- There should not be an explicit command for inspecting the current journal context in v1.
- The configured timezone should not be surfaced proactively to end users in normal journal-mode flow.
- The bot should not provide lightweight guidance when channel changes preserve continuity.
