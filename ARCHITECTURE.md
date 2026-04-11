# 39claw Architecture Reference

This document is the architecture reference for 39claw.
It should be treated as the implementation guide for future development.

If code and this document diverge, either the code should be corrected or this document should be updated deliberately.

## How to Use This Document

`ARCHITECTURE.md` is the authoritative architecture document for this repository.
It defines the intended system role, core boundaries, thread-mode model, persistence direction, and v1 scope.

If another design note summarizes the architecture differently, this document takes precedence.

The companion document at `docs/design-docs/architecture-overview.md` is intentionally shorter.
It exists as an onboarding-oriented map for quickly understanding the system shape before reading this reference in full.

## 1. Project Identity

- Project: `39claw`
- Primary runtime: Discord bot
- Primary language: Go
- LLM backend: Codex only
- Architectural style: Codex-native orchestration with local thread persistence

## 2. Core Direction

39claw is not intended to be a fully local coding agent runtime.
It does not own the agent loop, tool execution loop, or conversation reasoning loop.

Instead:

- Codex owns remote thread execution
- Codex owns tool orchestration
- 39claw owns Discord integration
- 39claw owns thread routing policy
- 39claw owns local persistence for Codex thread bindings

The application should stay intentionally thin.

### 2.1 Codex Working Model

39claw adopts Codex's repository-scoped operating model and exposes it through Discord.

Codex works against a specific working directory, typically a Git repository, where it can:

- read files
- edit files
- execute shell commands
- follow repository-level instructions

39claw does not redefine that model locally.
Instead, it routes Discord interactions into Codex threads that operate against the repository configured for the current bot instance.

The distinction between `daily` mode and `task` mode is therefore not a different execution engine.
It is a difference in the role of the repository that Codex is operating against.

- In `task` mode, the configured workdir must be a Git repository with an `origin` remote. It remains the operator-visible source checkout and validation anchor, while 39claw creates task-isolated worktrees from its own managed bare parent repository under `CLAW_DATADIR`.
- Shared managed-repository mutation in `task` mode must be serialized per managed repository path within the process so concurrent task starts do not contend on the same bare parent, while already-ready task worktrees keep their existing task-level concurrency behavior.
- In `daily` mode, the repository is a knowledge-oriented repository that primarily contains instructions and documentation, allowing Codex to answer questions by following local guidance and searching the knowledge base.

Both modes share the same Codex-native foundation.
They differ in repository purpose, continuity policy, and resulting user experience.

## 3. Design Principles

### 3.1 Codex-native first

The system should integrate directly with Codex concepts such as remote threads and thread resume behavior instead of recreating a local agent abstraction first.

### 3.2 UX defines thread behavior

Thread policy is a product decision, not only a storage decision.
The way users experience continuity, reset, and task boundaries should drive the design.

### 3.3 One bot instance, one global thread mode

The bot should run with one globally configured thread mode.
Per-user or per-channel policy overrides are out of scope for the initial design.

If different behavior is needed, separate bot instances should be run.

### 3.4 Persistence is required

Because Codex thread IDs are necessary to resume conversations, local persistence is not optional.
Losing the mapping between a logical thread key and a Codex thread ID breaks continuity.

### 3.5 Keep the local application small

39claw should focus on:

- message intake
- thread resolution
- Codex thread binding
- response delivery

It should avoid unnecessary local orchestration layers in v1.

## 4. System Role

39claw is a stateful gateway between Discord conversations and Codex threads.

Its job is to:

1. receive a Discord message
2. determine which logical thread bucket it belongs to
3. resolve or create the corresponding Codex thread
4. send the turn to Codex
5. return the result to Discord

## 5. High-Level Architecture

```text
Discord Runtime
  -> Message Application Service
    -> Thread Policy
    -> Thread Store
    -> Queue Coordinator
    -> Codex Gateway
  -> Response Presenter
```

## 6. Main Components

### 6.1 Discord Runtime

Responsibilities:

- receive Discord messages and command interactions
- determine whether the bot should respond
- normalize input into application requests
- send formatted responses back to Discord
- surface best-effort streamed progress by editing the in-flight Discord reply while a non-queued Codex turn is still running
- own the runtime lifecycle for in-flight and queued background work during shutdown

The runtime must not directly talk to storage or Codex implementation details.

### 6.2 Message Application Service

This is the central orchestration use case.
It should process one incoming user turn from start to finish.

Responsibilities:

1. accept a normalized message request
2. ask the thread policy for a logical thread key
3. coordinate same-key execution and bounded queue admission
4. in `daily` mode, run the durable-memory preflight before the first visible turn of a new local day when the previous day's binding exists
5. load any existing binding from the thread store
6. call the Codex gateway with or without an existing thread ID
7. persist the returned thread ID when a new binding is created or updated
8. stream best-effort progress updates for immediate turns when the runtime requests them
9. return an immediate response and, when needed, deliver a deferred follow-up reply

