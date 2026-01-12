package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"

	"github.com/grafana/mcp-k6/internal/logging"
	"github.com/grafana/mcp-k6/internal/sections"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ListSectionsTool exposes a tool for listing available k6 documentation sections.
//
//nolint:gochecknoglobals // Shared tool definition registered at startup.
var ListSectionsTool = mcp.NewTool(
	"list_sections",
	mcp.WithDescription(
		"Lists k6 documentation sections in a hierarchical tree structure. "+
			"Use this to understand documentation organization and discover related topics. "+
			"Navigate progressively: start at root, then use root_slug to expand branches. "+
			"Returns compact metadata (no content) to minimize context usage. "+
			"Use get_documentation to retrieve the full content for a specific section.",
	),
	mcp.WithString(
		"version",
		mcp.Description(
			"Optional: k6 version to list sections for (e.g., 'v1.4.x', 'v0.57.x'). "+
				"Defaults to latest version. Use 'all' to see available versions.",
		),
	),
	mcp.WithString(
		"category",
		mcp.Description(
			"Optional: Filter by top-level category (e.g., 'using-k6', 'javascript-api'). "+
				"Use without this parameter to see all categories.",
		),
	),
	mcp.WithNumber(
		"depth",
		mcp.Description(
			"Optional: Depth of hierarchy to return (default: 1, max: 5). "+
				"Depth counts how many levels of children are included in the tree.",
		),
	),
	mcp.WithString(
		"root_slug",
		mcp.Description(
			"Optional: List the contents under this slug (i.e., its children). "+
				"Use the slug from a previous list_sections response.",
		),
	),
)

const (
	defaultTreeDepth = 1
	maxTreeDepth     = 5
)

// listSectionsParams holds parsed and validated request parameters.
type listSectionsParams struct {
	Version  string
	Category string
	RootSlug string
	Depth    int
}

// listSectionsResponse is the JSON structure returned by the tool.
type listSectionsResponse struct {
	Tree              []*sections.SectionDTO `json:"tree"`
	Count             int                    `json:"count"`
	Total             int                    `json:"total"`
	Version           string                 `json:"version"`
	AvailableVersions []string               `json:"available_versions"`
	FilteredBy        *filterInfo            `json:"filtered_by,omitempty"`
	Depth             int                    `json:"depth"`
	Usage             string                 `json:"usage"`
	RootSlug          string                 `json:"root_slug,omitempty"`
}

type filterInfo struct {
	Category string `json:"category,omitempty"`
	RootSlug string `json:"root_slug,omitempty"`
}

type versionsResponse struct {
	Versions []string `json:"versions"`
	Latest   string   `json:"latest"`
	Message  string   `json:"message"`
}

// RegisterListSectionsTool registers the list sections tool with the MCP server.
func RegisterListSectionsTool(s *server.MCPServer, finder *sections.Finder) {
	handler := newListSectionsHandlerFunc(finder)
	s.AddTool(ListSectionsTool, withToolLogger("list_sections", handler))
}

// newListSectionsHandlerFunc returns an MCP tool handler bound to a finder.
func newListSectionsHandlerFunc(
	finder *sections.Finder,
) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.LoggerFromContext(ctx)
		logger.DebugContext(ctx, "Starting list_sections operation")

		params := parseListSectionsParams(request)
		logParams(ctx, logger, params)

		if params.Version == "all" {
			return handleVersionsRequest(ctx, logger, finder)
		}

		version, err := resolveVersion(finder, params.Version)
		if err != nil {
			logger.WarnContext(ctx, "Version not found",
				slog.String("version", params.Version),
				slog.Any("available_versions", finder.GetVersions()))
			return mcp.NewToolResultError(err.Error()), nil
		}

		sectionList, totalCount, err := fetchSections(ctx, logger, finder, params, version)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resp, err := buildListSectionsResponse(ctx, logger, finder, params, version, sectionList, totalCount)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		logger.InfoContext(ctx, "Sections listed successfully",
			slog.String("version", version),
			slog.Int("section_count", len(sectionList)),
			slog.String("category", params.Category),
			slog.Int("depth", params.Depth),
			slog.String("root_slug", params.RootSlug))

		return marshalResponse(ctx, logger, resp)
	}
}

