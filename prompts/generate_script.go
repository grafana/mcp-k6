// Package prompts provides MCP prompt definitions for the mcp-k6 server.
package prompts

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	k6mcp "github.com/grafana/mcp-k6"
	"github.com/grafana/mcp-k6/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// GenerateScriptPrompt is the MCP prompt definition for k6 script generation.
//
//nolint:gochecknoglobals // Shared prompt definition registered at startup.
var GenerateScriptPrompt = mcp.NewPrompt(
	"generate_script",
	mcp.WithPromptDescription("Generate a k6 script based on the user's request."),
	mcp.WithArgument(
		"description",
		mcp.ArgumentDescription("The description of the script to generate."),
	),
)

// RegisterGenerateScriptPrompt registers the generate_script prompt with the MCP server.
func RegisterGenerateScriptPrompt(s *server.MCPServer) {
	s.AddPrompt(GenerateScriptPrompt, withPromptLogger("generate_script", generateScript))
}

// Handle processes script generation prompt requests.
func generateScript(
	ctx context.Context,
	request mcp.GetPromptRequest,
) (*mcp.GetPromptResult, error) {
	logger := logging.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Starting script generation prompt")

	// Extract description from arguments
	description, exists := request.Params.Arguments["description"]
	if !exists {
		logger.WarnContext(ctx, "Missing required parameter 'description'")
		return nil, fmt.Errorf(
			"missing required parameter 'description'. " +
				"Please provide a description of the k6 script you want to generate",
		)
	}

	if description == "" {
		logger.WarnContext(ctx, "Empty description parameter")
		return nil, fmt.Errorf(
			"description parameter cannot be empty. " +
				"Please provide a detailed description of the k6 script you want to generate",
		)
	}

	logger.DebugContext(ctx, "Loading prompt template")

	// Load prompt template from embedded content
	templateContent, err := k6mcp.Prompts.ReadFile("prompts/generate_script.md")
	if err != nil {
		logger.ErrorContext(ctx, "Failed to read embedded prompt template",
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to read embedded prompt template: %w", err)
	}

	// Replace template variables
	promptText := strings.Replace(string(templateContent), "{{.Description}}", description, 1)

	result := mcp.NewGetPromptResult(
		"A k6 script",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleAssistant,
				mcp.NewTextContent(promptText),
			),
		},
	)

	logger.InfoContext(ctx, "Script generation prompt completed successfully",
		slog.Int("prompt_length", len(promptText)))

	return result, nil
}
