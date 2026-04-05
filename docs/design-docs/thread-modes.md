# Thread Modes

This document defines the two planned v1 thread modes for 39claw.

The selected mode is a global configuration value for each bot instance.

## Mode A: `daily`

### Intent

`daily` mode is designed for lightweight, natural conversation flow.
The user should not need to manage thread state explicitly.

### Thread Key Concept

The logical thread key is derived from:

- current local date

Conceptually:

```text
thread_key = local_date
```

### Behavior

- when a message arrives, 39claw computes the key automatically
- if a Codex thread is already bound to that key, 39claw resumes it
- if no binding exists, 39claw creates a new Codex thread
- when the date changes, a new logical thread is used automatically
- before the first visible turn for that new day, 39claw may resume the previous day's Codex thread once to refresh durable Markdown memory under `AGENT_MEMORY/`
- the visible turn for the new day still starts a fresh Codex thread even when durable memory is carried forward

### UX Properties

- no explicit thread command is required for normal use
- continuity exists within the same day
- the remote thread resets naturally across days
- durable preferences or long-lived context can still carry forward through runtime-managed memory files

### Tradeoffs

Benefits:

- simple and predictable
- low-friction user experience
- supports a shared-assistant feel for a bounded team environment
- very easy to explain

Costs:

- long-running work may still be split across remote thread boundaries
- unrelated same-day conversations may share context
- the definition of "day" depends on a configured timezone
- the durable-memory bridge depends on a write-capable Codex sandbox and managed files inside the configured workdir

## Mode B: `task`

### Intent

`task` mode is designed for longer-running, explicit work streams.
The user should be able to keep context attached to a named task instead of a date bucket.
Each task should also map to an isolated Git worktree instead of sharing one mutable checkout.

### Thread Key Concept

The logical thread key is derived from:

- user
- explicit task identity

Conceptually:

```text
thread_key = user + task_id
```

### Behavior

- the configured `CLAW_CODEX_WORKDIR` must be a Git repository
- a current task must exist before normal messages can be routed
- `task-new` creates task metadata immediately but defers worktree creation
- the first normal message for a task creates the task worktree lazily when needed
- messages are sent to the Codex thread associated with the active task and use that task's worktree once it exists
- changing tasks changes the target logical thread
- closed-task worktrees are retained only for the fifteen most recently closed ready tasks

### UX Requirements

Unlike `daily`, `task` mode requires task control commands or interactions.

At a minimum, v1 likely needs:

- create a task
- select the current task
- inspect the current task
- clear or close the current task context

### UX Properties

- better for multi-day or focused project work
- more explicit and durable than calendar-based continuity

### Tradeoffs

Benefits:

- strong control over context boundaries
- better fit for issue-based or project-based workflows
- task switching changes both conversation context and filesystem workspace

Costs:

- more operational and UX complexity
- requires explicit task lifecycle management
- requires Git-only startup validation for `task` mode
- requires worktree creation, retry, and pruning behavior

## Why Both Modes Exist

Both modes are built on the same Codex operating model.
Codex works against a repository-scoped working directory and follows the instructions defined there.

The difference is the role of the repository:

- `daily` uses a knowledge-oriented repository that primarily contains instructions and documentation
- `task` uses an execution-oriented repository where Codex can help perform real operational work

As a result, the modes support different user experiences:

- `daily` is conversation-oriented
- `task` is work-oriented

Supporting both allows 39claw to serve different operating styles without changing the Codex-native backend model.
