//go:build !fts5

// Package main provides helpful build error messages when the required fts5 build tag is missing.
package main

import "fmt"

// This file is compiled when the fts5 build tag is NOT present.
// It provides a helpful error message to guide users.

func main() {
	fmt.Print(`
ERROR: Missing required build tag 'fts5'

This application requires the 'fts5' build tag to compile properly.

To build or run this application, use:
  go build -tags fts5 ./cmd/mcp-k6
  go run -tags fts5 ./cmd/mcp-k6
  go install -tags fts5 github.com/grafana/mcp-k6/cmd/mcp-k6

The fts5 tag is required for SQLite FTS5 full-text search functionality.
`)
}
