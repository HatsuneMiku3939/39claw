# Add first-stage tag-driven release automation

This ExecPlan is a living document. The sections `Progress`, `Surprises & Discoveries`, `Decision Log`, and `Outcomes & Retrospective` must be kept up to date as work proceeds.

This document must be maintained in accordance with `.agents/PLANS.md`.

## Purpose / Big Picture

After this plan, 39claw should have a repeatable first-stage release path instead of relying on ad hoc operator judgment. A maintainer should be able to validate release readiness, follow a checked-in runbook, push a tag such as `v0.1.0`, and watch GitHub Actions create a draft GitHub Release with cross-platform release archives for the production `39claw` binary.

The user-visible proof should be simple. A maintainer should be able to run local validation commands, inspect a checked-in release checklist and runbook, and then see a tag-triggered GitHub Actions release workflow build artifacts from the repository state without requiring any undocumented manual steps.

## Progress

- [x] (2026-04-05 06:56Z) Re-read issue `#49`, `.agents/PLANS.md`, the current repository CI/release state, and the `go-cli-tag-release` skill so the implementation scope can be pinned down before editing.
- [x] (2026-04-05 06:58Z) Chose the first-stage release shape: one production binary (`cmd/39claw`), draft GitHub Releases, cross-platform archives, GoReleaser config validation in existing CI, and explicit release/runbook documentation.
- [x] (2026-04-05 07:00Z) Created `docs/exec-plans/active/` and added this living plan so the work can proceed under a checked-in ExecPlan.
- [x] (2026-04-05 07:04Z) Added `.github/workflows/release.yml`, `.goreleaser.yaml`, a CI release-config validation job, and Makefile targets for local release validation.
- [x] (2026-04-05 07:04Z) Added `internal/releaseconfig` tests to pin the first-stage GoReleaser contract.
- [x] (2026-04-05 07:06Z) Updated `README.md`, `docs/index.md`, `docs/design-docs/index.md`, `docs/operations/RELEASE_RUNBOOK.md`, and a release-decision design note so the release flow is documented end to end.
- [x] (2026-04-05 07:12Z) Ran `make test`, `make lint`, `go vet ./...`, `make release-check`, `make release-snapshot`, and version checks for both the local binary and the snapshot-built Linux binary.
- [x] (2026-04-05 07:13Z) Added `dist/` to `.gitignore` because the new local snapshot workflow generates release artifacts there.
- [x] (2026-04-05 07:13Z) Confirmed the first-stage repository work is complete and ready to archive; no live `v*` tag was pushed from this implementation environment.
- [x] (2026-04-05 07:15Z) Recorded the first live tagged release as explicit follow-up work in `docs/exec-plans/tech-debt-tracker.md` and archived this ExecPlan because the remaining step is operator-owned, not unfinished repository implementation.

## Surprises & Discoveries

- Observation: The repository already has `version.Version`, so the release pipeline can reuse an existing ldflags target instead of introducing a new version contract.
  Evidence: `version/version.go`

- Observation: The main CI workflow is intentionally small and already covers `make test` plus `make lint`, so the safest way to protect release config is to merge one additional job into `.github/workflows/ci.yml` instead of creating a separate workflow.
  Evidence: `.github/workflows/ci.yml`

- Observation: The repository contains two entrypoints, but `cmd/codexplay` is explicitly described as an experimental helper while `cmd/39claw` is the product entrypoint. Releasing both now would overfit the first-stage pipeline.
  Evidence: `ARCHITECTURE.md` package responsibilities for `cmd/39claw` and `cmd/codexplay`

- Observation: `make release-snapshot` generates a local `dist/` directory with archives, extracted binaries, metadata files, and checksums, so those artifacts should be ignored by Git.
  Evidence: `dist/39claw_0.0.0-SNAPSHOT-9e80efe_Linux_x86_64.tar.gz`, `dist/39claw_linux_amd64_v1/39claw`, and `dist/39claw_0.0.0-SNAPSHOT-9e80efe_checksums.txt`

- Observation: GoReleaser's snapshot flow injects the computed snapshot version into `version.Version`, so the built Linux binary reports `0.0.0-SNAPSHOT-9e80efe` instead of the local default `dev`.
  Evidence: `go run ./cmd/39claw version` printed `dev`, while `./dist/39claw_linux_amd64_v1/39claw version` printed `0.0.0-SNAPSHOT-9e80efe`

## Decision Log

- Decision: First-stage release automation will package only the `39claw` binary from `cmd/39claw`.
  Rationale: Issue `#49` is about the product release path for 39claw itself, while `cmd/codexplay` is an experimental helper and would complicate the first release contract without user-facing value.
  Date/Author: 2026-04-05 / Codex

- Decision: The release workflow will create draft GitHub Releases instead of immediately publishing them.
  Rationale: The issue explicitly asks for a minimal, safer first stage that avoids over-automation while the project is still evolving quickly.
  Date/Author: 2026-04-05 / Codex

