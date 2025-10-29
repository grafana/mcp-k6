// Package resources provides MCP resource definitions for the k6-mcp server.
package resources

import (
	"context"
	"fmt"

	k6mcp "github.com/grafana/k6-mcp"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// bestPracticesResource is the MCP resource definition for the k6 best practices guide.
//
//nolint:gochecknoglobals // Shared resource definition registered at startup.
var bestPracticesResource = mcp.NewResource(
	"docs://k6/best_practices",
	"k6 best practices",
	mcp.WithResourceDescription("Provides a list of best practices for writing k6 scripts."),
	mcp.WithMIMEType("text/markdown"),
)

// RegisterBestPracticesResource registers the best practices resource with the MCP server.
func RegisterBestPracticesResource(s *server.MCPServer) {
	s.AddResource(bestPracticesResource, bestPractices)
}

// bestPractices is the handler for the best practices resource.
func bestPractices(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	content, err := k6mcp.Resources.ReadFile("resources/best_practices.md")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded best practices resource: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "docs://k6/best_practices",
			MIMEType: "text/markdown",
			Text:     string(content),
		},
	}, nil
}
