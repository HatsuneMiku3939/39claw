# 39claw Docs

This directory is the documentation home for 39claw.

It is organized into a small set of document layers so contributors can quickly find the right level of detail.

## Document Map

- [Root Architecture Reference](../ARCHITECTURE.md)
  - The authoritative architecture document for the repository
- [Release Runbook](./operations/RELEASE_RUNBOOK.md)
  - Operator-facing release readiness, tagging, and post-release verification steps
- [Product Specs](./product-specs/index.md)
  - Product-facing user journeys, interaction rules, and expected behavior
- [Design Docs](./design-docs/index.md)
  - Supporting design notes, focused concepts, and onboarding-oriented summaries
- [ExecPlans](./exec-plans/index.md)
  - Living execution plans, active implementation tracks, and deferred follow-up notes
- [References](./references/index.md)
  - External references and bundled source material

## When to Use Each Layer

Use `docs/product-specs` when the question is about:

- what the user should experience
- what behavior should feel intuitive
- what product rule should remain stable even if implementation changes

Use `docs/design-docs` when the question is about:

- how the system is structured
- which responsibilities belong to which component
- how thread routing, storage, and integration boundaries should work

Use `docs/exec-plans` when the question is about:

- the step-by-step delivery plan for a concrete feature
- which implementation milestone is currently active
- which follow-up items were intentionally deferred during delivery

Use the root `ARCHITECTURE.md` when the question is about:

- the authoritative architecture decision
- the intended system boundaries or v1 scope
- resolving ambiguity between design notes

Use `docs/operations/RELEASE_RUNBOOK.md` when the question is about:

- how to decide whether a commit is releasable
- how to create and push a release tag
- how to verify the draft GitHub Release after automation runs

Use `docs/references` when the question is about:

- external SDK behavior
- source material used to guide implementation
- bundled reference projects or supporting documents

## Current Focus Areas

- defining user-facing behavior for `daily` and `task` modes
- keeping the Codex-native architecture boundaries clear
- validating the experimental Go Codex integration path

## Maintenance Notes

- Keep product behavior in `docs/product-specs`.
- Keep the authoritative architecture model in `ARCHITECTURE.md`.
- Keep supporting implementation design notes in `docs/design-docs`.
- Keep living execution plans and deferred follow-up notes in `docs/exec-plans`.
- Keep external source material in `docs/references`.
- Update this index when a new top-level documentation layer is introduced.
