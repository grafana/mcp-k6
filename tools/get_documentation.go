package tools

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	k6mcp "github.com/grafana/mcp-k6"
	"github.com/grafana/mcp-k6/internal/logging"
	"github.com/grafana/mcp-k6/internal/sections"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// GetDocumentationTool exposes a tool for retrieving specific documentation sections.
//
//nolint:gochecknoglobals // Shared tool definition registered at startup.
var GetDocumentationTool = mcp.NewTool(
	"get_documentation",
	mcp.WithDescription(
		"Retrieves the full markdown content of a specific k6 documentation section. "+
			"Use the slug from list_sections output (e.g., 'using-k6/scenarios', 'javascript-api/k6-http/request'). "+
			"Returns the complete markdown content with frontmatter metadata. "+
			"Supports multiple k6 versions - specify version parameter or defaults to latest. "+
			"Use this when you need detailed documentation for a specific topic.",
	),
	mcp.WithString(
		"slug",
		mcp.Required(),
		mcp.Description(
			"Section slug to retrieve (e.g., 'using-k6/scenarios', 'javascript-api/k6-http'). "+
				"Get valid slugs from list_sections tool. Supports aliases.",
		),
	),
	mcp.WithString(
		"version",
		mcp.Description(
			"Optional: k6 version (e.g., 'v1.4.x', 'v0.57.x'). Defaults to latest. "+
				"Use list_sections with version='all' to see available versions.",
		),
	),
)

// getDocParams holds parsed request parameters.
type getDocParams struct {
	Slug    string
	Version string
}

// getDocResponse is the JSON structure returned by the tool.
type getDocResponse struct {
	Section           sections.Section `json:"section"`
	Content           string           `json:"content"`
	Version           string           `json:"version"`
	AvailableVersions []string         `json:"available_versions"`
}

// RegisterGetDocumentationTool registers the get documentation tool with the MCP server.
func RegisterGetDocumentationTool(s *server.MCPServer, finder *sections.Finder) {
	handler := newGetDocumentationHandlerFunc(finder)
	s.AddTool(GetDocumentationTool, withToolLogger("get_documentation", handler))
}

// newGetDocumentationHandlerFunc returns an MCP tool handler bound to a finder.
func newGetDocumentationHandlerFunc(
	finder *sections.Finder,
) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.LoggerFromContext(ctx)
		logger.DebugContext(ctx, "Starting get_documentation operation")

		params, err := parseGetDocParams(request)
		if err != nil {
			logger.WarnContext(ctx, "Invalid parameters", slog.String("error", err.Error()))
			return mcp.NewToolResultError(err.Error()), nil
		}

		logger.DebugContext(ctx, "Parameters",
			slog.String("slug", params.Slug),
			slog.String("version", params.Version))

		version, _ := resolveVersion(finder, params.Version)

		section, err := lookupSection(ctx, logger, finder, params.Slug, version)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		content, err := readMarkdownContent(ctx, logger, section, version)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		logger.InfoContext(ctx, "Documentation retrieved successfully",
			slog.String("slug", params.Slug),
			slog.String("title", section.Title),
			slog.String("version", version),
			slog.Int("content_size", len(content)))

		resp := getDocResponse{
			Section:           *section,
			Content:           string(content),
			Version:           version,
			AvailableVersions: finder.GetVersions(),
		}

		return marshalResponse(ctx, logger, resp)
	}
}

func parseGetDocParams(request mcp.CallToolRequest) (*getDocParams, error) {
	slug, err := request.RequireString("slug")
	if err != nil {
		return nil, fmt.Errorf("missing or invalid slug parameter: %w", err)
	}

	return &getDocParams{
		Slug:    slug,
		Version: request.GetString("version", ""),
	}, nil
}

func lookupSection(
	ctx context.Context,
	logger *slog.Logger,
	finder *sections.Finder,
	slug, version string,
) (*sections.Section, error) {
	logger.DebugContext(ctx, "Looking up section",
		slog.String("slug", slug),
		slog.String("version", version))

	section, err := finder.GetBySlug(slug, version)
	if err != nil {
		logger.WarnContext(ctx, "Section not found",
			slog.String("slug", slug),
			slog.String("version", version),
			slog.String("error", err.Error()))

		return nil, fmt.Errorf(
			"section not found: %s in version %s. Use list_sections tool to find valid slugs",
			slug, version,
		)
	}

	return section, nil
}

func readMarkdownContent(
	ctx context.Context,
	logger *slog.Logger,
	section *sections.Section,
	version string,
) ([]byte, error) {
	markdownPath := filepath.Join("dist/markdown", version, section.RelPath)

	logger.DebugContext(ctx, "Reading markdown file",
		slog.String("path", markdownPath))

	content, err := k6mcp.MarkdownFiles.ReadFile(markdownPath)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to read markdown file",
			slog.String("path", markdownPath),
			slog.String("slug", section.Slug),
			slog.String("version", version),
			slog.String("error", err.Error()))

		return nil, fmt.Errorf(
			"failed to read documentation content for %s (version %s). "+
				"This may indicate a build issue. Please report this error",
			section.Slug, version,
		)
	}

	return content, nil
}
