# Daily Mode Example: Obsidian Vault Knowledge Base

This guide shows how to run one `daily` mode 39claw instance against an Obsidian vault or any other writable notes directory.

This is the right fit when you want:

- one shared Discord bot for lightweight knowledge-base work
- Codex operating directly inside your vault
- a daily thread reset with durable memory carried through `AGENT_MEMORY/`

This example assumes:

- your vault is not a Git repository
- you want Codex sandboxing to stay at `workspace-write`
- you will use `CLAW_CODEX_SKIP_GIT_REPO_CHECK=true`
- you installed `39claw` already through one of the packaged installation paths in [README.md](../README.md)

## Before You Start

Prepare these items first:

1. `39claw` installed through Homebrew or a Linux package as described in [README.md](../README.md#installation)
2. a working `codex` installation on the machine that will run 39claw
3. a Discord bot token for this specific `daily` instance
4. a Discord test guild ID if you want faster slash-command registration during setup
5. an existing Obsidian vault directory, for example `/Users/you/Documents/SecondBrain`
6. a writable local data directory for 39claw state

If `39claw` is not installed yet, stop here and finish one of these first:

1. install through Homebrew
2. install through the Linux `.deb` or `.rpm` package

Those installation paths are documented in [README.md](../README.md#installation).

If `codex` is not installed yet, use the `Codex Installation Guide` in [README.md](../README.md).

If you do not have a Discord bot token yet, use the `Discord Setup Guide` in [README.md](../README.md) to create the bot application, enable the required intent, and copy the bot token.

## Step 1: Pick the directories

In this example:

- vault path: `/Users/you/Documents/SecondBrain`
- 39claw data path: `/Users/you/.local/share/39claw-kb`
- command name: `kb`

Choose values that match your machine before you continue.

## Step 2: Create the local data directory

```bash
mkdir -p /Users/you/.local/share/39claw-kb
```

39claw will store its SQLite database at:

```text
/Users/you/.local/share/39claw-kb/39claw.sqlite
```

## Step 3: Create the environment file

Copy the repository example and then replace the placeholders:

```bash
cp .env.example .env.local
```

If you want a copy-paste starting point, use one of these sample files first:

- [daily-obsidian-vault.macos.env.local.sample](./daily-obsidian-vault.macos.env.local.sample)
- [daily-obsidian-vault.linux.env.local.sample](./daily-obsidian-vault.linux.env.local.sample)

Set `.env.local` like this:

```dotenv
CLAW_MODE=daily
CLAW_TIMEZONE=Asia/Tokyo
CLAW_DISCORD_TOKEN=replace-with-your-daily-bot-token
CLAW_DISCORD_COMMAND_NAME=kb
CLAW_DISCORD_GUILD_ID=replace-with-your-test-guild-id
CLAW_CODEX_WORKDIR=/Users/you/Documents/SecondBrain
CLAW_DATADIR=/Users/you/.local/share/39claw-kb
CLAW_CODEX_EXECUTABLE=/absolute/path/to/codex

CLAW_CODEX_SANDBOX_MODE=workspace-write
CLAW_CODEX_SKIP_GIT_REPO_CHECK=true
CLAW_CODEX_APPROVAL_POLICY=never
CLAW_LOG_FORMAT=text
```

Why these values matter:

1. `CLAW_MODE=daily` enables the shared day-based conversation flow.
2. `CLAW_CODEX_WORKDIR` points at the vault where Codex will operate.
3. `CLAW_CODEX_SANDBOX_MODE=workspace-write` allows 39claw to manage `AGENT_MEMORY/` in the vault.
4. `CLAW_CODEX_SKIP_GIT_REPO_CHECK=true` matches the non-Git vault setup.

## Step 4: Load the environment safely

If you use `direnv`, start with:

```bash
cp .envrc.example .envrc
direnv allow
```

If you do not use `direnv`, use this safe shell flow instead:

1. keep secrets only in `.env.local`
2. tighten file permissions so other local users cannot read it easily
3. load the file into the current shell without pasting secrets into command history

```bash
chmod 600 .env.local
set -a
. ./.env.local
set +a
```

After that, start the bot from the same shell session:

```bash
39claw
```

This keeps secrets out of:

1. tracked files
2. shell history
3. inline one-shot commands like `CLAW_DISCORD_TOKEN=... 39claw`

## Step 5: Add optional vault instructions

39claw will create and refresh `AGENT_MEMORY/` automatically, but normal visible turns only consult that memory if your own workdir instructions say so.

If you want the bot to use the durable memory files during visible replies, create or update:

```text
/Users/you/Documents/SecondBrain/AGENTS.md
```

Add guidance like this:

```md
# Daily Memory Guidance

If `AGENT_MEMORY/` exists in the current workspace, consult its durable memory files when they are relevant to the user's request.

- Read `AGENT_MEMORY/MEMORY.md` as the primary durable memory file.
- Read the most relevant dated note in `AGENT_MEMORY/YYYY-MM-DD.md` when bridge context is useful.
- Prefer the latest explicit user instruction when it conflicts with stored memory.
- Treat `AGENT_MEMORY/` as durable context only. Do not treat it as a source of temporary TODO items or transient chat history.
```

## Step 6: Confirm the installed binary works

Run:

```bash
39claw version
```

You should see a version string such as `dev` or a release version.

## Step 7: Start the daily-mode bot

Run:

```bash
39claw
```

If you are not using `direnv`, make sure you already loaded `.env.local` in the current shell during Step 4 before running this command.

On first startup, 39claw should:

1. connect to Discord
2. register the `/kb` command
3. create or refresh the managed `AGENT_MEMORY/` files inside the vault

## Step 8: Smoke-test the knowledge-base flow

In your Discord test channel:

1. run `/kb action:help`
2. mention the bot with a normal text question
3. mention the bot with text plus an image
4. send a non-mention message and confirm the bot stays silent

In the vault, confirm that these paths now exist:

```text
/Users/you/Documents/SecondBrain/AGENT_MEMORY/
/Users/you/Documents/SecondBrain/AGENT_MEMORY/MEMORY.md
```

## Step 9: Know the common failure cases

If startup fails, check these first:

1. `CLAW_CODEX_WORKDIR` must point to a real writable directory.
2. `CLAW_CODEX_SANDBOX_MODE` must not be `read-only` in `daily` mode.
3. `CLAW_DISCORD_COMMAND_NAME` must contain only lowercase letters, digits, or hyphens.
4. `CLAW_DISCORD_TOKEN` must belong to the bot application you invited to the server.

## Step 10: Move from test to real use

After the test guild is working:

1. keep the same vault path and data directory
2. keep the same Discord bot token if this bot identity is staying the same
3. remove `CLAW_DISCORD_GUILD_ID` only if you want global command registration
4. restart the process after any configuration change
