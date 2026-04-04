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
- a real Discord runtime adapter in `internal/runtime/discord`

The runtime now handles:

- mention-only normal conversation
- `/help`
- `/task current`, `/task list`, `/task new <name>`, `/task switch <id>`, and `/task close <id>`
- same-channel reply targeting for normal conversation
- ephemeral task-control responses
- Discord-safe response chunking with fenced-code preservation

The current test-backed behavior includes:

- mention-only handling versus ignored chatter
- same-day thread reuse
- next-day rollover
- task command orchestration for showing, listing, creating, switching, and closing tasks
- durable active-task state and open-task records in SQLite
- task-mode guidance when a normal mention arrives without an active task
- task thread reuse across days and task switches
- SQLite-backed thread-binding persistence across reopen
- SQLite-backed task and active-task persistence across reopen
- busy-thread rejection

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
39claw also supports optional Codex thread-option env vars so deployments can tune the runtime without patching source defaults.

For faster slash-command iteration in a disposable Discord server, set the optional `CLAW_DISCORD_GUILD_ID` environment variable.
When it is set, 39claw overwrites commands in that guild on startup.
When it is omitted, commands are registered globally.

A minimal smoke-test launch looks like this:

    CLAW_MODE=task \
    CLAW_TIMEZONE=Asia/Tokyo \
    CLAW_DISCORD_TOKEN=... \
    CLAW_DISCORD_GUILD_ID=... \
    CLAW_CODEX_WORKDIR=/absolute/path/to/repo \
    CLAW_SQLITE_PATH=/tmp/39claw-dev.sqlite \
    CLAW_CODEX_EXECUTABLE=/absolute/path/to/codex \
    go run ./cmd/39claw

Optional Codex thread-option overrides:

- `CLAW_CODEX_MODEL`
  - sets the Codex model name
- `CLAW_CODEX_SANDBOX_MODE`
  - accepts `read-only`, `workspace-write`, or `danger-full-access`
  - defaults to `workspace-write`
- `CLAW_CODEX_ADDITIONAL_DIRECTORIES`
  - adds writable directories using the OS path-list separator such as `:` on Unix
- `CLAW_CODEX_SKIP_GIT_REPO_CHECK`
  - accepts `true` or `false`
- `CLAW_CODEX_APPROVAL_POLICY`
  - accepts `never`, `on-request`, `on-failure`, or `untrusted`
  - defaults to `never`
- `CLAW_CODEX_MODEL_REASONING_EFFORT`
  - accepts `minimal`, `low`, `medium`, `high`, or `xhigh`
- `CLAW_CODEX_WEB_SEARCH_MODE`
  - accepts `disabled`, `cached`, or `live`
  - defaults to `live`
- `CLAW_CODEX_NETWORK_ACCESS`
  - accepts `true` or `false`

Smoke-test checklist:

- mention the bot in `daily` mode and confirm the reply targets the original message
- send unrelated chatter without a mention and confirm the bot stays silent
- run `/help` and confirm it matches the configured mode
- run `/task current` in `daily` mode and confirm the bot returns a clear not-available response
- run `/task new <name>` in `task` mode and confirm the success response is ephemeral
- send a long response through Codex and confirm the runtime splits it into Discord-safe chunks

Contributors can also run the standard local checks with the provided Make targets:

- `make test`
- `make lint`

The same checks run in GitHub Actions for pushes to `master` and for pull requests.
