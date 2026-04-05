---
name: go-cli-tag-release
description: Bootstrap or upgrade GitHub Actions and GoReleaser automation for Go CLI repositories that should publish releases when tags matching `v*` are pushed. Use when creating a new Go CLI repository or retrofitting an existing one to build cross-platform binaries, attach release archives, validate release config in CI, embed the release version into the binary, optionally publish Linux packages, and optionally update a Homebrew tap.
---

# Go Cli Tag Release

## Overview

Build a tag-driven release pipeline for a Go CLI repository. Reuse the templates in `assets/templates/`, adapt the placeholders from [references/customization-guide.md](references/customization-guide.md), and preserve the target repository's existing conventions when they conflict with the sample layout.

## Workflow

1. Inspect the target repository before editing anything.
2. Add or align version injection support.
3. Add the tag-triggered release workflow.
4. Add or update `.goreleaser.yaml`.
5. Add CI validation for GoReleaser config.
6. Add release config tests and local validation commands.
7. Update release documentation.
8. Run verification commands.

## Inspect The Repository

Collect these inputs first:

- Module path from `go.mod`
- Binary name and `cmd/...` entrypoint path
- Whether the repo already exposes a `version` command or version package
- Default branch name used by CI
- Existing GitHub Actions workflows and action pinning policy
- Whether Linux packages are desired
- Whether a Homebrew tap should be updated
- Whether `README.md` and `LICENSE` should be included in release archives

Use [references/source-map.md](references/source-map.md) to understand which files from the sample repository informed the workflow.

## Add Version Injection

Ensure the binary can expose a release version that GoReleaser injects with `ldflags`.

- If the repository already has a version package or variable, reuse it.
- Otherwise, start from `assets/templates/version/version.go`.
- Point the GoReleaser `ldflags` entry at `__MODULE_PATH__/version.Version` or the equivalent symbol already used by the repository.
- Avoid changing the public CLI contract unless the repository already has a `version` command or the user asks for one.

## Add Release Workflow

Use `assets/templates/.github/workflows/release.yml` as the base workflow.

- Trigger on `push.tags: ["v*"]`.
- Keep `fetch-depth: 0` so GoReleaser can access tags and history.
- Grant `contents: write` so GitHub Releases can be created.
- Pass `GITHUB_TOKEN` to GoReleaser.
- Keep `HOMEBREW_TAP_GITHUB_TOKEN` only if the repository will update a Homebrew tap.
- Match the repository's existing action version pinning policy. The sample repository mixes pinned SHAs and major tags; do not "normalize" that without a reason.

## Add GoReleaser Config

Use `assets/templates/.goreleaser.yaml` as the starting point and replace every placeholder described in [references/customization-guide.md](references/customization-guide.md).

- Keep `before.hooks: [go mod tidy]` unless the repository has a stronger pre-release hook policy.
- Keep `CGO_ENABLED=0` unless the binary requires CGO.
- Keep the stable archive naming template unless the repository already relies on another format.
- Include `README*` and `LICENSE*` in archives by default.
- Remove the `nfpms` section if Linux packages are not needed.
- Remove the `homebrew_casks` section and token env var if Homebrew publishing is not needed.
- Keep `release.draft: true` if the user wants a safer review step before publishing; otherwise explain the tradeoff and switch to published releases.

## Add CI Validation

Protect the release config in CI.

- If the repository already has a main CI workflow, merge in the job from `assets/templates/release-config-job.yaml`.
- If it has no suitable workflow, add `assets/templates/.github/workflows/release-check.yml`.
- Also add local commands from `assets/templates/Makefile.release.mk` if the repository uses `Makefile`.

## Add Tests

Use `assets/templates/internal/releaseconfig/release_config_test.go` as the baseline test file.

- Replace placeholders with the real module path and build ID.
- Keep the test table/style consistent with the repository's Go test conventions.
- If `nfpms` is enabled, add assertions for package formats and file name templates.
- If `homebrew_casks` is enabled, add assertions that the cask references known archive IDs.

## Update Documentation

Document the release flow in `README.md`.

- Mention that pushing a tag like `v0.1.0` triggers the release workflow.
- Document optional secrets such as `HOMEBREW_TAP_GITHUB_TOKEN`.
- Document local validation commands like `go test ./...`, `go vet ./...`, `goreleaser check`, and optional snapshot builds.
- Reuse `assets/templates/README-release-section.md` when the repository has no release section yet.

## Verify

Run the commands that fit the target repository:

```bash
go test ./...
go vet ./...
go run github.com/goreleaser/goreleaser/v2@latest check
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
```

If the repository has wrapper targets like `make test`, `make lint`, `make release-check`, or `make release-snapshot`, prefer those.

## Resources

- `references/customization-guide.md`: Placeholder definitions, decisions, and adaptation rules
- `references/source-map.md`: Source files and extracted patterns from the sample repository
- `assets/templates/.github/workflows/release.yml`: Tag-triggered release workflow template
- `assets/templates/.github/workflows/release-check.yml`: Standalone validation workflow template
- `assets/templates/release-config-job.yaml`: CI job snippet for `goreleaser check`
- `assets/templates/.goreleaser.yaml`: Base GoReleaser config with optional package publishing blocks
- `assets/templates/internal/releaseconfig/release_config_test.go`: Baseline Go release config tests
- `assets/templates/version/version.go`: Minimal version variable template
- `assets/templates/Makefile.release.mk`: Makefile targets for release validation
- `assets/templates/README-release-section.md`: Release documentation snippet

## Guardrails

- Do not overwrite existing CI or release files blindly; merge with the repository's current conventions.
- Do not assume `master`; detect the real default branch.
- Do not leave placeholder tokens behind in committed files.
- Do not keep Homebrew or Linux package blocks if the repository will not publish them.
- Do not skip README updates when commands, secrets, or release behavior change.
