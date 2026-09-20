.DEFAULT_GOAL := all

MODULE := $(shell go list -m)
ARGS ?=

.PHONY: help all deps style format build test lint run

help: ## Show available targets in Makefile order.
	@awk 'BEGIN { FS = ":.*## "; printf "Usage: make <target>\n\nTargets:\n" } /^[a-zA-Z_-]+:.*## / { printf "  %-8s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

all: format build test lint ## Format, build, test, and lint.

deps: ## Install pinned formatting and lint tools.
	go install golang.org/x/tools/cmd/goimports@v0.50.0
	go install mvdan.cc/gofumpt@v0.12.0
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2

style: ## Check formatting without rewriting files for CI.
	@files="$$(goimports -local $(MODULE) -l .)"; \
	[ -n "$$files" ] || files="$$(gofumpt -l .)"; \
	[ -z "$$files" ] || { \
		printf "%s\n" "$$files"; \
		echo "Run 'make format' to fix style issues."; \
		exit 1; \
	}

format: ## Format Go code and imports for local development.
	goimports -local $(MODULE) -l -w . && gofumpt -l -w .

build: ## Build the CLI as bin/plutus.
	mkdir -p ./bin && go build -o ./bin/plutus $(MODULE)

test: ## Run unit tests without cached results.
	go test -count 1 ./...

lint: ## Run the configured Go linters.
	golangci-lint run

run: ## Run the CLI locally with go run; pass arguments with ARGS="...".
	go run . $(ARGS)
