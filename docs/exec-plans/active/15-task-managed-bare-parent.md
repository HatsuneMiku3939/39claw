# Replace task-mode source-checkout parenting with a managed bare repository

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, `task` mode should stop using the operator's visible source checkout as the parent repository that owns task worktrees. Instead, startup should prepare a managed bare repository under `CLAW_DATADIR`, and every task worktree should be created from that managed bare parent. This keeps the human-facing checkout at `CLAW_CODEX_WORKDIR` untouched while removing the Git branch-occupancy problem that currently prevents task worktrees from switching to the default branch when the source checkout already has `main` or `master` checked out.

The user-visible proof is practical. In `task` mode with a source repository that has an `origin` remote, the first normal message for a new task should create a managed bare repository under `${CLAW_DATADIR}/repos`, create the task worktree under `${CLAW_DATADIR}/worktrees/<task_id>`, and allow Git commands inside that task worktree to switch to the default branch without being blocked by the operator's checkout at `CLAW_CODEX_WORKDIR`. If the configured source checkout has no `origin` remote, startup should fail immediately with a clear configuration error instead of falling into an ambiguous local-only synchronization model.

## Progress

- [x] (2026-04-10 09:48Z) Reviewed the existing task worktree implementation, the completed worktree-isolation plans, and the current product/design docs to confirm that `CLAW_CODEX_WORKDIR` still acts as both the operator checkout and the worktree parent repository.
- [x] (2026-04-10 09:48Z) Decided that the managed-bare redesign will require an `origin` remote in `task` mode and will not preserve no-remote support, because the local-only synchronization story is high-complexity and low-value.
- [ ] Implement startup validation that requires both a Git repository and an `origin` remote when `CLAW_MODE=task`.
- [ ] Add managed bare repository lifecycle support under `${CLAW_DATADIR}/repos/<repo-id>.git`, seeded from the source checkout path and refreshed from the real `origin` remote.
- [ ] Move task worktree creation to the managed bare parent while preserving task branch identity, remote push behavior, and closed-task pruning.
- [ ] Update architecture, design, product, and operator docs so the new task-mode repository model is fully described.
- [ ] Add automated coverage for startup validation, managed-bare creation, remote propagation, task worktree branch switching, and regression cases around task pruning.
- [ ] Run repository validation (`make test` / `make lint`, or the documented command-level fallback when `make` is unavailable), open the feature PR, merge it, and archive this plan into `docs/exec-plans/completed/`.

## Surprises & Discoveries

- Observation: The current Git branch-occupancy pain comes from the source checkout itself holding `main` or `master`, not from the task worktree branch naming scheme.
  Evidence: `internal/app/task_workspace.go` currently runs `git worktree add` directly in the repository at `CLAW_CODEX_WORKDIR`, and the source checkout therefore participates in Git's "one checked-out branch per repository" rule.

- Observation: The current product and design documents still describe `CLAW_CODEX_WORKDIR` as the shared source repository root for task worktree creation, so this redesign changes the repository model rather than merely changing a helper function.
  Evidence: `ARCHITECTURE.md`, `docs/design-docs/task-mode-worktrees.md`, and `docs/design-docs/implementation-spec.md`

- Observation: A managed bare parent solves the branch-occupancy problem only when the runtime stops treating the visible source checkout as the parent repository; simply detaching task worktrees would not help.
  Evidence: Git worktree branch occupancy is enforced per parent repository, and the current parent repository is the operator checkout at `CLAW_CODEX_WORKDIR`.

## Decision Log

- Decision: The redesign will introduce a managed bare repository under `${CLAW_DATADIR}/repos` and will create all task worktrees from that managed bare parent instead of from `CLAW_CODEX_WORKDIR`.
  Rationale: This removes the Git branch-occupancy conflict without mutating the operator's visible checkout.
  Date/Author: 2026-04-10 / Codex

- Decision: `task` mode will require an `origin` remote after this redesign lands.
  Rationale: The managed-bare model needs a clear remote synchronization story for fetch, push, and recovery, and no-remote support adds disproportionate complexity for little practical value.
  Date/Author: 2026-04-10 / Codex

- Decision: The managed bare repository will be seeded from the local source checkout path, then its `origin` remote configuration will be rewritten from the real source-checkout remote settings before task worktrees start using it.
  Rationale: Seeding from the local path captures the current committed local refs, while restoring the real remote settings preserves normal fetch and push behavior from task worktrees.
  Date/Author: 2026-04-10 / Codex

