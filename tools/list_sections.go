package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log/slog"
	"strings"

	"github.com/grafana/mcp-k6/internal/logging"
	"github.com/grafana/xk6-docs/docs"
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

// treeItem is the MCP-facing representation of a section node in the response.
// Its JSON shape is part of the public tool contract.
type treeItem struct {
	Slug        string      `json:"slug"`
	Title       string      `json:"title"`
	Description string      `json:"description,omitempty"`
	ChildCount  int         `json:"child_count"`
	HasMore     bool        `json:"has_more,omitempty"`
	Children    []*treeItem `json:"children,omitempty"`
}

// listSectionsResponse is the JSON structure returned by the tool.
type listSectionsResponse struct {
	Tree              []*treeItem `json:"tree"`
	Count             int         `json:"count"`
	Total             int         `json:"total"`
	Version           string      `json:"version"`
	AvailableVersions []string    `json:"available_versions"`
	FilteredBy        *filterInfo `json:"filtered_by,omitempty"`
	Depth             int         `json:"depth"`
	Usage             string      `json:"usage"`
	RootSlug          string      `json:"root_slug,omitempty"`
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
func RegisterListSectionsTool(s *server.MCPServer, catalog *docs.Catalog) {
	handler := newListSectionsHandlerFunc(catalog)
	s.AddTool(ListSectionsTool, withToolLogger("list_sections", handler))
}

// newListSectionsHandlerFunc returns an MCP tool handler bound to a catalog.
func newListSectionsHandlerFunc(
	catalog *docs.Catalog,
) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.LoggerFromContext(ctx)
		logger.DebugContext(ctx, "Starting list_sections operation")

		params := parseListSectionsParams(request)
		logParams(ctx, logger, params)

		if params.Version == "all" {
			return handleVersionsRequest(ctx, logger, catalog)
		}

		idx, err := catalog.Index(ctx, params.Version)
		if err != nil {
			logger.WarnContext(ctx, "Failed to load index",
				slog.String("version", params.Version),
				slog.String("error", err.Error()))
			return mcp.NewToolResultError(
				versionError(params.Version, catalog, err).Error(),
			), nil
		}

		tree, total, ok := buildResponseTree(idx, params)
		if !ok {
			logger.WarnContext(ctx, "Root slug not found",
				slog.String("root_slug", params.RootSlug),
				slog.String("version", idx.Version))
			return mcp.NewToolResultError(
				fmt.Sprintf("root slug not found: %s", params.RootSlug),
			), nil
		}

		resp := buildListSectionsResponse(idx.Version, catalog.Versions(), params, tree, total)

		logger.InfoContext(ctx, "Sections listed successfully",
			slog.String("version", idx.Version),
			slog.Int("section_count", len(tree)),
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
	catalog *docs.Catalog,
) (*mcp.CallToolResult, error) {
	versions := catalog.Versions()
	latest := catalog.Latest()

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

// buildResponseTree returns the response tree, the appropriate total count for
// the params, and a flag indicating whether the resolved root slug exists.
// The flag is only false when params.RootSlug was explicitly provided but
// unknown to the index.
func buildResponseTree(idx *docs.Index, params listSectionsParams) ([]*treeItem, int, bool) {
	if params.Category != "" {
		total := len(idx.ByCategory(params.Category))
		root := buildCategoryRoot(idx, params.Category, params.Depth)
		if root == nil {
			return []*treeItem{}, total, true
		}
		return []*treeItem{root}, total, true
	}

	if params.RootSlug != "" {
		if _, ok := idx.Lookup(params.RootSlug); !ok {
			return nil, 0, false
		}
	}

	return collectRoots(idx.Tree(params.RootSlug, params.Depth)), len(idx.Sections), true
}

// collectRoots collects level-0 nodes from a docs.Tree iterator and maps
// them into MCP response items. Tree already yields roots in weight order.
func collectRoots(seq iter.Seq2[int, *docs.Tree]) []*treeItem {
	var out []*treeItem
	for level, t := range seq {
		if level == 0 {
			out = append(out, mapTree(t))
		}
	}
	return out
}

// buildCategoryRoot returns a treeItem for the category root section. At
// depth > 1 it populates the root's children using the docs index tree.
// Returns nil if the category section does not exist.
func buildCategoryRoot(idx *docs.Index, category string, depth int) *treeItem {
	sec, ok := idx.Lookup(category)
	if !ok {
		return nil
	}
	item := &treeItem{
		Slug:        sec.Slug,
		Title:       sec.Title,
		Description: sec.Description,
		ChildCount:  len(sec.Children),
	}
	if depth > 1 {
		item.Children = collectRoots(idx.Tree(category, depth-1))
	}
	if len(sec.Children) > 0 && len(item.Children) == 0 {
		item.HasMore = true
	}
	return item
}

// mapTree maps a docs.Tree node to a treeItem, preserving any populated
// children. has_more is set when the section has stored children but the tree
// node has none populated (depth was exhausted before they were walked).
func mapTree(t *docs.Tree) *treeItem {
	item := &treeItem{
		Slug:        t.Slug,
		Title:       t.Title,
		Description: t.Description,
		ChildCount:  len(t.Section.Children),
	}
	for _, c := range t.Children {
		item.Children = append(item.Children, mapTree(c))
	}
	if len(t.Section.Children) > 0 && len(item.Children) == 0 {
		item.HasMore = true
	}
	return item
}

func buildListSectionsResponse(
	version string,
	availableVersions []string,
	params listSectionsParams,
	tree []*treeItem,
	total int,
) *listSectionsResponse {
	resp := &listSectionsResponse{
		Tree:              tree,
		Count:             len(tree),
		Total:             total,
		Version:           version,
		AvailableVersions: availableVersions,
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

	return resp
}

// versionError returns an actionable error when a requested documentation
// version could not be loaded. When version is empty (default/latest was
// requested), it returns the original catalog error unchanged.
func versionError(version string, catalog *docs.Catalog, original error) error {
	if version == "" {
		return original
	}
	available := catalog.Versions()
	if len(available) == 0 {
		return fmt.Errorf("version %s is not available and no versions were discovered", version)
	}
	return fmt.Errorf(
		"version %s is not available (available: %s). "+
			"Omit the version parameter to use the latest (%s)",
		version, strings.Join(available, ", "), catalog.Latest(),
	)
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
