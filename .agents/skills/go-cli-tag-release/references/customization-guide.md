# Customization Guide

## Placeholders

Replace these placeholders in the templates before finishing the task:

- `__MODULE_PATH__`: Go module path from `go.mod`
- `__BINARY_NAME__`: CLI binary name
- `__CMD_PATH__`: Entrypoint directory under `cmd/`
- `__DEFAULT_BRANCH__`: Repository default branch
- `__REPO_OWNER__`: GitHub owner or organization
- `__REPO_NAME__`: GitHub repository name
- `__PROJECT_DESCRIPTION__`: Short release description
- `__HOMEBREW_TAP_OWNER__`: Homebrew tap owner
- `__HOMEBREW_TAP_REPO__`: Homebrew tap repository
- `__LINUX_PACKAGE_NAME__`: Package name for `deb` and `rpm`

## Recommended Decisions

### Always keep

- `push.tags: ["v*"]` in the release workflow
- `fetch-depth: 0` in checkout
- `contents: write` for the release job
- `go mod tidy` in `before.hooks`
- Cross-platform `goos` targets for `linux`, `windows`, and `darwin`
- `goarch` targets for `amd64` and `arm64`
- `README*` and `LICENSE*` in archives
- `release-archives` as the primary archive ID

### Keep by default, but confirm

- `CGO_ENABLED=0`
- `release.draft: true`
- `release --clean`
- `release --snapshot --clean` for local verification

### Remove when not needed

- `nfpms`
- `homebrew_casks`
- `HOMEBREW_TAP_GITHUB_TOKEN`
- Release-config tests that assert optional sections you deleted

## Integration Rules

### CI workflow

- If a CI workflow already runs tests and lint, add a release-config validation job there.
- If no CI workflow exists, add a standalone `release-check.yml`.
- Match the repository's action version policy instead of introducing a new policy silently.

### Version injection

- Reuse the existing version symbol if one already exists.
- If the repo has no version package, add a minimal `version/version.go` with a default like `"dev"`.
- Keep comments in English when adding new Go files.

### Documentation

- Add a `Release` section to `README.md` when missing.
- Document exact commands for tagging and pushing:

```bash
git tag v0.1.0
git push origin v0.1.0
```

- Mention optional secrets only when the corresponding publishing feature is enabled.

## Validation Checklist

Before finishing, confirm all of the following:

- `go test ./...` passes
- `go vet ./...` passes
- `goreleaser check` passes
- Snapshot release build succeeds when practical
- No placeholder tokens remain in the edited repository
- README release instructions match the actual workflow
