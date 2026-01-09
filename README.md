# k6 MCP Server

An **experimental** MCP (Model Context Protocol) server for k6, written in Go. It offers script validation, test execution, fast full‑text documentation search (embedded SQLite FTS5), and guided script generation.

> ⚠️ This project is still experimental. Expect sharp edges, keep a local clone up to date, and share feedback or issues so we can iterate quickly.

## Features

### Tools
- **Script Validation**: `validate_k6_script` runs k6 scripts with minimal configuration (1 VU, 1 iteration) and returns actionable errors to help quickly produce correct code.
- **Test Execution**: `run_k6_script` runs k6 performance tests locally with configurable VUs, duration, stages, and options, and, when possible, extracts insights from the results.
- **Documentation Search (default)**: `search_k6_documentation` provides fast full‑text search over the official k6 docs (embedded SQLite FTS5 index) to help write modern, efficient k6 scripts.

### Resources
- **Best Practices Resources**: Comprehensive k6 scripting guidelines and patterns to help you write effective, idiomatic, and correct tests.
- **Type Definitions**: Up‑to‑date k6 TypeScript type definitions to improve accuracy and editor tooling.

### Prompts
- **Script Generation** with `generate_script`: Generate production‑ready k6 test scripts from plain‑English requirements. It automatically follows modern testing practices by leveraging embedded best practices, documentation, and type definitions.

## Getting Started

Choose your preferred installation method:

### Option 1: Docker (Recommended)

The easiest way to get started. The Docker image includes k6 and all dependencies.

