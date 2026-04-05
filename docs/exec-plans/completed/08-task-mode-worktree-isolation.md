# Add task-isolated Git worktrees to `task` mode

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, `task` mode should no longer run every task inside one shared checkout. A user should be able to create a task, send the first normal message, and have 39claw prepare a task-specific Git worktree under `CLAW_DATADIR` before Codex runs. Switching tasks should then switch both the Codex thread context and the isolated working directory used for execution.

The user-visible proof is practical. In `task` mode, creating two tasks and sending one normal message to each should produce two distinct task worktrees under `${CLAW_DATADIR}/worktrees`, each bound to its own task branch and Codex thread. Closing many tasks should retain only the fifteen most recently closed ready worktrees while leaving the task branches intact.

## Progress

- [x] (2026-04-04 23:57Z) Captured the new task-mode worktree direction, hard Git-repository requirement, lazy creation flow, and closed-task pruning policy in repository documentation.
- [x] (2026-04-05 00:24Z) Extended startup validation so `task` mode now rejects missing, non-directory, and non-Git `CLAW_CODEX_WORKDIR` values before Discord startup.
- [x] (2026-04-05 00:24Z) Added task worktree metadata to persistence, including additive SQLite migration and branch-name backfill for older rows.
- [x] (2026-04-05 00:24Z) Routed task-mode Codex turns through task-specific worktree paths while leaving `daily` mode on the configured global workdir.
- [x] (2026-04-05 00:24Z) Implemented lazy worktree creation, automatic retry after failed preparation, and closed-task worktree pruning with branch retention.
- [x] (2026-04-05 00:24Z) Added unit and integration coverage for startup validation, lazy creation, retry behavior, pruning, and schema migration.
- [x] (2026-04-05 00:24Z) Ran `make test` and `make lint` after the implementation landed.
- [x] (2026-04-05 02:04Z) Re-checked the repository state, confirmed the acceptance criteria remain satisfied, and archived this completed plan while recording the remaining operator-facing follow-up in `docs/exec-plans/tech-debt-tracker.md`.

## Surprises & Discoveries

- Observation: The current repository already has a useful separation between task lifecycle and thread binding lifecycle, but it does not yet have any persistent concept of task worktree state.
  Evidence: `internal/app/types.go` and `internal/store/sqlite/store.go`

- Observation: `CLAW_CODEX_SKIP_GIT_REPO_CHECK` already exists because the Codex CLI supports bypassing its own Git validation, but the current application does not impose a task-mode-specific repository requirement before Codex runs.
  Evidence: `internal/config/config.go`, `cmd/39claw/main.go`, and `internal/codex/exec.go`

- Observation: The current architecture and implementation spec still describe one working directory per bot instance, so both documents must be updated before implementation to avoid misleading later contributors.
  Evidence: `ARCHITECTURE.md` and `docs/design-docs/implementation-spec.md`

- Observation: Retrying a failed lazy worktree creation needs to tolerate the case where Git already created the reserved branch during an earlier partial attempt.
  Evidence: `internal/app/task_workspace.go` now checks for an existing task branch and switches between `git worktree add -b <branch>` and `git worktree add <path> <branch>` on retry.

- Observation: Additive SQLite migration also needs a backfill step because older task rows otherwise keep an empty `branch_name`, which breaks the "reserved branch at task creation" rule for reopened databases.
  Evidence: `internal/store/sqlite/store.go` runs `UPDATE tasks SET branch_name = 'task/' || task_id WHERE branch_name = ''` after schema migration.

## Decision Log

- Decision: Treat `CLAW_CODEX_WORKDIR` as a hard Git-repository requirement in `task` mode.
  Rationale: Task switching should mean switching between isolated repository workspaces, not only between saved conversation labels.
  Date/Author: 2026-04-04 / Codex

- Decision: Create task worktrees lazily on the first normal message instead of during `task-new`.
  Rationale: Task creation should remain lightweight, and filesystem cost should be paid only when a task is actually used for execution.
  Date/Author: 2026-04-04 / Codex

