# First-Stage Release Automation

Status: Active

## Purpose

This document explains how 39claw reuses the reusable `go-cli-tag-release` skill while adapting it to the project's current maturity level.

The repository now has a first-stage release path that is intentionally smaller than a mature CLI distribution pipeline.
The goal is repeatability and safety, not maximum automation.

## What Was Reused from `go-cli-tag-release`

39claw directly reused the following ideas from the skill:

- a `v*` tag-triggered GitHub Actions release workflow
- a GoReleaser-based release config
- ldflags-based version injection through `version.Version`
- CI validation for `.goreleaser.yaml`
- local `release-check` and `release-snapshot` commands
- README-level operator documentation for the release flow

These parts already matched the repository well because 39claw is a Go repository with a single primary production binary and a standard GitHub Actions CI workflow.

## 39claw-Specific Adaptations

39claw does not use the skill as a rigid template.
The repository makes several deliberate first-stage adaptations:

### One production binary

The release config builds only `cmd/39claw`.

The repository also contains `cmd/codexplay`, but that binary is an experimental helper for manually exercising the Codex integration layer.
It is not part of the user-facing product contract, so shipping it in the first release pipeline would widen the release surface without clear value.

### Draft releases only

The GitHub Actions release workflow creates a draft GitHub Release instead of publishing immediately.

This matches the current project stage:

- architecture is still evolving
- the repository has not established a release rhythm yet
- maintainers still need an explicit review checkpoint before publishing

### No package-manager publishing

The first-stage config now includes two operator-friendly distribution surfaces beyond raw archives:

- Homebrew cask updates for macOS
- Linux `.deb` and `.rpm` packages

These are still bounded additions because they are generated directly from the same GoReleaser build inputs and stay tied to the single `39claw` production binary.

### Explicit runbook and release gate

The skill provides technical release scaffolding, but 39claw also needs a documented release candidate gate because the project is not only shipping a binary.
It is also shipping a behavior contract defined across:

- `README.md`
- `ARCHITECTURE.md`
- `docs/design-docs`
- `docs/product-specs`

For that reason, the repository adds `docs/operations/RELEASE_RUNBOOK.md` and documents manual smoke expectations alongside automated checks.

## Release Scope

The current first-stage pipeline guarantees these behaviors:

- pushing a tag like `v0.1.0` triggers the release workflow
- GitHub Actions creates a draft release
- GoReleaser builds cross-platform archives for the `39claw` binary
- GoReleaser builds Linux `.deb` and `.rpm` packages for `amd64` and `arm64`
- GoReleaser updates the Homebrew cask in `HatsuneMiku3939/homebrew-tap`
- release archives include `README*` and `LICENSE*`
- CI validates `.goreleaser.yaml` before release changes merge
- maintainers can validate the release flow locally before pushing a tag

The current pipeline does not guarantee:

- automatic version selection
- automatic publication after build success
- helper-binary distribution

## Why This Shape Fits 39claw

39claw is a Codex-native Discord bot, not a long-stable general-purpose CLI with a broad packaging contract.
The project benefits from a release process that is:

- explicit
- easy to review
- easy to operate from source control state
- easy to harden later without undoing early decisions

That is why the repository reuses the skill's proven release automation core while intentionally deferring richer release-distribution features until a later phase.
