# Architecture Overview

This document describes the concept-level application architecture for 39bot.

## System Role

39bot is a stateful gateway between Discord conversations and Codex threads.

It does not act as a full local coding agent runtime.
Instead, it delegates agent execution to Codex and manages the local application-side policy.

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

The Discord runtime is responsible for:

- receiving messages and interaction commands
- deciding whether the bot should respond
- passing normalized requests into the application service
- sending formatted responses back to Discord

### Message Application Service

This is the main orchestration layer.
Its responsibility is to process one incoming user turn from start to finish.

At a concept level, it should:

1. receive a normalized user message
2. ask the thread policy for the logical thread key
3. load any existing Codex thread binding from storage
4. create a new Codex thread if needed
5. send the user turn to Codex
6. present the result back to Discord

### Thread Policy

The thread policy converts a Discord message context into a logical thread key.

The policy depends on the global thread mode.
Two v1 modes are planned:

- `daily`
- `task`

### Thread Store

The thread store persists the relationship between:

- a logical thread key
- a Codex thread ID

This is the local continuity layer that allows the bot to resume the correct remote conversation.

### Codex Gateway

The Codex gateway is the only component that talks to the Codex SDK or Codex API integration layer.

Its responsibilities include:

- creating threads
- resuming existing threads by ID
- sending a turn into a thread
- returning the assistant result in a normalized application format

### Response Presenter

The presenter adapts Codex responses for Discord output.

It should handle:

- message formatting
- trimming or chunking if Discord limits are exceeded
- error-friendly responses when backend requests fail

## Request Flow

```text
1. Discord receives a user message
2. Runtime normalizes the request
3. Application service asks the thread policy for a thread key
4. Thread store checks whether a Codex thread already exists
5. If not found, Codex gateway creates a new thread
6. Application service persists the new binding
7. Codex gateway sends the user turn
8. Response presenter formats the result
9. Discord runtime posts the reply
```

## What v1 Intentionally Does Not Include

The following concerns are intentionally outside the first concept:

- local agent loop implementation
- local tool orchestration
- multi-provider LLM support
- per-user or per-channel policy overrides
- web or TUI runtime surfaces

## Suggested Package Shape

The exact package layout may change, but the current direction is roughly:

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

## Bootstrap Status

The repository currently contains a minimal executable entrypoint at `cmd/39bot/main.go`.
This file is only a bootstrap placeholder and does not yet implement Discord runtime wiring or Codex integration.

The repository also includes an initial `internal/codex` package that experiments with direct Codex CLI integration in Go.
Its current scope is intentionally narrow and focused on thread start or resume behavior, streamed event handling, and local image input support.

An additional experimental CLI entrypoint at `cmd/codexplay` is available for manual integration checks against the real `codex` binary.
