# k6 MCP Server

An **experimental** MCP (Model Context Protocol) server for k6, written in Go. It offers script validation, test execution, fast full‑text documentation search (embedded SQLite FTS5), and guided script generation.

> ⚠️ This project is still experimental. Expect sharp edges, keep a local clone up to date, and share feedback or issues so we can iterate quickly.

## Features

### Prompts

- **Script Generation** with `/generate_k6_script`: Generate production‑ready k6 test scripts from plain‑English requirements. It automatically follows modern testing practices by leveraging embedded best practices, documentation, and type definitions.

### Tools

- **Script Validation**: `validate_k6_script` runs k6 scripts with minimal configuration (1 VU, 1 iteration) and returns actionable errors to help quickly produce correct code.
- **Test Execution**: `run_k6_script` runs k6 performance tests locally with configurable VUs, duration, stages, and options, and, when possible, extracts insights from the results.
- **Documentation Search (default)**: `search_k6_documentation` provides fast full‑text search over the official k6 docs (embedded SQLite FTS5 index) to help write modern, efficient k6 scripts.
- **Terraform (Grafana k6 Cloud)**: `generate_k6_cloud_terraform_load_test_resource` generates a Terraform resource for Grafana Cloud k6, letting you define and provision k6 Cloud tests with the Grafana k6 Terraform provider.

### Resources
- **Best Practices Resources**: Comprehensive k6 scripting guidelines and patterns to help you write effective, idiomatic, and correct tests.
- **Type Definitions**: Up‑to‑date k6 TypeScript type definitions to improve accuracy and editor tooling.


## Getting Started

### Prerequisites

Install the following:

- **Go 1.24.4+**: For building and running the MCP server
- **k6**: Must be installed and available in PATH for script execution
- **GNU Make**: Provides the automation targets used by this project (typically preinstalled on macOS/Linux)

Verify the tooling:
```bash
go version
k6 version
make --version
```

### Install

1. **Clone the repository**:
   ```bash
   git clone https://github.com/grafana/k6-mcp
   cd k6-mcp
   ```

2. **Prepare assets and install the server** (builds the documentation index, embeds resources, installs `k6-mcp` into your Go bin):
   ```bash
   make install
   ```

3. **Run the server locally**:
   ```bash
   make run
   ```

4. **Verify the binary** (optional, once `make install` has run):
   ```bash
   k6-mcp --version
   ```

Whenever docs or resources change, rebuild embeds with:
```bash
make prepare
```

### Editor Integrations

`k6-mcp` speaks MCP over stdio. After `make install`, the `k6-mcp` binary is available on your `PATH`; you can also run `make run` in a terminal to keep the server hot during development.

#### Cursor IDE

1. Ensure the server is installed (`make install`) or running in a terminal (`make run`).
2. Create or update `~/.cursor/mcp_servers.json` (or the profile-specific config) to include:
   ```json
   {
     "mcpServers": {
       "k6-mcp": {
         "command": "k6-mcp",
         "transport": "stdio",
         "env": {}
       }
     }
   }
   ```
3. Restart Cursor or reload the MCP configuration.
4. Call the tools from chat (validate scripts, run load tests, search docs, generate scripts).

#### Claude Code

Add the server to Claude Code with:
```bash
claude mcp add --scope=user --transport=stdio k6 k6-mcp
```
Use `--scope=local` if you prefer the configuration to live inside the current project. Reload the workspace to pick up the new server.

#### Claude Desktop

Place the following snippet in your Claude Desktop MCP configuration file (create it if necessary):
```json
{
  "mcpServers": {
    "k6-mcp": {
      "command": "k6-mcp",
      "transport": "stdio",
      "env": {}
    }
  }
}
```
Restart the desktop app or reload its MCP plugins afterwards.

#### Codex CLI

Codex CLI (experimental) supports MCP servers over stdio. Once `k6-mcp` is on your `PATH`:

1. Locate your Codex configuration (see `codex help config` for the exact path on your system).
2. Add or merge the following block under the top-level `mcpServers` key:
   ```json
   {
     "mcpServers": {
       "k6-mcp": {
         "command": "k6-mcp",
         "transport": "stdio",
         "env": {}
       }
     }
   }
   ```
3. Restart Codex or reload its configuration (`codex reload`) to make the new server available.


## Available Tools

### validate_script

Validate a k6 script by running it with minimal configuration (1 VU, 1 iteration).

Parameters:
- `script` (string, required)

Returns: `valid`, `exit_code`, `stdout`, `stderr`, `error`, `duration`

### run_test

Run k6 performance tests with configurable parameters.

Parameters:
- `script` (string, required)
- `vus` (number, optional)
- `duration` (string, optional)
- `iterations` (number, optional)
- `stages` (object, optional)
- `options` (object, optional)

Returns: `success`, `exit_code`, `stdout`, `stderr`, `error`, `duration`, `metrics`, `summary`

### search_documentation

Full‑text search over the embedded k6 docs index (SQLite FTS5).

Parameters:
- `keywords` (string, required): FTS5 query string
- `max_results` (number, optional, default 10, max 20)