func parseListSectionsParams(request mcp.CallToolRequest) listSectionsParams {
	depth := request.GetInt("depth", defaultTreeDepth)
	if depth < 1 {
		depth = defaultTreeDepth
	} else if depth > maxTreeDepth {
		depth = maxTreeDepth
	}

	return listSectionsParams{
		Version:  request.GetString("version", ""),
		Category: request.GetString("category", ""),
		RootSlug: request.GetString("root_slug", ""),
		Depth:    depth,
	}
}

func logParams(ctx context.Context, logger *slog.Logger, params listSectionsParams) {
	logger.DebugContext(ctx, "Parameters",
		slog.String("version", params.Version),
		slog.String("category", params.Category),
		slog.String("root_slug", params.RootSlug),
		slog.Int("depth", params.Depth))
}

func handleVersionsRequest(
	ctx context.Context,
	logger *slog.Logger,
	finder *sections.Finder,
) (*mcp.CallToolResult, error) {
	versions := finder.GetVersions()
	latest := finder.GetLatestVersion()

	logger.InfoContext(ctx, "Listing all versions",
		slog.Int("version_count", len(versions)),
		slog.String("latest", latest))

	resp := versionsResponse{
		Versions: versions,
		Latest:   latest,
		Message:  "Available k6 documentation versions. Use version parameter to filter sections.",
	}

	return marshalResponse(ctx, logger, resp)
}

func resolveVersion(finder *sections.Finder, version string) (string, error) {
	if version == "" {
		return finder.GetLatestVersion(), nil
	}

	if slices.Contains(finder.GetVersions(), version) {
		return version, nil
	}

	return "", fmt.Errorf("version not found: %s. Use version='all' to see available versions", version)
}

func fetchSections(
	ctx context.Context,
	logger *slog.Logger,
	finder *sections.Finder,
	params listSectionsParams,
	version string,
) ([]sections.Section, int, error) {
	if params.Category != "" {
		logger.DebugContext(ctx, "Filtering by category",
			slog.String("category", params.Category),
			slog.String("version", version))

		list, err := finder.GetByCategory(params.Category, version)
		if err != nil {
			logger.ErrorContext(ctx, "Category filter failed",
				slog.String("category", params.Category),
				slog.String("version", version),
				slog.String("error", err.Error()))
			return nil, 0, fmt.Errorf("category filter failed: %w", err)
		}
		return list, len(list), nil
	}

	logger.DebugContext(ctx, "Listing all sections",
		slog.String("version", version))

	list, err := finder.GetAll(version)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to get sections",
			slog.String("version", version),
			slog.String("error", err.Error()))
		return nil, 0, fmt.Errorf("failed to get sections: %w", err)
	}
	return list, len(list), nil
}

func buildListSectionsResponse(
	ctx context.Context,
	logger *slog.Logger,
	finder *sections.Finder,
	params listSectionsParams,
	version string,
	sectionList []sections.Section,
	totalCount int,
) (*listSectionsResponse, error) {
	treeNodes, err := sections.BuildSectionTree(sectionList, params.RootSlug, params.Depth)
	if err != nil {
		logger.WarnContext(ctx, "Failed to build section tree",
			slog.String("root_slug", params.RootSlug),
			slog.Int("depth", params.Depth),
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to build section tree: %w", err)
	}

	resp := &listSectionsResponse{
		Tree:              sections.NodesToDTO(treeNodes),
		Count:             len(treeNodes),
		Total:             totalCount,
		Version:           version,
		AvailableVersions: finder.GetVersions(),
		Depth:             params.Depth,
	}

	if params.RootSlug != "" {
		resp.RootSlug = params.RootSlug
	}

	if params.Category == "" {
		resp.Usage = "Use the 'slug' field with get_documentation tool to retrieve full content. " +
			"Use 'root_slug' to expand any branch and 'depth' to include more nested children."
	} else {
		resp.Usage = "Use the 'slug' field with get_documentation tool to retrieve full content. " +
			"Adjust 'root_slug' or 'depth' to explore deeper within this category."
	}

	if params.Category != "" || params.RootSlug != "" {
		resp.FilteredBy = &filterInfo{
			Category: params.Category,
			RootSlug: params.RootSlug,
		}
	}

	return resp, nil
}

func marshalResponse(ctx context.Context, logger *slog.Logger, v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		logger.ErrorContext(ctx, "Failed to marshal response",
			slog.String("error", err.Error()))
		return nil, err
	}
	return mcp.NewToolResultText(string(data)), nil
}
