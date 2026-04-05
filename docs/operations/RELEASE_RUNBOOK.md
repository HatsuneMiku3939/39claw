# Release Runbook

This document defines the first-stage release flow for 39claw.

The goal of this runbook is to make releases repeatable and reviewable while the product is still evolving quickly.
This is intentionally a conservative first stage:

- releases are created from explicit Git tags
- GitHub Actions creates a draft release instead of auto-publishing it
- the release candidate gate mixes automated checks with explicit maintainer judgment
- automatic version calculation is intentionally out of scope

## Release Candidate Gate

Do not create or push a release tag until all of the following are true:

1. The target commit is on `master` and has already passed normal CI.
2. Local validation succeeds:
   - `make test`
   - `make lint`
   - `go vet ./...`
   - `make release-check`
   - `make release-snapshot`
3. The checked-in documentation matches the current behavior:
   - `README.md`
   - `ARCHITECTURE.md`
   - relevant files under `docs/design-docs`
   - relevant files under `docs/product-specs`
4. Core runtime smoke paths have been exercised manually against a real or test Discord bot deployment:
   - mention-driven reply flow works
   - slash command help still works
   - `task` mode can create and switch tasks when enabled
   - image attachment handling still works when applicable
5. `go run ./cmd/39claw version` returns a sensible value locally and the release config still injects `version.Version`.
6. The `HOMEBREW_TAP_GITHUB_TOKEN` GitHub Actions secret is configured with write access to `HatsuneMiku3939/homebrew-tap`.
7. No ad hoc release-only edits are waiting outside version control.

If any gate fails, fix the repository state first and rerun the failed checks before tagging.

## Local Validation Workflow

Run all commands from the repository root:

```bash
make test
make lint
go vet ./...
make release-check
make release-snapshot
```

What these commands prove:

- `make test` verifies the Go test suite
- `make lint` verifies lint compliance
- `go vet ./...` catches common Go correctness mistakes
- `make release-check` validates `.goreleaser.yaml`
- `make release-snapshot` verifies that GoReleaser can build release artifacts locally without a real tag

`make release-snapshot` leaves local artifacts under `dist/`. That is expected for validation.

## Tagging a Release

After the release candidate gate passes:

1. Make sure your local `master` matches `origin/master`.
2. Choose the next semantic version tag, for example `v0.1.0`.
3. Create the annotated tag locally.
4. Push the tag to `origin`.

Example:

```bash
git checkout master
git pull --ff-only origin master
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

Pushing a tag that starts with `v` triggers `.github/workflows/release.yml`.

## What GitHub Actions Does

The release workflow:

1. checks out the repository with full history
2. sets up Go from `go.mod`
3. runs GoReleaser with `release --clean`
4. creates or replaces a draft GitHub Release for the pushed tag
5. uploads release archives for the supported platforms
6. uploads Linux `.deb` and `.rpm` package artifacts
7. updates the Homebrew cask in `HatsuneMiku3939/homebrew-tap`

The first-stage release automation currently builds only the production `39claw` binary from `cmd/39claw`.
It does not publish any secondary helper binary.

## Post-Tag Verification

After pushing the tag:

1. Open the GitHub Actions run triggered by the tag and confirm it passed.
2. Open the generated draft GitHub Release.
3. Verify the release title and tag.
4. Verify the attached archives exist for the supported platform matrix.
5. Verify Linux `.deb` and `.rpm` packages exist for the supported Linux architectures.
6. Verify the archive naming matches the documented GoReleaser template.
7. Confirm the Homebrew cask update landed in `HatsuneMiku3939/homebrew-tap`.
8. Download one archive or package if needed and confirm it contains:
   - the `39claw` binary for that platform
   - `README.md`
   - `LICENSE`
9. Run the binary with `version` and confirm it reports the release version instead of `dev`.

If anything looks wrong, do not publish the draft release.
Fix the repository state, delete the bad tag and draft release if needed, and rerun the process with a corrected tag.

## Publishing the Draft

This first-stage pipeline stops at a draft release on purpose.

When the draft contents are correct:

1. review the generated notes and artifact list
2. add any maintainer-written notes you want end users to see
3. publish the draft manually in GitHub

Keeping the final publish step manual is part of the current safety model.

## Explicit Non-Goals for This Stage

The current release path intentionally does not do the following:

- automatic semantic version calculation
- automatic release publication without review
- multi-environment deployment orchestration
- changelog policy enforcement beyond what GitHub and GoReleaser provide by default

These may be added in a later release-hardening pass after the repository establishes a real release rhythm.