- Decision: The first-stage pipeline will exclude Linux packages and Homebrew publishing.
  Rationale: The issue asks for a minimal safe baseline, and there is no existing package or tap contract in the repository that would justify the extra maintenance surface.
  Date/Author: 2026-04-05 / Codex

- Decision: The implementation will stop after local validation and checked-in release automation, without pushing a live `v*` tag from the coding environment.
  Rationale: Pushing a real release tag would create an external GitHub Release event and should remain an explicit operator action performed through the documented runbook, not an implicit side effect of landing repository changes.
  Date/Author: 2026-04-05 / Codex

## Outcomes & Retrospective

39claw now has a first-stage release path with a checked-in runbook, a tag-triggered draft-release workflow, a GoReleaser config, CI validation for release config changes, local release validation commands, and tests that pin the release contract. A maintainer can now validate release readiness from the repository root, push a `v*` tag, and rely on GitHub Actions plus GoReleaser to create a draft GitHub Release for the production `39claw` binary without inventing extra steps.

Validation is complete. `make test`, `make lint`, `go vet ./...`, `make release-check`, and `make release-snapshot` all passed. The local binary still reports `dev`, and the snapshot-built Linux binary reports the injected snapshot version, proving the ldflags path works end to end.

The one intentionally omitted action is a live release tag push. That was left as an operator-owned step because it creates an external GitHub Release event. The checked-in repository state is ready for that first live run, and the exact steps now live in `docs/operations/RELEASE_RUNBOOK.md`.

No blocking implementation work remains, so this plan no longer belongs in `active/`. The remaining follow-up is tracked explicitly in `docs/exec-plans/tech-debt-tracker.md`.

## Context and Orientation

39claw is a Go-based Discord bot that routes Discord turns into Codex threads. The production runtime entrypoint is `cmd/39claw`, while `cmd/codexplay` is an experimental helper CLI used to exercise the Codex integration manually. The repository already has a version surface at `version/version.go`, where the default build version is `dev`.

Today, the repository has a normal CI workflow in `.github/workflows/ci.yml` that runs `make test` and `make lint` on pushes to `master` and on pull requests. There is no `.goreleaser.yaml`, no tag-triggered release workflow, no release-specific Makefile targets, no checked-in release runbook, and no dedicated release decision note. There are also no prior Git tags or published GitHub Releases.

Terms used in this plan:

- release candidate gate: the explicit set of checks and manual review steps that decide whether the current `master` commit is ready to be tagged
- runbook: the checked-in operator document that explains exactly how to prepare, tag, monitor, and verify a release
- GoReleaser: the tool that builds platform-specific binaries, archives them, and creates the GitHub Release entry from a checked-in YAML config
- draft release: a GitHub Release that is created but not published to end users until a maintainer reviews it

The files most relevant to this plan are:

- `.github/workflows/ci.yml`
  - existing CI workflow where release-config validation should be added
- `Makefile`
  - local developer entrypoint for validation commands
- `cmd/39claw/main.go`
  - production binary entrypoint
- `version/version.go`
  - release-version symbol that GoReleaser should populate through ldflags
- `README.md`
  - top-level operator and user documentation that must include the release flow
- `ARCHITECTURE.md`
  - authoritative architecture guide that should stay consistent with the release scope
- `docs/design-docs/index.md`
  - index for supporting design notes where the release-automation decision note should be linked
- `docs/exec-plans/index.md`
  - index for living execution plans that must list this active plan while implementation is in progress

## Plan of Work

Start by adding the release automation skeleton. Create a `.goreleaser.yaml` configured for the `39claw` binary only, reuse `github.com/HatsuneMiku3939/39claw/version.Version` for ldflags version injection, keep `CGO_ENABLED=0`, build `linux`, `darwin`, and `windows` targets for `amd64` and `arm64`, and package `README*` plus `LICENSE*` into archives. Keep the GitHub release in draft mode, and do not add Linux package or Homebrew sections.

Next, add the GitHub Actions workflow layer. Introduce a new `.github/workflows/release.yml` triggered by pushed tags that match `v*`. Match the repository's current action-version style, grant `contents: write`, fetch full history for tags, and run GoReleaser with `release --clean`. Extend `.github/workflows/ci.yml` with a second job that validates the checked-in GoReleaser config using `goreleaser check` so bad release config cannot merge silently.

Then add local validation affordances and tests. Extend `Makefile` with `release-check` and `release-snapshot` targets that use `go run github.com/goreleaser/goreleaser/v2@latest ...`, and add a small Go test package such as `internal/releaseconfig` that parses `.goreleaser.yaml` and confirms the build ID, ldflags target, and archive naming contract. Add any required YAML dependency to `go.mod` in the smallest scope needed for those tests.

