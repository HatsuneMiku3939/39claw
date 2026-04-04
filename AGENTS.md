# AGENTS.md

This file is the working guide for contributors and coding agents operating in this repository.

Follow this document during development.
If implementation direction changes, update this file together with the relevant architecture documents.

## Project Summary

39claw is a Go-based, Codex-native Discord bot.

The application is intentionally thin:

- Codex owns remote thread execution
- Codex owns tool orchestration
- 39claw owns Discord integration
- 39claw owns thread routing policy
- 39claw owns local persistence for thread bindings

## Development Rules

### 1. Respect the architecture

Treat `ARCHITECTURE.md` as the primary implementation guide.

Do not introduce local agent-loop behavior unless the architecture is explicitly changed.
Do not bypass the intended boundaries between runtime, application orchestration, thread policy, storage, and Codex integration.

### 2. Keep the application small

Prefer simple, explicit orchestration.

Avoid:

- unnecessary abstraction layers
- speculative multi-provider support
- per-user or per-channel mode overrides in v1
- large framework-style dependency injection unless clearly justified

### 3. Keep documentation current

When code changes affect architecture, runtime flow, thread behavior, persistence assumptions, or package boundaries:

- update `ARCHITECTURE.md`
- update the relevant files under `docs/design-docs`
- update the relevant files under `docs/product-specs` when user-facing behavior or workflow expectations change
- update this file if contributor guidance changes

Document roles should remain clear:

- `AGENTS.md` should focus on implementation guidance and document navigation for contributors and coding agents
- `README.md` should focus on end-user-facing project information

### 4. Use the Makefile for basic checks

The repository includes a `Makefile` for common development checks.

- run unit tests with `make test`
- run lint checks with `make lint`

## Reference Documents

### Primary implementation reference

- `ARCHITECTURE.md`
  - The root architecture reference for the project
  - Defines system role, component boundaries, thread modes, persistence direction, and intended v1 scope

### Documentation index

- `docs/index.md`
  - Entry point for the repository documentation set

### Design notes

- `docs/design-docs/index.md`
  - Entry point for concept-level design notes
- `docs/design-docs/core-beliefs.md`
  - Project principles and architectural beliefs
- `docs/design-docs/architecture-overview.md`
  - Short onboarding summary of the system shape and request flow
- `docs/design-docs/thread-modes.md`
  - `daily` and `task` mode definitions, expected behavior, and tradeoffs
- `docs/design-docs/state-and-storage.md`
  - Local persistence requirements and state model

### Product specs

- `docs/product-specs/index.md`
  - Entry point for product-facing behavior and workflow expectations
- `docs/product-specs/daily-mode-user-flow.md`
  - Defines the intended user flow and expectations for `daily` mode
- `docs/product-specs/task-mode-user-flow.md`
  - Defines the intended user flow and expectations for `task` mode
- `docs/product-specs/discord-command-behavior.md`
  - Defines the intended Discord interaction rules and command UX expectations

### External references

- `docs/references/index.md`
  - Entry point for bundled external references
- `docs/references/codex-sdk/README.md`
  - Primary reference for Codex SDK integration, thread handling, and request flow
- `docs/references/py-pimono/README.md`
  - Useful for studying boundary-oriented architecture ideas
- `docs/references/py-pimono/ARCHITECTURE.md`
  - Useful for studying application-layer separation and orchestration structure

## Scope Note

`AGENTS.md` is intentionally short.
For thread modes, package boundaries, persistence direction, v1 scope, and non-goals, refer to `ARCHITECTURE.md`.
Use `docs/design-docs/architecture-overview.md` when a quick orientation is enough, but use `ARCHITECTURE.md` for implementation decisions.

## ExecPlans

When writing complex features or significant refactors, use an ExecPlan (as described in .agents/PLANS.md) from design to implementation.

## Maintenance Rule

If a future contributor needs to violate this guide to move the project forward, they should update the relevant architecture documents in the same change so the repository remains self-consistent.
