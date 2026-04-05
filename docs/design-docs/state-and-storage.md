# State and Storage

This document describes the concept-level local state required by 39claw.

## Why Local State Exists

Codex manages remote conversation threads, but 39claw still needs local state to know which remote thread should receive a new user message.

That means 39claw must persist thread bindings.

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

### 2. Durable Daily Memory Files

This state is required in `daily` mode.

It lives inside the configured Codex workdir instead of SQLite:

- `${CLAW_CODEX_WORKDIR}/AGENT_MEMORY/MEMORY.md`
- `${CLAW_CODEX_WORKDIR}/AGENT_MEMORY/YYYY-MM-DD.md`

`MEMORY.md` is the compact primary memory file that carries durable facts such as user preferences or long-lived workflow context across local-day boundaries.
The dated file records what the new-day preflight promoted, updated, or rejected when it resumed the previous daily Codex thread.

### 3. Active Task State

This state is only required for `task` mode.

It stores the currently selected task identity for a user within the current bot instance.

Conceptually:

```text
user -> active_task_id
```

This allows ordinary messages to be routed without forcing the user to repeat the task identifier in every message.

### 4. Task Worktree Metadata

This state is only required for `task` mode.

Each task needs enough metadata to create and later manage an isolated Git worktree.

That includes:

- reserved branch name
- detected base ref
- worktree path
- worktree lifecycle state
- creation, prune, and last-used timestamps

This metadata lets 39claw decide whether a task needs lazy worktree creation, whether a closed task is eligible for pruning, and which working directory Codex should use for the next turn.

## Storage Direction for v1

SQLite is the preferred v1 storage backend.

Reasons:

- simple local deployment
- persistence across restarts
- safe updates and queries
- no need for an external database service

## Schema Direction

The current concept points toward:

- `thread_bindings`
- `tasks`
- `active_tasks`

SQLite remains the source of truth for remote thread bindings and task metadata.
The `AGENT_MEMORY` Markdown files are intentionally stored in the Codex workdir so Codex can read and update them directly during the daily memory-bridge preflight.

The design should favor explicit structured columns over packing all data into a single opaque key string, unless implementation simplicity proves more valuable.

## Non-Goals for v1

The following are not currently required:

- full local transcript storage for every message
- analytics-oriented event warehousing
- cross-instance distributed coordination
- pruning or deleting task branches automatically

The initial storage model should stay focused on continuity and thread routing.
