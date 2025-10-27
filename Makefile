# Make targets equivalent to justfile recipes.

SHELL := /bin/bash

TODAY := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo local)
VERSION ?= dev

GO_TAGS := fts5 sqlite_fts5
LDFLAGS := -s -w -X github.com/grafana/k6-mcp/internal/buildinfo.Version=$(VERSION) \
	-X github.com/grafana/k6-mcp/internal/buildinfo.Commit=$(COMMIT) \
	-X github.com/grafana/k6-mcp/internal/buildinfo.Date=$(TODAY)

CMD_PACKAGES := $(shell go list ./cmd/...)

.PHONY: run install install-only build build-only release prepare clean index collect help list tests

run: prepare ## Run the k6-mcp server
	@go run -tags '$(GO_TAGS)' ./cmd/k6-mcp

install: prepare install-only ## Install the k6-mcp server (VERSION=dev)

install-only: ## Install the k6-mcp server without preparing assets first (VERSION=dev)
	@go install -tags '$(GO_TAGS)' -ldflags "$(LDFLAGS)" ./cmd/k6-mcp

build: prepare build-only ## Build the k6-mcp server (VERSION=dev)

build-only: ## Build the k6-mcp server without preparing assets first (VERSION=dev)
	@go build -tags '$(GO_TAGS)' -ldflags "$(LDFLAGS)" -o k6-mcp ./cmd/k6-mcp

test: prepare## Run the tests
	@go test -tags '$(GO_TAGS)' ./...

tests: test ## Alias for test

vet: prepare ## Run the vet command
	@go vet ./...

release: prepare ## Create a release-style build (VERSION=dev)
	@go build -tags '$(GO_TAGS)' -trimpath -ldflags "$(LDFLAGS)" -o k6-mcp ./cmd/k6-mcp

prepare: ## Prepare the k6-mcp server for distribution
	@go run -tags '$(GO_TAGS)' ./cmd/prepare

clean: ## Clean generated artifacts
	@rm -rf dist k6-mcp prepare

index: ## Regenerate the documentation index database
	@go run -tags '$(GO_TAGS)' ./cmd/prepare --index-only

collect: ## Collect TypeScript type definitions into dist/
	@go run ./cmd/prepare --collect-only

terraform: ## Collect Grafana Terraform provider resource definitions (for k6 Cloud) into dist/
	@go run ./cmd/prepare --terraform-only

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

list: help ## Alias for help
