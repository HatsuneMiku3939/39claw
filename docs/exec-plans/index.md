# 39claw ExecPlans

This directory stores living execution plans for concrete implementation work.

An ExecPlan is the repository's step-by-step delivery document for a feature, refactor, or milestone-sized change.
Plans in this directory should be written and maintained in line with `.agents/PLANS.md`.

## Structure

- `active/`
  - contains plans that are still being implemented or are ready to be picked up next
- `completed/`
  - contains plans that have been finished and are kept for historical reference
- `tech-debt-tracker.md`
  - tracks follow-up work that was intentionally deferred during implementation

## Working Rules

- Create a new plan in `active/` before starting large feature work or a significant refactor.
- Keep each plan self-contained so a new contributor can continue from only the plan and the current working tree.
- Move a plan from `active/` to `completed/` only after its acceptance criteria and validation steps are satisfied.
- Record intentionally deferred work in `tech-debt-tracker.md` instead of leaving it implicit.

## Current Active Plans

These plans are intended to be executed in the order listed below. Most plans follow numeric order, but infrastructure prerequisites may require picking up a later-numbered plan first when it explicitly prepares the repository for another active plan.

- [Build fake runtime validation infrastructure for adapter-level tests](./active/11-fake-runtime-validation.md)
- [Build a versioned SQLite migration runner and bootstrap path](./active/13-sqlite-migration-runner.md)
- [Add shared daily generation rotation and `action:clear` to `daily` mode](./active/12-daily-clear-generation.md)

## Recently Completed Plans

- [Prefer the remote default branch when creating `task` worktrees](./completed/14-task-worktree-remote-base.md)
- [Add first-stage tag-driven release automation](./completed/10-first-stage-release-automation.md)
- [Build the foundation, contracts, and bootstrap path](./completed/01-foundation-and-contracts.md)
- [Implement `daily` mode routing and persistence](./completed/02-daily-mode-routing.md)
- [Implement `task` mode task workflow and command orchestration](./completed/03-task-mode-workflow.md)
- [Implement the Discord runtime, commands, and response presentation](./completed/04-discord-runtime-and-presentation.md)
- [Implement capped per-thread message queueing for busy Codex turns](./completed/05-queued-message-handling.md)
- [Implement Discord image attachment intake for Codex turns](./completed/06-discord-image-input.md)
- [Replace shared `/help` and `/task` slash commands with one instance-specific root command](./completed/07-instance-specific-root-command.md)
- [Add task-isolated Git worktrees to `task` mode](./completed/08-task-mode-worktree-isolation.md)
- [Add a durable memory bridge to `daily` mode](./completed/09-daily-memory-bridge.md)
