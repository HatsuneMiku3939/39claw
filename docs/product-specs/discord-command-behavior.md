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
- one instance-specific root command such as `/release`
- action choices such as `help` or `task-new`

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
When the bot is mentioned, a normal message may contain typed text, one or more image attachments, or both.
If the mention is present but the message contains neither text nor a usable image attachment, the bot should stay silent.
If the turn starts immediately, the bot may first post a short placeholder reply and then edit that same reply as Codex streams progress or partial assistant output.
If another turn for the same logical conversation is already running, the bot should acknowledge queued acceptance immediately and post the real answer later as a reply to the original triggering message.
If five waiting messages are already queued for that logical conversation, the bot should return a clear retry-later response instead of queueing more work.

### 3. Command interactions should always be explicit

If a user invokes a task or control command, the bot should behave as a command surface, not as ambiguous free-form conversation.

### 4. Unsupported input should not create noisy UX

If the user input does not meet response criteria, the bot should stay silent.

## Task Command Behavior

`task` mode requires explicit workflow control, so the task interaction surface should be especially clear.

### Root command decision

For v1, task-control interactions should be exposed through one instance-specific root command.
The command name should come from the bot-instance configuration, so different deployments can expose different root commands inside the same Discord server.

This means:

- every bot instance should expose exactly one slash-command search result
- bot instances configured for `task` mode should expose task actions through that root command
- bot instances configured for `daily` mode should expose `help` plus `clear` through that root command
- users should not need to guess between natural-language task control and slash-command task control

### Minimum command capabilities

The root command should support user-facing actions for:

- `/<instance-command> action:task-current`
  - show the current task name and ID
- `/<instance-command> action:task-list`
  - show task names and IDs
- `/<instance-command> action:task-new task_name:<name>`
  - create a new task and switch the active task to it
- `/<instance-command> action:task-switch task_id:<id>`
  - switch the active task to the specified task
- `/<instance-command> action:task-close task_id:<id>`
  - close the specified task

Every instance should also support:

- `/<instance-command> action:help`
  - show the commands available for that bot instance

Instances running in `daily` mode should also support:

- `/<instance-command> action:clear`
  - rotate the shared same-day daily session to a fresh generation when the current one is idle

### Expected command properties

The root-command action surface should be:

- explicit
- easy to discover
- hard to misunderstand
- consistent in naming and output structure

`action:help` should:

- list the supported command surface for the current bot instance
- explain task actions only when that workflow is available for the current bot instance
- help users recover from missing-context situations without reading internal docs

### Success behavior

When a task action succeeds, the response should tell the user:

- what changed
- what the active task is now, if relevant
- what they can do next

For `action:task-new`, the success response should set the expectation that the first normal message may prepare the task workspace before Codex answers.

For `action:task-close`, the success response does not need to describe background worktree pruning in detail, but the product behavior should remain consistent with the task retention policy.

When `action:help` succeeds, the response should give a concise list of supported actions and a short explanation of when to use them.

When `action:clear` succeeds in `daily` mode, the response should tell the user that the next mention will use a fresh shared same-day thread.
When `action:clear` is rejected because the current shared generation is busy or queued, the response should explain that the user needs to retry after the current work finishes.

### Failure behavior

When a task action fails, the response should:

- say what could not be completed
- explain the reason in user-facing language
- suggest the next action when possible

If a task action is invoked against a bot instance running in `daily` mode, the bot should explain that task actions are not available for that instance rather than pretending the command was accepted.

## Ambiguous Input Handling

### Missing task context

If `task` mode requires an active task and none exists, the bot should not guess.
It should tell the user what is missing and what `/<instance-command> action:task-*` command to use next.

### Workspace preparation failure

If a normal message targets a task whose workspace cannot be prepared yet, the bot should not claim that Codex processed the turn.
It should explain that the task workspace could not be prepared and tell the user to retry.

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

When responses reference local workspace files, the Discord-facing text should avoid exposing the configured absolute `CLAW_CODEX_WORKDIR`.
Workspace-local file references should be rewritten into readable workspace-relative paths, and percent-encoded path segments should be decoded before they are shown to users.

### Error messages

Error responses should be:

- short enough to scan
- specific enough to act on
- written in user-facing language

Queued acknowledgments should also stay short and explicit.
They should make it clear that the message was accepted and that the final answer will arrive later.
For immediate non-queued turns, streamed progress updates should feel like one continuously improving reply rather than a burst of separate bot messages.

## Help and Discoverability

The product should make it easy for users to discover supported command behavior without reading internal documentation.

Possible mechanisms include:

- an instance-specific root command with `action:help`
- a `daily`-mode `action:clear` response that confirms the next mention will start fresh
- task-mode guidance that points users toward task actions when task context is missing
- short affordance text after successful state-changing commands

`action:help` does not need deep mode-specific customization in v1.
It should still reflect the commands that are actually available in the current bot instance.
`daily` mode bot instances should avoid advertising unsupported task workflow.
In the Discord slash-command UI, task-control behavior should appear as action choices under one root command rather than as multiple searchable leaf commands.

## Non-Goals

This command behavior layer is not intended to:

- describe internal command parsing implementation
- define Discord permission infrastructure in technical detail
- keep task-control behavior ambiguous between free-form conversation and explicit commands

## Decisions

- v1 normal-message triggering should be mention-only.
- each bot instance should register exactly one slash command whose name identifies that instance in Discord search.
- `action:help` should stay structurally simple in v1, but it should only describe commands that are actually available in the current bot instance.
- `daily` mode may expose `action:clear` on that same root command so users can intentionally rotate the shared same-day generation.
- Unsupported invocation patterns should be ignored rather than acknowledged with lightweight feedback.
- In `task` mode, the configured workdir is a Git repository source for task worktrees rather than only one shared execution directory.
