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
- one source working directory
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
- `version`

The recommended delivery order is:

1. foundation and shared interfaces
2. `journal` mode routing and persistence
3. `thread` mode state and command workflow
4. Discord command and presentation refinement
5. scheduled task orchestration, persistence, and delivery

## Internal Interfaces to Freeze

The following internal contracts should be treated as stable v1 design targets even if exact Go type names evolve during implementation.

- `MessageRequest`
  - carries Discord user ID, channel ID, message ID, optional message content, optional local image paths, mention or command metadata, and received time
- `MessageResponse`
  - carries rendered response text and presentation hints needed for Discord reply, chunking, and ephemeral command responses
- `ThreadPolicy`
  - resolves a logical thread key from the configured mode and the current message or task context
- `ThreadStore`
  - loads, upserts, and deletes thread bindings and manages task records plus active task state
- `ScheduledTaskStore`
  - manages `scheduled_tasks`, `scheduled_task_runs`, and `scheduled_task_deliveries` records
- `CodexGateway`
  - runs a turn against an existing Codex thread when a thread ID is present
  - creates the first remote thread implicitly when the first turn runs without a saved thread ID
  - returns a normalized final response plus the thread ID that should be persisted
- `ScheduledTaskService`
  - owns due-run claiming, execution handoff, execute-now execution, and Discord report delivery recording
- `ScheduledTaskMCPServer`
  - implements MCP-backed conversational create, list, get, update, enable, disable, delete, and execute-now workflows for scheduled tasks
  - validates tool input and mutates scheduled-task definitions through `ScheduledTaskStore`
- `TaskCommandService`
  - implements the `action:task-current`, `action:task-list`, `action:task-new`, `action:task-switch`, `action:task-close`, and `action:task-reset-context` workflow behind the configured root command
- `DailyCommandService`
  - implements the `action:clear` workflow behind the configured root command when the bot instance runs in `journal` mode

The application layer should depend on these responsibilities rather than on Discord SDK details or raw SQL.

The concrete v1 message path lives in the application layer rather than in the Discord runtime.
The message service is responsible for:

- ignoring unsupported non-mention chatter
- resolving the logical thread bucket and any active journal generation metadata
- rejecting overlapping turns for the same logical thread key
- running the journal durable-memory preflight before the first visible turn of a new journal generation when the previous generation's thread binding exists
- loading and upserting SQLite thread bindings
- calling the Codex gateway and returning a normalized reply payload

## Persistence Defaults

SQLite is the required v1 storage backend.

Schema evolution should use embedded, versioned, up-only SQLite migrations tracked through a dedicated `schema_migrations` table rather than relying on ad hoc startup-only column checks inside store CRUD code.

The storage model uses seven tables:

- `thread_bindings`
  - stores `mode`, `logical_thread_key`, `codex_thread_id`, nullable `task_id`, `created_at`, and `updated_at`
  - enforces one binding per `(mode, logical_thread_key)`
- `daily_sessions`
  - stores `local_date`, `generation`, `logical_thread_key`, nullable `previous_logical_thread_key`, `activation_reason`, `is_active`, `created_at`, and `updated_at`
  - enforces one active row per `local_date`
- `tasks`
  - stores `task_id`, `discord_user_id`, `task_name`, `status`, `branch_name`, nullable `base_ref`, nullable `worktree_path`, `worktree_status`, `created_at`, `updated_at`, nullable `closed_at`, nullable `worktree_created_at`, nullable `worktree_pruned_at`, and nullable `last_used_at`
  - uses ULID strings for `task_id`
  - stores `task_name` as the canonical immutable routing-safe slug for the task
  - requires task names to match `^[a-z](?:[a-z0-9-]{1,30}[a-z0-9])?$` without consecutive hyphens
  - requires each user's open task names to be unique, while still allowing a closed task's former name to be reused later
- `active_tasks`
  - stores `discord_user_id`, `task_id`, and `updated_at`
  - enforces one active task per Discord user within a bot instance
- `scheduled_tasks`
  - stores `scheduled_task_id`, `name`, `schedule_kind`, `schedule_expr`, `prompt`, `enabled`, nullable `report_target`, `created_at`, `updated_at`, and nullable `disabled_at`
  - uses ULID strings for `scheduled_task_id`
  - stores the canonical schedule definition that 39claw owns locally
