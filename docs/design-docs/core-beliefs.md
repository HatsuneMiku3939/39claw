# Core Beliefs

This document records the design beliefs that currently shape 39claw.

## 1. 39claw should be Codex-native

39claw should not re-implement an agent loop if Codex already provides one.

This means:

- Codex owns the conversational thread lifecycle on the remote side.
- Codex handles tool calling and internal loop progression.
- 39claw focuses on user experience, thread routing, and platform integration.

## 2. User experience should drive thread policy

The most important product question is not how to build an agent loop.
It is how users should experience continuity, context, and reset behavior.

This means thread design must be based on UX expectations such as:

- when context should continue
- when context should reset
- how predictable the behavior feels inside Discord

## 3. 39claw should be a thin orchestrator

39claw should remain a small application that coordinates a few responsibilities well:

- receive Discord messages
- resolve the target Codex thread
- create a thread when needed
- send the turn to Codex
- return the result to Discord
- persist thread bindings

## 4. Runtime policy should be simple and predictable

The initial product should avoid per-user, per-channel, or dynamic policy switching.

Instead:

- each bot instance runs with one global thread mode
- different behaviors can be delivered by running separate bot instances

This keeps operational behavior easy to reason about and reduces support complexity.

## 5. Persistence is a product feature, not an implementation detail

Because Codex thread IDs are required for resume behavior, persistence is part of the product model.

If the local application loses the mapping between a logical thread key and a Codex thread ID:

- continuity breaks
- resume fails
- the user experience becomes inconsistent

For that reason, persistent storage is required even in early versions.

## 6. v1 should optimize for clarity over flexibility

v1 should support the intended experience clearly before adding broad configurability.

This means:

- Codex only, no multi-provider abstraction pressure
- Discord first, no extra runtime surfaces yet
- a small set of thread modes
- simple, explicit operational behavior
