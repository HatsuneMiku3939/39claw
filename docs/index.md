# 39bot Docs

This directory is the documentation home for 39bot.

It is organized into a small set of document layers so contributors can quickly find the right level of detail.

## Document Map

- [Product Specs](./product-specs/index.md)
  - Product-facing user journeys, interaction rules, and expected behavior
- [Design Docs](./design-docs/index.md)
  - Architecture direction, internal design boundaries, and system concepts
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
- Keep implementation design in `docs/design-docs`.
- Keep external source material in `docs/references`.
- Update this index when a new top-level documentation layer is introduced.
