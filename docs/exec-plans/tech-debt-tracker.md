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

### Discord runtime smoke test follow-up

- Status: open
- Date: 2026-04-05
- Related plan: `docs/exec-plans/completed/04-discord-runtime-and-presentation.md`, `docs/exec-plans/completed/06-discord-image-input.md`
- Owner: Unassigned

### Context

The Discord runtime implementation has landed and the delivery plans have been archived, but the final manual smoke test against a disposable Discord server was not run before archival because disposable bot credentials were not available in the implementation environment.

This gap now covers both the general runtime flow and the live validation of Discord-hosted image attachments. Automated tests prove the image download, cleanup, multipart-input forwarding, and deferred reply behavior, but they do not prove the final live Discord path with real attachment hosting and reply delivery.

This follow-up remains intentionally open. For now, the repository accepts a documented gap instead of trying to automate disposable Discord bot provisioning or full live-server validation inside the normal development workflow.

### Risk

Automated tests prove message mapping, command routing, chunking, presentation behavior, image download handling, and multipart-input forwarding, but they cannot prove that live Discord registration, hosted attachment fetches, and reply flow behave exactly as expected in a real server.

Because the gap is documented rather than closed, contributors may incorrectly assume that the live Discord path has already been validated end to end. Any real-server integration regression would therefore be discovered later than a fully automated or regularly executed smoke test would allow.

### Next step

Keep this entry open until one of the following becomes true:

- a sustainable disposable-bot workflow is available for repeatable live smoke testing
- a release-hardening pass decides that manual real-server validation is now worth the setup cost
- a production or staging issue suggests the live Discord path needs explicit end-to-end confirmation

Until then, treat this as explicit technical debt rather than an implicit missing task. When one of the triggers above is met, run the documented smoke test from `README.md` with a disposable Discord bot token and guild ID, including both normal reply and image-attachment scenarios, then either mark this entry resolved or record any runtime-specific fixes in a follow-up ExecPlan.

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