- `scheduled_task_runs`
  - stores `scheduled_run_id`, `scheduled_task_id`, `mode`, `scheduled_for`, `attempt`, `status`, nullable `codex_thread_id`, nullable `workdir_path`, nullable `temp_worktree_path`, nullable `started_at`, nullable `finished_at`, nullable `error_code`, nullable `error_message`, nullable `response_text`, `created_at`, and `updated_at`
  - records each scheduled execution attempt before Codex dispatch begins
- `scheduled_task_deliveries`
  - stores `scheduled_delivery_id`, `scheduled_run_id`, `report_target`, nullable `discord_message_id`, `status`, nullable `delivered_at`, nullable `error_code`, nullable `error_message`, `created_at`, and `updated_at`
  - tracks whether a completed run report reached Discord successfully

Task status is `open` or `closed`.
Task worktree status is `pending`, `ready`, `failed`, or `pruned`.
Closing a task marks it `closed` and removes its `active_tasks` mapping when that task is currently active.
`action:task-list` should show open tasks and clearly mark the active task for the requesting user.
Deleting a thread-mode thread binding must not change the task record, active-task mapping, branch name, or worktree path. That operation is how `action:task-reset-context` clears only Codex conversation continuity.

The logical thread key defaults are:

- `journal`: configured local date formatted as `YYYY-MM-DD` for the outer bucket, with the active visible thread key normalized to `YYYY-MM-DD#<generation>`
- `thread`: `discord_user_id + task_id`

In `thread` mode, the effective task for a normal message is the saved active task unless the first meaningful token supplies a one-shot `task:<name>` override.
That override may follow leading whitespace or the guild-channel bot mention, may be followed by body text on the same line or the next line, and applies only to the current message.
Queue admission, thread binding lookup, and worktree selection must freeze that effective task at acceptance time so later task switches do not reroute already-accepted work.

When the bot runs in `journal` mode, 39claw also manages a durable memory projection inside `${CLAW_CODEX_WORKDIR}/AGENT_MEMORY`.
`MEMORY.md` is the primary durable-memory file, and `YYYY-MM-DD.<generation>.md` stores the bridge note created during the first-message preflight for a new journal generation.
Scheduled tasks are independent from interactive message routing and own their own persisted definition plus run lifecycle.
Scheduled-task management is not part of the Discord slash-command surface; it is exposed to Codex through a local 39claw-owned MCP server and managed conversationally through the `scheduled_tasks_*` tool family.
When a scheduled task executes in `journal` mode, it uses `CLAW_CODEX_WORKDIR` directly.
When a scheduled task executes in `thread` mode, it must create a fresh temporary worktree from the managed bare parent, run Codex there, and remove that temporary worktree after the run completes.

## Discord Behavior Defaults

Normal conversation is mention-only in guild channels and direct-message-triggered in DMs in v1.
When a qualifying normal message is handled, the bot replies in the same channel and targets the triggering message as the reply root.
Qualifying normal messages may include text, image attachments, or both as long as the guild-channel bot mention is present or the message arrived in a direct message, and at least one usable input remains after attachment filtering.
When a normal-message reply finishes successfully, 39claw should add a `✅` reaction to the primary bot reply message as a best-effort completion marker when Discord permissions allow it.

Each bot instance should expose one slash-command surface whose root name comes from `CLAW_DISCORD_COMMAND_NAME`.
That root command should always expose `action:help`.
Task-control command responses are ephemeral by default.
When a bot instance runs in `journal` mode, task actions must return a clear not-available response instead of pretending the command worked.
When a bot instance runs in `journal` mode, the root command should also expose `action:clear`.
When a bot instance runs in `thread` mode, the root command should expose `action:task-current`, `action:task-list`, `action:task-new`, `action:task-switch`, `action:task-close`, and `action:task-reset-context`.
Scheduled-task management should be available in both modes through MCP-backed conversational management because scheduled execution is orthogonal to the interactive thread mode.

