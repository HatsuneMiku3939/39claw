# Source Map

This skill was derived from the release automation pattern implemented in:

- `.github/workflows/release.yml`: Tag-triggered GitHub Actions release job using GoReleaser
- `.goreleaser.yaml`: Cross-platform build, archive naming, Linux package publishing, Homebrew cask publishing, and release draft settings
- `.github/workflows/ci.yml`: CI validation job using `goreleaser check`
- `internal/releaseconfig/release_config_test.go`: Tests that lock down release config expectations
- `Makefile`: Local validation targets for `release-check` and `release-snapshot`
- `README.md`: User-facing documentation for tagging and release behavior
- `version/version.go`: Version injection target for GoReleaser `ldflags`

## Extracted Pattern

1. Trigger releases from pushed tags that start with `v`.
2. Build release artifacts with GoReleaser from a full Git checkout.
3. Inject the release version into the binary at build time.
4. Produce stable archive names for downstream automation.
5. Validate the GoReleaser config in CI before tags are pushed.
6. Expose local developer commands for release validation.
7. Document the release trigger and optional secrets in `README.md`.

## Optional Features Present In The Sample

- Linux `.deb` and `.rpm` packages through `nfpms`
- Homebrew cask updates through `homebrew_casks`
- Draft GitHub Releases instead of immediate publish

Carry these forward only when the target repository actually needs them.
