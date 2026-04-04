# 39bot Product Specs

This directory captures product-facing behavior and user experience expectations for 39bot.

These documents should answer questions such as:

- what user journey the bot should support
- what behavior should feel intuitive or low-friction
- what product rules should remain stable even if implementation details change

## Relationship to Other Documents

- `docs/product-specs`
  - user-facing behavior, flows, expectations, and product rules
- `docs/design-docs`
  - architectural direction, component responsibilities, and internal design boundaries
- `docs/references`
  - external references and source material

Product specs should stay concrete and scenario-oriented.
If a document starts describing package boundaries or infrastructure internals, that content likely belongs in `docs/design-docs` instead.

## Current Documents

- [Daily Mode User Flow](./daily-mode-user-flow.md)
  - Defines the expected user experience for conversation flow in `daily` mode
- [Task Mode User Flow](./task-mode-user-flow.md)
  - Defines the expected user experience for explicit task-oriented flow in `task` mode
- [Discord Command Behavior](./discord-command-behavior.md)
  - Defines the intended Discord interaction rules, command behavior, and response expectations

## Planned Documents

The following specs are expected to be useful as the product surface expands:

- `error-handling-and-recovery.md`
- `response-formatting-guidelines.md`

## Status Notes

Unless otherwise stated, product specs in this directory should be treated as draft working documents for v1.
They describe intended behavior and should be updated as product decisions become more concrete.
