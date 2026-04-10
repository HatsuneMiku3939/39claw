# Task Mode Worktrees

This document defines the current v1 design for task-isolated Git worktrees in `task` mode.

The goal is to make a `task` mean more than a saved conversation label.
Each task should represent an isolated repository workspace with its own Codex thread, branch name, and working directory lifecycle.

## Why This Exists

Without filesystem isolation, `task` mode can preserve conversation context but still mixes file edits in one shared checkout.
That weakens the meaning of task switching and makes later task-oriented features harder to define.

With task-isolated worktrees:

- each task owns its own working directory
- switching tasks also switches the Codex execution workspace
- repository state for one task does not silently leak into another task

This turns `task` mode into an execution-oriented workflow instead of a long-lived chat label.

## Core Decisions

- `task` mode is valid only when `CLAW_CODEX_WORKDIR` points to a Git repository.
- In `task` mode, `CLAW_CODEX_WORKDIR` is the source repository root, not the final Codex working directory for every turn.
- Each task owns one task-specific branch name.
- Each task may also own one task-specific Git worktree created from that source repository.
- Worktrees are created lazily on the first normal message that needs to run Codex for the task.
- The base ref for worktree creation is detected automatically by preferring the remote default branch state.
- When the source repository has an `origin` remote, worktree preparation should try `git fetch origin --prune` as a best-effort refresh before resolving the base ref.
- Base-ref resolution should prefer `origin/HEAD`, then `origin/main`, then `origin/master`, and only then fall back to local `main` or `master`.
- Closing a task does not delete its branch.
- Closed-task worktrees are treated as disposable cache-like workspaces.
- The system keeps the most recent fifteen closed-task worktrees and prunes older closed-task worktrees with forced removal.

## Repository Model

In `daily` mode, the configured Codex working directory remains the directory passed through `CLAW_CODEX_WORKDIR`.

In `task` mode, the repository model changes:

- source repository root: `CLAW_CODEX_WORKDIR`
- task worktree root: `${CLAW_DATADIR}/worktrees/<task_id>`
- Codex working directory for a task turn: the task's `worktree_path` once the worktree is ready

This means the configured workdir remains globally important, but in `task` mode it acts as the shared Git source from which task worktrees are derived.

## Task State Model

The task record keeps both task lifecycle state and worktree lifecycle state.
Those are separate concerns and must not be collapsed into one field.

### Task status

- `open`
- `closed`

### Worktree status

- `pending`
  - the task exists but no worktree has been created yet
- `ready`
  - the worktree exists and can be used for Codex turns
- `failed`
  - a worktree creation attempt failed and should be retried on the next normal message
- `pruned`
  - the task once had a worktree, but that worktree was removed during closed-task cleanup

## Task Creation

`/<instance-command> action:task-new task_name:<name>` creates the task record and sets it active.

Task creation does not create a worktree immediately.
Instead, it records the metadata needed for later creation:

- `task_id`
- `task_name`
- `discord_user_id`
- `branch_name`
- `status=open`
- `worktree_status=pending`

The branch name should be generated once at task creation time and treated as immutable task identity metadata.

## Lazy Worktree Creation

The first normal message sent to an active task with `worktree_status=pending` or `worktree_status=failed` triggers worktree preparation.

The preparation flow is:

1. load the active task record
2. refresh `origin` metadata with a best-effort `git fetch origin --prune` when the source repository has an `origin` remote
3. detect the base ref by preferring `origin/HEAD`, then `origin/main`, then `origin/master`, and only then falling back to local `main` or `master`
4. create the task worktree under `${CLAW_DATADIR}/worktrees/<task_id>`
5. create or attach the reserved task branch for that worktree
6. persist `base_ref`, `worktree_path`, `worktree_created_at`, and `worktree_status=ready`
7. run Codex with the task-specific worktree path as the working directory

If any step fails, Codex must not run for that turn.

## Task Switching

Task switching remains intentionally small.

`/<instance-command> action:task-switch task_name:<name>` changes only the active task pointer for the requesting user, with `task_id` used only as an ambiguity fallback.
It does not:

- run Git checkout in the current shell
- mutate another task's worktree
- eagerly prepare the destination task worktree

The next normal message determines whether the destination task needs lazy worktree creation before Codex runs.

## Task Closing and Worktree Retention

Closing a task changes the task status to `closed` and clears active-task state if the closed task was active.

Closing a task does not delete:

- the task record
- the task branch
- the task's Codex thread binding

After close succeeds, the system applies closed-task worktree retention:

- open-task worktrees are never pruned
- closed tasks are sorted by `closed_at` descending
- the most recent fifteen closed tasks with `worktree_status=ready` keep their worktrees
- older closed tasks with `worktree_status=ready` are pruned by `git worktree remove --force`
- when pruning succeeds, the task becomes `worktree_status=pruned` and records `worktree_pruned_at`

The branch is intentionally left behind so repository history and manual recovery options remain available even after workspace cleanup.

## Failure and Retry Model

Task creation should fail only if the task record itself cannot be created or activated.
No Git operation should run during `task-new`.

Lazy worktree creation failure is handled at normal-message time:

- the user receives a short actionable failure message
- the server logs the detailed cause
- the task remains `open`
- the task moves to `worktree_status=failed`
- the next normal message retries worktree preparation automatically

If the best-effort `git fetch origin --prune` step fails, the system should log the refresh failure but still continue base-ref detection using any already-available remote-tracking refs and then the local fallback branches.

Pruning failure must not reopen or invalidate the closed task.
If pruning fails, the system should keep the task in `closed + ready`, log the failure, and try again during a later cleanup opportunity.

## Persistence Direction

The `tasks` table now carries task worktree metadata in addition to task identity:

- `branch_name`
- `base_ref`
- `worktree_path`
- `worktree_status`
- `worktree_created_at`
- `worktree_pruned_at`
- `last_used_at`

`branch_name` is fixed at task creation.
`base_ref` and `worktree_path` are populated on first successful worktree creation and remain stable afterward.
`last_used_at` is updated whenever a normal message successfully uses the task context.

## User Experience Consequences

This design changes the meaning of `task` mode in important ways:

- a task is now an isolated workspace, not only an isolated conversation
- task switching becomes a context switch between independent repository workspaces
- task creation is fast because the filesystem cost is deferred
- old closed-task workspaces do not grow without bound on disk

The tradeoff is that the first normal message for a task may perform extra Git setup work before Codex can answer.

## Implementation Notes

This design affects:

- startup validation for `task` mode
- task persistence schema
- task command orchestration
- normal-message orchestration before Codex execution
- Codex working-directory resolution
- task-close cleanup behavior

Implementation sequencing belongs in the active ExecPlan, but all later implementation work should preserve the product and lifecycle rules in this document.
