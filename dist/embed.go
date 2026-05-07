// Package dist provides embedded distribution assets for the k6 MCP server.
package dist

import "embed"

// TypeDefinitions contains embedded TypeScript type definitions for k6.
//
//go:embed definitions/types/k6/**
var TypeDefinitions embed.FS
