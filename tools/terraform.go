package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/grafana/mcp-k6/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const grafanaProviderKey = "registry.terraform.io/grafana/grafana"

// SearchTerraformTool exposes a tool for searching Grafana Terraform provider resources.
//
//nolint:gochecknoglobals // Shared tool definition registered at startup.
var SearchTerraformTool = mcp.NewTool(
	"search_terraform",
	mcp.WithDescription(
		"Search for k6 Cloud-related resources in the Grafana Terraform provider. "+
			"Queries the installed provider schema and filters resources by name.",
	),
	mcp.WithString(
		"root",
		mcp.Description("Root directory of the Terraform project (default: current directory)."),
		mcp.DefaultString("."),
	),
	mcp.WithString(
		"term",
		mcp.Description("Search term to filter resources by name (default: 'k6'). Case-insensitive."),
		mcp.DefaultString("k6"),
	),
)

// RegisterSearchTerraformTool registers the search_terraform tool with the MCP server.
func RegisterSearchTerraformTool(s *server.MCPServer) {
	s.AddTool(SearchTerraformTool, withToolLogger("search_terraform", searchTerraform))
}

func searchTerraform(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	logger := logging.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Starting Terraform search")

	// Check if terraform is available
	terraformPath, err := exec.LookPath("terraform")
	if err != nil {
		logger.WarnContext(ctx, "Terraform executable not found", slog.String("error", err.Error()))
		return mcp.NewToolResultError(
			"Terraform is not installed or not available in PATH. " +
				"Please install Terraform: https://developer.hashicorp.com/terraform/install",
		), nil
	}
	logger.DebugContext(ctx, "Found terraform executable", slog.String("path", terraformPath))

	root := request.GetString("root", ".")
	term := strings.ToLower(request.GetString("term", "k6"))
	logger.DebugContext(ctx, "Search parameters", slog.String("root", root), slog.String("term", term))

	schema, err := runTerraformSchema(ctx, logger, terraformPath, root)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Check if Grafana provider exists
	grafanaProvider, ok := schema.ProviderSchemas[grafanaProviderKey]
	if !ok {
		logger.WarnContext(ctx, "Grafana provider not found in schema")
		return mcp.NewToolResultError(
			"Grafana provider not found. Either the specified root directory ('" + root +
				"') is not a valid Terraform project, or the Grafana provider is not installed. " +
				"Ensure you have the Grafana provider configured and run 'terraform init'.",
		), nil
	}

	// Filter resources by search term
	filtered := make(map[string]json.RawMessage)
	for name, resource := range grafanaProvider.ResourceSchemas {
		if strings.Contains(strings.ToLower(name), term) {
			filtered[name] = resource
		}
	}

	logger.InfoContext(ctx, "Terraform search completed",
		slog.Int("total_resources", len(grafanaProvider.ResourceSchemas)),
		slog.Int("filtered_resources", len(filtered)),
		slog.String("term", term))

	resultJSON, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		logger.ErrorContext(ctx, "Failed to marshal results", slog.String("error", err.Error()))
		return mcp.NewToolResultError("Failed to marshal results: " + err.Error()), nil
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

type tfSchema struct {
	ProviderSchemas map[string]struct {
		ResourceSchemas map[string]json.RawMessage `json:"resource_schemas"`
	} `json:"provider_schemas"`
}

func runTerraformSchema(ctx context.Context, logger *slog.Logger, tfPath, root string) (*tfSchema, error) {
	cmd := exec.CommandContext(ctx, tfPath, "providers", "schema", "-json")
	cmd.Dir = root

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := strings.TrimSpace(string(output))
		logger.ErrorContext(ctx, "Failed to run terraform command",
			slog.String("error", err.Error()), slog.String("output", outputStr))
		if outputStr != "" {
			return nil, fmt.Errorf("failed to run 'terraform providers schema -json': %s", outputStr)
		}
		return nil, fmt.Errorf("failed to run 'terraform providers schema -json': %w", err)
	}

	var schema tfSchema
	if err := json.Unmarshal(output, &schema); err != nil {
		logger.ErrorContext(ctx, "Failed to parse terraform schema", slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to parse terraform schema: %w", err)
	}

	return &schema, nil
}
