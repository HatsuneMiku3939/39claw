# Architecture Overview

This document is a short onboarding-oriented summary of the 39claw architecture.

It is not the authoritative implementation reference.
For architectural decisions, scope boundaries, and source-of-truth behavior, see the root `ARCHITECTURE.md`.

Use this document when you want a quick mental model of the system before reading the full reference.

## System Role

39claw is a stateful gateway between Discord conversations and Codex threads.

It does not act as a full local coding agent runtime.
Instead, it delegates agent execution to Codex and manages the local application-side policy.

## Codex Working Model

39claw adopts Codex's repository-scoped operating model.
Each bot instance is configured against a repository-shaped working directory, and Discord interactions are routed into Codex threads that operate against that repository.

This leads to two distinct mode families on the same foundation:

- `daily`
  - knowledge-oriented interaction against repository instructions and documentation, with fresh daily threads plus a runtime-managed durable-memory bridge under `AGENT_MEMORY/`
- `task`
  - execution-oriented interaction against a Git work repository where each task eventually runs inside its own task-specific worktree

The detailed rationale for these modes lives in `ARCHITECTURE.md` and `thread-modes.md`.

## High-Level Components

```text
Discord Runtime
  -> Message Application Service
    -> Thread Policy
    -> Thread Store
    -> Codex Gateway
  -> Response Presenter
```

## Component Responsibilities

### Discord Runtime

Receives Discord inputs and delivers formatted responses.

### Message Application Service

Processes one user turn end to end by resolving the thread target, delegating to Codex, and returning the normalized result.

### Thread Policy

Converts Discord context into a logical thread key according to the globally configured mode.

### Thread Store

Persists the local continuity data that lets 39claw resume the correct Codex thread, plus task and task-worktree metadata in `task` mode.

### Codex Gateway

Owns the direct integration with the Codex SDK or Codex API layer, including the effective working directory for each turn.

### Response Presenter

Adapts normalized application output into Discord-safe responses.

## Request Flow

```text
1. Discord receives a user message
2. Runtime normalizes the request
3. Application service asks the thread policy for a thread key
4. In `daily` mode, the first turn of a new local day may run a hidden preflight refresh against the previous daily thread to update `AGENT_MEMORY`
5. Thread store checks whether a Codex thread already exists for the visible turn
6. Application service sends the turn through the Codex gateway with the saved thread ID when one exists
7. If no saved thread exists yet, the first turn creates one and returns its thread ID
8. Application service persists the returned binding
9. Response presenter formats the result
10. Discord runtime posts the reply

The runtime-owned part stops at creating and refreshing the memory files themselves.
Whether Codex consults those files during normal visible turns is controlled by the user-owned instructions already present in the workdir, such as `AGENTS.md`.
```

## Read Next

- root `ARCHITECTURE.md`
  - authoritative architecture reference
- `thread-modes.md`
  - mode definitions, behavior, and tradeoffs
- `state-and-storage.md`
  - persistence model and storage expectations
