# Prefer the remote default branch when creating `task` worktrees

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, a newly prepared `task` worktree should start from the shared remote default branch state instead of inheriting whatever commits happen to exist on the local source checkout. That means two tasks created around the same time should branch from the same remote baseline even if the operator's local `main` or `master` already contains unpublished commits.

The user-visible proof is concrete. In `task` mode, if the source repository has a local-only commit on `master` but `origin/master` points to an older shared commit, the first normal message for a new task should create a worktree whose `base_ref` points at the remote-tracking branch and whose checked-out history excludes the local-only commit. If remote metadata is stale, the bot should attempt `git fetch origin --prune` before detecting the base ref. If the fetch fails, the bot should still fall back safely to the best available local default branch instead of blocking task execution.

## Progress

- [x] (2026-04-06 08:06Z) Reviewed the current task worktree design, implementation, and repository workflow requirements, and confirmed that base-ref detection is still local-branch-only.
- [x] (2026-04-05 21:21Z) Added remote-aware base-ref resolution to the task workspace manager, including a best-effort `git fetch origin --prune` before detection when `origin` exists.
- [x] (2026-04-05 21:21Z) Added focused tests proving that remote-tracking refs are preferred over local default branches and that task worktree creation still falls back to local `master` when `origin` refresh fails.
- [x] (2026-04-05 21:21Z) Updated task-mode design and product documentation so the remote-first base-ref behavior is now described consistently.
- [x] (2026-04-05 21:21Z) Ran `make test` and `make lint` after the implementation landed.
- [ ] Archive this completed plan from `active/` to `completed/`, update `docs/exec-plans/index.md`, and carry any intentionally deferred follow-up into `docs/exec-plans/tech-debt-tracker.md` if needed.

## Surprises & Discoveries

- Observation: The current implementation stores `base_ref` on the task record, so once the first successful worktree creation chooses a branch reference, later turns will keep reusing that original decision.
  Evidence: `internal/app/task_workspace.go`

- Observation: The current design documentation promises automatic detection of `main` or `master`, but it does not yet distinguish between local branches and remote-tracking branches.
  Evidence: `docs/design-docs/task-mode-worktrees.md` and `docs/design-docs/implementation-spec.md`

- Observation: This repository itself does not currently expose `refs/remotes/origin/HEAD`, so the production-friendly detection order must keep explicit fallbacks to `origin/main` and `origin/master`.
  Evidence: `git symbolic-ref --short refs/remotes/origin/HEAD` exits with `fatal: ref refs/remotes/origin/HEAD is not a symbolic ref` in `/home/filepang/playground/39claw`

- Observation: Real Git integration tests can reproduce the correctness goal by creating a local-only commit after pushing the shared baseline to a bare remote, then asserting that the task worktree still resolves to the remote-tracking commit.
  Evidence: `internal/app/task_workspace_test.go`

## Decision Log

- Decision: Prefer remote-tracking default-branch references for new task worktrees, but keep a local-branch fallback path.
  Rationale: The feature exists to reduce accidental divergence between concurrent tasks, but task execution should remain resilient when `origin` metadata is missing or temporarily unreachable.
  Date/Author: 2026-04-06 / Codex

- Decision: Treat `git fetch origin --prune` as best effort rather than a hard precondition.
  Rationale: A hard fetch requirement would turn transient network or credential issues into avoidable task-mode outages. A best-effort refresh still improves correctness without sacrificing availability.
  Date/Author: 2026-04-06 / Codex

## Outcomes & Retrospective

Implementation is complete and validated locally. The task workspace manager now refreshes `origin` on a best-effort basis, prefers the remote default branch state for first-time worktree creation, and still falls back safely to local `main` or `master` when the remote cannot be used. The remaining work is repository workflow only: archive this finished plan, open the pull request, confirm CI, and merge.

## Context and Orientation

39claw is a Go-based Discord bot that routes messages into Codex threads. In `task` mode, each task can own an isolated Git worktree created lazily from a shared source repository configured by `CLAW_CODEX_WORKDIR`. The task workspace lifecycle lives in `internal/app/task_workspace.go`. The function `EnsureReady` prepares the worktree on the first normal message, and the helper `detectBaseRef` currently checks only local `main` and `master` branches.

The relevant files for this plan are:

- `internal/app/task_workspace.go`
  - task worktree creation, base-ref detection, and pruning logic
- `internal/app/task_workspace_test.go`
  - integration-style tests that create temporary Git repositories and exercise real worktree behavior
- `docs/design-docs/task-mode-worktrees.md`
  - the design note that defines task worktree lifecycle rules
- `docs/design-docs/implementation-spec.md`
  - the implementation-facing summary of task-mode behavior
- `docs/product-specs/task-mode-user-flow.md`
  - the user-facing task-mode workflow expectations
- `docs/exec-plans/index.md`
  - the active/completed plan index that must reflect this work while it is in progress and after it is archived

Terms used in this plan:

- remote-tracking branch: the local Git reference that mirrors a branch on the remote repository, such as `origin/master`
- remote default branch: the branch that should represent the shared team baseline for new worktrees; in this repository it may be discoverable via `origin/HEAD`, `origin/main`, or `origin/master`
- local fallback: the existing behavior of using local `main` or `master` when the remote default branch cannot be resolved

## Plan of Work

