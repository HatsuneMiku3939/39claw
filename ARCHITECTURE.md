# 39bot Architecture Reference

This document is the architecture reference for 39bot.
It should be treated as the implementation guide for future development.

If code and this document diverge, either the code should be corrected or this document should be updated deliberately.

## 1. Project Identity

- Project: `39bot`
- Primary runtime: Discord bot
- Primary language: Go
- LLM backend: Codex only
- Architectural style: Codex-native orchestration with local thread persistence

## 2. Core Direction

39bot is not intended to be a fully local coding agent runtime.
It does not own the agent loop, tool execution loop, or conversation reasoning loop.

Instead:

- Codex owns remote thread execution
- Codex owns tool orchestration
- 39bot owns Discord integration
- 39bot owns thread routing policy
- 39bot owns local persistence for Codex thread bindings

The application should stay intentionally thin.

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

39bot should focus on:

- message intake
- thread resolution
- Codex thread binding
- response delivery

It should avoid unnecessary local orchestration layers in v1.

## 4. System Role

39bot is a stateful gateway between Discord conversations and Codex threads.

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

The runtime must not directly talk to storage or Codex implementation details.

### 6.2 Message Application Service

This is the central orchestration use case.
It should process one incoming user turn from start to finish.

Responsibilities:

1. accept a normalized message request
2. ask the thread policy for a logical thread key
3. load any existing binding from the thread store
4. create a Codex thread if no binding exists
5. send the user turn through the Codex gateway
6. return a normalized response for presentation

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

In `task` mode, a separate state store is also needed to track the currently selected task for a user within the current bot instance.

### 6.5 Codex Gateway

The Codex gateway wraps the Codex SDK or Codex integration layer.

Responsibilities:

- create remote threads
- resume remote threads by ID
- send a turn to Codex
- normalize Codex output for the application layer

All Codex-specific details should stay behind this boundary.

### 6.6 Response Presenter

The presenter adapts normalized application output to Discord-safe responses.

Responsibilities:

- Discord message formatting
- length trimming or chunking
- error-friendly output

## 7. Thread Modes

## 7.1 `daily`

Purpose:

- support lightweight daily continuity without explicit task management

Logical key concept:

```text
thread_key = user + local_date
```

Behavior:

- incoming messages automatically resolve to today's bucket
- if a thread exists for that key, resume it
- otherwise create a new Codex thread
- when the date changes, the logical bucket changes automatically

Properties:

- no explicit thread command is required for normal usage
- simple and low-friction
- best for conversation-oriented flows

Tradeoffs:

- long-running work may be split across date boundaries
- timezone must be an explicit configuration concern

## 7.2 `task`

Purpose:

- support longer-running work streams with explicit task identity

Logical key concept:

```text
thread_key = user + task_id
```

Behavior:

- normal messages require an active task context
- messages route to the thread bound to the active task
- changing the active task changes the target thread

Minimum v1 UX requirements:

- create a task
- select the active task
- inspect the active task
- clear or close the active task

Properties:

- better for project-oriented or issue-oriented work
- keeps context stable across days

Tradeoffs:

- requires more explicit user interaction
- requires task-state persistence in addition to thread binding

## 8. Request Flow

```text
1. Discord receives a user message
2. Runtime normalizes the request
3. Application service resolves the logical thread key
4. Thread store looks up an existing Codex thread ID
5. If missing, Codex gateway creates a new thread
6. The new binding is persisted
7. Application service sends the user turn to Codex
8. Response presenter formats the result
9. Discord runtime posts the reply
```

## 9. Persistence Model

### 9.1 Required state

The minimum persistent state for v1 is:

- thread bindings
- active task selection for `task` mode

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
cmd/39bot
cmd/codexplay
internal/app
internal/runtime/discord
internal/thread
internal/store/sqlite
internal/codex
internal/config
internal/observe
```

Suggested responsibilities:

- `cmd/39bot`
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
  - logging and observability support

## 12. Relationship to Design Docs

The more exploratory design notes live under `docs/design-docs`.

Those files are useful for background and rationale.
This file is the shorter reference document that should guide implementation decisions at the project root.

Relevant supporting documents:

- `docs/design-docs/index.md`
- `docs/design-docs/core-beliefs.md`
- `docs/design-docs/architecture-overview.md`
- `docs/design-docs/thread-modes.md`
- `docs/design-docs/state-and-storage.md`

## 13. Maintenance Rule

When architecture changes materially, this file should be updated in the same change whenever practical.
