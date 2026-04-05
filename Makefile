all: help

.PHONY: help
help: Makefile
	@sed -n 's/^##//p' $< | awk 'BEGIN {FS = ":"}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

## test: Run unittest
.PHONY: test
test:
	@go test $(TESTARGS) ./...

## lint: Run lint
.PHONY: lint
lint:
	@./scripts/lint -c .golangci.yml

## release-check: Validate the GoReleaser configuration
.PHONY: release-check
release-check:
	@go run github.com/goreleaser/goreleaser/v2@latest check

## release-snapshot: Build snapshot release artifacts locally
.PHONY: release-snapshot
release-snapshot:
	@go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean
