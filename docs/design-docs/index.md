# 39claw Design Docs

This directory captures the current concept-level design of 39claw.

These documents are companions to the root `ARCHITECTURE.md`, not replacements for it.
Use `ARCHITECTURE.md` as the authoritative architecture reference and use the files in this directory as focused supporting notes.

The project direction is intentionally small and opinionated:

- 39claw is a Codex-native Discord bot.
- Codex is responsible for the agent loop and tool orchestration.
- 39claw is responsible for routing user messages into the correct Codex thread.
- Thread behavior is selected by a single global mode per bot instance.

## Documents

- [Core Beliefs](./core-beliefs.md) - explains the project principles behind the design
- [Architecture Overview](./architecture-overview.md) - provides a short onboarding-oriented map of the system shape
- [Implementation Spec](./implementation-spec.md) - fixes the concrete v1 implementation defaults that sit between the architecture and product specs
- [Thread Modes](./thread-modes.md) - explains the mode model, behavior, and tradeoffs
- [State and Storage](./state-and-storage.md) - explains persistence requirements and storage boundaries

## Current v1 Direction

- Main runtime: Discord
- LLM backend: Codex only
- Supported thread modes:
  - `daily`
  - `task`
- Configuration scope: global per bot instance
- Persistent local storage: required for Codex thread binding

## Notes

These documents describe the current design direction and should stay aligned with `ARCHITECTURE.md`.
