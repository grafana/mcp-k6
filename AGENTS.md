# Repository Guidelines

## Project Structure & Module Organization
The module `github.com/grafana/mcp-k6` is split by entrypoints in `cmd/`: `mcp-k6` starts the server and `prepare` refreshes embedded assets. Feature logic lives under `internal/` (`runner`, `sections`, `security`, `validator`, `logging`). Generated artifacts land in `dist/` and can be regenerated with `just prepare` or `just docs`. MCP resource bundles sit in `resources/`, while sample scripts live under `k6/`.

## Build, Test, and Development Commands
Use `just run` for local development; it launches the server after rebuilding docs assets if needed. `just build`, `just install`, and `just release` create binaries with embedded version metadata. `just prepare` refreshes docs and types; `just clean` removes generated output. Without `just`, fall back to `go run ./cmd/mcp-k6` for dev and `go build -o mcp-k6 ./cmd/mcp-k6` for binaries.

## Coding Style & Naming Conventions
Target Go 1.24.4. Always format with `gofmt` (tabs, trailing newline) and maintain import order via `goimports` or `golangci-lint run --enable-only=gofmt,goimports`. Keep package names aligned with their directories, export only what other packages need, and reuse the helpers in `internal/logging` for consistent output. Document new build tags before introducing them.

## Testing Guidelines
Place table-driven tests in `*_test.go` next to the code they cover and use `testdata/` folders when fixtures are required. Run `go test ./...` (or `go test -v ./...`) before every PR. For new tools or handlers, include integration-style tests that assert MCP request/response behaviour and guard against missing scripts, invalid input, and security edge cases.

## Commit & Pull Request Guidelines
`main` currently has no published commits, so establish a clean history with Conventional Commit prefixes such as `feat:`, `fix:`, or `chore:`. Rewrite quick fixups locally. PRs should describe intent, list validation steps (`go test`, `golangci-lint run`), link issues, and attach screenshots or logs when user-visible behaviour changes. Call out generated files or manual setup steps in the description.

## Security & Configuration Tips
Rebuild the docs assets (`just prepare` or `just docs`) after updating documentation sources. Preserve existing security measures: respect size limits, the 50 VU cap, and secure temporary-file helpers in `internal/security`. Note new environment variables or ports in `README.md` and prefer restrictive file permissions (`0600`) when touching filesystem paths.
