.DEFAULT_GOAL := all

MODULE := $(shell go list -m)
ARGS ?=

.PHONY: help all deps style format generate web-build build test lint run

help: ## Show available targets in Makefile order.
	@awk 'BEGIN { FS = ":.*## "; printf "Usage: make <target>\n\nTargets:\n" } /^[a-zA-Z_-]+:.*## / { printf "  %-8s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

all: format build test lint ## Format, build, test, and lint.

GO_PACKAGES := $(shell go list ./... | grep -v /node_modules/)

deps: ## Install pinned formatting, lint, sqlc, Goose, and frontend tools.
	go install golang.org/x/tools/cmd/goimports@v0.50.0
	go install mvdan.cc/gofumpt@v0.12.0
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2
	go install github.com/pressly/goose/v3/cmd/goose@v3.28.0
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
	cd web && npm install

style: ## Check formatting without rewriting files for CI.
	@files="$$(goimports -local $(MODULE) -l .)"; \
	[ -n "$$files" ] || files="$$(gofumpt -l .)"; \
	[ -z "$$files" ] || { \
		printf "%s\n" "$$files"; \
		echo "Run 'make format' to fix style issues."; \
		exit 1; \
	}
	cd web && npm run format:check

format: ## Format Go code and imports for local development.
	goimports -local $(MODULE) -l -w . && gofumpt -l -w .
	cd web && npm run format

generate: ## Generate sqlc and OpenAPI artifacts.
	sqlc generate
	oapi-codegen -config api/oapi-codegen.yaml api/openapi/openapi.yaml
	cd web && npm run generate

web-build: ## Build the production SPA into web/dist for go:embed.
	cd web && npm run build

build: ## Build the CLI as bin/plutus.
	mkdir -p ./bin && go build -o ./bin/plutus ./cmd/plutus

test: ## Run unit tests without cached results.
	go test -count 1 $(GO_PACKAGES)

lint: ## Run the configured Go linters and frontend checks.
	golangci-lint run
	cd web && npm run typecheck && npm run lint

run: ## Run the CLI locally with go run; pass arguments with ARGS="...".
	go run ./cmd/plutus $(ARGS)
