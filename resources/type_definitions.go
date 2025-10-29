package resources

import (
	"context"
	"io/fs"
	"strings"

	k6mcp "github.com/grafana/k6-mcp"
	"github.com/grafana/k6-mcp/internal"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const typeDefinitionsResourceURI = "types://k6"

// RegisterTypeDefinitionsResources registers the type definitions resources with the MCP server.
func RegisterTypeDefinitionsResources(s *server.MCPServer) {
	_ = fs.WalkDir(k6mcp.TypeDefinitions, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, internal.DistDTSFileSuffix) {
			bytes, err := k6mcp.TypeDefinitions.ReadFile(path)
			if err != nil {
				return err
			}

			relPath := strings.TrimPrefix(path, internal.DefinitionsPath)
			uri := typeDefinitionsResourceURI + "/" + relPath
			displayName := relPath

			fileBytes := bytes
			fileURI := uri
			resource := mcp.NewResource(
				fileURI,
				displayName,
				mcp.WithResourceDescription("Provides type definitions for k6."),
				mcp.WithMIMEType("application/json"),
			)

			s.AddResource(resource, func(_ context.Context, _ mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return []mcp.ResourceContents{
					mcp.TextResourceContents{
						URI:      fileURI,
						MIMEType: "application/json",
						Text:     string(fileBytes),
					},
				}, nil
			})
		}
		return nil
	})
}