When a bot instance runs in `thread` mode, normal messages without an active task must not be routed to Codex.
They should return actionable guidance that points the user to `action:task-new`, `action:task-list`, or `action:task-switch` on the configured root command.
When a bot instance runs in `journal` mode, the first visible turn of a new journal generation should still start a fresh Codex thread, but 39claw must first run a hidden durable-memory refresh against the previous recorded generation's thread when that previous binding exists.
If that preflight fails or times out, 39claw should log the failure and continue with the visible turn instead of blocking the user.
If `action:clear` is invoked while the current active journal generation still has in-flight or queued work, 39claw should reject the clear request with an ephemeral retry-later response instead of rotating immediately.
39claw must not create or modify user-owned instruction files such as `AGENTS.md`; if a deployment wants visible turns to consult `AGENT_MEMORY`, the deployment must express that through its own checked-in instructions.
When a bot instance runs in `thread` mode, `CLAW_CODEX_WORKDIR` must be a Git repository with an `origin` remote.
`task-new` creates task metadata only and reserves a branch name derived from the task name with Git-safe normalization; if normalization produces no usable suffix, it falls back to the task ID. The first normal message for a pending or failed task creates the task worktree lazily from a managed bare parent under `${CLAW_DATADIR}/repos`, preferring the remote default branch by trying `origin/HEAD`, then `origin/main`, then `origin/master`, and only then falling back to local `main` or `master` inside that managed repository.
Before detecting that base ref, 39claw should synchronize the managed bare parent against the source checkout's `origin` configuration and try `git fetch origin --prune`; a fetch failure should not block task execution by itself when cached remote refs are already available.
Those managed-parent mutation steps should be serialized per managed repository path within the running process so concurrent task starts do not overlap on one shared bare parent.
Once the task worktree is ready, Codex runs with the task-specific `worktree_path` as the effective working directory for that turn.
`action:task-reset-context` deletes the saved thread binding for the active task only when that task has no in-flight or queued work, so the next normal message starts a fresh Codex thread in the same worktree.
Closed tasks keep their task branches in the managed bare parent, but only the fifteen most recently closed ready tasks keep their worktrees; older closed ready worktrees are force-pruned.

Unsupported guild-channel non-mention chatter is ignored.
Qualifying posts that contain no text and no usable image attachments are also ignored.
Long responses are chunked into Discord-safe messages while preserving code fences when practical.
Before a response is sent to Discord, local workspace file references should be rewritten so the absolute `CLAW_CODEX_WORKDIR` path is not exposed, and percent-encoded path segments should be decoded for display.
Failure to add the completion reaction must not fail or delay reply delivery.
Only one Codex turn may run at a time for a given logical thread key.
If another message arrives for the same logical thread while a turn is running, the bot should accept up to five waiting messages in an in-memory FIFO queue.
Queued messages should receive an immediate acknowledgment and later receive their final answer as a follow-up reply to the original message.
If five waiting messages already exist for that logical thread key, the bot should return a retry-later response instead of accepting more queued work.
During runtime shutdown, 39claw should stop accepting new Discord events first, keep the Discord session open while already-admitted work drains for up to five seconds, and then cancel the shared runtime context if draining does not finish in time.
When shutdown forces cancellation, deferred queued replies may be dropped instead of being delivered after the runtime has started closing.
Structured logs should make it obvious whether queued work completed normally, was canceled during shutdown, or had its deferred reply dropped.
The default runtime log format should be JSON so operators can search and aggregate queue, latency, deferred-delivery, and token-usage events without a separate metrics system.
The minimum high-value events are `queue_admission`, `codex_turn_started`, `codex_turn_finished`, `queued_turn_started`, `queued_turn_finished`, and `deferred_reply_delivery`.

## Configuration Defaults

v1 configuration should be provided through environment variables.
The expected variables are:

- `CLAW_MODE`
- `CLAW_TIMEZONE`
- `CLAW_DISCORD_TOKEN`
- `CLAW_DISCORD_COMMAND_NAME`
- `CLAW_DISCORD_GUILD_ID`
- `CLAW_CODEX_WORKDIR`
- `CLAW_DATADIR`
- `CLAW_CODEX_EXECUTABLE`
- `CLAW_CODEX_HOME`
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
- `CLAW_LOG_FORMAT`
- `CLAW_SCHEDULED_REPORT_TARGET`