Start in `internal/app/task_workspace.go`. Add a best-effort remote refresh step that runs `git fetch origin --prune` before base-ref resolution for a task that does not yet have `BaseRef` stored. Keep failures non-fatal, but log them so operators can diagnose stale remote metadata. Then replace the current local-only `detectBaseRef` helper with remote-aware detection that tries `origin/HEAD`, then `origin/main`, then `origin/master`, and only afterward falls back to local `main` and `master`.

Keep the data model stable. `Task.BaseRef` should continue storing the chosen ref string so later turns and reopen flows remain deterministic. The worktree creation command should still use the resolved ref exactly once when the task branch does not exist yet.

Next, extend `internal/app/task_workspace_test.go` with real Git scenarios. One test should create a bare remote plus a cloned source repository, add a local-only commit on `master`, and prove that a newly created task worktree uses the remote-tracking branch instead of inheriting the unpublished commit. Another test should cover the safe fallback case where the source repository has no usable remote default branch but still has a valid local default branch.

Finally, update the task worktree design and implementation docs so they explain the remote-first rule and best-effort refresh behavior in plain language. If the final implementation introduces any meaningful operator caveat, record it in this plan before archiving.

## Concrete Steps

Run all commands from `/home/filepang/playground/39claw`.

1. Confirm the repository starts from a passing baseline.

    make test
    make lint

2. Implement remote-aware base-ref resolution and best-effort fetch in:

    - `internal/app/task_workspace.go`
    - `internal/app/task_workspace_test.go`

3. Update the affected docs:

    - `docs/design-docs/task-mode-worktrees.md`
    - `docs/design-docs/implementation-spec.md`
    - `docs/product-specs/task-mode-user-flow.md`
    - `docs/exec-plans/index.md`

4. Run focused tests while iterating.

    go test ./internal/app

5. Run the full repository checks before committing.

    make test
    make lint

    Observed result on 2026-04-05:

        ok  	github.com/HatsuneMiku3939/39claw/cmd/39claw	0.208s
        ok  	github.com/HatsuneMiku3939/39claw/cmd/codexplay	(cached)
        ok  	github.com/HatsuneMiku3939/39claw/internal/app	(cached)
        ok  	github.com/HatsuneMiku3939/39claw/internal/codex	(cached)
        ok  	github.com/HatsuneMiku3939/39claw/internal/config	(cached)
        ok  	github.com/HatsuneMiku3939/39claw/internal/dailymemory	(cached)
        ok  	github.com/HatsuneMiku3939/39claw/internal/observe	(cached)
        ok  	github.com/HatsuneMiku3939/39claw/internal/releaseconfig	(cached)
        ok  	github.com/HatsuneMiku3939/39claw/internal/runtime/discord	0.035s
        ok  	github.com/HatsuneMiku3939/39claw/internal/store/sqlite	(cached)
        ok  	github.com/HatsuneMiku3939/39claw/internal/thread	(cached)
        ?   	github.com/HatsuneMiku3939/39claw/version	[no test files]
        0 issues.
        Linting passed

6. Create a conventional-commit change, open a GitHub pull request with the required template sections, wait for CI to pass, merge with a merge commit, and then archive this plan into `docs/exec-plans/completed/`.

## Validation and Acceptance

This plan is complete when all of the following are true:

- a new task worktree prefers the remote default branch reference when the source repository has a usable `origin` remote
- best-effort `git fetch origin --prune` runs before first-time base-ref detection and does not block task execution when it fails
- the chosen `base_ref` is persisted on the task and remains stable for later task turns
- automated tests prove both the remote-first path and the local fallback path
- `make test` passes
- `make lint` passes

Acceptance is behavioral. A contributor should be able to create a source repository where local `master` has extra unpublished commits, prepare a new task worktree, and observe that the task worktree starts from the shared remote baseline rather than the local-only history.

## Idempotence and Recovery

The new detection flow must remain safe to retry. If a task already has `BaseRef` stored, later turns must reuse it without re-detecting. If `git fetch origin --prune` fails because the network is unavailable or credentials are missing, worktree creation should continue by probing the existing remote-tracking refs and then the local default branches. Tests should construct their own temporary repositories so rerunning them does not depend on developer-local Git state.

## Artifacts and Notes

Expected detection order after this plan:

    git fetch origin --prune   # best effort
    resolve origin/HEAD if it points to a commit
    otherwise resolve origin/main
    otherwise resolve origin/master
    otherwise resolve local main
    otherwise resolve local master

Expected observable proof for the remote-first test:

    source repo local master: commit B
    origin/master: commit A
    new task worktree branch: task/<id> based on commit A
    task BaseRef: origin/master (or the equivalent remote default ref)

## Interfaces and Dependencies

The implementation should stay within the existing `internal/app` package and continue using the standard library only. The task workspace manager should keep using `runGit(ctx, args...)` so subprocess behavior and logging stay centralized. No new configuration flag is needed; the repository should simply become smarter about choosing the base ref for new task worktrees.

Revision Note: 2026-04-06 / Codex - Created this active ExecPlan after deciding to prefer the remote default branch for new task worktrees while keeping a safe local fallback.

Revision Note: 2026-04-05 / Codex - Updated the living sections after implementing remote-first task worktree base-ref detection, adding tests, and recording local validation results.
