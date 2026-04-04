---
name: archive-completed-execplan
description: Archive a finished ExecPlan that is incorrectly still listed under `docs/exec-plans/active` in this repository. Use when Codex is asked to clean up, archive, reconcile, or finalize completed active plans, including checking whether the plan is truly done, updating its living-document sections, moving it to `docs/exec-plans/completed`, updating `docs/exec-plans/index.md`, and recording deferred follow-up work in `docs/exec-plans/tech-debt-tracker.md` when needed.
---

# Archive Completed ExecPlan

## Overview

Archive a completed ExecPlan cleanly and conservatively. Confirm that the plan is actually ready to leave `active/`, finish the plan's living-document updates, move it into `completed/`, and keep the plan index plus deferred-work tracker aligned with the new state.

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

If the user also wants a commit, run `make lint` and `make test` before committing because this repository requires both checks before commit.

## Report the Result

Report whether the plan was archived or intentionally left active. Name the files changed, mention any deferred work recorded, and call out any manual follow-up or uncertainty that still remains.
