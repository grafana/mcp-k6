package tools

import (
	"context"
	"fmt"
	"log/slog"

	k6docslib "github.com/grafana/k6-docs-lib"
	"github.com/grafana/mcp-k6/internal/docs"
	"github.com/grafana/mcp-k6/internal/logging"
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
			"Optional: documentation version to read from. Defaults to the version matching "+
				"the installed k6 binary. Use list_sections with version='all' to inspect "+
				"the version currently available on this server.",
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
	Section           k6docslib.Section `json:"section"`
	Content           string            `json:"content"`
	Version           string            `json:"version"`
	AvailableVersions []string          `json:"available_versions"`
}

// RegisterGetDocumentationTool registers the get documentation tool with the MCP server.
func RegisterGetDocumentationTool(s *server.MCPServer, provider *docs.Provider) {
	handler := newGetDocumentationHandlerFunc(provider)
	s.AddTool(GetDocumentationTool, withToolLogger("get_documentation", handler))
}

// newGetDocumentationHandlerFunc returns an MCP tool handler bound to a provider.
func newGetDocumentationHandlerFunc(
	provider *docs.Provider,
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

		version, err := resolveVersion(provider, params.Version)
		if err != nil {
			logger.WarnContext(ctx, "Version not found",
				slog.String("version", params.Version),
				slog.Any("available_versions", provider.GetVersions()))
			return mcp.NewToolResultError(err.Error()), nil
		}

		section, err := lookupSection(ctx, logger, provider, params.Slug, version)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		content, err := readMarkdownContent(ctx, logger, provider, section, version)
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
			AvailableVersions: provider.GetVersions(),
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
	provider *docs.Provider,
	slug, version string,
) (*k6docslib.Section, error) {
	logger.DebugContext(ctx, "Looking up section",
		slog.String("slug", slug),
		slog.String("version", version))

	section, ok := provider.Lookup(slug, version)
	if !ok {
		logger.WarnContext(ctx, "Section not found",
			slog.String("slug", slug),
			slog.String("version", version))

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
	provider *docs.Provider,
	section *k6docslib.Section,
	version string,
) ([]byte, error) {
	logger.DebugContext(ctx, "Reading markdown file",
		slog.String("slug", section.Slug),
		slog.String("rel_path", section.RelPath))

	content, err := provider.ReadContent(section, version)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to read markdown file",
			slog.String("slug", section.Slug),
			slog.String("version", version),
			slog.String("error", err.Error()))

		if docs.IsInvalidRelPath(err) {
			return nil, fmt.Errorf(
				"documentation metadata for %s (version %s) contains an invalid content path",
				section.Slug, version,
			)
		}

		return nil, fmt.Errorf(
			"failed to read documentation content for %s (version %s). "+
				"This may indicate a cache issue. Please report this error",
			section.Slug, version,
		)
	}

	return content, nil
}
