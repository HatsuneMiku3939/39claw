# Implementation Spec

This document captures the concrete v1 implementation defaults for 39claw.
`ARCHITECTURE.md` remains the authoritative architecture reference, and the documents under `docs/product-specs` remain the source of truth for user-facing behavior.
This file exists to fix implementation choices that are still ambiguous in those higher-level documents so contributors can begin building without reopening the same decisions.

This document is intentionally short.
If implementation direction changes, update this file together with `ARCHITECTURE.md` and any affected product specs.
Large feature work may still use an ExecPlan after these defaults are set.

## Fixed v1 Implementation Defaults

39claw should be implemented as a thin runtime pipeline:

```text
Discord runtime -> application service -> thread policy -> SQLite store -> Codex gateway -> Discord presenter
```

The bot runs with one global configuration per instance:

- one thread mode
- one working directory
- one timezone

v1 does not introduce a local agent loop or local tool orchestration.
Codex remains responsible for remote thread execution and tool use.

The expected package direction for v1 is:

- `internal/config`
- `internal/observe`
- `internal/runtime/discord`
- `internal/app`
- `internal/thread`
- `internal/store/sqlite`
- `internal/codex`

The recommended delivery order is:

1. foundation and shared interfaces
2. `daily` mode routing and persistence
3. `task` mode state and command workflow
4. Discord command and presentation refinement

## Internal Interfaces to Freeze

The following internal contracts should be treated as stable v1 design targets even if exact Go type names evolve during implementation.

- `MessageRequest`
  - carries Discord user ID, channel ID, message ID, message content, mention or command metadata, and received time
- `MessageResponse`
  - carries rendered response text and presentation hints needed for Discord reply, chunking, and ephemeral command responses
- `ThreadPolicy`
  - resolves a logical thread key from the configured mode and the current message or task context
- `ThreadStore`
  - loads and upserts thread bindings and manages task records plus active task state
- `CodexGateway`
  - runs a turn against an existing Codex thread when a thread ID is present
  - creates the first remote thread implicitly when the first turn runs without a saved thread ID
  - returns a normalized final response plus the thread ID that should be persisted
- `TaskCommandService`
  - implements `/task current`, `/task list`, `/task new <name>`, `/task switch <id>`, and `/task close <id>`

The application layer should depend on these responsibilities rather than on Discord SDK details or raw SQL.

The concrete v1 message path lives in the application layer rather than in the Discord runtime.
The message service is responsible for:

- ignoring unsupported non-mention chatter
- resolving the logical thread key
- rejecting overlapping turns for the same logical thread key
- loading and upserting SQLite thread bindings
- calling the Codex gateway and returning a normalized reply payload

## Persistence Defaults

SQLite is the required v1 storage backend.

The storage model uses three tables:

- `thread_bindings`
  - stores `mode`, `logical_thread_key`, `codex_thread_id`, nullable `task_id`, `created_at`, and `updated_at`
  - enforces one binding per `(mode, logical_thread_key)`
- `tasks`
  - stores `task_id`, `discord_user_id`, `task_name`, `status`, `created_at`, `updated_at`, and nullable `closed_at`
  - uses ULID strings for `task_id`
  - allows duplicate task names for the same user
- `active_tasks`
  - stores `discord_user_id`, `task_id`, and `updated_at`
  - enforces one active task per Discord user within a bot instance

Task status is `open` or `closed`.
Closing a task marks it `closed` and removes its `active_tasks` mapping when that task is currently active.
`/task list` should show open tasks and clearly mark the active task for the requesting user.

The logical thread key defaults are:

- `daily`: configured local date formatted as `YYYY-MM-DD`
- `task`: `discord_user_id + task_id`

## Discord Behavior Defaults

Normal conversation is mention-only in v1.
When a qualifying normal message is handled, the bot replies in the same channel and targets the triggering message as the reply root.

`/help` and `/task ...` are slash-command surfaces.
Task-control command responses are ephemeral by default.
When a bot instance runs in `daily` mode, `/task ...` must return a clear not-available response instead of pretending the command worked.

