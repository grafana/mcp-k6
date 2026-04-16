// Package k6mcp provides embedded resources for the k6 MCP server.
package k6mcp

import (
	"embed"
)

// Prompts contains embedded prompt markdown files.
//
//go:embed prompts/*.md
var Prompts embed.FS

// Resources contains static, embedded resource files such as prompts and templates.
//
//go:embed resources/*.md
var Resources embed.FS
