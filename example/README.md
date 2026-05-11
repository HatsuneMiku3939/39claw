# Example Setups

This directory contains end-user-oriented setup guides for common 39claw deployments.

Each guide is intentionally step-by-step and assumes you want a practical starting point rather than a full configuration matrix.

## Available Guides

- [Journal mode in an Obsidian vault](./journal-obsidian-vault.md)
  - Run one shared knowledge-base bot in a writable vault directory with `CLAW_CODEX_SKIP_GIT_REPO_CHECK=true`
- [Thread mode for one Git repository](./thread-repository.md)
  - Run one task-oriented bot that creates isolated worktrees from a real Git repository

## OS-Specific `.env.local` Samples

Copy one of these files into your own `.env.local` and then replace the placeholders:

- [journal-obsidian-vault.macos.env.local.sample](./journal-obsidian-vault.macos.env.local.sample)
- [journal-obsidian-vault.linux.env.local.sample](./journal-obsidian-vault.linux.env.local.sample)
- [thread-repository.macos.env.local.sample](./thread-repository.macos.env.local.sample)
- [thread-repository.linux.env.local.sample](./thread-repository.linux.env.local.sample)

## Suggested Two-Instance Layout

If you want both modes at the same time, use two separate bot instances:

1. one `journal` instance for the knowledge base
2. one `thread` instance for repository work

Recommended command names:

- `/kb` for the knowledge-base `journal` instance
- `/dev` or `/release` for the repository `thread` instance

## Important Deployment Notes

1. Run the two instances with separate Discord bot applications and separate tokens if you want them online at the same time.
2. Each 39claw process bulk-overwrites the slash-command schema for its own Discord application at startup.
3. `journal` mode needs a writable Codex sandbox because 39claw manages `AGENT_MEMORY/` inside the configured workdir.
4. `thread` mode requires `CLAW_CODEX_WORKDIR` to be the root of a Git repository with an `origin` remote.
