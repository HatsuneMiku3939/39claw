# 39claw

![39claw banner](./.github/assets/39claw.png)

39claw is a Discord bot that connects your Discord conversations to Codex.

It is built for teams or individuals who want to work with Codex from inside Discord without inventing their own thread-routing rules. You mention the bot, 39claw decides which Codex thread should receive the turn, and the reply comes back into the same channel.

## What You Get

- mention-based conversation in Discord
- two conversation modes for different workflows
- one instance-specific slash command with explicit action choices
- SQLite-backed continuity across restarts
- image attachment support for mention-triggered turns
- queued acknowledgments when the same conversation is already busy

## Installation

### Homebrew

Install the macOS cask from the custom tap.

```bash
brew tap hatsunemiku3939/homebrew-tap
brew install --cask 39claw
```

You can also install it without a separate tap step.

```bash
brew install --cask hatsunemiku3939/homebrew-tap/39claw
```

The cask installs an unsigned binary. If macOS blocks execution, inspect the binary first and then remove the quarantine attribute manually.

```bash
xattr -dr com.apple.quarantine "$(brew --prefix)/Caskroom/39claw/<version>/39claw"
```

### Linux packages

Release assets for Linux include both `.deb` and `.rpm` packages for `amd64` and `arm64`.

Download the package that matches your distribution and CPU architecture from the [GitHub Releases page](https://github.com/HatsuneMiku3939/39claw/releases) before installing it locally.

Install a Debian package:

```bash
sudo dpkg -i ./39claw_<version>_amd64.deb
```

Install an RPM package:

```bash
sudo rpm -i ./39claw-<version>-1.x86_64.rpm
```

### GitHub release archives

Download the archive that matches your platform from the [GitHub Releases page](https://github.com/HatsuneMiku3939/39claw/releases).

The first stable release is assumed to be `v1.0.0`, and the release archives follow this naming pattern:

- `39claw_v1.0.0_Linux_x86_64.tar.gz`
- `39claw_v1.0.0_Linux_arm64.tar.gz`
- `39claw_v1.0.0_Darwin_x86_64.zip`
- `39claw_v1.0.0_Darwin_arm64.zip`
- `39claw_v1.0.0_Windows_x86_64.zip`
- `39claw_v1.0.0_Windows_arm64.zip`

After downloading the archive:

1. extract it
2. move the `39claw` binary somewhere on your `PATH`
3. run `39claw version` to confirm the installed release version

Example for Linux `amd64`:

```bash
curl -LO https://github.com/HatsuneMiku3939/39claw/releases/download/v1.0.0/39claw_v1.0.0_Linux_x86_64.tar.gz
tar -xzf 39claw_v1.0.0_Linux_x86_64.tar.gz
chmod +x 39claw
sudo mv ./39claw /usr/local/bin/39claw
39claw version
```

Example for macOS `arm64`:

```bash
curl -LO https://github.com/HatsuneMiku3939/39claw/releases/download/v1.0.0/39claw_v1.0.0_Darwin_arm64.zip
unzip 39claw_v1.0.0_Darwin_arm64.zip
chmod +x 39claw
sudo mv ./39claw /usr/local/bin/39claw
39claw version
```

Example for Windows `amd64` with PowerShell:

```powershell
curl.exe -L -o 39claw_v1.0.0_Windows_x86_64.zip https://github.com/HatsuneMiku3939/39claw/releases/download/v1.0.0/39claw_v1.0.0_Windows_x86_64.zip
tar -xf 39claw_v1.0.0_Windows_x86_64.zip
.\39claw.exe version
```

### Build from source

If you prefer to build from source instead of using a release archive:

```bash
go build -o ./bin/39claw ./cmd/39claw
./bin/39claw version
```

## Conversation Modes

39claw runs in exactly one mode per bot instance.

### `daily`

Use `daily` mode when you want a lightweight shared assistant for day-to-day work.

- messages on the same local date share the same conversation context
- the next local date starts a fresh Codex thread automatically
- durable preferences and long-lived context can be projected into runtime-managed files under `AGENT_MEMORY/`
- if you want visible turns to consult that memory, add the necessary guidance to your own `AGENTS.md`
- users do not need to create or switch tasks before talking

If you want visible turns to consult the projected memory files, add guidance like this to your own `AGENTS.md`:

```md
# Daily Memory Guidance

If `AGENT_MEMORY/` exists in the current workspace, consult its durable memory files when they are relevant to the user's request.

- Read `AGENT_MEMORY/MEMORY.md` as the primary durable memory file.
- Read the most relevant dated note in `AGENT_MEMORY/YYYY-MM-DD.md` when bridge context is useful.
- Prefer the latest explicit user instruction when it conflicts with stored memory.
- Treat `AGENT_MEMORY/` as durable context only. Do not treat it as a source of temporary TODO items or transient chat history.
```

### `task`

Use `task` mode when you want durable work streams that continue across multiple days.

- each user works through an explicit active task
- each task eventually runs in its own task-specific Git worktree
- `/<instance-command> action:task-*` controls which task is active
- normal conversation does not run until a task is selected

## How It Behaves in Discord

### Normal conversation

- 39claw responds only when the bot is mentioned
- a qualifying message may contain text, images, or both
- if the mention includes no text and no usable image, the bot stays silent
- replies are posted in the same channel as replies to the triggering message

### Commands

Each bot instance registers exactly one slash command.
Its name comes from `CLAW_DISCORD_COMMAND_NAME`, so one instance might expose `/release` while another exposes `/docs`.

For every instance:

- `/<instance-command> action:help`
  - show the supported command surface for the current bot instance

In `task` mode, the same root command also supports:

- `/<instance-command> action:task-current`
  - show the active task
- `/<instance-command> action:task-list`
  - list open tasks and mark the active one
- `/<instance-command> action:task-new task_name:<name>`
  - create a task and make it active
- `/<instance-command> action:task-switch task_id:<id>`
  - switch the active task
- `/<instance-command> action:task-close task_id:<id>`
  - close a task

In `daily` mode, the root command exposes only `action:help`.

### Busy conversations

39claw runs one Codex turn at a time for a given conversation context.

- if another message arrives while that context is busy, the bot can queue up to five waiting messages
- queued messages receive a short acknowledgment immediately
- the final answer arrives later as a reply to the queued message
- if the queue is already full, the bot returns a retry-later response

Queued messages are held in memory, so they are lost if the bot process exits before they run.

## Requirements

Before you start, make sure you have:

- a Discord bot token
- the `codex` executable available on the machine that runs 39claw
- a writable SQLite file path
- a working directory that Codex should operate in
- a write-capable Codex workdir if you plan to use `daily` mode, because 39claw manages `AGENT_MEMORY/` files there
- a Git repository workdir if you plan to use `task` mode
- Go installed if you plan to run from source

## Local Secret Workflow

The recommended local-development workflow is:

1. copy `.env.example` to `.env.local`
2. replace every placeholder in `.env.local`
3. copy `.envrc.example` to `.envrc`
4. run `direnv allow`
5. start 39claw without pasting secrets into shell history

`.env.local`, `.envrc`, and `.direnv/` are ignored by Git in this repository.
Keep real Discord tokens, Codex API keys, and any other credentials only in those ignored files.
Checked-in examples must contain placeholders only.
If you do not use `direnv`, keep the same rule: load secrets from an ignored local file instead of tracked scripts or inline launch snippets.

<details>
<summary>Codex Installation Guide</summary>

39claw launches the local `codex` CLI, so install Codex before starting the bot.

Recommended installation options:

- install with npm
- install with Homebrew
- download a release binary from GitHub if you prefer a manual install

### Install with npm

```bash
npm install -g @openai/codex
```

### Install with Homebrew

```bash
brew install --cask codex
```

### Install from a GitHub release

If you do not want to use a package manager, download the correct archive for your platform from the Codex GitHub releases page and extract the binary.

After extraction, you will usually want to rename the binary to `codex` and place it somewhere on your `PATH`.

### Confirm the install

Run:

```bash
codex
```

The official quick start then recommends signing in with ChatGPT. Codex can also be used with an API key, but that requires additional setup on the Codex side.

Official references:

- [OpenAI Codex README](https://github.com/openai/codex/blob/main/README.md)
- [Codex documentation](https://developers.openai.com/codex)

</details>

<details>
<summary>Discord Setup Guide</summary>

If this is your first time running a Discord bot, do this before the quick start.

### 1. Create the bot application

- open the Discord Developer Portal
- create a new application
- add a Bot user to that application
- copy the bot token and store it in your ignored `.env.local` file as `CLAW_DISCORD_TOKEN`

### 2. Enable the required intent

In the bot settings, enable **Message Content Intent**.

39claw reads the content of mention-triggered messages, so this intent is required for normal conversation in guild channels.

### 3. Generate an invite URL

Under OAuth2, generate an invite URL for the bot.

Use these scopes:

- `bot`
- `applications.commands`

`applications.commands` is required because 39claw registers one instance-specific root slash command such as `/release`.

### 4. Grant the right bot permissions

Recommended permissions:

- `View Channels`
- `Send Messages`
- `Read Message History`

Useful optional permissions:

- `Embed Links`
- `Attach Files`

### 5. Invite the bot to a test server

Invite the bot to an existing Discord server that you control.

39claw does not create a new server for you. It connects to Discord and listens in the server where the bot has been installed.

### 6. Pick a safe place to test

For first-time setup, it is easiest to use one dedicated test server or one dedicated bot channel.

If you set `CLAW_DISCORD_GUILD_ID`, 39claw registers slash commands in that guild for faster testing. This is usually the best choice while you are still validating the setup.

### 7. Know what to expect in the channel

- normal conversation is mention-only
- slash commands are explicit and do not require a mention
- in `task` mode, users must create or switch to a task before normal conversation will run

</details>

## Quick Start

### 1. Choose a mode

Pick one:

- `CLAW_MODE=daily`
- `CLAW_MODE=task`

If you choose `task`, `CLAW_CODEX_WORKDIR` must point to a Git repository.
39claw treats that repository as the source repository for task-specific worktrees stored under `CLAW_DATADIR`.
If startup finds a missing or non-Git task workdir, the bot exits with a clear configuration error before it connects to Discord.

### 2. Set the required environment variables

The safe-default setup is to keep your local values in `.env.local` and load them through `.envrc`.
Start by copying `.env.example` to `.env.local`, then replace the placeholders with real local values.

These variables are required in `.env.local`:

- `CLAW_MODE`
- `CLAW_TIMEZONE`
- `CLAW_DISCORD_TOKEN`
- `CLAW_DISCORD_COMMAND_NAME`
- `CLAW_CODEX_WORKDIR`
- `CLAW_DATADIR`
- `CLAW_CODEX_EXECUTABLE`

Recommended startup flow:

```bash
cp .env.example .env.local
cp .envrc.example .envrc
direnv allow
go run ./cmd/39claw version
go run ./cmd/39claw
```

39claw stores its SQLite database at `39claw.sqlite` inside `CLAW_DATADIR`.

If `CLAW_DISCORD_GUILD_ID` is set, 39claw registers commands in that guild for faster testing. If it is omitted, commands are registered globally.

### CLI command

39claw currently exposes one local CLI subcommand:

- `go run ./cmd/39claw version`
  - print the build version string and exit without starting the Discord runtime

Regular startup still uses `go run ./cmd/39claw` with no subcommand.

### 3. Mention the bot in Discord

Try one of these:

- mention the bot with a text prompt
- mention the bot with text plus an image
- mention the bot with only an image

If you are running in `task` mode, create a task first with `/<your-command> action:task-new task_name:<name>`.
The first normal message for a new task may spend a moment preparing that task's dedicated worktree before Codex replies.

## Configuration Reference

### Required variables

- `CLAW_MODE`
  - `daily` or `task`
- `CLAW_TIMEZONE`
  - the timezone used for daily rollover
- `CLAW_DISCORD_TOKEN`
  - Discord bot token
- `CLAW_DISCORD_COMMAND_NAME`
  - the unique root slash command name for this bot instance
- `CLAW_CODEX_WORKDIR`
  - working directory passed to Codex
- `CLAW_DATADIR`
  - directory used for local state; the SQLite database path is fixed to `39claw.sqlite` inside this directory
- `CLAW_CODEX_EXECUTABLE`
  - path to the `codex` executable

### Useful optional variables

- `CLAW_DISCORD_GUILD_ID`
  - register slash commands in one guild for faster iteration
- `CLAW_LOG_LEVEL`
  - logging level, default `info`
- `CLAW_LOG_FORMAT`
  - log format, `json` by default, or `text`
- `CLAW_CODEX_MODEL`
  - choose a Codex model
- `CLAW_CODEX_BASE_URL`
  - override the Codex base URL
- `CLAW_CODEX_API_KEY`
  - provide an API key when needed
- `CLAW_CODEX_SANDBOX_MODE`
  - `read-only`, `workspace-write`, or `danger-full-access`
- `CLAW_CODEX_ADDITIONAL_DIRECTORIES`
  - extra writable directories, separated by the OS path-list separator
- `CLAW_CODEX_SKIP_GIT_REPO_CHECK`
  - `true` or `false`
- `CLAW_CODEX_APPROVAL_POLICY`
  - `never`, `on-request`, `on-failure`, or `untrusted`
- `CLAW_CODEX_MODEL_REASONING_EFFORT`
  - `minimal`, `low`, `medium`, `high`, or `xhigh`
- `CLAW_CODEX_WEB_SEARCH_MODE`
  - `disabled`, `cached`, or `live`
- `CLAW_CODEX_NETWORK_ACCESS`
  - `true` or `false`

## Observability

39claw emits structured logs through `log/slog`.
The default output format is JSON so logs can be indexed directly by common log backends.
Set `CLAW_LOG_FORMAT=text` only when human-readable local debugging is more important than machine parsing.

High-value runtime events include:

- `queue_admission`
  - fields include `outcome=execute_now|queued|queue_full`, plus `queue_position` when queued
- `codex_turn_started`
  - fields include `thread_resumed`, `prompt_char_count`, `image_count`, and `working_directory_set`
- `codex_turn_finished`
  - fields include `outcome`, `latency_ms`, `thread_id`, and token usage fields when Codex reports them
- `queued_turn_started` and `queued_turn_finished`
  - fields include `queue_wait_ms` and whether shutdown or reply delivery interrupted the queued flow
- `deferred_reply_delivery`
  - fields include `outcome=success|failure|dropped_on_shutdown`

Most message-path events also carry routing context such as `component`, `mode`, `logical_key`, `channel_id`, `reply_to_id`, `user_id`, and `task_id` when available.

## Smoke Test Checklist

After startup, confirm the basics:

- mention the bot and confirm the reply targets the original message
- mention the bot with text plus an image and confirm the request is handled
- mention the bot with only an image and confirm the bot still answers
- send unrelated chatter without a mention and confirm the bot stays silent
- run `/<your-command> action:help` and confirm it matches the configured mode
- in `task` mode, run `/<your-command> action:task-new task_name:smoke-test` and confirm the response is ephemeral
- send overlapping messages for the same conversation and confirm the later one is queued
- send a long Codex response and confirm the bot splits it into readable Discord-safe chunks

## Release

39claw now has a first-stage tag-driven release path for the production `39claw` binary.
The flow is intentionally conservative:

- release tags are created manually
- GitHub Actions builds the release from a pushed `v*` tag
- GoReleaser creates a draft GitHub Release instead of publishing immediately
- Linux `.deb` and `.rpm` packages are attached to the draft release
- the macOS Homebrew cask is updated in the `HatsuneMiku3939/homebrew-tap` repository

Before tagging, run the local validation commands from the repository root:

```bash
make test
make lint
go vet ./...
make release-check
make release-snapshot
```

After the release candidate gate passes, create and push a tag such as:

```bash
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

Pushing a tag that starts with `v` triggers the GitHub Actions release workflow and creates a draft GitHub Release with cross-platform archives for `39claw`.

The same release workflow also publishes Linux package artifacts and updates the Homebrew cask in the `HatsuneMiku3939/homebrew-tap` repository. Set the `HOMEBREW_TAP_GITHUB_TOKEN` GitHub Actions secret to a token with write access to that repository before pushing a release tag.

For the complete release candidate checklist, tagging steps, and post-release verification flow, use [RELEASE_RUNBOOK.md](./docs/operations/RELEASE_RUNBOOK.md).

## Current Status

39claw is still early-stage software, but the current build already supports:

- mention-only normal conversation
- mention-triggered image attachment intake
- `daily` and `task` conversation modes
- one instance-specific root slash command
- `action:help`, `action:task-current`, `action:task-list`, `action:task-new`, `action:task-switch`, and `action:task-close`
- SQLite-backed persistence for thread bindings and task state
- queued acknowledgments with deferred follow-up replies

## Learn More

For deeper detail, start here:

- [docs/product-specs/discord-command-behavior.md](./docs/product-specs/discord-command-behavior.md)
- [docs/product-specs/daily-mode-user-flow.md](./docs/product-specs/daily-mode-user-flow.md)
- [docs/product-specs/task-mode-user-flow.md](./docs/product-specs/task-mode-user-flow.md)
- [docs/index.md](./docs/index.md)
- [ARCHITECTURE.md](./ARCHITECTURE.md)
- [RELEASE_RUNBOOK.md](./docs/operations/RELEASE_RUNBOOK.md)
