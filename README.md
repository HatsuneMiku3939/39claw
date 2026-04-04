# 39claw

39claw is a Go-based, Codex-native Discord bot.

It acts as a thin bridge between Discord conversations and Codex threads.
The bot is designed to keep Discord integration, thread routing, and local persistence inside 39claw while leaving agent execution and tool orchestration to Codex.

## Current Status

39claw is still in an early development stage.
The repository now includes a real startup spine for `cmd/39claw`:

- environment-driven configuration in `internal/config`
- `slog` logger construction in `internal/observe`
- SQLite-backed state initialization in `internal/store/sqlite`
- application-layer contracts in `internal/app`
- thread-policy and execution-guard seams in `internal/thread`
- a higher-level Codex gateway in `internal/codex`
- a minimal Discord runtime shell in `internal/runtime/discord`

The Discord runtime is still intentionally thin in this stage.
It proves wiring, dependency boundaries, and graceful shutdown, but it does not yet implement the full `daily` or `task` interaction behavior.

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
- `docs/exec-plans`
  - living execution plans for active and completed implementation work
- `docs/references`
  - external reference material

## Developer Notes

For manual integration checks, use `cmd/codexplay` to exercise the adapter against the real `codex` CLI.
For the main bot bootstrap, configure the required environment variables from `docs/design-docs/implementation-spec.md` before running `go run ./cmd/39claw`.

Contributors can also run the standard local checks with the provided Make targets:

- `make test`
- `make lint`

The same checks run in GitHub Actions for pushes to `master` and for pull requests.