FTS5 tips:
- Space‑separated words imply AND: `checks thresholds` → `checks AND thresholds`
- Quotes for exact phrases: `"load testing"`
- Operators supported: `AND`, `OR`, `NEAR`, parentheses, prefix `http*`

Returns an array of results with `title`, `content`, `path`.

## Available Resources

### Best Practices Guide

Access comprehensive k6 scripting best practices covering:
- Test structure and organization
- Performance optimization techniques
- Error handling and validation patterns
- Authentication and security practices
- Browser testing guidelines
- Modern k6 features and protocols

**Resource URI:** `docs://k6/best_practices`

### Script Generation Template

AI-powered k6 script generation with structured workflow:
- Research and discovery phase
- Best practices integration
- Production-ready script creation
- Automated validation and testing
- File system integration

**Resource URI:** `prompts://k6/generate_script`


## Development

### Make Commands (Recommended)

```bash
# Build and run the MCP server (generates the SQLite index if missing)
make run

# Build binary locally (generates the SQLite index if missing)
make build

# Install into your Go bin (generates the SQLite index if missing)
make install

# Optimized release build (stripped, reproducible paths)
make release

# Prepare embedded assets (docs index, types, prompts)
make prepare

# (Re)generate the embedded SQLite docs index
make index

# Collect TypeScript definitions into dist/
make collect

# Clean generated artifacts
make clean

# List available targets
make help
```

### Manual Commands

If you prefer not to use `make`:

```bash
# 1) Generate the SQLite FTS5 docs index (required for build/run because it is embedded)
go run -tags 'fts5 sqlite_fts5' ./cmd/prepare --index-only

# 2) Start the MCP server
go run -tags 'fts5 sqlite_fts5' ./cmd/k6-mcp

# Build a local binary
go build -tags 'fts5 sqlite_fts5' -o k6-mcp ./cmd/k6-mcp

# Release‑style build (macOS example)
CGO_ENABLED=1 go build -tags 'fts5 sqlite_fts5' -trimpath -ldflags '-s -w' -o k6-mcp ./cmd/k6-mcp

# Run tests
go test ./...

# Lint
golangci-lint run
```

### Project Structure

```
├── cmd/
│   ├── k6-mcp/               # MCP server entry point
│   └── indexer/              # Builds the SQLite FTS5 docs index into dist/index.db
├── dist/
│   └── index.db              # Embedded SQLite FTS5 index (generated)
├── internal/
│   ├── runner/               # Test execution engine
│   ├── search/               # Full‑text search and indexer
│   ├── security/             # Security utilities
│   └── validator/            # Script validation
├── resources/                # MCP resources
│   ├── practices/            # Best practices guide
│   └── prompts/              # AI prompt templates
├── python-services/          # Optional utilities (embeddings, verification)
└── k6/scripts/               # Generated k6 scripts
```

## Security

The MCP server implements comprehensive security measures:

- **Input validation**: Size limits (1MB maximum) and dangerous pattern detection
- **Secure execution**: Blocks Node.js modules, system access, and malicious code patterns
- **File handling**: Restricted permissions (0600) and secure temporary file management
- **Resource limits**: Command execution timeouts (30s validation, 5m tests), max 50 VUs
- **Environment isolation**: Minimal k6 execution environment with proper cleanup
- **Docker hardening**: Non-root user, read-only filesystem, no new privileges

## Usage Examples

### Basic Script Validation

```bash
# In your MCP-enabled editor, ask:
"Can you validate this k6 script?"

# Then provide your k6 script content
```

### Performance Testing

```bash
# In your MCP-enabled editor, ask:
"Run a load test with 10 VUs for 2 minutes using this script"

# The system will execute the test and provide detailed metrics
```

### Documentation Search

```bash
# In your MCP-enabled editor, ask:
"Search for k6 authentication examples"
"How do I use thresholds in k6?"
"Show me WebSocket testing patterns"
```

### Script Generation

```bash
# In your MCP-enabled editor, ask:
"Generate a k6 script to test a REST API with authentication"
"Create a browser test for an e-commerce checkout flow"
"Generate a WebSocket load test script"
```

## Troubleshooting

### Build fails with “dist/index.db: no matching files”
Generate the docs index first:
```bash
make index
```

### Search returns no results
- Ensure the index exists: `ls dist/index.db`
- Rebuild the index: `make index`
- Try simpler queries, or quote phrases: `"load testing"`

### MCP Server Not Found
If your editor can't find the k6-mcp server:
1. Ensure it's installed: `make install`
2. Check your editor's MCP configuration
3. Verify the server starts: `k6-mcp` (should show MCP server output)

### Test Execution Failures
If k6 tests fail to execute:
1. Verify k6 is installed: `k6 version`
2. Check script syntax with the validate tool first
3. Ensure resources don't exceed limits (50 VUs, 5m duration)

## Contributing

1. Fork the repository
2. Create a feature branch
3. Run tests: `go test ./...`
4. Run linter: `golangci-lint run`
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.
