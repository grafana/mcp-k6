# mcp-k6 LLM evals

This directory contains an LLM-driven evaluation suite for mcp-k6, built with
[pytest](https://docs.pytest.org/) and [deepeval](https://deepeval.com/). It
evaluates how effectively an LLM uses the mcp-k6 MCP tools end-to-end: given a
natural-language prompt, a model is handed the real mcp-k6 tool set over a
live stdio connection, allowed to call tools freely, and the resulting tool
calls and final response are scored against expectations (correct tools
called, output quality).

## Prerequisites

- [uv](https://docs.astral.sh/uv/) for Python dependency management
- Go (matching the version in the repo's `go.mod`), to build the `mcp-k6`
  binary
- `k6` available in `PATH`, for `run_script` and `validate_script` to work
- `OPENAI_API_KEY` and `ANTHROPIC_API_KEY` in `tests/.env` (or the
  environment) — models under test include both an OpenAI and an Anthropic
  model, and deepeval's judge model uses `OPENAI_API_KEY` by default

## Setup

```bash
# from the repo root
go build -o mcp-k6 ./cmd/mcp-k6

cd tests
uv sync --all-groups
uv run pytest
```

## Environment variables

| Variable            | Default        | Description                                        |
| -------------------- | -------------- | --------------------------------------------------- |
| `MCP_K6_PATH`         | `../mcp-k6`    | Path to the built mcp-k6 binary                     |
| `OPENAI_API_KEY`      | —              | Used for the `gpt-4o` model under test and as the deepeval judge model |
| `ANTHROPIC_API_KEY`   | —              | Used for the `anthropic/claude-sonnet-4-5-20250929` model under test |

## Notes

- The `docs_test.py` and `run_test.py` tests need network access: doc tests
  fetch the k6 documentation bundle on first use (cached afterward), and
  `run_script` tests make live HTTP requests against
  `https://quickpizza.grafana.com` with a tiny load (1 VU, a couple of
  iterations).
- The `info` tool is intentionally not evaluated here — it errors out when
  run in a plain local environment because k6 cloud login parsing fails
  outside of an authenticated k6 cloud session, which would make evals of it
  flaky for reasons unrelated to LLM tool-use quality.
- deepeval's judge model (used to score `GEval` and `MCPUseMetric` criteria)
  uses `OPENAI_API_KEY` by default, independent of which model is under test
  in a given parametrized case.
