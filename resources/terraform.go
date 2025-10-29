package resources

import (
	"context"
	"fmt"

	mcpk6 "github.com/grafana/mcp-k6"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// terraformResource is the MCP resource definition for the k6 Cloud Terraform resources.
//
//nolint:gochecknoglobals // Shared resource definition registered at startup.
var terraformResource = mcp.NewResource(
	"docs://k6/terraform",
	"Terraform for k6 Cloud",
	mcp.WithResourceDescription("Documentation on k6 Cloud Terraform resources using the Grafana Terraform provider."),
	mcp.WithMIMEType("text/markdown"),
)

// RegisterTerraformResource registers the Terraform resource with the MCP server.
func RegisterTerraformResource(s *server.MCPServer) {
	s.AddResource(terraformResource, terraform)
}

// terraform is the handler for the Terraform resource.
func terraform(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	content, err := mcpk6.DistResources.ReadFile("dist/resources/TERRAFORM.md")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded Terraform resource: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "docs://k6/terraform",
			MIMEType: "text/markdown",
			Text:     string(content),
		},
	}, nil
}
