package tools

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/grafana/mcp-k6/internal/logging"
	"github.com/grafana/xk6-docs/docs"
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

// responseSection mirrors the legacy MCP response shape for a section. The
// docs.Section type does not include hierarchy, so the handler maps to this
// struct and derives hierarchy from the relative path.
type responseSection struct {
	Slug        string   `json:"slug"`
	RelPath     string   `json:"rel_path"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Weight      int      `json:"weight"`
	Aliases     []string `json:"aliases,omitempty"`
	Category    string   `json:"category"`
	Hierarchy   []string `json:"hierarchy"`
	IsIndex     bool     `json:"is_index"`
}

// getDocResponse is the JSON structure returned by the tool.
type getDocResponse struct {
	Section           responseSection `json:"section"`
	Content           string          `json:"content"`
	Version           string          `json:"version"`
	AvailableVersions []string        `json:"available_versions"`
}

// RegisterGetDocumentationTool registers the get documentation tool with the MCP server.
func RegisterGetDocumentationTool(s *server.MCPServer, catalog *docs.Catalog) {
	handler := newGetDocumentationHandlerFunc(catalog)
	s.AddTool(GetDocumentationTool, withToolLogger("get_documentation", handler))
}

// newGetDocumentationHandlerFunc returns an MCP tool handler bound to a catalog.
func newGetDocumentationHandlerFunc(
	catalog *docs.Catalog,
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

		idx, err := catalog.Index(ctx, params.Version)
		if err != nil {
			logger.WarnContext(ctx, "Failed to load index",
				slog.String("version", params.Version),
				slog.String("error", err.Error()))
			return mcp.NewToolResultError(
				versionError(params.Version, catalog, err).Error(),
			), nil
		}

		section, err := lookupSection(ctx, logger, idx, params.Slug)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		content, err := readMarkdownContent(ctx, logger, catalog, idx.Version, section)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		logger.InfoContext(ctx, "Documentation retrieved successfully",
			slog.String("slug", params.Slug),
			slog.String("title", section.Title),
			slog.String("version", idx.Version),
			slog.Int("content_size", len(content)))

		resp := getDocResponse{
			Section:           toResponseSection(section),
			Content:           string(content),
			Version:           idx.Version,
			AvailableVersions: catalog.Versions(),
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
	idx *docs.Index,
	slug string,
) (*docs.Section, error) {
	logger.DebugContext(ctx, "Looking up section",
		slog.String("slug", slug),
		slog.String("version", idx.Version))

	section, ok := idx.Lookup(slug)
	if !ok {
		logger.WarnContext(ctx, "Section not found",
			slog.String("slug", slug),
			slog.String("version", idx.Version))

		return nil, fmt.Errorf(
			"section not found: %s in version %s. Use list_sections tool to find valid slugs",
			slug, idx.Version,
		)
	}

	return section, nil
}

func readMarkdownContent(
	ctx context.Context,
	logger *slog.Logger,
	catalog *docs.Catalog,
	version string,
	section *docs.Section,
) ([]byte, error) {
	logger.DebugContext(ctx, "Reading markdown",
		slog.String("slug", section.Slug),
		slog.String("version", version))

	content, err := catalog.Read(ctx, version, section.Slug)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to read markdown",
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

// toResponseSection maps a docs.Section to the legacy MCP response shape,
// deriving hierarchy from the relative path's directory components.
func toResponseSection(sec *docs.Section) responseSection {
	return responseSection{
		Slug:        sec.Slug,
		RelPath:     sec.RelPath,
		Title:       sec.Title,
		Description: sec.Description,
		Weight:      sec.Weight,
		Aliases:     sec.Aliases,
		Category:    sec.Category,
		Hierarchy:   hierarchyFromRelPath(sec.RelPath),
		IsIndex:     sec.IsIndex,
	}
}

// hierarchyFromRelPath returns the directory components of relPath, matching
// the legacy buildHierarchy semantics: the markdown filename is dropped and
// each remaining path segment becomes a hierarchy entry.
func hierarchyFromRelPath(relPath string) []string {
	idx := strings.LastIndex(relPath, "/")
	if idx <= 0 {
		return []string{}
	}
	dir := relPath[:idx]
	parts := strings.Split(dir, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		out = append(out, p)
	}
	return out
}
