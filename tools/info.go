// Package tools contains MCP tool definitions exposed by the k6-mcp server.
package tools

import (
	"context"
	"encoding/json"

	"github.com/grafana/k6-mcp/internal/buildinfo"
	"github.com/grafana/k6-mcp/internal/k6env"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// InfoTool exposes runtime information about k6-mcp and the local k6 environment.
//
//nolint:gochecknoglobals // Shared tool definition registered at startup.
var InfoTool = mcp.NewTool(
	"info",
	mcp.WithDescription("Get details about the k6-mcp server, the local k6 binary, and k6 Cloud login status."),
)

// RegisterInfoTool registers the info tool with the MCP server.
func RegisterInfoTool(s *server.MCPServer) {
	s.AddTool(InfoTool, info)
}

func info(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Locate the k6 executable
	k6Info, err := k6env.Locate(ctx)
	if err != nil {
		//nolint:nilerr // Error is reported via the MCP error result.
		return mcp.NewToolResultError("Failed to locate k6 executable on the user's system; reason: " + err.Error()), nil
	}

	// Extract the located k6 binary's k6Version
	k6Version, err := k6Info.Version(ctx)
	if err != nil {
		//nolint:nilerr // Error is reported via the MCP error result.
		return mcp.NewToolResultError("Failed to get user's k6 binary version; reason: " + err.Error()), nil
	}

	// Check if the user is logged in to k6 cloud
	isLoggedIn, err := k6Info.IsLoggedIn(ctx)
	if err != nil {
		//nolint:nilerr // Error is reported via the MCP error result.
		return mcp.NewToolResultError("Failed to check if k6 is logged in; reason: " + err.Error()), nil
	}

	// Create the response
	response := InfoResponse{
		Version:   buildinfo.Version,
		K6Version: k6Version,
		LoggedIn:  isLoggedIn,
	}

	// Marshal the response to JSON
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		//nolint:nilerr // Error is reported via the MCP error result.
		return mcp.NewToolResultError("Failed to marshal info response; reason: " + err.Error()), nil
	}

	return mcp.NewToolResultText(string(jsonResponse)), nil
}

// InfoResponse is the response to the info tool.
type InfoResponse struct {
	// Version is the version of the k6-mcp server.
	Version string `json:"version"`

	// K6Version is the version of the k6 binary present in the system and
	// being used by the server.
	K6Version string `json:"k6_version"`

	// LoggedIn is a boolean indicating if the user is logged in to k6 cloud.
	LoggedIn bool `json:"logged_in"`
}
