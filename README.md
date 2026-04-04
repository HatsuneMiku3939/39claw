# 39claw

39claw is a Go-based, Codex-native Discord bot.

It acts as a thin bridge between Discord conversations and Codex threads.
The bot is designed to keep Discord integration, thread routing, and local persistence inside 39claw while leaving agent execution and tool orchestration to Codex.

## Current Status

39claw is still in an early development stage.
The repository currently includes a minimal bootstrap executable at `cmd/39claw/main.go` and an experimental Go Codex adapter under `internal/codex`.

The current direction is documented in the root architecture and product documents rather than in the executable surface alone.
For the intended system shape, thread model, and user-facing behavior, start with the documents linked below.

## Documentation

- `ARCHITECTURE.md`
  - authoritative architecture reference for the repository
- `docs/index.md`
  - entry point for the documentation set
- `docs/product-specs`
  - user-facing journeys, command behavior, and workflow expectations
- `docs/design-docs`
  - supporting architecture and design notes
- `docs/references`
  - external reference material

## Developer Notes

For manual integration checks, use `cmd/codexplay` to exercise the adapter against the real `codex` CLI.
Contributors can also run the standard local checks with the provided Make targets:

- `make test`
- `make lint`

The same checks run in GitHub Actions for pushes to `master` and for pull requests.