**Prerequisites:**
- **Docker**: Install from [docker.com](https://www.docker.com/get-started)

**Pull the official image**:
   ```bash
   docker pull grafana/mcp-k6:latest
   ```

That's it! You're ready to run the containerized server. Proceed to [Editor Integrations](#editor-integrations) to configure your editor.

### Option 2: Homebrew macOS

Install mcp-k6 using Homebrew. k6 will be automatically installed as a dependency.

**Installation:**

```bash
# Add the grafana homebrew tap repository
brew tap grafana/grafana

# Install mcp-k6
brew install mcp-k6

# Verify installation
mcp-k6 --version
```

To update to a newer version, download and install the latest formula again, or use `brew upgrade` if installed from the tap.

Proceed to [Editor Integrations](#editor-integrations) to configure your editor.

### Option 3: Package Installation (Linux)

Pre-built packages are available for Debian/Ubuntu and RHEL/Fedora/CentOS distributions.

> **Version naming:** Git tags include the `v` prefix (e.g. `v0.2.0`), but release files drop it. Replace `VERSION` below with just the numeric part (`0.2.0`).

**Prerequisites:**
- **k6** (recommended): Should be installed for script execution

#### Debian/Ubuntu (.deb)

Download and install the `.deb` package from the [latest release](https://github.com/grafana/mcp-k6/releases/latest):

```bash
# For amd64 (x86_64)
curl -LO https://github.com/grafana/mcp-k6/releases/latest/download/mcp-k6_VERSION_linux_amd64.deb
sudo dpkg -i mcp-k6_0.2.0_linux_amd64.deb

# For arm64
curl -LO https://github.com/grafana/mcp-k6/releases/latest/download/mcp-k6_VERSION_linux_arm64.deb
sudo dpkg -i mcp-k6_0.2.0_linux_arm64.deb
```

Or install directly with `apt`:
```bash
# Download the package
curl -LO https://github.com/grafana/mcp-k6/releases/latest/download/mcp-k6_VERSION_linux_amd64.deb

# Install with apt (resolves dependencies)
sudo apt install ./mcp-k6_0.2.0_linux_amd64.deb
```

#### RHEL/Fedora/CentOS (.rpm)

Download and install the `.rpm` package from the [latest release](https://github.com/grafana/mcp-k6/releases/latest):

```bash
# For amd64 (x86_64)
curl -LO https://github.com/grafana/mcp-k6/releases/latest/download/mcp-k6_VERSION_linux_amd64.rpm
sudo rpm -i mcp-k6_0.2.0_linux_amd64.rpm

# For arm64
curl -LO https://github.com/grafana/mcp-k6/releases/latest/download/mcp-k6_VERSION_linux_arm64.rpm
sudo rpm -i mcp-k6_0.2.0_linux_arm64.rpm
```

Or use `dnf`/`yum`:
```bash
# Fedora/RHEL 8+
sudo dnf install https://github.com/grafana/mcp-k6/releases/latest/download/mcp-k6_VERSION_linux_amd64.rpm

# CentOS/RHEL 7
sudo yum install https://github.com/grafana/mcp-k6/releases/latest/download/mcp-k6_VERSION_linux_amd64.rpm
```

**Verify installation:**
```bash
mcp-k6 --version
```

The binary is installed to `/usr/bin/mcp-k6`. Proceed to [Editor Integrations](#editor-integrations) to configure your editor.


### Option 4: Native Installation

For development or if you prefer running the server natively.

**Prerequisites:**
- **Go 1.24.4+**: For building and running the MCP server
- **k6**: Must be installed and available in PATH for script execution
- **GNU Make**: Provides the automation targets used by this project (typically preinstalled on macOS/Linux)

Verify the tooling:
```bash
go version
k6 version
make --version
```

**Installation:**

1. **Clone the repository**:
   ```bash
   git clone https://github.com/grafana/mcp-k6
   cd mcp-k6
   ```

2. **Prepare assets and install the server** (builds the documentation index, embeds resources, installs `mcp-k6` into your Go bin):
   ```bash
   make install
   ```

3. **Run the server locally** (optional):
   ```bash
   make run
   ```

4. **Verify the binary** (optional):
   ```bash
   mcp-k6 --version
   ```

Whenever docs or resources change, rebuild embeds with:
```bash
make prepare
```

### Editor Integrations

`mcp-k6` speaks MCP over stdio. Choose the configuration that matches your installation method.

#### Cursor IDE

Create or update `~/.cursor/mcp_servers.json` (or the profile-specific config):

**Docker:**
```json
{
  "mcpServers": {
    "k6": {
      "command": "docker",
      "args": [
        "run",
        "--rm",
        "-i",
        "grafana/mcp-k6",
        "-t",
        "stdio"
      ]
    }
  }
}
```

**Native:**
```json
{
  "mcpServers": {
    "mcp-k6": {
      "command": "mcp-k6",
      "transport": "stdio",
      "env": {}
    }
  }
}
```

Restart Cursor or reload the MCP configuration, then call the tools from chat (validate scripts, run load tests, search docs, generate scripts).

#### Claude Code

Add the server to Claude Code:

**Docker:**
```bash
claude mcp add --scope=user --transport=stdio k6 docker run --rm -i mcp-k6
```

**Native:**
```bash
claude mcp add --scope=user --transport=stdio k6 mcp-k6
```

Use `--scope=local` if you prefer the configuration to live inside the current project. Reload the workspace to pick up the new server.

#### Claude Desktop

Place one of the following snippets in your Claude Desktop MCP configuration file (create it if necessary):

**Docker:**
```json
{
  "mcpServers": {
    "k6": {
      "command": "docker",
      "args": [
        "run",
        "--rm",
        "-i",
        "grafana/mcp-k6",
        "-t",
        "stdio"
      ]
    }
  }
}
```

**Native:**
```json
{
  "mcpServers": {
    "mcp-k6": {
      "command": "mcp-k6",
      "transport": "stdio",
      "env": {}
    }
  }
}
```

Restart the desktop app or reload its MCP plugins afterwards.

#### Codex CLI

Codex CLI (experimental) supports MCP servers over stdio.

1. Locate your Codex configuration (see `codex help config` for the exact path on your system).
2. Add or merge one of the following blocks under the top-level `mcpServers` key:

**Docker:**
```json
{
  "mcpServers": {
    "k6": {
      "command": "docker",
      "args": [
        "run",
        "--rm",
        "-i",
        "grafana/mcp-k6",
        "-t",
        "stdio"
      ]
    }
  }
}
```

**Native:**
```json
{
  "mcpServers": {
    "mcp-k6": {
      "command": "mcp-k6",
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

### Script Generation Template

AI-powered k6 script generation with structured workflow:
- Research and discovery phase
- Best practices integration
- Production-ready script creation
- Automated validation and testing
- File system integration

**Resource URI:** `prompts://k6/generate_script`

## Development

Run `make list` to get a list of available Make commands.

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
If your editor can't find the mcp-k6 server:
1. Ensure it's installed: `make install`
2. Check your editor's MCP configuration
3. Verify the server starts: `mcp-k6` (should show MCP server output)

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