`CLAW_MODE`, `CLAW_TIMEZONE`, `CLAW_DISCORD_TOKEN`, `CLAW_DISCORD_COMMAND_NAME`, `CLAW_CODEX_WORKDIR`, `CLAW_DATADIR`, and `CLAW_CODEX_EXECUTABLE` are required.
`CLAW_DISCORD_GUILD_ID`, `CLAW_CODEX_BASE_URL`, `CLAW_CODEX_API_KEY`, `CLAW_CODEX_HOME`, `CLAW_CODEX_MODEL`, `CLAW_CODEX_SANDBOX_MODE`, `CLAW_CODEX_ADDITIONAL_DIRECTORIES`, `CLAW_CODEX_SKIP_GIT_REPO_CHECK`, `CLAW_CODEX_APPROVAL_POLICY`, `CLAW_CODEX_MODEL_REASONING_EFFORT`, `CLAW_CODEX_WEB_SEARCH_MODE`, `CLAW_CODEX_NETWORK_ACCESS`, `CLAW_LOG_LEVEL`, `CLAW_LOG_FORMAT`, and `CLAW_SCHEDULED_REPORT_TARGET` are optional.
`CLAW_MODE` accepts `journal` or `thread`.
`CLAW_TIMEZONE` must be set explicitly for each deployment.
`CLAW_DISCORD_COMMAND_NAME` must be unique per bot instance, normalized to lowercase, and validated conservatively before Discord registration.
When `CLAW_CODEX_HOME` is set, 39claw must inject it into the spawned Codex CLI process as `CODEX_HOME`.
When `CLAW_MODE=thread`, `CLAW_CODEX_WORKDIR` must point to a Git repository with an `origin` remote and acts as the operator-visible source checkout plus validation target for thread-mode startup.
Task-mode startup and first-use preparation must maintain a managed bare parent repository under `${CLAW_DATADIR}/repos`, and task worktrees must be created from that managed parent rather than directly from the visible source checkout.
When `CLAW_MODE=journal`, startup must materialize the managed durable-memory skill and the `AGENT_MEMORY` directory inside `CLAW_CODEX_WORKDIR`.
When `CLAW_SCHEDULED_REPORT_TARGET` is set, it becomes the default `report_target` for scheduled tasks that do not override the destination explicitly. Accepted formats are `channel:<id>` and `dm:<user_id>`.
`CLAW_LOG_LEVEL` defaults to `info` when omitted.
When `CLAW_DISCORD_GUILD_ID` is set, slash commands are overwritten in that guild for faster development feedback.
`CLAW_CODEX_SANDBOX_MODE` defaults to `workspace-write` when omitted.
`journal` mode does not support `read-only` sandboxing because the durable-memory bridge must write inside `CLAW_CODEX_WORKDIR`.
`CLAW_CODEX_APPROVAL_POLICY` defaults to `never` when omitted.
`CLAW_CODEX_WEB_SEARCH_MODE` defaults to `live` when omitted.
`CLAW_LOG_FORMAT` defaults to `json` when omitted and may be set to `text` for local debugging.
`CLAW_CODEX_ADDITIONAL_DIRECTORIES` uses the OS path-list separator such as `:` on Unix systems.
`CLAW_DATADIR` points to a directory, and the SQLite database file is always stored as `39claw.sqlite` inside that directory.
For local development, the safe-default workflow is an ignored `.env.local` file loaded through an ignored `.envrc`.
Checked-in examples such as `.env.example` and `.envrc.example` must use placeholders only and must not contain live credentials.

## Validation Strategy Defaults

The primary validation story should be reusable automated coverage rather than broad Discord-only live smoke testing.
Even though v1 ships only a Discord runtime, validation should already be organized so future runtimes such as Slack or Telegram can reuse the same categories.

The repository should treat validation as three layers:

- runtime-agnostic behavior contracts
  - cover the application and runtime boundary behaviors that should remain stable regardless of the current chat platform
  - include qualifying message intake, ignored-message conditions, logical-thread handoff, queue admission outcomes, queued acknowledgment behavior, deferred reply delivery handoff, command-intent normalization, and normalized response expectations at the app/runtime boundary
