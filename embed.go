// Package k6mcp provides embedded resources for the k6 MCP server.
package k6mcp

import (
	"embed"
)

// TypeDefinitions contains embedded TypeScript type definitions for k6.
//
//go:embed dist/definitions/types/k6/**
var TypeDefinitions embed.FS

// Prompts contains embedded prompt markdown files.
//
//go:embed prompts/*.md
var Prompts embed.FS

// Resources contains static, embedded resource files such as prompts and templates.
//
//go:embed resources/*.md
var Resources embed.FS

// SectionsIndex contains the JSON index of documentation sections for all embedded k6 versions.
//
//go:embed dist/sections.json
var SectionsIndex []byte

// MarkdownFiles contains embedded markdown documentation files for all embedded k6 versions.
// Files are organized as: dist/markdown/{version}/**/*.md
//
//go:embed all:dist/markdown
var MarkdownFiles embed.FS
