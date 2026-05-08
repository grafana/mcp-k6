# Make targets equivalent to justfile recipes.

SHELL := /bin/bash

TODAY := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo local)
VERSION ?= dev
XK6_MCP_VERSION ?= v0.0.3
XK6_VERSION ?= v1.2.6
E2E_K6_VERSION ?= v1.7.0

LDFLAGS := -s -w -X github.com/grafana/mcp-k6/internal/buildinfo.Version=$(VERSION) \
	-X github.com/grafana/mcp-k6/internal/buildinfo.Commit=$(COMMIT) \
	-X github.com/grafana/mcp-k6/internal/buildinfo.Date=$(TODAY)

CMD_PACKAGES := $(shell go list ./cmd/...)

.PHONY: run install build release clean help list test test-unit tests test-all test-e2e test-e2e-setup vet reviewable

run: ## Run the mcp-k6 server
	@go run ./cmd/mcp-k6

install: ## Install the mcp-k6 server (VERSION=dev)
	@go install -ldflags "$(LDFLAGS)" ./cmd/mcp-k6

build: ## Build the mcp-k6 server (VERSION=dev)
	@go build -ldflags "$(LDFLAGS)" -o mcp-k6 ./cmd/mcp-k6

test: ## Run Go unit tests
	@go test ./...

tests: test ## Alias for test

test-unit: test ## Alias for test

test-all: test test-e2e ## Run all tests (unit + e2e)

vet: ## Run the vet command
	@go vet ./...

reviewable: test-all vet ## Run the reviewable command
	@gofmt -l .
	@golangci-lint run
	@gosec -quiet ./...
	@govulncheck ./...

release:
	@goreleaser build --snapshot --clean

clean: ## Clean generated artifacts
	@rm -rf release k6-mcp e2e/k6

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
	@test -f e2e/k6 || xk6 build --k6-version $(E2E_K6_VERSION) --with github.com/dgzlopes/xk6-mcp@$(XK6_MCP_VERSION) --output e2e/k6

list: help ## Alias for help
