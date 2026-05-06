package resources

import (
	"context"
	"io/fs"
	"strings"

	"github.com/grafana/mcp-k6/dist"
	"github.com/grafana/mcp-k6/internal"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const typeDefinitionsResourceURI = "types://k6"

// RegisterTypeDefinitionsResources registers the type definitions resources with the MCP server.
func RegisterTypeDefinitionsResources(s *server.MCPServer) {
	_ = fs.WalkDir(dist.TypeDefinitions, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, internal.DistDTSFileSuffix) {
			bytes, err := dist.TypeDefinitions.ReadFile(path)
			if err != nil {
				return err
			}

			relPath := strings.TrimPrefix(path, internal.EmbeddedDefinitionsPath)
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
