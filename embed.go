// Package k6mcp provides embedded resources for the k6 MCP server.
package k6mcp

import (
	"embed"
)

// EmbeddedDB contains the SQLite database file for k6 documentation search.
//
//go:embed dist/index.db
var EmbeddedDB []byte

// TypeDefinitions contains embedded TypeScript type definitions for k6.
//
//go:embed dist/definitions/types/k6/**
var TypeDefinitions embed.FS

// Resources contains embedded resource files such as prompts and templates.
//
//go:embed resources/**
var Resources embed.FS