After the automation exists, update the documentation set. `README.md` must gain a release section that explains local validation, tag creation, and what happens when a `v*` tag is pushed. Add a checked-in `docs/operations/RELEASE_RUNBOOK.md` describing the release candidate gate, the exact tagging workflow, post-release verification, and what remains intentionally manual in this first stage. Add a design note under `docs/design-docs/` describing how the reusable `go-cli-tag-release` skill was adapted for 39claw, then link that note from `docs/design-docs/index.md` and from broader docs indexes when appropriate.

Finally, validate the result end to end. Run the repository checks (`make test`, `make lint`, and `go vet ./...`), run the new release config tests, run `make release-check`, and run `make release-snapshot`. Capture the successful commands and any notable output in this plan. If any scope is intentionally deferred, record it either in this plan's retrospective or in `docs/exec-plans/tech-debt-tracker.md` before moving the plan out of `active/`.

## Concrete Steps

Run all commands from `/home/filepang/playground/39claw`.

1. Create the release automation files and documentation described in this plan.
2. Run:

       make test
       make lint
       go vet ./...
       make release-check
       make release-snapshot

3. Confirm the snapshot build creates release artifacts without requiring a Git tag.
4. Confirm no placeholder tokens such as `__MODULE_PATH__` remain in tracked files.

This section must be updated with concrete evidence once the commands have been run.

Implemented validation evidence:

    $ make test
    ok   github.com/HatsuneMiku3939/39claw/internal/releaseconfig  (cached)
    ... other packages omitted ...

    $ make lint
    0 issues.
    Linting passed

    $ go vet ./...
    [no output]

    $ make release-check
      • checking                                  path=.goreleaser.yaml
      • 1 configuration file(s) validated

    $ make release-snapshot
      • snapshotting                              version=0.0.0-SNAPSHOT-9e80efe
      • release succeeded after 52s

    $ go run ./cmd/39claw version
    dev

    $ ./dist/39claw_linux_amd64_v1/39claw version
    0.0.0-SNAPSHOT-9e80efe

## Validation and Acceptance

The implementation is acceptable when all of the following are true:

- `README.md` and `docs/operations/RELEASE_RUNBOOK.md` describe the same release candidate gate and tag flow.
- `.github/workflows/release.yml` triggers on `v*` tags and runs GoReleaser with GitHub release permissions.
- `.github/workflows/ci.yml` validates the GoReleaser config on normal CI runs.
- `.goreleaser.yaml` builds exactly the production `39claw` binary, injects `version.Version`, and archives `README*` and `LICENSE*`.
- Running `go run ./cmd/39claw version` still works locally and can be overridden by GoReleaser ldflags in the config.
- `make test`, `make lint`, `go vet ./...`, `make release-check`, and `make release-snapshot` all succeed.
- A maintainer reading only the checked-in docs can determine whether `master` is releasable and can push a release tag without inventing extra steps.

## Idempotence and Recovery

The file-creation steps in this plan are additive and safe to rerun. `make release-check` and `make release-snapshot` are intended to be repeatable local validation commands. If `make release-snapshot` leaves a `dist/` directory behind, rerunning the same command is safe because GoReleaser will clean the output before rebuilding.

If a validation command fails midway, fix the underlying config or docs mismatch and rerun only the failed command plus any broader command it feeds into. Do not create or push a Git tag until the local checks and CI release-config validation both pass.

## Artifacts and Notes

This section will be updated with the exact validation transcripts and any notable release-config excerpts after implementation lands.

Key artifacts produced during validation:

    dist/39claw_0.0.0-SNAPSHOT-9e80efe_Linux_x86_64.tar.gz
    dist/39claw_0.0.0-SNAPSHOT-9e80efe_Darwin_x86_64.zip
    dist/39claw_0.0.0-SNAPSHOT-9e80efe_Windows_x86_64.zip
    dist/39claw_0.0.0-SNAPSHOT-9e80efe_checksums.txt

## Interfaces and Dependencies

Add these concrete repository interfaces and dependencies by the end of the plan:

- `.github/workflows/release.yml`
  - a tag-triggered GitHub Actions workflow for GoReleaser-based draft releases
- `.goreleaser.yaml`
  - a GoReleaser v2 config whose build `id` and `binary` are both `39claw`
- `Makefile`
  - `release-check` and `release-snapshot` targets that run GoReleaser through `go run`
- `internal/releaseconfig/release_config_test.go`
  - Go tests that parse `.goreleaser.yaml` with `gopkg.in/yaml.v3`
- `docs/operations/RELEASE_RUNBOOK.md`
  - the operator runbook for release readiness, tagging, and post-release verification
- `docs/design-docs/first-stage-release-automation.md`
  - a design note that explains what was reused from `go-cli-tag-release` and what remains 39claw-specific

Revision note: 2026-04-05 / Codex. Created the initial ExecPlan for issue `#49` after scoping the work against the existing repository and the reusable `go-cli-tag-release` skill.

Revision note: 2026-04-05 / Codex. Updated the plan with final validation evidence, recorded the deferred first live tag push in `docs/exec-plans/tech-debt-tracker.md`, and marked the plan ready for archival because repository implementation is complete.
