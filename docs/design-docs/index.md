# 39bot Design Docs

This directory captures the current concept-level design of 39bot.

The project direction is intentionally small and opinionated:

- 39bot is a Codex-native Discord bot.
- Codex is responsible for the agent loop and tool orchestration.
- 39bot is responsible for routing user messages into the correct Codex thread.
- Thread behavior is selected by a single global mode per bot instance.

## Documents

- [Core Beliefs](./core-beliefs.md)
- [Architecture Overview](./architecture-overview.md)
- [Thread Modes](./thread-modes.md)
- [State and Storage](./state-and-storage.md)

## Current v1 Direction

- Main runtime: Discord
- LLM backend: Codex only
- Supported thread modes:
  - `daily`
  - `task`
- Configuration scope: global per bot instance
- Persistent local storage: required for Codex thread binding

## Notes

These documents describe the current design direction, not a final implementation contract.