- Decision: Detect the task base ref by checking `main` first and `master` second.
  Rationale: The repository should prefer a simple, fixed v1 rule over additional configuration.
  Date/Author: 2026-04-04 / Codex

- Decision: Prune only worktrees, not task branches, when closed-task retention exceeds fifteen ready worktrees.
  Rationale: This keeps disk usage bounded while preserving repository history and manual recovery options.
  Date/Author: 2026-04-04 / Codex

- Decision: Failed lazy worktree creation should be retried automatically on the next normal message.
  Rationale: Many failure causes are transient or operator-fixable, so the product should recover without introducing extra task-repair commands in v1.
  Date/Author: 2026-04-04 / Codex

- Decision: Archive this ExecPlan and track later operator-facing improvements as explicit tech debt instead of keeping the implementation plan active.
  Rationale: The shipped worktree isolation behavior is complete and validated, while the remaining ideas are optional usability enhancements rather than unfinished core scope.
  Date/Author: 2026-04-05 / Codex

## Outcomes & Retrospective

This plan is now implemented in the repository. `task` mode startup rejects non-Git source repositories, task creation reserves branch metadata with `worktree_status=pending`, the first normal task message lazily creates a task-specific worktree under `${CLAW_DATADIR}/worktrees/<task_id>`, and later turns reuse that task-specific working directory and Codex thread binding. Closing tasks now keeps task branches but prunes older closed ready worktrees beyond the configured retention window.

The most important lesson was that the feature touches three kinds of state at once: Discord-visible task workflow, Codex thread continuity, and Git workspace lifecycle. Keeping those aligned required a narrow app-layer worktree manager plus additive store migration rather than folding Git operations into the task command flow directly. The remaining follow-up work is operational rather than architectural: if future product needs want manual cleanup commands or richer worktree status output in Discord, those can build on the now-persistent task worktree metadata without changing the core model again.

This plan now leaves `active/` because the core worktree-isolation behavior, persistence migration, retry path, pruning policy, and repository validation are all already implemented and covered. The document would otherwise imply that contributors still need to land core functionality when the only remaining work is optional product hardening captured as explicit follow-up debt.

## Context and Orientation

39claw is a Discord bot that routes user messages into Codex threads. In `daily` mode, one bot instance uses one shared working directory and one date-derived logical thread key at a time. In `task` mode today, the logical thread key is already `discord_user_id + task_id`, but execution still assumes one global Codex working directory per bot instance.

This plan changes that assumption. In the new design, `task` mode still stores task identity and active-task state in SQLite, but each task also owns metadata for a lazily created Git worktree. The source repository root remains the configured `CLAW_CODEX_WORKDIR`, while the actual Codex working directory for a ready task becomes `${CLAW_DATADIR}/worktrees/<task_id>`.

The most relevant files are:

- `cmd/39claw/main.go`
  - startup wiring, thread options, and integration assembly
- `internal/config/config.go`
  - environment loading and validation
- `internal/app/types.go`
  - task and thread-binding data shapes
- `internal/app/task_service.go`
  - task command orchestration
- `internal/app/message_service_impl.go`
  - normal-message orchestration before Codex runs
- `internal/store/sqlite/store.go`
  - schema creation and task persistence
- `internal/thread`
  - task-mode key resolution
- `internal/codex`
  - Codex gateway and CLI execution wrapper
- `ARCHITECTURE.md`
  - authoritative architecture document
- `docs/design-docs/task-mode-worktrees.md`
  - design rules this plan must implement

Terms used in this plan:

- source repository: the Git repository configured by `CLAW_CODEX_WORKDIR`
- task worktree: the task-specific checkout stored under `CLAW_DATADIR`
- base ref: the Git branch used as the starting point for creating the task worktree, chosen from `main` or `master`
- prune: remove an old closed-task worktree from disk while leaving the task branch in Git

## Plan of Work

Start by updating startup validation. In `internal/config` or a nearby validation layer, detect whether the configured workdir is a Git repository whenever `CLAW_MODE=task`. Fail startup with a clear error if the path is missing, not a directory, or not a Git repository. Keep `daily` mode behavior unchanged.