### 6.3 Thread Policy

The thread policy converts message context into a logical thread key.

v1 must support two global modes:

- `daily`
- `task`

The policy layer should contain the mode-specific routing rules.

### 6.4 Thread Store

The thread store persists the mapping between:

- logical thread key
- Codex thread ID

In `task` mode, the thread store must also track task records, the currently selected task for a user within the current bot instance, and task worktree metadata such as branch name, worktree path, and worktree lifecycle state.

### 6.5 Queue Coordinator

The queue coordinator serializes work per logical thread key.

Responsibilities:

- allow the first turn for an idle key to execute immediately
- accept up to five additional waiting turns for the same key
- reject further turns once that waiting queue is full
- release the next queued turn in FIFO order when the current turn completes

The queue is intentionally in memory only.
It is not part of the durable SQLite state.

### 6.6 Codex Gateway

The Codex gateway wraps the Codex SDK or Codex integration layer.

Responsibilities:

- resume remote threads by ID
- send a turn to Codex
- accept the effective working directory for the turn
- return the resulting remote thread ID for persistence when the first turn creates it
- normalize Codex output for the application layer

All Codex-specific details should stay behind this boundary.

### 6.7 Response Presenter

The presenter adapts normalized application output to Discord-safe responses.

Responsibilities:

- Discord message formatting
- length trimming or chunking
- error-friendly output

## 7. Thread Modes

## 7.1 `daily`

Purpose:

- support lightweight daily continuity without explicit task management
- support shared, knowledge-oriented conversation against a repository that primarily contains instructions and documentation

Logical key concept:

```text
daily_bucket = local_date
active_thread_key = local_date + "#" + generation
```

Behavior:

- incoming messages automatically resolve to today's daily bucket
- each daily bucket has exactly one active shared generation at a time
- if a thread exists for the active generation key, resume it by passing the saved thread ID into the next turn
- otherwise run the first turn without a saved thread ID and persist the returned thread ID for that generation
- `/<instance-command> action:clear` rotates the active shared same-day generation to a fresh logical thread key
- when the date changes, the active generation resets to `#1` for the new bucket and the next visible turn starts a fresh Codex thread for that new key
- before the first visible turn of a new generation, 39claw resumes the previous recorded daily generation once and runs a runtime-managed durable-memory refresh into `${CLAW_CODEX_WORKDIR}/AGENT_MEMORY/MEMORY.md` plus `${CLAW_CODEX_WORKDIR}/AGENT_MEMORY/YYYY-MM-DD.<generation>.md`
- `action:clear` is rejected while the current active generation still has in-flight or queued work
- Codex answers by following repository guidance and consulting the documentation in that repository

Properties:

- no explicit thread command is required for normal usage
- simple and low-friction
- best for shared, conversation-oriented assistant flows
- same-day continuity comes from the currently active shared generation
- users can explicitly reset the shared same-day thread without waiting for the next day
- cross-day durable memory can be projected into Markdown files inside the workdir without 39claw modifying user-owned `AGENTS.md`
- whether normal visible turns consult those memory files is controlled by the user-owned instructions in the workdir

Tradeoffs:

- long-running work still crosses a fresh remote-thread boundary at the start of a new day
- unrelated same-day conversations may influence one another inside the shared daily context
- `action:clear` rotates the shared daily context for the whole bot instance rather than for one user
- timezone must be an explicit configuration concern
- the durable-memory bridge requires a write-capable Codex sandbox because 39claw must update files inside `CLAW_CODEX_WORKDIR`

## 7.2 `task`

Purpose:

- support longer-running work streams with explicit task identity
- support execution-oriented repository work through Discord using task-isolated Git worktrees

Logical key concept:

```text
thread_key = user + task_id
```

Behavior:

- `task` mode requires `CLAW_CODEX_WORKDIR` to be a Git repository with an `origin` remote
- normal messages require an active task context
- each task reserves its own branch identity in a managed bare parent repository under `${CLAW_DATADIR}/repos`
- messages route to the thread bound to the active task
- the first normal message for a task may lazily create the managed bare parent and the task worktree before Codex runs
- changing the active task changes the target thread
- `/<instance-command> action:task-reset-context` keeps the active task and worktree unchanged but removes only the saved Codex thread continuity for that task
- `action:task-reset-context` is rejected while the active task still has in-flight or queued work
- each task maps to a distinct Codex conversation thread, so switching tasks also switches execution context and working directory once the task worktree exists

Minimum v1 UX requirements:

- create a task
- select the active task
- inspect the active task
- reset the active task's Codex conversation continuity without recreating its workspace
- close the active task

Properties:

- better for project-oriented or issue-oriented work
- keeps context stable across days
- makes parallel long-running work practical without mixing task context

Tradeoffs:

- requires more explicit user interaction
- requires task-state persistence in addition to thread binding
- requires Git worktree lifecycle management and cleanup policy

## 8. Request Flow

```text
1. Discord receives a user message
2. Runtime normalizes the request
3. Application service resolves the logical thread key
4. Queue coordinator either starts the turn immediately, queues it, or rejects it when the waiting queue is full
5. If the turn starts immediately, the thread store looks up any existing Codex thread ID
6. If missing, Codex creates a new thread
7. The binding is persisted
8. Discord runtime posts either a queued acknowledgment or, for immediate turns, an editable placeholder reply
9. While a non-queued Codex turn is running, streamed Codex events may update that placeholder with progress or partial assistant text
10. If the turn was queued, the application service later executes it and the runtime posts the deferred reply
```

## 8.1 Concurrency Model

Concurrency is bounded per logical thread key.

- Different logical thread keys may execute Codex turns in parallel.
- A single logical thread key may have only one active Codex turn at a time.
- Each key may hold up to five additional waiting messages in an in-memory FIFO queue.
- Queued work is intentionally not durable across process restart.

### 8.2 Shutdown Semantics

Runtime shutdown should follow a bounded graceful-drain policy.

- The Discord runtime stops accepting new events before shutdown draining begins.
- In-flight immediate turns and already-queued turns may continue during a short graceful-drain window while the Discord session remains open.
- The runtime should wait up to five seconds for queued and in-flight work to finish cleanly.
- If the drain window expires, the runtime cancels the shared runtime context for outstanding work and drops any deferred replies that can no longer be delivered predictably.
- Logs must clearly distinguish queued work that completed, was canceled during shutdown, or was dropped because shutdown forced delivery to stop.

### 8.3 Observability

Runtime observability should start with structured logs.
JSON is the preferred default format so queue behavior, Codex turn latency, deferred delivery health, and token usage can be queried without a dedicated metrics pipeline.

The minimum high-value events are:

- `queue_admission`
  - capture `execute_now`, `queued`, and `queue_full` outcomes, plus queue position when queued
- `codex_turn_started`
  - capture whether an existing thread was resumed and the basic input shape
- `codex_turn_finished`
  - capture success or failure, latency, thread identity, and token usage when available
- `queued_turn_started` and `queued_turn_finished`
  - capture queue wait time and whether shutdown or delivery problems interrupted the queued path
- `deferred_reply_delivery`
  - capture success, normal failure, or shutdown-driven drop outcomes

## 9. Persistence Model

### 9.1 Required state

The minimum persistent state for v1 is:

- thread bindings
- active task selection for `task` mode
- task records with task worktree metadata for `task` mode

The bounded queued-message backlog is not persisted.
It exists only in memory while the process is running.

### 9.2 Binding concept

The core relationship is:

```text
logical_thread_key -> codex_thread_id
```

### 9.3 Storage direction

SQLite is the preferred v1 backend because it provides:

- simple local deployment
- restart-safe persistence
- easy lookup and update behavior
- no external infrastructure dependency

## 10. Intended v1 Scope

v1 should include:

- Discord runtime
- Codex integration
- global thread mode selection
- `daily` mode
- `task` mode
- local persistent thread binding
- local persistent active task state
- task-isolated Git worktrees for `task` mode
- structured logging with `log/slog`

v1 should not include:

- local agent loop implementation
- local tool orchestration
- multi-provider LLM support
- per-user or per-channel mode overrides
- web UI
- TUI runtime

## 11. Suggested Package Direction

The exact package structure may evolve, but the intended shape is:

```text
cmd/39claw
cmd/codexplay
internal/app
internal/runtime/discord
internal/thread
internal/store/sqlite
internal/codex
internal/config
internal/observe
version
```

Suggested responsibilities:

- `cmd/39claw`
  - application entrypoint
- `cmd/codexplay`
  - experimental CLI for manually exercising the Codex integration layer
- `internal/app`
  - top-level use cases and orchestration
- `internal/runtime/discord`
  - Discord-specific input and output handling
- `internal/thread`
  - thread policy and logical thread key handling
- `internal/store/sqlite`
  - SQLite-backed persistence
- `internal/codex`
  - Codex SDK integration layer
- `internal/config`
  - configuration loading and validation
- `internal/observe`
  - logging and operational observability helpers
- `version`
  - build version metadata shared by executable entrypoints

## 12. Relationship to Design Docs

The more exploratory design notes live under `docs/design-docs`.

Those files are useful for background and rationale.
This file is the authoritative reference document that should guide implementation decisions at the project root.

Relevant supporting documents:

- `docs/design-docs/index.md`
- `docs/design-docs/core-beliefs.md`
- `docs/design-docs/architecture-overview.md`
- `docs/design-docs/thread-modes.md`
- `docs/design-docs/state-and-storage.md`

## 13. Maintenance Rule

When architecture changes materially, this file should be updated in the same change whenever practical.
