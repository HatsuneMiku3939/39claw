# Discord Command Behavior

Status: Active

## Purpose

This document defines the intended Discord-facing interaction rules for 39claw.

Its job is to answer questions such as:

- what kinds of Discord input the bot should respond to
- when the bot should stay silent
- how task-control interactions should behave
- how responses should be formatted for Discord constraints

This document is product-facing.
It describes expected behavior, not implementation details.

## Product Goal

Users should be able to understand when 39claw will respond, how to control task-oriented behavior when needed, and what kind of reply shape to expect inside Discord.

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
- `/task ...` task management commands for `task` mode bot instances
- `/help`

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

v1 should use mention-only triggering for normal-message interaction.

### 3. Command interactions should always be explicit

If a user invokes a task or control command, the bot should behave as a command surface, not as ambiguous free-form conversation.

### 4. Unsupported input should not create noisy UX

If the user input does not meet response criteria, the bot should stay silent.

## Task Command Behavior

`task` mode requires explicit workflow control, so the task interaction surface should be especially clear.

### Command family decision

For v1, task-control interactions should be exposed through a `/task ...` command family.

This means:

- bot instances configured for `task` mode should accept `/task ...` commands
- bot instances configured for `daily` mode should not expose task-control behavior as normal workflow
- `/help` should remain available as a general command surface
- users should not need to guess between natural-language task control and slash-command task control

### Minimum command capabilities

The `/task ...` command family should support user-facing actions for:

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

### Expected command properties

`/task ...` commands should be:

- explicit
- easy to discover
- hard to misunderstand
- consistent in naming and output structure

`/help` should:

- list the supported command surface
- explain that `/task ...` is available only in `task` mode bot instances
- help users recover from missing-context situations without reading internal docs

### Success behavior

When a `/task ...` command succeeds, the response should tell the user:

- what changed
- what the active task is now, if relevant
- what they can do next

When `/help` succeeds, the response should give a concise list of supported commands and a short explanation of when to use them.

### Failure behavior

When a `/task ...` command fails, the response should:

- say what could not be completed
- explain the reason in user-facing language
- suggest the next action when possible

If `/task ...` is invoked against a bot instance running in `daily` mode, the bot should explain that task commands are not available for that instance rather than pretending the command was accepted.

## Ambiguous Input Handling

### Missing task context

If `task` mode requires an active task and none exists, the bot should not guess.
It should tell the user what is missing and what `/task ...` command or action to use next.

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

- a `/help` command
- task-mode guidance that points users toward `/task ...` commands when task context is missing
- short affordance text after successful state-changing commands

`/help` does not need mode-specific variation in v1.
`daily` mode bot instances should simply avoid exposing unsupported slash command behavior.

## Non-Goals

This command behavior layer is not intended to:

- describe internal command parsing implementation
- define Discord permission infrastructure in technical detail
- keep task-control behavior ambiguous between free-form conversation and explicit commands

## Decisions

- v1 normal-message triggering should be mention-only.
- `daily` mode and `task` mode do not need different help text in v1; `daily` mode simply does not expose slash-command workflow.
- Unsupported invocation patterns should be ignored rather than acknowledged with lightweight feedback.
