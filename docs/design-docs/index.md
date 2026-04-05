# 39claw Design Docs

This directory captures the current concept-level design of 39claw.

These documents are companions to the root `ARCHITECTURE.md`, not replacements for it.
Use `ARCHITECTURE.md` as the authoritative architecture reference and use the files in this directory as focused supporting notes.

The project direction is intentionally small and opinionated:

- 39claw is a Codex-native Discord bot.
- Codex is responsible for the agent loop and tool orchestration.
- 39claw is responsible for routing user messages into the correct Codex thread.
- 39claw serializes same-key work locally through a queue coordinator and may reply later when queued work drains.
- Thread behavior is selected by a single global mode per bot instance.

## Documents

- [Core Beliefs](./core-beliefs.md) - explains the project principles behind the design
- [Architecture Overview](./architecture-overview.md) - provides a short onboarding-oriented map of the system shape, queue coordinator, and queued request flow
- [Implementation Spec](./implementation-spec.md) - fixes the concrete v1 implementation defaults that sit between the architecture and product specs
- [First-Stage Release Automation](./first-stage-release-automation.md) - explains how the reusable Go release skill is adapted into 39claw's minimal draft-release flow
- [Thread Modes](./thread-modes.md) - explains the mode model, behavior, and tradeoffs
- [State and Storage](./state-and-storage.md) - explains persistence requirements and storage boundaries
- [Task Mode Worktrees](./task-mode-worktrees.md) - defines task-isolated Git worktrees, lazy creation, and closed-task pruning

## Current v1 Direction

- Main runtime: Discord
- LLM backend: Codex only
- Supported thread modes:
  - `daily`
  - `task`
- Configuration scope: global per bot instance
- Persistent local storage: required for Codex thread binding
- Same-key concurrency: bounded in-memory queueing with deferred replies for queued turns

## Notes

These documents describe the current design direction and should stay aligned with `ARCHITECTURE.md`.
