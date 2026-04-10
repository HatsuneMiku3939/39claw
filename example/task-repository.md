# Task Mode Example: One Repository With Isolated Task Worktrees

This guide shows how to run one `task` mode 39claw instance against a real Git repository.

This is the right fit when you want:

- durable project work across multiple days
- explicit task creation and task switching in Discord
- one isolated Git worktree per task under `CLAW_DATADIR`

This example assumes:

- you can clone the source repository locally before starting 39claw
- `CLAW_CODEX_WORKDIR` points at that repository root
- that checkout has an `origin` remote configured
- task worktrees will be created under the 39claw data directory
- you want a convenience-first autonomous development preset
- you installed `39claw` already through one of the packaged installation paths in [README.md](../README.md)

## Before You Start

Prepare these items first:

1. `39claw` installed through Homebrew or a Linux package as described in [README.md](../README.md#installation)
2. a working `codex` installation on the machine that will run 39claw
3. a Discord bot token for this specific `task` instance
4. a Discord test guild ID if you want faster slash-command registration during setup
5. a GitHub repository URL or another clone URL, for example `https://github.com/your-org/project-alpha.git`
6. a writable local data directory for 39claw state and task worktrees
7. a local parent directory where you want to clone the repository

If `39claw` is not installed yet, stop here and finish one of these first:

1. install through Homebrew
2. install through the Linux `.deb` or `.rpm` package

Those installation paths are documented in [README.md](../README.md#installation).

If `codex` is not installed yet, use the `Codex Installation Guide` in [README.md](../README.md).

If you do not have a Discord bot token yet, use the `Discord Setup Guide` in [README.md](../README.md) to create the bot application, enable the required intent, and copy the bot token.

## Step 1: Clone the repository locally

In this example, the source repository will live at:

```text
/Users/you/src/project-alpha
```

Clone it from GitHub:

```bash
mkdir -p /Users/you/src
git clone https://github.com/your-org/project-alpha.git /Users/you/src/project-alpha
```

If the repository is private, use the clone URL that matches your normal Git authentication flow, such as an SSH URL.

## Step 2: Confirm the repository is a Git root

Check that the chosen workdir is the repository root:

```bash
cd /Users/you/src/project-alpha
git status --short --branch
git remote get-url origin
test -e .git && echo "git root ok"
```

If `.git` is missing at that exact path, or `git remote get-url origin` fails, point `CLAW_CODEX_WORKDIR` at the real repository root and configure `origin` before continuing.

## Step 3: Create the local data directory

```bash
mkdir -p /Users/you/.local/share/39claw-dev
```

39claw will store:

1. the SQLite database at `/Users/you/.local/share/39claw-dev/39claw.sqlite`
2. a managed bare task repository under `/Users/you/.local/share/39claw-dev/repos/`
3. task worktrees under `/Users/you/.local/share/39claw-dev/worktrees/`

## Step 4: Create the environment file

Copy the repository example and then replace the placeholders:

```bash
cp .env.example .env.local
```

If you want a copy-paste starting point, use one of these sample files first:

- [task-repository.macos.env.local.sample](./task-repository.macos.env.local.sample)
- [task-repository.linux.env.local.sample](./task-repository.linux.env.local.sample)

Set `.env.local` like this:

```dotenv
CLAW_MODE=task
CLAW_TIMEZONE=Asia/Tokyo
CLAW_DISCORD_TOKEN=replace-with-your-task-bot-token
CLAW_DISCORD_COMMAND_NAME=dev
CLAW_DISCORD_GUILD_ID=replace-with-your-test-guild-id
CLAW_CODEX_WORKDIR=/Users/you/src/project-alpha
CLAW_DATADIR=/Users/you/.local/share/39claw-dev
CLAW_CODEX_EXECUTABLE=/absolute/path/to/codex
# Optional: override the Codex home directory used by the spawned codex CLI.
# CLAW_CODEX_HOME=/Users/you/.codex-dev

CLAW_CODEX_SANDBOX_MODE=danger-full-access
CLAW_CODEX_WEB_SEARCH_MODE=live
CLAW_CODEX_NETWORK_ACCESS=true
CLAW_CODEX_APPROVAL_POLICY=never
CLAW_LOG_FORMAT=text
```

Why these values matter:

1. `CLAW_MODE=task` enables task creation, switching, and task-specific threads.
2. `CLAW_CODEX_WORKDIR` is the operator-visible source checkout and must have an `origin` remote.
3. `CLAW_CODEX_HOME`, when set, tells 39claw which `CODEX_HOME` value to pass to the spawned Codex CLI.
4. 39claw creates a managed bare parent under `CLAW_DATADIR/repos/` and each new active task can create its own worktree under `CLAW_DATADIR/worktrees/`.
5. `CLAW_CODEX_SANDBOX_MODE=danger-full-access` gives Codex the widest local write access for autonomous repository work.
6. `CLAW_CODEX_WEB_SEARCH_MODE=live` lets the bot look up fresh web information when implementation work needs it.
7. `CLAW_CODEX_NETWORK_ACCESS=true` lets the bot open links and use network-backed tooling during a task.
8. `CLAW_CODEX_APPROVAL_POLICY=never` avoids approval interruptions during autonomous development flows.

This is an intentionally aggressive autonomous-development preset. Master, this can be risky: Codex can modify files broadly and use the network during task execution, so use it only when that level of autonomy matches your repository and trust model.

## Step 5: Load the environment safely

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

## Step 6: Confirm the installed binary works

Run:

```bash
39claw version
```

You should see a version string such as `dev` or a release version.

## Step 7: Start the task-mode bot

Run:

```bash
39claw
```

If you are not using `direnv`, make sure you already loaded `.env.local` in the current shell during Step 5 before running this command.

On startup, 39claw should:

1. connect to Discord
2. register the `/dev` command
3. validate that `/Users/you/src/project-alpha` is a Git repository root with an `origin` remote

## Step 8: Create the first task in Discord

In your Discord test channel:

1. run `/dev action:help`
2. run `/dev action:task-new task_name:Release prep`
3. run `/dev action:task-current`
4. mention the bot with a repository task, for example `@bot update the README installation section`

The first normal message for that task may take a moment longer because 39claw can create the task worktree before Codex starts.

## Step 9: Confirm the managed repository and worktree were created

After the first normal task message, inspect the data directory:

```bash
find /Users/you/.local/share/39claw-dev/repos -maxdepth 2 -type d | sort
find /Users/you/.local/share/39claw-dev/worktrees -maxdepth 2 -type d | sort
```

You should see one bare repository under `repos/` and a task-specific directory under `worktrees/`.

## Step 10: Smoke-test task switching

In Discord:

1. run `/dev action:task-new task_name:Docs cleanup`
2. mention the bot once for that new task
3. run `/dev action:task-list`
4. switch back with `/dev action:task-switch task_id:<id>`

Expected result:

1. each task keeps its own Codex thread
2. each task can keep its own isolated worktree
3. normal conversation does not run until an active task exists

## Step 11: Know the common failure cases

If startup fails, check these first:

1. `CLAW_CODEX_WORKDIR` must exist and be a directory.
2. `CLAW_CODEX_WORKDIR` must be the root of a Git repository.
3. `CLAW_CODEX_WORKDIR` must have a working `origin` remote.
4. `CLAW_DISCORD_COMMAND_NAME` must contain only lowercase letters, digits, or hyphens.
5. `CLAW_DISCORD_TOKEN` must belong to the bot application you invited to the server.
6. `danger-full-access` and network-enabled execution should match your security expectations before you leave the bot running unattended.

## Step 12: Move from test to real use

After the test guild is working:

1. keep the repository path stable
2. keep the data directory stable so task state survives restarts
3. remove `CLAW_DISCORD_GUILD_ID` only if you want global command registration
4. restart the process after any configuration change
