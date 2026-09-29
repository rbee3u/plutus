.DEFAULT_GOAL := all

# Keep the format, build, test, and lint gates in order, even with make -j.
.NOTPARALLEL:

MODULE := $(shell go list -m)
ARGS ?=

# Version string baked into the binary (`plutus --version`). Falls back to "dev" outside git.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

# Cross-compilation target for `make dist` (the remote test and prod servers).
DIST_GOOS ?= linux
DIST_GOARCH ?= amd64
DIST_BIN := dist/plutus-$(DIST_GOOS)-$(DIST_GOARCH)

.PHONY: help all deps style format generate web-build build test test-live lint dist run dev

help: ## Show available targets in Makefile order.
	@awk 'BEGIN { FS = ":.*## "; printf "Usage: make <target>\n\nTargets:\n" } /^[a-zA-Z_-]+:.*## / { printf "  %-10s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

all: format build test lint ## Format, build, test, and lint.

GO_PACKAGES = $(shell go list ./... | grep -v /node_modules/)

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

build: web-build ## Build the SPA and CLI as bin/plutus with the git version baked in.
	mkdir -p ./bin && go build -ldflags "$(LDFLAGS)" -o ./bin/plutus ./cmd/plutus

test: web-build ## Run Go and frontend unit tests offline (throwaway plutus_gotest_* databases; no network, no live keys).
	go test -count 1 $(GO_PACKAGES)
	cd web && npm run test

test-live: ## Run live smoke tests; set PLATFORM=binance|okx|bybit|kraken for one Platform.
	@case "$$PLATFORM" in \
		"") packages='./internal/adapters/...'; require_keys=0 ;; \
		binance|okx|bybit|kraken) packages="./internal/adapters/$$PLATFORM"; require_keys=1 ;; \
		*) echo 'PLATFORM must be binance, okx, bybit, or kraken' >&2; exit 2 ;; \
	esac; \
	PLUTUS_GOTEST_REQUIRE_LIVE_KEYS=$$require_keys go test -count 1 -tags live -run 'Live' $$packages

lint: web-build ## Run the configured Go linters and frontend checks.
	golangci-lint run
	golangci-lint run --build-tags live ./internal/adapters/...
	cd web && npm run typecheck && npm run lint

dist: web-build ## Cross-compile a static binary for the remote test and prod servers into dist/.
	mkdir -p ./dist && CGO_ENABLED=0 GOOS=$(DIST_GOOS) GOARCH=$(DIST_GOARCH) go build -trimpath -ldflags "-s -w $(LDFLAGS)" -o $(DIST_BIN) ./cmd/plutus
	@echo "built $(DIST_BIN) ($(VERSION))"

run: web-build ## Run the CLI locally with go run; pass arguments with ARGS="...".
	go run ./cmd/plutus $(ARGS)

dev: ## Build and serve the local development environment on :8080 without resetting its data.
	./scripts/local-serve.sh
