MODULE := $(shell go list -m)
ARGS ?=

# Version string baked into the binary (`plutus --version`). Falls back to "dev" outside git.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X main.version=$(VERSION)

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

.PHONY: help deps style format generate web-build build test lint run

help:
	@printf "%s\n" \
		"Usage: make [target] [VARIABLE=value ...]" \
		"" \
		Targets: \
		"  make help       Show available targets (default)" \
		"  make deps       Install pinned Go tools and frontend dependencies" \
		"  make style      Check Go and frontend formatting without modifying files" \
		"  make format     Format Go imports, Go code, and frontend sources" \
		"  make generate   Generate SQL query code and Go/TypeScript API bindings" \
		"  make web-build  Build embedded frontend assets in web/dist" \
		"  make build      Build the frontend and static CLI in dist/ (default: host platform)" \
		"  make test       Run offline Go and frontend unit tests using temporary test databases" \
		"  make lint       Run Go linters, TypeScript type checks, and ESLint" \
		"  make run        Run the CLI with go run; pass CLI arguments via ARGS" \
		"" \
		Examples: \
		"  make run ARGS=--help" \
		"  make build GOOS=linux GOARCH=arm64"

GO_PACKAGES = $(shell go list ./... | grep -v /node_modules/)

deps:
	go install golang.org/x/tools/cmd/goimports@v0.50.0
	go install mvdan.cc/gofumpt@v0.12.0
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2
	go install github.com/pressly/goose/v3/cmd/goose@v3.28.0
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1
	cd web && npm install

style:
	@files=$$(goimports -local $(MODULE) -l .); \
	[ -n "$$files" ] || files=$$(gofumpt -l .); \
	[ -z "$$files" ] || { \
		printf "%s\n" "$$files"; \
		echo "Run 'make format' to fix style issues."; \
		exit 1; \
	}
	cd web && npm run format:check

format:
	goimports -local $(MODULE) -l -w . && gofumpt -l -w .
	cd web && npm run format

generate:
	sqlc generate
	oapi-codegen -config api/oapi-codegen.yaml api/openapi/openapi.yaml
	cd web && npm run generate

web-build:
	cd web && npm run build

build: web-build
	mkdir -p ./dist && CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags "-s -w $(LDFLAGS)" -o ./dist/plutus-$(GOOS)-$(GOARCH) ./cmd/plutus
	@echo "built dist/plutus-$(GOOS)-$(GOARCH) ($(VERSION))"

test: web-build
	go test -count 1 $(GO_PACKAGES)
	cd web && npm run test

lint: web-build
	golangci-lint run
	cd web && npm run typecheck && npm run lint

run: web-build
	go run ./cmd/plutus $(ARGS)
