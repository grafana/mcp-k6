# Make targets equivalent to justfile recipes.

SHELL := /bin/bash

TODAY := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo local)
VERSION ?= dev
XK6_MCP_VERSION ?= v0.0.3
XK6_VERSION ?= v1.2.6

LDFLAGS := -s -w -X github.com/grafana/mcp-k6/internal/buildinfo.Version=$(VERSION) \
	-X github.com/grafana/mcp-k6/internal/buildinfo.Commit=$(COMMIT) \
	-X github.com/grafana/mcp-k6/internal/buildinfo.Date=$(TODAY)

CMD_PACKAGES := $(shell go list ./cmd/...)

.PHONY: run install install-only build build-only release prepare clean docs types ensure-embed help list test test-unit tests test-all test-e2e test-e2e-setup

run: prepare ## Run the mcp-k6 server
	@go run ./cmd/mcp-k6

install: prepare install-only ## Install the mcp-k6 server (VERSION=dev)

install-only: ## Install the mcp-k6 server without preparing assets first (VERSION=dev)
	@go install -ldflags "$(LDFLAGS)" ./cmd/mcp-k6

build: prepare build-only ## Build the mcp-k6 server (VERSION=dev)

build-only: ## Build the mcp-k6 server without preparing assets first (VERSION=dev)
	@go build -ldflags "$(LDFLAGS)" -o mcp-k6 ./cmd/mcp-k6

test: prepare ## Run Go unit tests
	@go test ./...

tests: test ## Alias for test

test-unit: test ## Alias for test

test-all: test test-e2e ## Run all tests (unit + e2e)

vet: prepare ## Run the vet command
	@go vet ./...

reviewable: prepare test-all vet ## Run the reviewable command
	@gofmt -l .
	@golangci-lint run
	@gosec -quiet ./...
	@govulncheck ./...

release:
	@goreleaser build --snapshot --clean

prepare: ensure-embed ## Prepare the mcp-k6 server for distribution
	@go run ./cmd/prepare

clean: ## Clean generated artifacts
	@rm -rf dist release k6-mcp prepare e2e/k6

docs: ensure-embed ## Prepare documentation assets (sections index + markdown)
	@go run ./cmd/prepare --docs-only

types: ensure-embed ## Collect TypeScript type definitions into dist/
	@go run ./cmd/prepare --types-only

ensure-embed: ## Ensure placeholder embed assets exist before prepare
	@mkdir -p dist/markdown dist
	@[ -f dist/sections.json ] || printf '%s\n' '{"versions":[],"latest":"","sections":{}}' > dist/sections.json
	@[ -f dist/markdown/placeholder.md ] || printf '%s\n' 'placeholder' > dist/markdown/placeholder.md

help: ## List available targets
	@echo "Available targets:"
	@awk -F '##' '/^[a-zA-Z0-9_.-]+:.*##/ { \
		target = $$1; \
		gsub(/[[:space:]]+$$/, "", target); \
		split(target, parts, ":"); \
		desc = $$2; \
		gsub(/^[[:space:]]+/, "", desc); \
		gsub(/[[:space:]]+$$/, "", desc); \
		printf "    %-24s %s\n", parts[1], desc; \
	}' $(MAKEFILE_LIST)

test-e2e: build test-e2e-setup ## Run end-to-end MCP tests
	@MCP_K6_BIN=$(CURDIR)/mcp-k6 e2e/k6 run --vus 1 --iterations 1 --no-usage-report --no-summary e2e/tools_test.js
	@MCP_K6_BIN=$(CURDIR)/mcp-k6 e2e/k6 run --vus 1 --iterations 1 --no-usage-report --no-summary e2e/resources_test.js
	@MCP_K6_BIN=$(CURDIR)/mcp-k6 e2e/k6 run --vus 1 --iterations 1 --no-usage-report --no-summary e2e/prompts_test.js

test-e2e-setup: ## Build the xk6-mcp custom k6 binary for e2e tests
	@command -v xk6 >/dev/null 2>&1 || go install go.k6.io/xk6/cmd/xk6@$(XK6_VERSION)
	@test -f e2e/k6 || xk6 build --with github.com/dgzlopes/xk6-mcp@$(XK6_MCP_VERSION) --output e2e/k6

list: help ## Alias for help
