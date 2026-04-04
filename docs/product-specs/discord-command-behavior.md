# Discord Command Behavior

Status: Draft

## Purpose

This document defines the intended Discord-facing interaction rules for 39bot.

Its job is to answer questions such as:

- what kinds of Discord input the bot should respond to
- when the bot should stay silent
- how task-control interactions should behave
- how responses should be formatted for Discord constraints

This document is product-facing.
It describes expected behavior, not implementation details.

## Product Goal

Users should be able to understand when 39bot will respond, how to control task-oriented behavior when needed, and what kind of reply shape to expect inside Discord.

## Scope

This document covers:

- supported Discord interaction types
- response trigger rules
- command behavior expectations
- user-facing error guidance
- Discord-specific output formatting expectations

This document does not define internal package responsibilities or storage implementation.

## Supported Interaction Types

The v1 product should be designed around a small, understandable set of Discord inputs.

### 1. Normal message interaction

Normal message interaction refers to messages that the bot is expected to treat as user turns.

Examples may include:

- direct mentions of the bot
- messages in channels where the bot is explicitly enabled
- replies to a bot-authored message, if that behavior is intentionally supported

### 2. Command interaction

Command interaction refers to explicit control operations used to manage bot state or workflow.

Examples may include:

- slash commands
- task management commands
- status or help commands

### 3. Non-supported or ignored interaction

The bot should ignore inputs that are outside the supported contract unless the product explicitly defines a response.

Examples may include:

- unrelated channel chatter
- malformed input that does not match any supported interaction type
- duplicate or accidental invocation where silence is more appropriate than noise

## Response Trigger Rules

### 1. The bot should not respond to everything

The product should define a clear response contract so users are not surprised by either over-eager replies or unexplained silence.

### 2. Normal conversation triggers should feel predictable

If the bot responds to normal conversation, the conditions should be easy for users to understand.

Examples:

- explicit mention required
- dedicated bot channel behavior
- reply-to-bot behavior

The exact trigger set can evolve, but it should stay explainable.

### 3. Command interactions should always be explicit

If a user invokes a task or control command, the bot should behave as a command surface, not as ambiguous free-form conversation.

### 4. Unsupported input should not create noisy UX

If the user input does not meet response criteria, the bot should usually stay silent unless a lightweight clarification is clearly better.

## Task Command Behavior

`task` mode requires explicit workflow control, so the task interaction surface should be especially clear.

### Minimum command capabilities

The product should support user-facing actions for:

- creating a task
- selecting the active task
- inspecting the active task
- clearing or closing the active task

### Expected command properties

Task commands should be:

- explicit
- easy to discover
- hard to misunderstand
- consistent in naming and output structure

### Success behavior

When a task command succeeds, the response should tell the user:

- what changed
- what the active task is now, if relevant
- what they can do next

### Failure behavior

When a task command fails, the response should:

- say what could not be completed
- explain the reason in user-facing language
- suggest the next action when possible

## Ambiguous Input Handling

### Missing task context

If `task` mode requires an active task and none exists, the bot should not guess.
It should tell the user what is missing and what command or action to use next.

### Ambiguous command intent

If the bot cannot confidently distinguish between a normal request and a control action, the product should prefer an explicit command surface over risky guessing.

### Conflicting state

If the current state makes an action unsafe or unclear, the bot should explain the conflict rather than silently doing the wrong thing.

## Output Formatting Expectations

Discord has practical display constraints, so the bot should format responses with readability in mind.

### Message length

If a response is too long for a single Discord message, the bot should:

- trim safely
- chunk cleanly
- preserve important context and formatting

The split should feel intentional, not broken.

### Code and structured output

When returning code, commands, or logs, the bot should use formatting that remains readable in Discord.

Examples:

- fenced code blocks for code
- short summaries before large technical output
- avoidance of excessively noisy raw dumps when a summary would be clearer

### Error messages

Error responses should be:

- short enough to scan
- specific enough to act on
- written in user-facing language

## Help and Discoverability

The product should make it easy for users to discover supported command behavior without reading internal documentation.

Possible mechanisms include:

- a help command
- task-mode guidance when task context is missing
- short affordance text after successful state-changing commands

## Non-Goals

This command behavior layer is not intended to:

- describe internal command parsing implementation
- define Discord permission infrastructure in technical detail
- lock in every slash command name before product decisions are stable

## Open Questions

- Which normal-message triggers should be supported in v1: mention-only, dedicated channels, replies, or some combination?
- Should `daily` mode and `task` mode expose different help text or command hints?
- Which task controls should be slash commands versus natural-language guidance flows?
- Should the bot prefer silent ignore or lightweight feedback for unsupported invocation patterns?
