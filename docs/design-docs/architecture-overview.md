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
  - knowledge-oriented interaction against repository instructions and documentation, with one active shared generation per local day plus a runtime-managed durable-memory bridge under `AGENT_MEMORY/`
- `task`
  - execution-oriented interaction against an operator-visible Git checkout with an `origin` remote, where 39claw manages a separate bare parent repository and each task eventually runs inside its own task-specific worktree

The detailed rationale for these modes lives in `ARCHITECTURE.md` and `thread-modes.md`.

## High-Level Components

```text
Discord Runtime
  -> Message Application Service
    -> Thread Policy
    -> Thread Store
    -> Queue Coordinator
    -> Codex Gateway
  -> Response Presenter
```

## Component Responsibilities

### Discord Runtime

Receives Discord inputs, delivers formatted responses, and edits in-flight non-queued replies when streamed Codex progress is available.

### Message Application Service

Processes one user turn end to end by resolving the thread target, coordinating same-key queue admission, delegating to Codex, and returning either the final normalized result or an immediate queued acknowledgment.

### Thread Policy

Converts Discord context into a logical thread bucket according to the globally configured mode.
In `daily` mode, the policy still resolves only the local date and the application layer expands that bucket into the active generation key.

### Thread Store

Persists the local continuity data that lets 39claw resume the correct Codex thread, plus task and task-worktree metadata in `task` mode.

### Queue Coordinator

Serializes work per logical thread key.
The first turn for an idle key runs immediately, up to five additional waiting turns may queue in memory, and further turns receive a retry-later response until capacity returns.

### Codex Gateway

Owns the direct integration with the Codex SDK or Codex API layer, including the effective working directory for each turn and the translation of streamed Codex events into app-facing progress updates.

### Response Presenter

Adapts normalized application output into Discord-safe responses.

## Request Flow

```text
1. Discord receives a user message
2. Runtime normalizes the request
3. Application service resolves the logical thread key and any frozen routing context needed for later queued execution
4. Queue coordinator decides whether the turn runs immediately, waits in the same-key queue, or is rejected because five waiting turns already exist
5. If the turn was queued, the runtime immediately posts a queued acknowledgment reply
6. When the turn starts running, `daily` mode may first resolve or create the active same-day generation and then run a hidden preflight refresh against the previous recorded daily generation to update `AGENT_MEMORY`
7. Thread store checks whether a Codex thread already exists for the visible generation key
8. Application service sends the turn through the Codex gateway with the saved thread ID when one exists
9. If no saved thread exists yet, the first turn creates one and returns its thread ID
10. Application service persists the returned binding
11. For non-queued work, the runtime may post and edit a placeholder reply while the turn is still running
12. Response presenter formats the result
13. Discord runtime posts the final response immediately for non-queued work or later as a deferred reply for queued work

The runtime-owned part stops at creating and refreshing the memory files themselves.
Whether Codex consults those files during normal visible turns is controlled by the user-owned instructions already present in the workdir, such as `AGENTS.md`.
```

## Concurrency Model

- Different logical thread keys may still make progress in parallel.
- Work for the same logical thread key is serialized through the queue coordinator.
- The waiting queue is intentionally in memory only, so queued turns are lost if the process exits before they run.
- The short overview here stays subordinate to `ARCHITECTURE.md`, which remains the authoritative source for the full request flow and shutdown behavior.

## Validation View

The shipping runtime is currently Discord, but the validation strategy should already separate reusable product behavior from Discord-specific external-platform hardening.

- automated coverage should be the primary source of confidence
  - cover message qualification, ignored-message rules, logical-thread resolution handoff, queue admission, queued acknowledgments, deferred reply handoff, command normalization, and normalized response expectations at the app/runtime boundary
- adapter-level fake runtime tests should drive representative runtime events and capture the visible outputs without requiring a live Discord deployment
- live Discord smoke remains a narrow optional hardening layer for command-registration propagation, hosted attachment fetches, permission quirks, and final delivery edge cases

This keeps the current Discord implementation practical while making the validation shape easier to reuse if future runtimes such as Slack or Telegram are introduced later.

## Read Next

- root `ARCHITECTURE.md`
  - authoritative architecture reference
- `thread-modes.md`
  - mode definitions, behavior, and tradeoffs
- `state-and-storage.md`
  - persistence model and storage expectations
