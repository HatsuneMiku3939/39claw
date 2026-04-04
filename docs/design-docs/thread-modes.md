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

### UX Properties

- no explicit thread command is required for normal use
- continuity exists within the same day
- context resets naturally across days

### Tradeoffs

Benefits:

- simple and predictable
- low-friction user experience
- supports a shared-assistant feel for a bounded team environment
- very easy to explain

Costs:

- long-running work may be split across calendar boundaries
- unrelated same-day conversations may share context
- the definition of "day" depends on a configured timezone

## Mode B: `task`

### Intent

`task` mode is designed for longer-running, explicit work streams.
The user should be able to keep context attached to a named task instead of a date bucket.

### Thread Key Concept

The logical thread key is derived from:

- user
- explicit task identity

Conceptually:

```text
thread_key = user + task_id
```

### Behavior

- a current task must exist before normal messages can be routed
- messages are sent to the Codex thread associated with the active task
- changing tasks changes the target logical thread

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

Costs:

- more operational and UX complexity
- requires explicit task lifecycle management

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
