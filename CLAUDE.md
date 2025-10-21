# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is an experimental MCP (Model Context Protocol) server for k6, built in Go. The project uses the `mcp-go` library to create an MCP server that communicates via stdio and provides k6 script validation, execution, and documentation search capabilities.

## Architecture

- **cmd/k6-mcp/main.go**: Entry point that creates and serves an MCP server with three main tools: validate, run, and search
- **internal/validator/**: Core k6 script validation logic with security measures
- **internal/runner/**: k6 test execution with configurable parameters and result parsing
- **internal/search/**: Semantic search functionality using Chroma vector database
- **internal/security/**: Security utilities for input validation and dangerous pattern detection
- **k6-docs/**: Git submodule containing official k6 documentation for search indexing
- Uses `github.com/mark3labs/mcp-go` v0.32.0 as the core MCP library
- Uses `github.com/amikos-tech/chroma-go` for vector database interactions

## Common Commands

### Development
```bash
# Run the MCP server
go run ./cmd/k6-mcp

# Build the project
go build -o k6-mcp ./cmd/k6-mcp

# Install dependencies
go mod tidy

# Update dependencies
go get -u ./...
```

### Just Commands (Recommended)
The project uses `just` for common tasks. Install with `brew install just`:

```bash
# Initialize git submodules (k6-docs)
just initialize

# Run the MCP server
just run

# Start Chroma vector database for search functionality
just chroma

# Ingest k6 documentation into vector database
just ingest

# Verify vector database ingestion
just verify
```

### Testing
```bash
# Run tests (when available)
go test ./...

# Run tests with verbose output
go test -v ./...
```

### Code Quality
```bash
# Run golangci-lint (install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
golangci-lint run

# Run golangci-lint with auto-fix
golangci-lint run --fix

# Run specific linters
golangci-lint run --enable-only=gofmt,goimports
```

## Code Quality Requirements

**IMPORTANT**: All code must pass golangci-lint checks before being committed. The project uses a comprehensive linting configuration that enforces:

- Idiomatic Go practices following the Go team's standards
- Code style consistency with popular Go projects (Kubernetes, Docker, etc.)
- Security best practices via gosec
- Performance optimizations
- Proper error handling and context usage
- Documentation standards for exported functions

Always run `golangci-lint run` before committing changes. The configuration is optimized for Go 1.24+ and includes 40+ linters covering style, bugs, performance, and security.

## Coding Practices and Memories

### Development Guidelines
- Always verify that Go code you write passes the golangci lint run against the .golangci.yml configuration
- Always assume we use poetry when interacting with Python
- Always prioritize idiomatic, modern, best practices when writing code

## MCP Server Configuration

The server is configured with:
- Name: "k6"
- Version: "1.0.0"  
- Resource capabilities enabled (read/write)
- Logging middleware
- Recovery middleware
- Stdio transport for communication

## Available Tools

### validate
Validates k6 scripts by executing them with minimal configuration (1 VU, 1 iteration).

**Parameters:**
- `script` (string, required): The k6 script content to validate (JavaScript/TypeScript)

**Returns:**
- JSON object with validation results including:
  - `valid` (bool): Whether the script is valid
  - `exit_code` (int): k6 exit code
  - `stdout` (string): Standard output from k6
  - `stderr` (string): Standard error from k6
  - `error` (string): Error message if validation failed
  - `duration` (string): Time taken for validation

### run
Executes k6 performance tests with configurable parameters for load testing scenarios.

**Parameters:**
- `script` (string, required): The k6 script content to run (JavaScript/TypeScript)
- `vus` (number, optional): Number of virtual users (default: 1, max: 50)
- `duration` (string, optional): Test duration (default: "30s", max: "5m")
- `iterations` (number, optional): Number of iterations per VU (overrides duration)
- `stages` (array, optional): Load profile stages for ramping (array of {duration, target})
- `options` (object, optional): Additional k6 options as JSON object

**Returns:**
- JSON object with execution results including:
  - `success` (bool): Whether the test completed successfully
  - `exit_code` (int): k6 exit code
  - `stdout` (string): Standard output from k6
  - `stderr` (string): Standard error from k6
  - `error` (string): Error message if execution failed
  - `duration` (string): Total execution time
  - `metrics` (object): Raw k6 metrics data
  - `summary` (object): Parsed test summary with key performance metrics

### search
Searches k6 documentation using semantic similarity via Chroma vector database.

**Parameters:**
- `query` (string, required): The search query to find relevant k6 documentation
- `max_results` (number, optional): Maximum number of results to return (default: 5, max: 20)

**Returns:**
- JSON object with search results including:
  - `query` (string): The original search query
  - `results` (array): Array of search results with content, metadata, and similarity scores
  - `result_count` (int): Number of results returned

**Prerequisites:**
- Chroma vector database must be running (`just chroma`)
- k6 documentation must be ingested (`just ingest`)

**Security Features:**
- Input validation and size limits
- Secure temporary file handling with restricted permissions (0600)
- Command execution timeouts (30s for validation, 5m for runs)
- Dangerous pattern detection (blocks Node.js modules, system access)
- Limited maximum VUs (50) and duration (5 minutes) to prevent resource abuse

## Documentation Access

The project provides k6 documentation access through two mechanisms:

### 1. Search Tool (Recommended)
Uses semantic search via Chroma vector database for intelligent documentation retrieval:
- Semantic similarity matching for natural language queries
- Returns relevant documentation snippets with context
- Requires Chroma database setup (`just chroma` and `just ingest`)

### 2. Direct File Access
The k6-docs git submodule contains the complete official k6 documentation:
- Located in `k6-docs/` directory
- Structured markdown files organized by category
- Updated via `git submodule update --remote k6-docs`

**Available Documentation Categories:**
- **get-started/**: Getting started guides and tutorials  
- **javascript-api/**: Complete k6 JavaScript API reference
- **examples/**: Practical examples and use cases
- **testing-guides/**: Performance testing methodologies
- **using-k6/**: k6 features and configuration
- **using-k6-browser/**: Browser testing with k6
- **release-notes/**: Version-specific release information

## Project Structure

```
├── cmd/k6-mcp/
│   └── main.go               # MCP server entry point and tool registration
├── internal/
│   ├── runner/               # k6 test execution with configurable parameters
│   │   └── runner.go         # Test execution, result parsing, timeout handling
│   ├── search/               # Semantic search functionality
│   │   └── search.go         # Chroma vector database integration
│   ├── security/             # Security utilities and input validation
│   │   └── security.go       # Dangerous pattern detection, input sanitization
│   └── validator/            # Core k6 script validation logic
│       └── validator.go      # Script validation, temp file handling, k6 execution
├── k6-docs/                  # Git submodule: Official k6 documentation repository
├── python-services/          # Python services for documentation ingestion
├── volumes/                  # Docker volume mounts for databases
├── justfile                  # Task runner with common development commands
├── docker-compose.chroma.yml # Chroma vector database configuration
├── docker-compose.milvus.yml # Milvus vector database configuration
└── .golangci.yml            # Comprehensive linting configuration
```

## Development Setup

### Prerequisites
1. **Go 1.24.4+**: For building and running the MCP server
2. **Docker**: For running vector databases (Chroma/Milvus)
3. **Just**: Command runner for development tasks (`brew install just`)
4. **k6**: Must be installed and available in PATH for script execution

### Initial Setup
```bash
# Clone with submodules
git clone --recursive https://github.com/grafana/k6-mcp

# Or initialize submodules after cloning
just initialize

# Start vector database
just chroma

# Ingest documentation (requires Python services)
just ingest
```

## Git Submodule Management

The k6 documentation is included as a git submodule. To update the documentation:

```bash
# Initialize submodule on first clone
git submodule update --init --recursive

# Update documentation to latest version
git submodule update --remote k6-docs

# Commit the submodule update
git add k6-docs
git commit -m "docs: update k6 documentation to latest version"
```