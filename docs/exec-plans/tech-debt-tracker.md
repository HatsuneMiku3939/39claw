# Tech Debt Tracker

This file records follow-up work that is intentionally deferred while delivering an active ExecPlan.

## How to Use This File

- Add one entry when implementation deliberately leaves a non-blocking gap behind.
- Link the relevant ExecPlan and explain why the work was deferred.
- Remove or mark an entry resolved when the follow-up lands.

## Entry Template

### Title

- Status: `open` or `resolved`
- Date:
- Related plan:
- Owner:

### Context

Explain the current behavior and why the gap exists.

### Risk

Explain the user-facing or contributor-facing downside if the debt remains.

### Next step

Describe the smallest safe follow-up that should address the debt.

## Current Entries

### First live tagged release follow-up

- Status: open
- Date: 2026-04-05
- Related plan: `docs/exec-plans/completed/10-first-stage-release-automation.md`
- Owner: Unassigned

### Context

The repository now contains a complete first-stage tag-driven release path, including a checked-in runbook, release workflow, GoReleaser config, CI validation, and successful local snapshot verification. What has not yet been done from this implementation environment is pushing the first real `v*` tag that would create a draft GitHub Release in the GitHub repository.

This gap is intentional. Pushing a real release tag is an operator action with external effects, so it was left outside the coding change itself even though the repository is now ready for it.

### Risk

Until one maintainer performs the first live tagged release, the project still lacks proof that the exact GitHub-hosted draft-release flow works end to end against the production repository permissions and release page. Local snapshot builds and `goreleaser check` prove the repository configuration, but they do not prove the final hosted release event.

### Next step

Follow `docs/operations/RELEASE_RUNBOOK.md` from a clean, up-to-date `master` checkout and perform the first live tagged release, starting with a small version such as `v0.1.0`. After the draft GitHub Release is created successfully and the attached archives are verified, mark this entry resolved or replace it with any concrete follow-up discovered during the first live run.

### Runtime validation strategy and narrow live Discord hardening follow-up

- Status: open
- Date: 2026-04-05
- Related plan: `docs/exec-plans/completed/04-discord-runtime-and-presentation.md`, `docs/exec-plans/completed/06-discord-image-input.md`
- Owner: Unassigned

### Context

The Discord runtime implementation has landed and the delivery plans have been archived, but the remaining validation debt should no longer be described as broad missing live Discord smoke coverage.

The repository already has meaningful automated validation for message mapping, command routing, chunking, image download handling, multipart-input forwarding, queueing, and deferred reply behavior. It now also has a reusable fake-runtime vocabulary under `internal/testutil/runtimeharness` plus a Discord contract-style suite under `internal/runtime/discord` that drives the real runtime startup path through a fake session for mentions, streamed edits, queued replies, command interactions, and attachment-aware message flow.

The live Discord gap should therefore be treated as narrow hardening work around behaviors that automated tests and fake runtimes cannot fully prove, such as real command-registration propagation, hosted attachment fetches, permission or intent quirks, and final reply delivery behavior in a real Discord deployment.

This follow-up remains intentionally open. For now, the repository accepts a documented gap instead of trying to automate disposable Discord bot provisioning or broad live-server validation inside the normal development workflow.

### Risk

If this debt keeps a Discord-smoke-centric framing, contributors may over-invest in platform-specific manual checks while under-investing in reusable automated coverage at the application and runtime-adapter boundaries.

Automated tests still cannot prove that real Discord registration, hosted attachment fetches, permission settings, and final reply delivery behave exactly as expected in a live deployment. If those narrow external-platform risks are left implicit, contributors may either assume too much confidence or run ad hoc live checks without a clear trigger.

### Next step

Treat this entry as a two-part follow-up:

- keep the fake-runtime suites current as new runtime-visible behavior is added so contributors continue to invest in automated contract coverage first
- reserve live Discord checks for the remaining platform-only behaviors instead of reopening broad smoke-test expectations

Keep the live Discord check as optional hardening until one of the following becomes true:

- a release-hardening pass decides that Discord-specific external behavior now warrants manual confirmation
- a sustainable disposable-bot workflow is available for repeatable targeted live checks
- a production or staging issue suggests the real Discord path needs explicit end-to-end confirmation

When one of those triggers is met, run the documented Discord checklist from `README.md` with a disposable bot token and guild ID, focusing on the narrow live-platform remainder such as reply delivery, hosted attachments, and command-registration behavior. Then either mark this entry resolved or record any runtime-specific fixes in a follow-up ExecPlan.

### Task worktree operator ergonomics follow-up

- Status: open
- Date: 2026-04-05
- Related plan: `docs/exec-plans/completed/08-task-mode-worktree-isolation.md`
- Owner: Unassigned

### Context

Task-isolated worktrees now exist and the repository validates the core lifecycle correctly, but the shipped v1 surface still keeps worktree lifecycle mostly implicit. Contributors and operators can recover from failed creation through automatic retry, yet there is no dedicated Discord-visible diagnostic or repair affordance for inspecting worktree state, forcing them to rely on logs or the SQLite store when they want to understand why a task workspace is pending, failed, ready, or pruned.

This gap is intentionally deferred because the core scope was isolated task execution, not workspace administration UX. Keeping the follow-up explicit here avoids leaving the completed ExecPlan in `active/` just to remember an optional hardening idea.

### Risk

When task worktree preparation fails or old worktrees are pruned, users may have limited self-service visibility into what happened. That can increase operator support burden and make it slower to distinguish transient setup failures from expected closed-task cleanup behavior.

### Next step

Add the smallest useful operator-facing surface for task worktree state before expanding the workflow further. A safe first increment is to expose each task's current worktree lifecycle state in existing task command responses such as `task-current` and `task-list`, then decide later whether a dedicated repair or cleanup command is still necessary.
