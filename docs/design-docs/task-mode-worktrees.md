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

- `task` mode is valid only when `CLAW_CODEX_WORKDIR` points to a Git repository with an `origin` remote.
- In `task` mode, `CLAW_CODEX_WORKDIR` is the operator-visible source checkout and startup validation target, not the parent repository that directly owns task worktrees.
- Each task owns one task-specific branch name.
- 39claw maintains one managed bare parent repository under `${CLAW_DATADIR}/repos/<repo-id>.git` for the configured source checkout.
- Each task may also own one task-specific Git worktree created from that managed bare parent.
- Worktrees are created lazily on the first normal message that needs to run Codex for the task.
- The base ref for worktree creation is detected automatically inside the managed bare parent by preferring the remote default branch state.
- Worktree preparation synchronizes the managed bare parent to the source checkout's `origin` URL and best-effort `pushurl`, then tries `git fetch origin --prune` plus `git remote set-head origin --auto` before resolving the base ref.
- Because all tasks for one source checkout share that managed bare parent, in-process mutation against the same managed repository path is serialized during lazy preparation and closed-task pruning.
- Base-ref resolution should prefer `origin/HEAD`, then `origin/main`, then `origin/master`, and only then fall back to local `main` or `master`.
- Closing a task does not delete its branch from the managed bare parent.
- Closed-task worktrees are treated as disposable cache-like workspaces.
- The system keeps the most recent fifteen closed-task worktrees and prunes older closed-task worktrees with forced removal.

## Repository Model

In `daily` mode, the configured Codex working directory remains the directory passed through `CLAW_CODEX_WORKDIR`.

In `task` mode, the repository model changes:

- source checkout root: `CLAW_CODEX_WORKDIR`
- managed bare parent root: `${CLAW_DATADIR}/repos/<repo-id>.git`
- task worktree root: `${CLAW_DATADIR}/worktrees/<task_id>`
- Codex working directory for a task turn: the task's `worktree_path` once the worktree is ready

This means the configured workdir remains globally important, but in `task` mode it acts as the human-facing checkout and remote-configuration source. Task branches and task worktrees belong to the managed bare parent instead of to the visible checkout.

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

The branch name should be generated once at task creation time from a Git-safe slug of `task_name` and treated as immutable task identity metadata.
If the task name collapses to an unusable value after normalization, the implementation should fall back to the task ID so branch reservation still succeeds.

## Lazy Worktree Creation

The first normal message sent to an active task with `worktree_status=pending` or `worktree_status=failed` triggers worktree preparation.

The preparation flow is:

1. load the active task record
2. create or validate the managed bare parent under `${CLAW_DATADIR}/repos/<repo-id>.git`
3. synchronize that managed bare parent to the source checkout's `origin` URL and optional `pushurl`
4. refresh managed-`origin` metadata with a best-effort `git fetch origin --prune`
5. detect the base ref by preferring `origin/HEAD`, then `origin/main`, then `origin/master`, and only then falling back to local `main` or `master`
6. create the task worktree under `${CLAW_DATADIR}/worktrees/<task_id>`
7. create or attach the reserved task branch for that worktree inside the managed bare parent
8. persist `base_ref`, `worktree_path`, `worktree_created_at`, and `worktree_status=ready`
9. run Codex with the task-specific worktree path as the working directory

If any step fails, Codex must not run for that turn.

## Task Switching

Task switching remains intentionally small.

`/<instance-command> action:task-switch task_name:<name>` changes only the active task pointer for the requesting user, with `task_id` used only as an ambiguity fallback.
It does not:

- run Git checkout in the current shell
- mutate another task's worktree
- eagerly prepare the destination task worktree

The next normal message determines whether the destination task needs lazy worktree creation before Codex runs.

## Task Context Reset

`/<instance-command> action:task-reset-context` keeps the active task record and any existing task worktree exactly as they are, but removes the saved Codex thread binding for that task.

That command does not:

- recreate the worktree
- delete the worktree
- change the task branch
- change which task is active

It only changes Codex conversation continuity. The next normal message for that active task starts a fresh Codex thread in the same worktree.

The command is rejected while that task has in-flight or queued work so no reply arrives after the user believes the context was reset.

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

The branch is intentionally left behind in the managed bare parent so repository history and manual recovery options remain available even after workspace cleanup.

## Failure and Retry Model

Task creation should fail only if the task record itself cannot be created or activated.
No Git operation should run during `task-new`.

Lazy worktree creation failure is handled at normal-message time:

- the user receives a short actionable failure message
- the server logs the detailed cause
- the task remains `open`
- the task moves to `worktree_status=failed`
- the next normal message retries worktree preparation automatically

If the best-effort `git fetch origin --prune` step fails, the system should log the refresh failure but still continue base-ref detection using any already-available remote-tracking refs in the managed bare parent and then the local fallback branches.
That refresh and the other shared managed-repository mutations should not race with another task preparing or pruning against the same managed bare parent inside the same process.

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

Task branches are intentionally stored in the managed bare parent rather than in the visible source checkout.
That keeps the operator checkout branch-neutral with respect to bot-owned worktrees while preserving normal `git push origin ...` behavior from inside task worktrees.

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