- adapter-level fake runtime coverage
  - simulate platform inputs and capture runtime-visible outputs without depending on a live Discord deployment
  - cover representative normal-message events, command-style intents, attachment metadata inputs, reply targets, payload text, visibility hints, and deferred-delivery timing semantics
  - keep the shared fake-runtime vocabulary under `internal/testutil/runtimeharness`
  - keep the Discord contract-style suite under `internal/runtime/discord` and make `go test ./internal/runtime/discord -run 'TestRuntimeContract' -v` the first runtime-specific check before optional live hardening
- live-platform hardening
  - stay narrow and optional instead of acting as the primary quality gate
  - focus on external-platform behaviors such as real command-registration propagation, hosted attachment fetches, permission or intent quirks, and final delivery edge cases

## Validation Targets

The initial implementation should demonstrate the following observable behavior.
Most of these outcomes should be proven through automated contract coverage plus fake runtime tests, while the narrow live-platform remainder should be handled as optional hardening:

- In `journal` mode, the first qualifying mention creates generation `#1`, a second same-day mention reuses the active generation, and the first mention on the next local date creates a fresh `#1` generation after the durable-memory preflight refreshes `AGENT_MEMORY` from the last active prior-day generation when that previous binding exists.
- In `journal` mode, `/<instance-command> action:clear` rotates the shared same-day generation only when the current generation is idle, and the next mention creates or resumes a fresh same-day binding after the durable-memory preflight refreshes `AGENT_MEMORY` from the previous recorded generation when that previous binding exists.
- In `journal` mode, startup does not create or rewrite `AGENTS.md`.
- A guild mention or direct message with text plus image attachments reaches Codex as multipart input.
- A guild mention or direct message with only one or more usable image attachments is accepted and answered.
- In `thread` mode, a normal mention without an active task returns guidance instead of routing to Codex unless that message provides a valid one-shot `task:<name>` override.
- `/<instance-command> action:task-current` shows the active task for the requesting user.
- `/<instance-command> action:task-new task_name:<name>` creates a task with a routing-safe slug name and sets it active for the requesting user.
- The first normal message for a new task creates or refreshes a managed bare parent under `${CLAW_DATADIR}/repos`, then creates a task worktree lazily under `${CLAW_DATADIR}/worktrees/<task_id>`, and then runs Codex inside that worktree.
- `/<instance-command> action:task-switch task_name:<name>` changes the default routing target for later normal messages that do not provide an override, with `task_id` reserved only for compatibility with pre-slug tasks during migration.
- `/<instance-command> action:task-close task_name:<name>` closes the uniquely named open task and clears active state when the closed task was active.
- `/<instance-command> action:task-reset-context` keeps the active task and worktree but clears only the saved Codex thread continuity for that task.
- In `thread` mode, `task:<name> <body>` and `task:<name>` followed by a newline route only the current message to the named open task without changing the active task.
- In `thread` mode, `task:<name>` with attachments but no body is valid, while a message that has neither body nor attachments is rejected.
- Invalid task-name format, missing tasks, closed tasks, and policy failures in one-shot override handling return explicit user-facing guidance instead of guessing a target.
- Closed-task worktree retention keeps only the fifteen most recently closed ready worktrees and never deletes the task branches held by the managed bare parent.
- Existing `journal` and `thread` bindings survive process restart through SQLite-backed state.
- Scheduled task definitions survive restart, execute at most once per claimed due time, and persist Discord delivery outcomes separately from Codex execution results.
- Guild non-mention chatter is ignored, unsupported non-image-only qualifying posts stay silent, supported slash commands respond correctly, and long replies are chunked cleanly.
- Simultaneous requests for the same logical thread do not execute overlapping Codex turns.
- Simultaneous requests for the same logical thread receive queued acknowledgments and later deferred replies until the waiting queue reaches five items.

Live Discord hardening remains useful only for the smaller set of behaviors that require the real platform, such as:

- command-registration propagation in Discord
- hosted attachment fetch behavior with real Discord-hosted files
- permission or intent configuration quirks in a deployed bot
- final message delivery edge cases that a fake runtime cannot reproduce faithfully
