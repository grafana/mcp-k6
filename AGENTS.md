# Repository Guidelines

## Project Structure & Module Organization
The module `github.com/grafana/mcp-k6` has a single entrypoint in `cmd/mcp-k6` that starts the MCP server. Feature logic lives under `internal/` (`buildinfo`, `helpers`, `k6env`, `logging`, `security`). Documentation is served via the shared `github.com/grafana/xk6-docs/docs` package — downloaded on demand, cached locally, and kept fresh via ETag staleness checks. MCP tool handlers live in `tools/`, prompts in `prompts/`, and resources in `resources/`. Sample scripts live under `k6/`.

## Build, Test, and Development Commands
Use `make run` for local development. `make build`, `make install`, and `make release` create binaries with embedded version metadata; `make clean` removes generated output. `go run ./cmd/mcp-k6` and `go build -o mcp-k6 ./cmd/mcp-k6` work directly too.

## Coding Style & Naming Conventions
Target Go 1.24.4. Always format with `gofmt` (tabs, trailing newline) and maintain import order via `goimports` or `golangci-lint run --enable-only=gofmt,goimports`. Keep package names aligned with their directories, export only what other packages need, and reuse the helpers in `internal/logging` for consistent output. Document new build tags before introducing them.

## Testing Guidelines
Place table-driven tests in `*_test.go` next to the code they cover and use `testdata/` folders when fixtures are required. Run `go test ./...` (or `go test -v ./...`) before every PR. For new tools or handlers, include integration-style tests that assert MCP request/response behaviour and guard against missing scripts, invalid input, and security edge cases.

## Commit & Pull Request Guidelines
`main` currently has no published commits, so establish a clean history with Conventional Commit prefixes such as `feat:`, `fix:`, or `chore:`. Rewrite quick fixups locally. PRs should describe intent, list validation steps (`go test`, `golangci-lint run`), link issues, and attach screenshots or logs when user-visible behaviour changes. Call out generated files or manual setup steps in the description.

## Security & Configuration Tips
Preserve existing security measures: respect size limits, the 50 VU cap, and secure temporary-file helpers in `internal/security`. Note new environment variables or ports in `README.md` and prefer restrictive file permissions (`0600`) when touching filesystem paths.
