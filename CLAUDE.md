# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

An experimental MCP (Model Context Protocol) server for k6, written in Go. Provides k6 script validation, test execution, documentation browsing, and guided script generation via MCP tools, resources, and prompts.

## Architecture

### Core Components
- **cmd/mcp-k6/main.go**: MCP server entry point that registers tools, resources, and prompts via stdio transport
- **register.go**: k6 subcommand registration (`k6 x mcp`); import the root package via xk6 to activate
- **tools/**: MCP tool implementations (validate, run, list_sections, get_documentation, info) with direct registration pattern
- **prompts/**: MCP prompt implementations (script generation); embeds `*.md` templates directly
- **resources/**: MCP resources (best practices guide); embeds `*.md` directly
- **internal/security/**: Input validation, output sanitization, and environment security checks
- **internal/k6env/**: k6 executable detection and version management
- **internal/logging/**: Structured logging (slog) with context-based logger injection; includes logrus bridge (`logrus_handler.go`) for the k6 subcommand path

### Key Dependencies
- `github.com/mark3labs/mcp-go` - Core MCP library for server implementation
- `go.k6.io/k6/v2` - k6 subcommand registration API (`subcommand.RegisterExtension`)
- `github.com/sirupsen/logrus` - Bridged to slog for the k6 subcommand path
- k6 binary must be available in PATH for script execution

### Build System
- **Make-based**: All build commands use Makefile targets
- **Embedded assets**: Prompt and resource markdown files embedded at build time using `go:embed`. Documentation content is fetched at runtime via `github.com/grafana/xk6-docs/docs`.
- **k6 subcommand**: Build a custom k6 binary with `xk6 build --with github.com/grafana/mcp-k6=.`; the extension registers as `k6 x mcp`

## Common Commands

### Make Commands (Primary)
```bash
# Run the MCP server
make run

# Build the binary
make build

# Install to Go bin directory
make install

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
# Run the MCP server
go run ./cmd/mcp-k6

# Build binary
go build -o mcp-k6 ./cmd/mcp-k6

# Run tests
go test ./...

# Run a single test
go test -run TestName ./path/to/package

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
- **Tools**: validate_script, run_script, list_sections, get_documentation, search_terraform, info
- **Resources**: Best practices guide
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

### list_sections
Lists documentation sections as a depth-limited tree for progressive browsing.

**Implementation**: `tools/list_sections.go`

### get_documentation
Returns full markdown content for a specific documentation section.

**Implementation**: `tools/get_documentation.go`

### info
Returns k6 environment information (version, path, login status).

**Implementation**: `tools/info.go`

## Documentation Browsing Architecture

Documentation is fetched at runtime via the `github.com/grafana/xk6-docs/docs` library (a `docs.Catalog` is constructed in `mcpserver/server.go`). Bundles are downloaded and cached on first use; there is no build-time indexing step.

**Key files**:
- `tools/list_sections.go`: Section tree browsing
- `tools/get_documentation.go`: Markdown content retrieval

## Prerequisites

1. **Go 1.24.4+**: For building and running the MCP server
2. **GNU Make**: For build automation (preinstalled on macOS/Linux)
3. **k6**: Must be installed and available in PATH for script execution

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

# Build and install
make install

# Verify installation
mcp-k6 --version
```

## Common Development Tasks

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
