// Package handlers provides MCP request handlers for tool calls and prompts.
package handlers

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

// ToolHandler defines an interface for MCP tool calls handlers.
type ToolHandler interface {
	Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
}

// PromptHandler defines an interface for MCP prompt request handlers.
type PromptHandler interface {
	Handle(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error)
}
