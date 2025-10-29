// Package tools provides MCP tool definitions for the k6-mcp server.
package tools

import (
	"context"
	"log/slog"

	"github.com/grafana/k6-mcp/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// withToolLogger wraps a tool handler to inject a logger into context and provide panic recovery.
// The logger is configured with the tool name and made available via logging.LoggerFromContext.
func withToolLogger(toolName string, handler server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Create tool-specific logger and add to context
		logger := logging.WithTool(toolName)
		ctx = logging.ContextWithLogger(ctx, logger)

		// Panic recovery with logging
		defer func() {
			if r := recover(); r != nil {
				logger.ErrorContext(ctx, "panic in tool execution",
					slog.String("tool", toolName),
					slog.Any("panic", r))
			}
		}()

		return handler(ctx, request)
	}
}
