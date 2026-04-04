---
name: archive-completed-execplan
description: Archive a finished ExecPlan that is incorrectly still listed under `docs/exec-plans/active` in this repository. Use when Codex is asked to clean up, archive, reconcile, or finalize completed active plans, including checking whether the plan is truly done, creating a dedicated branch for the cleanup, updating the plan's living-document sections, moving it to `docs/exec-plans/completed`, updating `docs/exec-plans/index.md`, recording deferred follow-up work in `docs/exec-plans/tech-debt-tracker.md` when needed, and then committing and opening a GitHub pull request.
---

# Archive Completed ExecPlan

## Overview

Archive a completed ExecPlan cleanly and conservatively. Confirm that the plan is actually ready to leave `active/`, finish the plan's living-document updates, move it into `completed/`, and keep the plan index plus deferred-work tracker aligned with the new state.

When the plan is ready to archive, complete the full repository workflow instead of stopping at file edits: create a dedicated branch, make the cleanup changes, validate them, commit them, and open a pull request.

## Read the Required Repository Documents

Read these files before editing:

- `AGENTS.md`
- `.agents/PLANS.md`
- `docs/exec-plans/index.md`
- `docs/exec-plans/completed/README.md`
- the target file under `docs/exec-plans/active/`

Read additional design or product docs only when the target plan depends on them for completion judgment.

## Decide Whether the Plan Is Ready to Archive

Archive the plan only when repository evidence shows the implementation work is complete for that plan.

Require these conditions unless the user explicitly overrides them:

- The plan's acceptance criteria and validation steps are satisfied.
- The `Progress` section can be brought to a fully completed state for the planned implementation work.
- Any remaining work is non-blocking follow-up, not unfinished scope.

Leave the plan in `active/` when required implementation or required validation is still missing. Explain the blocker instead of archiving prematurely.

When completion is ambiguous, inspect the relevant code, tests, and documentation first. If the archive decision still depends on an unresolved judgment call, stop and ask the user rather than guessing.

If no active plan is ready to archive, do not create a branch or PR. Report the findings and leave the repository untouched.

## Create a Dedicated Branch Before Editing

If the target plan is ready to archive, create a dedicated branch before modifying files.

Follow these rules:

- Never commit directly to `master` or `main`.
- Use the repository default prefix `feature/`.
- Pick a branch name that describes the cleanup, for example `feature/archive-execplan-05`.
- Check the worktree first and avoid disturbing unrelated user changes.
- If unrelated staged or modified files would contaminate the archive change, stop and ask the user instead of sweeping them into the branch.

## Finalize the Plan Before Moving It

Update the target plan while it is still in `active/`.

Do all of the following:

- Add a final `Progress` entry that says the plan was archived and why.
- Update `Outcomes & Retrospective` so it explains what shipped, what remains, and why the plan no longer belongs in `active/`.
- Update `Decision Log` or `Surprises & Discoveries` when the archive decision depends on a notable implementation fact.
- Add a `Revision Note` at the bottom describing the archival change and the reason.
- Keep the document self-contained and aligned with `.agents/PLANS.md`.

Do not rewrite unrelated scope just to make the archive look tidy.

## Record Deferred Work Explicitly

When the plan is complete but intentionally leaves a non-blocking gap, add or update an entry in `docs/exec-plans/tech-debt-tracker.md`.

Link the completed plan path, explain why the work was deferred, and describe the smallest safe next step. Do not leave deferred work implied only inside the plan body.

## Move and Relink the Plan

Move the file from `docs/exec-plans/active/` to `docs/exec-plans/completed/` without renaming it unless the user explicitly asks for renumbering.

Update `docs/exec-plans/index.md` so the plan disappears from `Current Active Plans` and appears in `Recently Completed Plans`.

Fix links that still reference the old active path. Preserve the repository's numeric ordering; do not renumber other plans just because one plan moved.

## Validate the Archive

Confirm all of the following before finishing:

- the plan file exists only under `docs/exec-plans/completed/`
- `docs/exec-plans/index.md` points at the completed path
- `docs/exec-plans/tech-debt-tracker.md` points at the completed path when deferred work exists
- the plan's living-document sections reflect the final completed state

Run `make lint` and `make test` before committing because this repository requires both checks before commit.

If these checks fail, do not commit. Report the failure and the reason.

## Commit and Open a Pull Request

After the archive edits and validation succeed, finish the Git workflow.

Follow these rules:

- Stage only the files that belong to the archive cleanup.
- Write an English Conventional Commit message under 100 characters.
- Use GitHub CLI for push and PR creation.
- Write the PR title in English.
- Write the PR body in English Markdown with exactly these sections:
  `## Summary`
  `## Background`
  `## Related issue(s)`
  `## Implementation details`
  `## Test coverage`
  `## Breaking changes`
  `## Notes`
- End the PR body with `Created by Codex`.
- Add appropriate labels such as `documentation` or `enhancement`.

Unless the user explicitly asks for a different stopping point, the expected finish line for this skill is:

1. a dedicated branch exists
2. the completed plan has been archived
3. validation passed
4. a commit exists
5. a PR is open

## Report the Result

Report whether the plan was archived or intentionally left active. Name the files changed, mention any deferred work recorded, and call out the branch name, commit, PR URL, and any manual follow-up or uncertainty that still remains.
