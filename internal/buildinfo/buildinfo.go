// Package buildinfo exposes version metadata populated at build time via -ldflags.
//
// Example:
//
//	go build \
//	  -ldflags "-X github.com/grafana/k6-mcp/internal/buildinfo.Version=1.3.0 \
//	            -X github.com/grafana/k6-mcp/internal/buildinfo.Commit=abcdef1 \
//	            -X github.com/grafana/k6-mcp/internal/buildinfo.Date=2025-08-21T12:00:00Z" \
//	  ./cmd/k6-mcp
package buildinfo

//nolint:gochecknoglobals // Build-time variables set via -ldflags
var (
	// Version is the semantic version of the binary (e.g., 1.3.0). Defaults to "dev".
	Version = "dev"

	// Commit is the short git commit hash used for the build. Defaults to ""
	Commit = ""

	// Date is the build date/time in RFC3339 format. Defaults to "".
	Date = ""
)