Next, extend the task data model in `internal/app/types.go` and the SQLite schema in `internal/store/sqlite/store.go`. Add persistent task fields for `branch_name`, `base_ref`, `worktree_path`, `worktree_status`, `worktree_created_at`, `worktree_pruned_at`, and `last_used_at`. Because the repository already has live schema creation through `CREATE TABLE IF NOT EXISTS`, add additive migration logic that checks for missing columns and adds them. Preserve existing task rows by giving the new fields safe defaults such as `worktree_status='pending'` when older rows are encountered.

Then implement a small task workspace service in the app layer or a focused internal package. That service should generate branch names during `task-new`, detect the base ref, create task worktrees lazily, and prune old closed-task worktrees. Keep the task command service responsible for user-facing task workflow, but move Git-specific worktree operations out of it if that keeps the task command layer readable.

After that, update normal-message handling in `internal/app/message_service_impl.go` so `task` mode resolves the active task, ensures a usable worktree when the task is `pending` or `failed`, and only then calls the Codex gateway. The Codex gateway path must accept a per-turn working directory override so task-mode turns can use the task worktree while `daily` mode continues using the global workdir. Record `last_used_at` for successful task use.

Finally, update close behavior. When `task-close` succeeds, trigger closed-task pruning for tasks with `worktree_status=ready`. Sort closed tasks by `closed_at` descending, keep the most recent fifteen ready worktrees, and remove older ready worktrees with `git worktree remove --force`. If pruning fails for a task, log the failure and leave that task in `closed + ready` so a later cleanup pass can try again.

## Concrete Steps

Run all commands from `/home/filepang/playground/39claw`.

1. Confirm the current repository state before implementation.

    make test
    make lint

    Expected result:

        all Go tests pass
        lint passes with 0 issues

2. Update documentation first so implementation has a fixed target.

    Edit:

    - `ARCHITECTURE.md`
    - `docs/design-docs/architecture-overview.md`
    - `docs/design-docs/implementation-spec.md`
    - `docs/design-docs/state-and-storage.md`
    - `docs/design-docs/thread-modes.md`
    - `docs/design-docs/task-mode-worktrees.md`
    - `docs/product-specs/task-mode-user-flow.md`
    - `docs/product-specs/discord-command-behavior.md`
    - `README.md`

3. Implement startup validation, schema changes, and task workspace orchestration in:

    - `internal/config/config.go`
    - `internal/config/config_test.go`
    - `internal/app/types.go`
    - `internal/app/task_service.go`
    - `internal/app/message_service_impl.go`
    - `internal/store/sqlite/store.go`
    - new focused worktree helper package or service files if needed

4. Add or update tests in:

    - `cmd/39claw/main_test.go`
    - `internal/app/task_service_test.go`
    - `internal/app/message_service_test.go`
    - `internal/store/sqlite/store_test.go`
    - any new package-specific tests for Git worktree behavior and pruning logic

5. Run focused tests while iterating.

    go test ./cmd/39claw ./internal/app ./internal/config ./internal/store/sqlite

    Expected result:

        ok   github.com/HatsuneMiku3939/39claw/cmd/39claw
        ok   github.com/HatsuneMiku3939/39claw/internal/app
        ok   github.com/HatsuneMiku3939/39claw/internal/config
        ok   github.com/HatsuneMiku3939/39claw/internal/store/sqlite

6. Run the full repository checks after the implementation lands.

    make test
    make lint

    Observed result on 2026-04-05:

        ok   github.com/HatsuneMiku3939/39claw/cmd/39claw
        ok   github.com/HatsuneMiku3939/39claw/internal/app
        ok   github.com/HatsuneMiku3939/39claw/internal/config
        ok   github.com/HatsuneMiku3939/39claw/internal/store/sqlite
        ok   github.com/HatsuneMiku3939/39claw/internal/thread
        0 issues.
        Linting passed

