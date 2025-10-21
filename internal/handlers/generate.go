package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	k6mcp "github.com/grafana/k6-mcp"
	"github.com/grafana/k6-mcp/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
)

// ScriptGenerator handles k6 script generation prompt requests.
type ScriptGenerator struct{}

var _ PromptHandler = &ScriptGenerator{}

// NewScriptGenerator creates a new ScriptGenerator handler instance.
func NewScriptGenerator() *ScriptGenerator {
	return &ScriptGenerator{}
}

// Handle processes script generation prompt requests.
func (s ScriptGenerator) Handle(
	ctx context.Context,
	request mcp.GetPromptRequest,
) (*mcp.GetPromptResult, error) {
	// Add request correlation ID
	requestID := uuid.New().String()
	ctx = logging.ContextWithRequestID(ctx, requestID)
	startTime := time.Now()

	args := request.Params.Arguments

	// Convert args to interface{} map for logging
	logArgs := make(map[string]interface{})
	for k, v := range args {
		logArgs[k] = v
	}

	// Log request start
	logging.RequestStart(ctx, "generate_script", logArgs)

	// Extract description from arguments
	description, exists := args["description"]
	if !exists {
		err := fmt.Errorf("missing required parameter: description")
		logging.RequestEnd(ctx, "generate_script", false, time.Since(startTime), err)
		return nil, fmt.Errorf(
			"missing required parameter 'description'. " +
				"Please provide a description of the k6 script you want to generate",
		)
	}

	if description == "" {
		err := fmt.Errorf("description parameter cannot be empty")
		logging.RequestEnd(ctx, "generate_script", false, time.Since(startTime), err)
		return nil, fmt.Errorf(
			"description parameter cannot be empty. " +
				"Please provide a detailed description of the k6 script you want to generate",
		)
	}

	// Load prompt template from embedded content
	templateContent, err := k6mcp.Resources.ReadFile("resources/prompts/generate_script.md")
	if err != nil {
		logging.RequestEnd(ctx, "generate_script", false, time.Since(startTime), err)
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

	// Log request completion
	logging.RequestEnd(ctx, "generate_script", true, time.Since(startTime), nil)

	return result, nil
}
