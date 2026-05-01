# Thread Modes

This document defines the two planned v1 thread modes for 39claw.

The selected mode is a global configuration value for each bot instance.

## Mode A: `journal`

### Intent

`journal` mode is designed for lightweight, natural conversation flow.
The user should not need to manage thread state explicitly.

### Thread Key Concept

The journal routing bucket is derived from:

- current local date

Conceptually:

```text
daily_bucket = local_date
active_thread_key = local_date + "#" + generation
```

### Behavior

- when a message arrives, 39claw computes the current local-date bucket automatically
- each local-date bucket has exactly one active shared generation at a time
- if a Codex thread is already bound to the active generation key, 39claw resumes it
- if no binding exists for the active generation key, 39claw creates a new Codex thread
- `/<instance-command> action:clear` rotates the active shared generation to a fresh same-day key such as `YYYY-MM-DD#2`
- if the active shared generation still has in-flight or queued work, `action:clear` is rejected instead of interleaving old and new replies
- when the date changes, a new bucket is used automatically and its first active generation starts at `#1`
- before the first visible turn for a new generation, 39claw may resume the previous recorded journal generation once to refresh durable Markdown memory under `AGENT_MEMORY/`
- the visible turn for a new generation still starts a fresh Codex thread even when durable memory is carried forward

### UX Properties

- no explicit thread command is required for normal use
- continuity exists within the currently active same-day generation
- users can intentionally reset the shared same-day thread without waiting for midnight
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
- resetting the shared same-day thread affects the whole bot instance, not just the user who issued `action:clear`
- the definition of "day" depends on a configured timezone
- the durable-memory bridge depends on a write-capable Codex sandbox and managed files inside the configured workdir

## Mode B: `thread`

### Intent

`thread` mode is designed for longer-running, explicit work streams.
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

- the configured `CLAW_CODEX_WORKDIR` must be a Git repository with an `origin` remote
- task names are immutable routing-safe slugs and must be unique among the requesting user's open tasks
- a current task must exist before normal messages can be routed unless the message provides a valid one-shot `task:<name>` override
- `task-new` creates task metadata immediately but defers worktree creation
- the first normal message for a task creates or refreshes the managed bare parent under `${CLAW_DATADIR}/repos` and then creates the task worktree lazily when needed
- messages are sent to the Codex thread associated with the active task by default and use that task's worktree once it exists
- a valid one-shot `task:<name>` prefix routes only the current message to another open task and does not change the saved active task
- changing tasks changes the default target logical thread for later normal messages that do not provide an override
- `/<instance-command> action:task-reset-context` keeps the current task active and the task worktree unchanged while dropping only the saved Codex thread binding for that task
- `action:task-reset-context` is rejected while that task has in-flight or queued work
- closed-task worktrees are retained only for the fifteen most recently closed ready tasks

### UX Requirements

Unlike `journal`, `thread` mode requires task control commands or interactions.

At a minimum, v1 likely needs:

- create a task
- select the current task
- inspect the current task
- reset only the current task's Codex conversation continuity
- close the current task

### UX Properties

- better for multi-day or focused project work
- more explicit and durable than calendar-based continuity

### Tradeoffs

Benefits:

- strong control over context boundaries
- better fit for issue-based or project-based workflows
- task switching changes both conversation context and filesystem workspace
- users can intentionally restart the Codex conversation for one task without rebuilding its workspace

Costs:

- more operational and UX complexity
- requires explicit task lifecycle management
- requires Git-only startup validation for `thread` mode
- requires worktree creation, retry, and pruning behavior

## Why Both Modes Exist

Both modes are built on the same Codex operating model.
Codex works against a repository-scoped working directory and follows the instructions defined there.

The difference is the role of the repository:

- `journal` uses a knowledge-oriented repository that primarily contains instructions and documentation
- `thread` uses an execution-oriented repository where Codex can help perform real operational work

As a result, the modes support different user experiences:

- `journal` is conversation-oriented
- `thread` is work-oriented

Supporting both allows 39claw to serve different operating styles without changing the Codex-native backend model.
