// Package tools contains MCP tool definitions exposed by the mcp-k6 server.
package tools

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/grafana/mcp-k6/internal/buildinfo"
	"github.com/grafana/mcp-k6/internal/k6env"
	"github.com/grafana/mcp-k6/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// InfoTool exposes runtime information about mcp-k6 and the local k6 environment.
//
//nolint:gochecknoglobals // Shared tool definition registered at startup.
var InfoTool = mcp.NewTool(
	"info",
	mcp.WithDescription("Get details about the mcp-k6 server, the local k6 binary, and k6 Cloud login status."),
)

// RegisterInfoTool registers the info tool with the MCP server.
func RegisterInfoTool(
	s *server.MCPServer,
) {
	s.AddTool(InfoTool, withToolLogger("info", info))
}

// HandleInfo is the handler implementation for the info tool.
// It can be wrapped with middleware before being passed to RegisterInfoTool.
func info(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	logger := logging.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Starting info tool execution")

	// Locate the k6 executable
	k6Info, err := k6env.Locate(ctx)
	if err != nil {
		logger.WarnContext(ctx, "Failed to locate k6 executable",
			slog.String("error", err.Error()))
		return mcp.NewToolResultError("Failed to locate k6 executable on the user's system; reason: " + err.Error()), nil
	}
	logger.DebugContext(ctx, "k6 executable located successfully")

	// Extract the located k6 binary's k6Version
	k6Version, err := k6Info.Version(ctx)
	if err != nil {
		logger.WarnContext(ctx, "Failed to get k6 version",
			slog.String("error", err.Error()))
		return mcp.NewToolResultError("Failed to get user's k6 binary version; reason: " + err.Error()), nil
	}
	logger.DebugContext(ctx, "Retrieved k6 version",
		slog.String("k6_version", k6Version))

	// Check if the user is logged in to k6 cloud
	isLoggedIn, err := k6Info.IsLoggedIn(ctx)
	if err != nil {
		logger.WarnContext(ctx, "Failed to check k6 login status",
			slog.String("error", err.Error()))
		return mcp.NewToolResultError("Failed to check if k6 is logged in; reason: " + err.Error()), nil
	}
	logger.DebugContext(ctx, "k6 login status checked",
		slog.Bool("logged_in", isLoggedIn))

	// Create the response
	response := InfoResponse{
		Version:   buildinfo.Version,
		K6Version: k6Version,
		LoggedIn:  isLoggedIn,
	}

	// Marshal the response to JSON
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to marshal info response",
			slog.String("error", err.Error()),
			slog.Any("response", response))
		return mcp.NewToolResultError("Failed to marshal info response; reason: " + err.Error()), nil
	}

	logger.InfoContext(ctx, "Info tool completed successfully",
		slog.String("k6_version", k6Version),
		slog.Bool("logged_in", isLoggedIn))

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// InfoResponse is the response to the info tool.
type InfoResponse struct {
	// Version is the version of the mcp-k6 server.
	Version string `json:"version"`

	// K6Version is the version of the k6 binary present in the system and
	// being used by the server.
	K6Version string `json:"k6_version"`

	// LoggedIn is a boolean indicating if the user is logged in to k6 cloud.
	LoggedIn bool `json:"logged_in"`
}
