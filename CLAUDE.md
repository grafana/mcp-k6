# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

An experimental MCP (Model Context Protocol) server for k6, written in Go. Provides k6 script validation, test execution, fast full-text documentation search (embedded SQLite FTS5), and guided script generation via MCP tools, resources, and prompts.

## Architecture

### Core Components
- **cmd/mcp-k6/main.go**: MCP server entry point that registers tools, resources, and prompts via stdio transport
- **cmd/prepare/main.go**: Build-time tool that generates embedded assets (SQLite FTS5 docs index, TypeScript types, Terraform docs)
- **tools/**: MCP tool implementations (validate, run, search, info) with direct registration pattern
- **prompts/**: MCP prompt implementations (script generation)
- **internal/search/**: Full-text search using SQLite FTS5 with embedded documentation index
- **internal/security/**: Input validation, output sanitization, and environment security checks
- **internal/k6env/**: k6 executable detection and version management
- **internal/logging/**: Structured logging (slog) with context-based logger injection
- **resources/**: MCP resources (best practices guide, TypeScript type definitions)

### Key Dependencies
- `github.com/mark3labs/mcp-go` v0.32.0 - Core MCP library for server implementation
- SQLite with FTS5 extension - Embedded full-text search (requires `fts5` and `sqlite_fts5` build tags)
- k6 binary must be available in PATH for script execution

### Build System
- **Make-based**: All build commands use Makefile targets that handle asset preparation
- **Embedded assets**: Documentation index (`dist/index.db`) and TypeScript types embedded at build time using `go:embed`
- **Build tags required**: Always use `-tags 'fts5 sqlite_fts5'` for proper SQLite FTS5 support

## Common Commands

### Make Commands (Primary)
```bash
# Run the MCP server (generates index if missing)
make run

# Build the binary (generates index if missing)
make build

# Build without regenerating assets (faster for development)
make build-only

# Install to Go bin directory
make install

# Install without regenerating assets
make install-only

# Prepare embedded assets (docs index + TypeScript types + Terraform)
make prepare

# Regenerate only the documentation index
make index

# Collect only TypeScript type definitions
make collect

# Collect only Terraform documentation
make terraform

# Run tests
make test

# Run vet checks
make vet

# Create optimized release build
make release

# Clean generated artifacts
make clean

# List all available targets
make help
```

### Manual Commands (Without Make)
If you need to run commands directly:

```bash
# Generate the SQLite FTS5 docs index (REQUIRED before build/run)
go run -tags 'fts5 sqlite_fts5' ./cmd/prepare --index-only

# Run the MCP server
go run -tags 'fts5 sqlite_fts5' ./cmd/mcp-k6

# Build binary
go build -tags 'fts5 sqlite_fts5' -o mcp-k6 ./cmd/mcp-k6

# Run tests
go test -tags 'fts5 sqlite_fts5' ./...

# Run a single test
go test -tags 'fts5 sqlite_fts5' -run TestName ./path/to/package

# Install dependencies
go mod tidy
```

### Code Quality
```bash
# Run golangci-lint
golangci-lint run

# Auto-fix issues
golangci-lint run --fix
```

## Code Quality Requirements

**IMPORTANT**: All code must pass golangci-lint checks before being committed. The project uses a comprehensive linting configuration (.golangci.yml) optimized for Go 1.24+ with 40+ linters covering style, bugs, performance, and security.

Always run `golangci-lint run` before committing changes.

## Development Guidelines

### Build Tags
- **CRITICAL**: Always use `-tags 'fts5 sqlite_fts5'` when building, running, or testing
- Without these tags, the SQLite FTS5 full-text search will not work
- Makefile handles this automatically; only needed for manual Go commands

### Tool Implementation Pattern

Tools follow a consistent pattern in the `tools/` directory:

1. **Tool Definition**: Export a `*mcp.Tool` variable (e.g., `ValidateTool`)
2. **Registration Function**: Export `Register*Tool(s *server.MCPServer)` function
3. **Handler Function**: Private handler with signature `func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)`
4. **Logger Injection**: Use `withToolLogger()` wrapper in registration for automatic logger injection and panic recovery
5. **Results**: Return JSON-marshaled results via `mcp.NewToolResultText()`

**Example structure:**
```go
var MyTool = mcp.NewTool("my_tool", ...)

func RegisterMyTool(s *server.MCPServer) {
    s.AddTool(MyTool, withToolLogger("my_tool", myHandler))
}

func myHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    logger := logging.LoggerFromContext(ctx)
    // ... implementation
}
```

### Logging Pattern

The codebase uses **context-based logger injection**:

- **Logger injection**: `withToolLogger()` in `tools/tool.go` injects a logger into context and provides panic recovery
- **Retrieving logger**: Use `logging.LoggerFromContext(ctx)` in handlers
- **Structured logging**: Use slog with contextual attributes (see `internal/logging/helpers.go` for helpers)
- **Logging levels**:
  - `Debug`: Entry/exit, execution flow, parameter values
  - `Info`: Successful completions with key results
  - `Warn`: Recoverable errors, validation failures, timeouts
  - `Error`: Critical failures (environment issues, execution errors)

**Helper functions available:**
- `logging.ValidationEvent()` - Validation-specific events
- `logging.ExecutionEvent()` - Command execution logging
- `logging.FileOperation()` - File operation logging
- `logging.SecurityEvent()` - Security event logging

**Example:**
```go
func myHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    logger := logging.LoggerFromContext(ctx)
    logger.DebugContext(ctx, "Starting operation", slog.String("param", value))

    // ... do work

    logger.InfoContext(ctx, "Operation completed successfully", slog.Int("result_count", count))
    return result, nil
}
```

### Security Practices
- All script input must go through `security.ValidateScriptContent()`
- Use `security.ValidateEnvironment()` for command execution environment checks
- Sanitize outputs with `security.SanitizeOutput()`
- Create temp files via `createSecureTempFile()` helper with proper cleanup (returns cleanup function)
- Never log full script content - use size/hash instead

### Adding New MCP Tools

1. Create new file in `tools/` directory (e.g., `tools/mytool.go`)
2. Define tool with exported variable: `var MyTool = mcp.NewTool(...)`
3. Create registration function: `func RegisterMyTool(s *server.MCPServer)`
4. Use `withToolLogger()` to wrap handler: `s.AddTool(MyTool, withToolLogger("my_tool", handler))`
5. Register in `cmd/mcp-k6/main.go` during server initialization
6. Add comprehensive logging at entry/exit and error points

## MCP Server Capabilities

The server provides:
- **Tools**: 4 tools (validate_script, run_script, search_documentation, info)
- **Resources**: Best practices guide + TypeScript type definitions + Terraform documentation
- **Prompts**: Script generation prompt (generate_script)
- **Transport**: Stdio-based MCP communication
- **Logging**: Context-based logger injection with panic recovery for all tools

## Available Tools

### validate_script
Validates k6 scripts by running with minimal configuration (1 VU, 1 iteration). Returns enhanced validation results with issues, recommendations, and next steps.

**Implementation**: `tools/validate.go`
**Key features**: Static analysis, error pattern detection, actionable suggestions
**Timeout**: 30s validation timeout

### run_script
Executes k6 tests with configurable VUs/duration/iterations. Returns execution results with metrics and next steps.

**Implementation**: `tools/run.go`
**Limits**: Max 50 VUs, max 5m duration
**Timeout**: Default execution timeout

### search_documentation
Full-text search over embedded k6 docs using SQLite FTS5. Supports FTS5 query syntax (AND, OR, NEAR, phrases, prefix matching).

**Implementation**: `tools/search.go`
**Backend**: Embedded SQLite database with FTS5 virtual table
**Index generation**: `cmd/prepare/main.go` and `internal/search/index.go`

### info
Returns k6 environment information (version, path, login status).

**Implementation**: `tools/info.go`

## Documentation Search Architecture

The server uses **embedded SQLite FTS5** (full-text search) for documentation queries:

1. **Build-time indexing**: `cmd/prepare/main.go` scans k6 docs and creates `dist/index.db`
2. **Embedded database**: `dist/index.db` is embedded in the binary via `go:embed`
3. **Runtime extraction**: On startup, the embedded DB is written to a temporary file
4. **FTS5 queries**: `search_documentation` tool queries the FTS5 virtual table

**Key files**:
- `internal/search/index.go`: Documentation indexing logic
- `internal/search/sqlite.go`: SQLite FTS5 query implementation
- `internal/search/full-text.go`: Full-text search interface
- `cmd/mcp-k6/main.go`: Database extraction and temp file management (`openDB`, `closeDB`, `removeDBFile`)

## Working with Embedded Assets

### Documentation Index
- Generated by: `make index` or `make prepare`
- Source: k6 documentation markdown files in `k6-docs/` (Git submodule)
- Output: `dist/index.db` (embedded via `embed.go`)
- **Important**: Must regenerate after updating k6-docs submodule

### TypeScript Definitions
- Collected by: `make collect` or `make prepare`
- Source: k6 TypeScript `.d.ts` files
- Output: `dist/` directory (embedded via `internal/dist.go`)
- Exposed as MCP resources with `types://k6/` URI scheme

### Terraform Documentation
- Collected by: `make terraform` or `make prepare`
- Source: Grafana Terraform provider documentation
- Output: `dist/resources/TERRAFORM.md`
- Exposed as MCP resource at `docs://k6/terraform`

## Prerequisites

1. **Go 1.24.4+**: For building and running the MCP server
2. **GNU Make**: For build automation (preinstalled on macOS/Linux)
3. **k6**: Must be installed and available in PATH for script execution
4. **Git**: For submodule management (k6 docs are in a submodule)

Verify installation:
```bash
go version
make --version
k6 version
```

## Initial Setup

```bash
# Clone repository
git clone https://github.com/grafana/mcp-k6-server
cd mcp-k6-server

# Prepare assets and install
make install

# Verify installation
mcp-k6 --version
```

## Common Development Tasks

### Updating k6 Documentation
When the k6-docs submodule is updated:

```bash
# Update submodule to latest
git submodule update --remote k6-docs

# Regenerate the search index
make index

# Rebuild and reinstall
make install
```

### Testing Changes Locally
```bash
# Make code changes in internal/ or tools/

# Lint your changes
golangci-lint run

# Run tests
make test

# Test the server locally
make run
```

### Adding New Documentation Sources
If adding new markdown files to be indexed:

1. Add files to `k6-docs/` submodule (or configure in `cmd/prepare/main.go`)
2. Run `make index` to regenerate the SQLite database
3. Verify with `make run` and test the search tool

### Faster Development Iteration

For rapid iteration without regenerating assets:

```bash
# Build without regenerating assets (much faster)
make build-only

# Or install without regenerating assets
make install-only
```

Use these when working on code that doesn't affect embedded assets.
