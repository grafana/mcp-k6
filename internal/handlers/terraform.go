package handlers

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	k6mcp "github.com/grafana/k6-mcp"
	"github.com/mark3labs/mcp-go/mcp"
)

// TerraformHandler handles Terraform resource generation tool requests.
type TerraformHandler struct{}

var _ ToolHandler = &TerraformHandler{}

// NewTerraformHandler creates a new TerraformHandler instance.
func NewTerraformHandler() *TerraformHandler {
	return &TerraformHandler{}
}

// Handle processes Terraform resource generation requests.
func (h *TerraformHandler) Handle(
	_ context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	templateData, err := parseTemplateArgs(request.GetArguments())
	if err != nil {
		//nolint:nilerr // Error conveyed in MCP result, not Go error
		return mcp.NewToolResultError("Failed to parse template arguments; reason: " + err.Error()), nil
	}

	renderedTemplate, err := renderTemplate(templateData)
	if err != nil {
		//nolint:nilerr // Error conveyed in MCP result, not Go error
		return mcp.NewToolResultError("Failed to render template; reason: " + err.Error()), nil
	}

	return mcp.NewToolResultText(renderedTemplate), nil
}

// TerraformTemplate is the data structure holding the arguments for
// rendering the Terraform template.
type TerraformTemplate struct {
	LoadTestName         string
	LoadTestResourceName string
	Script               string
	ProjectID            string
}

// parseTemplateArgs parses the template arguments and returns a TerraformTemplate.
func parseTemplateArgs(args map[string]interface{}) (*TerraformTemplate, error) {
	data := &TerraformTemplate{}

	// Helper function to reduce repetition
	getStringArg := func(key string) (string, error) {
		val, exists := args[key]
		if !exists {
			return "", fmt.Errorf("missing required parameter '%s'", key)
		}
		strVal, ok := val.(string)
		if !ok || strVal == "" {
			return "", fmt.Errorf("parameter '%s' must be a non-empty string", key)
		}
		return strVal, nil
	}

	var err error
	if data.LoadTestName, err = getStringArg("load_test_name"); err != nil {
		return nil, err
	}
	if data.LoadTestResourceName, err = getStringArg("load_test_resource_name"); err != nil {
		return nil, err
	}
	if data.Script, err = getStringArg("script"); err != nil {
		return nil, err
	}
	if data.ProjectID, err = getStringArg("project_id"); err != nil {
		return nil, err
	}

	return data, nil
}

// renderTemplate renders the Terraform template with the given data.
//
// Importantly, it defines the custom indent function that is used in the template.
func renderTemplate(data *TerraformTemplate) (string, error) {
	funcMap := template.FuncMap{
		"indent": indent,
	}

	templateName := "terraform_load_test.tf.tmpl"
	tmpl, err := template.New(templateName).
		Funcs(funcMap).
		ParseFS(k6mcp.Resources, "resources/templates/"+templateName)
	if err != nil {
		return "", fmt.Errorf("failed to parse Terraform template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute Terraform template: %w", err)
	}

	return buf.String(), nil
}

// The core of the solution: our custom template function.
// It takes a prefix string and the content string.
func indent(prefix string, content string) string {
	// Split the content into lines
	lines := strings.Split(content, "\n")

	// Add the prefix to each line
	for i, line := range lines {
		// We don't want to indent empty lines, which could happen with a trailing newline
		if len(strings.TrimSpace(line)) > 0 {
			lines[i] = prefix + line
		}
	}

	// Join the lines back together
	return strings.Join(lines, "\n")
}