When a bot instance runs in `task` mode, normal messages without an active task must not be routed to Codex.
They should return actionable guidance that points the user to `/task new <name>`, `/task list`, or `/task switch <id>`.

Unsupported non-mention chatter is ignored.
Long responses are chunked into Discord-safe messages while preserving code fences when practical.
Only one Codex turn may run at a time for a given logical thread key.
If another message arrives for the same logical thread while a turn is running, the bot should return a busy or retry response rather than queueing implicitly.

## Configuration Defaults

v1 configuration should be provided through environment variables.
The expected variables are:

- `CLAW_MODE`
- `CLAW_TIMEZONE`
- `CLAW_DISCORD_TOKEN`
- `CLAW_DISCORD_GUILD_ID`
- `CLAW_CODEX_WORKDIR`
- `CLAW_SQLITE_PATH`
- `CLAW_CODEX_EXECUTABLE`
- `CLAW_CODEX_BASE_URL`
- `CLAW_CODEX_API_KEY`
- `CLAW_CODEX_MODEL`
- `CLAW_CODEX_SANDBOX_MODE`
- `CLAW_CODEX_ADDITIONAL_DIRECTORIES`
- `CLAW_CODEX_SKIP_GIT_REPO_CHECK`
- `CLAW_CODEX_APPROVAL_POLICY`
- `CLAW_CODEX_MODEL_REASONING_EFFORT`
- `CLAW_CODEX_WEB_SEARCH_MODE`
- `CLAW_CODEX_NETWORK_ACCESS`
- `CLAW_LOG_LEVEL`

`CLAW_MODE`, `CLAW_TIMEZONE`, `CLAW_DISCORD_TOKEN`, `CLAW_CODEX_WORKDIR`, `CLAW_SQLITE_PATH`, and `CLAW_CODEX_EXECUTABLE` are required.
`CLAW_DISCORD_GUILD_ID`, `CLAW_CODEX_BASE_URL`, `CLAW_CODEX_API_KEY`, `CLAW_CODEX_MODEL`, `CLAW_CODEX_SANDBOX_MODE`, `CLAW_CODEX_ADDITIONAL_DIRECTORIES`, `CLAW_CODEX_SKIP_GIT_REPO_CHECK`, `CLAW_CODEX_APPROVAL_POLICY`, `CLAW_CODEX_MODEL_REASONING_EFFORT`, `CLAW_CODEX_WEB_SEARCH_MODE`, `CLAW_CODEX_NETWORK_ACCESS`, and `CLAW_LOG_LEVEL` are optional.
`CLAW_MODE` accepts `daily` or `task`.
`CLAW_TIMEZONE` must be set explicitly for each deployment.
`CLAW_LOG_LEVEL` defaults to `info` when omitted.
When `CLAW_DISCORD_GUILD_ID` is set, slash commands are overwritten in that guild for faster development feedback.
`CLAW_CODEX_SANDBOX_MODE` defaults to `workspace-write` when omitted.
`CLAW_CODEX_APPROVAL_POLICY` defaults to `never` when omitted.
`CLAW_CODEX_WEB_SEARCH_MODE` defaults to `live` when omitted.
`CLAW_CODEX_ADDITIONAL_DIRECTORIES` uses the OS path-list separator such as `:` on Unix systems.

## Validation Targets

The initial implementation should demonstrate the following observable behavior:

- In `daily` mode, the first qualifying mention creates a thread binding, a second same-day mention reuses it, and the first mention on the next local date creates a new binding.
- In `task` mode, a normal mention without an active task returns guidance instead of routing to Codex.
- `/task current` shows the active task for the requesting user.
- `/task new <name>` creates a task and sets it active for the requesting user.
- `/task switch <id>` changes the routing target for subsequent normal messages.
- `/task close <id>` closes the task and clears active state when the closed task was active.
- Existing `daily` and `task` bindings survive process restart through SQLite-backed state.
- Non-mention chatter is ignored, supported slash commands respond correctly, and long replies are chunked cleanly.
- Simultaneous requests for the same logical thread do not execute overlapping Codex turns.
