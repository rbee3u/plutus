MODULE := $(shell go list -m)
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
ARGS ?=

.PHONY: help deps generate style format build test lint run

help:
	@printf "%s\n" \
		"Usage: make [target] [VARIABLE=value ...]" \
		"" \
		Targets: \
		"  make help       Show available targets (default)" \
		"  make deps       Install pinned Go tools and frontend dependencies" \
		"  make generate   Generate SQL query code and Go/TypeScript API bindings" \
		"  make style      Check Go and frontend formatting without modifying files" \
		"  make format     Format Go imports, Go code, and frontend sources" \
		"  make build      Build the frontend and static CLI in dist/ (default: host platform)" \
		"  make test       Run offline Go and frontend unit tests using temporary test databases" \
		"  make lint       Run Go linters, TypeScript type checks, and ESLint" \
		"  make run        Run the CLI with go run; pass CLI arguments via ARGS" \
		"" \
		Examples: \
		"  make run ARGS=--help" \
		"  make build GOOS=linux GOARCH=arm64"

deps:
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
	go install github.com/pressly/goose/v3/cmd/goose@v3.28.0
	go install golang.org/x/tools/cmd/goimports@v0.50.0
	go install mvdan.cc/gofumpt@v0.12.0
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0
	npm --prefix web install

generate:
	oapi-codegen -config api/oapi-codegen.yaml api/openapi/openapi.yaml
	sqlc generate
	npm --prefix web run generate

style:
	@files=$$(goimports -local $(MODULE) -l .); \
	[ -n "$$files" ] || files=$$(gofumpt -l .); \
	[ -z "$$files" ] || { \
		printf "%s\n" "$$files"; \
		echo "Run 'make format' to fix style issues."; \
		exit 1; \
	}
	npm --prefix web run format:check

format:
	goimports -local $(MODULE) -l -w .
	gofumpt -l -w .
	npm --prefix web run format

build:
	npm --prefix web run build
	mkdir -p ./dist
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-trimpath -ldflags "-s -w -X main.version=$(VERSION)" \
		-o ./dist/plutus-$(GOOS)-$(GOARCH) ./cmd/plutus
	@echo "built dist/plutus-$(GOOS)-$(GOARCH) ($(VERSION))"

test:
	npm --prefix web run build
	go test -count 1 ./...
	npm --prefix web run test

lint:
	npm --prefix web run build
	golangci-lint run
	npm --prefix web run typecheck
	npm --prefix web run lint

run:
	npm --prefix web run build
	go run ./cmd/plutus $(ARGS)
