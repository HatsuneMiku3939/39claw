.PHONY: release-check release-snapshot

## release-check: Validate the GoReleaser configuration
release-check:
	go run github.com/goreleaser/goreleaser/v2@latest check

## release-snapshot: Build snapshot release artifacts locally
release-snapshot:
	go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
