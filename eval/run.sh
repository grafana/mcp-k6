#!/usr/bin/env bash
# Agent eval for mcp-k6 — a single `k6 run` that drives Claude Code against the
# real mcp-k6 server and scores it with k6's experimental ageval module.
#
# This script only does setup (build the server, write the MCP config, put k6 on
# PATH); the agent itself is run *by* the k6 test (ExternalAgent), not here.
#
# Prereqs:
#   - `claude` CLI installed and logged in (https://claude.com/claude-code)
#   - K6_BIN = a k6 binary built with the k6/experimental/ageval module
#     (also exposed as `k6` on PATH so validate_script/run_script work). Default `k6`.
#   - ANTHROPIC_API_KEY_JUDGE (or ANTHROPIC_API_KEY) exported for the LLM-as-judge.
#
# Usage:
#   K6_BIN=/path/to/k6-with-ageval ./eval/run.sh
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$DIR/.." && pwd)"
K6="${K6_BIN:-k6}"

echo "==> Building mcp-k6..."
( cd "$ROOT" && make build )

# Expose the k6 binary as `k6` on PATH so mcp-k6's validate_script/run_script work.
K6ABS="$(command -v "$K6" || true)"
if [ -n "$K6ABS" ]; then
  BINDIR="$(mktemp -d)"
  ln -sf "$K6ABS" "$BINDIR/k6"
  export PATH="$BINDIR:$PATH"
else
  echo "WARN: '$K6' not found; mcp-k6's validate_script will report k6 missing (the agent still calls it)."
fi

echo "==> Writing Claude Code MCP config (server key 'k6' -> tools mcp__k6__*)..."
cat > "$DIR/mcp-config.json" <<EOF
{
  "mcpServers": {
    "k6": { "command": "$ROOT/mcp-k6", "args": [] }
  }
}
EOF

echo "==> Running the eval (k6 drives Claude Code against mcp-k6)..."
( cd "$ROOT" && "$K6" run eval/generate_script.test.js )
