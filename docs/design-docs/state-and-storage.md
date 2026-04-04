# State and Storage

This document describes the concept-level local state required by 39bot.

## Why Local State Exists

Codex manages remote conversation threads, but 39bot still needs local state to know which remote thread should receive a new user message.

That means 39bot must persist thread bindings.

## Primary Persistence Need

The core binding is:

```text
logical_thread_key -> codex_thread_id
```

Without this mapping:

- the bot cannot resume the correct Codex thread
- continuity becomes unreliable after restart

## Required v1 State

### 1. Thread Bindings

This state exists in both `daily` and `task` modes.

It stores:

- logical thread key
- Codex thread ID
- creation time
- last update time

### 2. Active Task State

This state is only required for `task` mode.

It stores the currently selected task identity for a user within the current bot instance.

Conceptually:

```text
user -> active_task_id
```

This allows ordinary messages to be routed without forcing the user to repeat the task identifier in every message.

## Storage Direction for v1

SQLite is the preferred v1 storage backend.

Reasons:

- simple local deployment
- persistence across restarts
- safe updates and queries
- no need for an external database service

## Schema Direction

The exact schema is not fixed yet, but the current concept points toward:

- `thread_bindings`
- `active_tasks`

The design should favor explicit structured columns over packing all data into a single opaque key string, unless implementation simplicity proves more valuable.

## Non-Goals for v1

The following are not currently required:

- full local transcript storage for every message
- analytics-oriented event warehousing
- cross-instance distributed coordination

The initial storage model should stay focused on continuity and thread routing.