7. Record proof artifacts showing:

    - `task` mode startup fails when `CLAW_CODEX_WORKDIR` is not a Git repository
    - the first message for a new task creates a worktree under `${CLAW_DATADIR}/worktrees/<task_id>`
    - switching tasks routes later turns through different worktree paths
    - closing enough tasks prunes only old ready worktrees, not branches

## Validation and Acceptance

This plan is complete when all of the following are true:

- `task` mode startup rejects a non-Git `CLAW_CODEX_WORKDIR`
- `daily` mode startup still accepts a non-Git workdir when other requirements are met
- `task-new` creates a task with reserved branch metadata and `worktree_status=pending`
- the first normal message for a pending task creates the task worktree lazily and then runs Codex in that worktree
- a failed worktree creation returns a user-facing failure response and retries on the next normal message
- `task-switch` changes only active task selection, not eager Git state
- `task-close` keeps task branches but prunes ready worktrees beyond the fifteen most recent closed tasks
- open-task worktrees are never pruned
- existing task-mode thread continuity still works across days and restarts
- `make test` passes
- `make lint` passes

The acceptance bar is behavioral. A contributor should be able to observe isolated worktree directories on disk and prove that different tasks execute against different task-specific working directories.

## Idempotence and Recovery

Schema migration must be additive and safe to rerun. Startup or test code should tolerate a database that already contains the new columns. Worktree creation should be written so a repeated attempt can recover from partial failure by cleaning up any incomplete worktree directory before retrying, while still leaving the reserved task branch intact.

Pruning is intentionally best-effort after the task close operation succeeds. If pruning fails, do not reopen the task or clear its `closed_at`. Log the failure, keep the worktree metadata in `ready`, and allow later cleanup to retry. Forced worktree removal is allowed because the task is already closed and old closed-task worktrees are treated as disposable local workspace state.

## Artifacts and Notes

Important expected task-mode lifecycle after this plan:

    /<instance-command> action:task-new task_name:Release prep
    -> task created with branch metadata, no worktree yet

    mention bot with normal prompt
    -> lazy worktree creation
    -> Codex runs in ${CLAW_DATADIR}/worktrees/<task_id>

    /<instance-command> action:task-switch task_id:<other>
    -> active task changes only

    /<instance-command> action:task-close task_id:<id>
    -> task closes
    -> old closed ready worktrees beyond retention are force-pruned
    -> task branch remains in the source repository

Implemented proof points in automated coverage:

    cmd/39claw/main_test.go
    -> rejects non-Git task-mode startup workdirs

    internal/app/message_service_test.go
    -> proves lazy task workdir selection, task switching to distinct worktree paths, and automatic retry after failed workspace setup

    internal/app/task_workspace_test.go
    -> creates and prunes real Git worktrees in a temporary repository

    internal/store/sqlite/store_test.go
    -> proves additive migration and closed-ready task ordering for pruning

## Interfaces and Dependencies

At the end of this plan, the repository should expose a task model shaped like:

    type Task struct {
        TaskID             string
        DiscordUserID      string
        TaskName           string
        Status             TaskStatus
        BranchName         string
        BaseRef            string
        WorktreePath       string
        WorktreeStatus     TaskWorktreeStatus
        CreatedAt          time.Time
        UpdatedAt          time.Time
        ClosedAt           *time.Time
        WorktreeCreatedAt  *time.Time
        WorktreePrunedAt   *time.Time
        LastUsedAt         *time.Time
    }

Revision note (2026-04-05 00:24Z): Updated the living sections after implementing the full task worktree isolation flow, adding persistence migration, Git worktree orchestration, validation, tests, and final verification results.

The persistence layer should support reading and updating those fields without the app layer needing raw SQL knowledge. The Codex execution path should accept a task-specific working directory override while preserving the existing global configuration path for `daily` mode.

Revision Note: 2026-04-04 / Codex - Created this active ExecPlan after the task worktree isolation design was agreed and documented.
Revision Note: 2026-04-05 / Codex - Archived this completed ExecPlan after reconciling the living-document sections and moving the remaining operator-facing ideas into the tech-debt tracker.