- Decision: This redesign will not attempt automatic bidirectional synchronization between the visible source checkout and the managed bare repository.
  Rationale: Automatic local-only sync creates ambiguous source-of-truth semantics, especially when the source checkout is dirty or when task branches diverge inside the managed bare parent.
  Date/Author: 2026-04-10 / Codex

## Outcomes & Retrospective

This plan is not implemented yet. The intended outcome is a simpler and safer task-mode repository model: operators keep using `CLAW_CODEX_WORKDIR` as a normal checkout, while 39claw owns a separate managed bare repository for task worktree lifecycle. The expected benefit is better Git ergonomics inside task worktrees and a clearer operational boundary between human workspace state and bot-managed workspace state.

The main risk area is not the worktree command itself; it is the surrounding contract change. Startup validation, remote propagation, branch retention, recovery instructions, and the user/operator documentation all have to describe the same repository model or later contributors will misunderstand which repository owns task branches and remotes.

## Context and Orientation

39claw is a Go-based Discord bot that routes messages into Codex threads. In `task` mode, each task has a task record, a Codex thread binding, and lazily created Git worktree metadata. Today, `CLAW_CODEX_WORKDIR` is both the operator-visible checkout and the Git repository that directly owns all task worktrees. The task worktree manager lives in `internal/app/task_workspace.go`, and it currently runs `git worktree add` directly against that visible source checkout.

This plan changes the repository model. After implementation, the operator-visible checkout at `CLAW_CODEX_WORKDIR` remains the configuration anchor and the initial seed source, but it will no longer be the repository that owns task worktrees. Instead, startup will materialize a managed bare repository under `${CLAW_DATADIR}/repos/<repo-id>.git`. A "bare repository" is a Git repository that stores refs, objects, and configuration but does not itself contain a checked-out working tree. Git can still create linked worktrees from it, which makes it a good parent for bot-managed task worktrees because it does not keep `main` or `master` checked out anywhere.

The most relevant files are:

- `cmd/39claw/main.go`
  - startup wiring, runtime assembly, and validation flow
- `internal/config/config.go`
  - environment loading and task-mode path validation
- `internal/app/task_workspace.go`
  - task worktree creation, base-ref detection, branch handling, and pruning
- `internal/app/task_workspace_test.go`
  - integration-style tests that exercise real Git repositories and worktrees
- `internal/app/types.go`
  - task data model fields such as `branch_name`, `base_ref`, `worktree_path`, and `worktree_status`
- `ARCHITECTURE.md`
  - the top-level architecture contract
- `docs/design-docs/task-mode-worktrees.md`
  - the task worktree design document
- `docs/design-docs/implementation-spec.md`
  - the implementation-facing behavior summary
- `docs/product-specs/task-mode-user-flow.md`
  - the user-facing task workflow
- `docs/operations/RELEASE_RUNBOOK.md`
  - operator-facing release and verification guidance
- `docs/exec-plans/index.md`
  - the living index that must show this plan while it is active

Terms used in this plan:

- source checkout: the operator-visible Git checkout configured through `CLAW_CODEX_WORKDIR`
- managed bare parent: the bare Git repository created and maintained by 39claw under `${CLAW_DATADIR}/repos`
- task worktree: the task-specific checkout under `${CLAW_DATADIR}/worktrees/<task_id>` created from the managed bare parent
- remote propagation: copying the real remote URL and related fetch/push configuration from the source checkout into the managed bare parent so task worktrees can fetch and push normally

## Plan of Work

Start by updating the repository contracts before touching code. `ARCHITECTURE.md`, `docs/design-docs/task-mode-worktrees.md`, `docs/design-docs/implementation-spec.md`, `docs/design-docs/architecture-overview.md`, `docs/product-specs/task-mode-user-flow.md`, `docs/product-specs/discord-command-behavior.md`, and `README.md` must all stop describing `CLAW_CODEX_WORKDIR` as the repository that directly owns task worktrees. They must instead describe it as the operator-visible seed checkout and configuration anchor, while the task worktree parent moves under `${CLAW_DATADIR}`. The docs must also state clearly that `task` mode now requires an `origin` remote and that no-remote repositories fail fast at startup.

