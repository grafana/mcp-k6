package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"

	"github.com/grafana/mcp-k6/internal/logging"
	"github.com/grafana/mcp-k6/internal/search"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// SearchDocumentationTool exposes a tool for searching k6 documentation.
//
//nolint:gochecknoglobals // Shared tool definition registered at startup.
var SearchDocumentationTool = mcp.NewTool(
	"search_documentation",
	mcp.WithDescription(
		"Search up-to-date k6 documentation using SQLite FTS5 full-text search. "+
			"Use proactively while authoring or validating scripts to find best practices, "+
			"troubleshoot errors, discover examples/templates, and learn idiomatic k6 usage. "+
			"Query semantics: space-separated terms are ANDed by default; use quotes for exact phrases; "+
			"FTS5 operators (AND, OR, NEAR, parentheses) and prefix wildcards (e.g., http*) are supported. "+
			"Returns structured results with title, content, and path.",
	),
	mcp.WithString(
		"keywords",
		mcp.Required(),
		mcp.Description(
			"FTS5 query string. Use space-separated terms (implicit AND), quotes for exact phrases, "+
				"and optional FTS5 operators. Examples: 'load' → matches load; 'load testing' → matches load AND testing; "+
				"'\"load testing\"' → exact phrase; 'thresholds OR checks'; 'stages NEAR/5 ramping'; 'http*' for prefix.",
		),
	),
	mcp.WithNumber(
		"max_results",
		mcp.Description(
			"Maximum number of results to return (default: 10, max: 20). "+
				"Use 5–10 for focused results, 15–20 for broader coverage.",
		),
	),
)

// RegisterSearchDocumentationTool registers the search documentation tool with the MCP server.
func RegisterSearchDocumentationTool(s *server.MCPServer, db *sql.DB) {
	searcher := search.NewFullTextSearcher(db)
	handler := newSearchDocumentationHandlerFunc(searcher)
	s.AddTool(SearchDocumentationTool, withToolLogger("search_documentation", handler))
}

const (
	defaultMaxResults = 10
	maxAllowedResults = 20
	minAllowedResults = 1
)

// newSearchDocumentationHandlerFunc returns an MCP tool handler bound to a searcher.
// This uses a closure to encapsulate dependencies (constructor/factory style)
// while conforming to the AddTool expected function signature.
func newSearchDocumentationHandlerFunc(
	searcher search.Search,
) func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := logging.LoggerFromContext(ctx)
		logger.DebugContext(ctx, "Starting documentation search")

		keywords, err := request.RequireString("keywords")
		if err != nil {
			logger.WarnContext(ctx, "Missing or invalid keywords parameter",
				slog.String("error", err.Error()))
			return nil, err
		}

		maxResults := request.GetInt("max_results", defaultMaxResults)
		originalMaxResults := maxResults
		if maxResults < minAllowedResults {
			maxResults = defaultMaxResults
			logger.DebugContext(ctx, "max_results below minimum, using default",
				slog.Int("requested", originalMaxResults),
				slog.Int("using", maxResults))
		} else if maxResults > maxAllowedResults {
			maxResults = maxAllowedResults
			logger.DebugContext(ctx, "max_results exceeds maximum, capping",
				slog.Int("requested", originalMaxResults),
				slog.Int("using", maxResults))
		}

		logger.DebugContext(ctx, "Executing documentation search",
			slog.String("keywords", keywords),
			slog.Int("max_results", maxResults))

		results, err := searcher.Search(ctx, keywords, search.Options{MaxResults: maxResults})
		if err != nil {
			logger.ErrorContext(ctx, "Documentation search failed",
				slog.String("keywords", keywords),
				slog.String("error", err.Error()))
			return nil, err
		}

		logger.DebugContext(ctx, "Search completed",
			slog.Int("result_count", len(results)))

		resultJSON, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			logger.ErrorContext(ctx, "Failed to marshal search results",
				slog.String("error", err.Error()),
				slog.Int("result_count", len(results)))
			return nil, err
		}

		logger.InfoContext(ctx, "Documentation search completed successfully",
			slog.String("keywords", keywords),
			slog.Int("results_returned", len(results)))

		return mcp.NewToolResultText(string(resultJSON)), nil
	}
}
