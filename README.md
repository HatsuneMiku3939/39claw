# 39bot

39bot is a Go-based, Codex-native Discord bot.

## Current Status

The repository currently includes a minimal bootstrap executable at `cmd/39bot/main.go`.
It prints a dummy `hello world` message and exists only to validate the initial Go module, test, and lint workflow.

## Development Checks

Run local checks with the provided Make targets:

- `make test`
- `make lint`

The same checks run in GitHub Actions for pushes to `master` and for pull requests.
