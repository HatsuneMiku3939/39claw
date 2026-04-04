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