Next, extend startup validation in `internal/config/config.go` and any startup helpers used from `cmd/39claw/main.go`. The validation must continue rejecting non-Git repositories in `task` mode, and it must now also reject a source checkout that lacks an `origin` remote. The error message must be explicit enough that an operator immediately understands the new product contract, for example by naming `CLAW_CODEX_WORKDIR` and explaining that managed-bare task mode needs an `origin` remote.

Then redesign the task workspace manager in `internal/app/task_workspace.go`. Introduce a small managed-bare lifecycle layer that derives a stable bare-repository path under `${CLAW_DATADIR}/repos`. The stable repo identifier should be based on the source checkout path or other deterministic local identity so repeated startups reuse the same managed bare parent. On first use, seed that bare parent from the source checkout path so current committed local refs are available. Immediately after seeding, rewrite the managed bare repository's `origin` remote configuration from the real remote settings discovered in the source checkout. On later startups and later task preparations, refresh the managed bare parent with `git fetch origin --prune` as a best-effort step before choosing the base ref for a new task worktree.

Keep task branch identity stable. `Task.BranchName` should continue to represent the task branch, but after this redesign that branch will live in the managed bare parent rather than in the operator-visible source checkout. The `EnsureReady` flow should create or attach the task branch inside the managed bare parent and materialize the task worktree under `${CLAW_DATADIR}/worktrees/<task_id>`. Base-ref detection should run against the managed bare parent, still preferring `origin/HEAD`, `origin/main`, and `origin/master` before any local fallback that remains meaningful in the managed-bare repository.

Remote push behavior must remain intact. The managed bare parent and any worktree created from it must expose the real remote URLs, including push behavior when the source checkout had a dedicated push URL. This means the implementation must inspect the source checkout remote configuration and copy the meaningful remote settings into the managed bare parent. At minimum, the plan should preserve `origin.url`, and it should also preserve `remote.origin.pushurl` when present so a task worktree can still run `git push origin HEAD` against the correct remote target. If the source checkout has additional remotes that matter to task workflows, document whether the implementation will copy all remotes or only `origin`, and keep that decision consistent across docs and tests.

After the managed bare parent exists, revisit pruning and recovery. Closed-task worktree pruning should continue removing only worktrees, not task branches, but the branch-retention story now refers to the managed bare parent instead of the source checkout. Operator documentation must explain where task branches now live and how an operator can inspect or recover them. If future recovery needs require exposing the managed bare repository path in logs or docs, the plan should specify exactly where that guidance belongs.

Finally, add focused automated coverage in `internal/app/task_workspace_test.go`, `internal/config/config_test.go`, and any startup tests under `cmd/39claw`. The tests should prove the new startup requirement, managed-bare creation, remote propagation, base-ref refresh, branch-retention behavior, and the ergonomic win that motivated this plan: a task worktree can switch to the default branch without being blocked by the source checkout because the source checkout is no longer the parent repository that owns the worktree.

## Concrete Steps

Run all commands from `/home/filepang/workspaces/39claw/39claw`.

1. Confirm the repository starts from a passing baseline.

    make test
    make lint

    If `make` is unavailable in the execution environment, use:

        go test ./...
        ./scripts/lint -c .golangci.yml

2. Update the repository contracts before implementation.

    Edit:

    - `ARCHITECTURE.md`
    - `README.md`
    - `docs/design-docs/architecture-overview.md`
    - `docs/design-docs/implementation-spec.md`
    - `docs/design-docs/task-mode-worktrees.md`
    - `docs/product-specs/task-mode-user-flow.md`
    - `docs/product-specs/discord-command-behavior.md`
    - `docs/operations/RELEASE_RUNBOOK.md`

3. Implement the managed-bare task-mode runtime path.

    Edit:

    - `cmd/39claw/main.go`
    - `internal/config/config.go`
    - `internal/config/config_test.go`
    - `internal/app/task_workspace.go`
    - `internal/app/task_workspace_test.go`
    - any focused helper file introduced under `internal/app/` for managed-bare repository lifecycle support

4. Run focused tests while iterating.

    go test ./cmd/39claw ./internal/app ./internal/config

    Expected result:

        ok   github.com/HatsuneMiku3939/39claw/cmd/39claw
        ok   github.com/HatsuneMiku3939/39claw/internal/app
        ok   github.com/HatsuneMiku3939/39claw/internal/config

5. Manually prove the user-visible Git ergonomics improvement in a temporary repository fixture or scripted test scenario.

    The proof should show:

    - the operator source checkout still has the default branch checked out
    - a new task worktree is created from the managed bare parent under `${CLAW_DATADIR}/worktrees/<task_id>`
    - inside that task worktree, `git switch <default-branch>` succeeds because the managed bare parent is branch-neutral

6. Run the full repository checks before committing.

    make test
    make lint

    If `make` is unavailable:

        go test ./...
        ./scripts/lint -c .golangci.yml

7. Commit with a conventional commit message, open a GitHub pull request with the repository's required template sections, wait for CI to pass, merge with a merge commit, update local `master`, and archive this plan into `docs/exec-plans/completed/`.

## Validation and Acceptance

This plan is complete when all of the following are true:

- `task` mode startup rejects a `CLAW_CODEX_WORKDIR` that is not a Git repository
- `task` mode startup also rejects a `CLAW_CODEX_WORKDIR` that lacks an `origin` remote
- startup or first task preparation materializes a managed bare repository under `${CLAW_DATADIR}/repos/<repo-id>.git`
- new task worktrees are created from that managed bare parent instead of from the visible source checkout
- a task worktree can switch to the default branch even when the visible source checkout also has that branch checked out
- task worktrees preserve fetch and push behavior against the real remote because the managed bare parent has the correct remote configuration
- task branch retention and closed-task pruning still work, with task branches now living in the managed bare parent
- repository docs consistently describe the new managed-bare model and the `origin` remote requirement
- `make test` passes, or `go test ./...` passes when `make` is unavailable
- `make lint` passes, or `./scripts/lint -c .golangci.yml` passes when `make` is unavailable

Acceptance is behavioral. A contributor should be able to inspect `${CLAW_DATADIR}/repos`, see the managed bare parent, create a task, and observe that the task worktree behaves like an isolated Git checkout whose branch operations are not blocked by the human-facing source checkout at `CLAW_CODEX_WORKDIR`.

## Idempotence and Recovery

The managed bare repository path must be deterministic so repeated startups reuse the same repository instead of cloning a new bare parent every time. Seeding or remote-propagation steps must be safe to rerun: if the managed bare parent already exists, refresh it instead of recreating it. If startup validation fails because `origin` is missing, exit before mutating `${CLAW_DATADIR}` so operators can fix the checkout safely and retry.

Because this design intentionally avoids automatic synchronization back into the source checkout, recovery guidance must be explicit. If an operator wants to inspect task branches, they should look in the managed bare parent or fetch from it intentionally; the implementation must not try to push task-branch state back into the source checkout automatically. Closed-task pruning must continue to be best-effort and must never delete task branches from the managed bare parent.

## Artifacts and Notes

Expected repository shape after implementation:

    CLAW_CODEX_WORKDIR/
        .git/
        ...operator-visible checkout...

    CLAW_DATADIR/
        39claw.sqlite
        repos/
            <repo-id>.git/
        worktrees/
            <task_id>/

Expected startup validation failure when `origin` is missing:

    error: task mode requires CLAW_CODEX_WORKDIR to have an origin remote: /absolute/path/to/repo

Expected ergonomic proof inside a prepared task worktree:

    git branch --show-current
    task/<task-id>

    git switch main
    Switched to branch 'main'

The exact default-branch name may be `master` instead of `main`; the proof should use whatever branch the repository advertises through `origin/HEAD`.

## Interfaces and Dependencies

The implementation should stay within the existing Go standard library plus the repository's current Git subprocess approach. Keep using `internal/app.GitTaskWorkspaceManager` as the app-layer owner of task worktree lifecycle, and continue running Git through its centralized `runGit(ctx, args...)` helper so logging and subprocess behavior stay consistent. New helper functions or helper files under `internal/app/` are acceptable when they isolate managed-bare responsibilities such as repository-path derivation, remote propagation, or idempotent bare-repository refresh.

No new third-party libraries are needed. Prefer extending the current config validation and startup assembly rather than adding a new subsystem. If a new persisted field becomes necessary to record the managed bare repository identity or path, document the exact schema change in this plan before implementation and add the corresponding migration plus tests in the same milestone.

Revision Note: 2026-04-10 / Codex - Created this active ExecPlan after deciding that the task-mode branch-occupancy problem should be solved by moving task worktrees onto a managed bare parent repository and by explicitly requiring an `origin` remote instead of preserving no-remote support.
