## Release

Pushing a tag that starts with `v` to the remote repository triggers the GitHub Actions release workflow.

```bash
git tag v0.1.0
git push origin v0.1.0
```

Use `goreleaser check` before tagging, and run a snapshot release locally when practical.

If Homebrew publishing is enabled, set `HOMEBREW_TAP_GITHUB_TOKEN` in GitHub Actions secrets before pushing the tag.
