package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/grafana/k6-mcp/internal/logging"
	"github.com/grafana/k6-mcp/internal/search"
	"github.com/mark3labs/mcp-go/mcp"
)

// FullTextSearchHandler Handlers aggregates all MCP tool handlers with their dependencies.
type FullTextSearchHandler struct {
	DB *sql.DB
}

var _ ToolHandler = &FullTextSearchHandler{}

// NewFullTextSearchHandler New returns a Handlers instance with provided dependencies.
func NewFullTextSearchHandler(db *sql.DB) *FullTextSearchHandler {
	return &FullTextSearchHandler{DB: db}
}

// Handle HandleSearch handles the search tool requests.
func (h *FullTextSearchHandler) Handle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Add request correlation ID
	requestID := uuid.New().String()
	ctx = logging.ContextWithRequestID(ctx, requestID)
	startTime := time.Now()

	args := request.GetArguments()

	// Log request start
	logging.RequestStart(ctx, "search", args)

	// Extract query from arguments
	const searchGuidance = "Search for development workflow help: '\"script validation\"', " +
		"'\"threshold setup\"', '\"HTTP patterns\"'. For troubleshooting: '\"debugging errors\"', " +
		"'\"common issues\"'. For learning: '\"getting started\"', '\"examples\"', '\"best practices\"'."

	queryValue, exists := args["keywords"]
	if !exists {
		err := fmt.Errorf("missing required parameter: keywords")
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError(
			"Missing required parameter 'keywords'. " + searchGuidance,
		), nil
	}

	query, ok := queryValue.(string)
	if !ok {
		err := fmt.Errorf("keywords parameter must be a string")
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError(
			"Parameter 'keywords' must be a string containing your search terms. " +
				"Multi-word queries should be quoted. Received: " + fmt.Sprintf("%T", queryValue),
		), nil
	}

	if query == "" {
		err := fmt.Errorf("keywords parameter cannot be empty")
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError(
			"Keywords parameter cannot be empty. " + searchGuidance,
		), nil
	}

	// Parse search options
	options := search.DefaultOptions()

	// Parse max_results if provided
	if maxResultsValue, exists := args["max_results"]; exists {
		if maxResults, ok := maxResultsValue.(float64); ok {
			maxResultsInt := int(maxResults)
			if maxResultsInt <= 0 {
				maxResultsInt = 5
			}
			options.MaxResults = maxResultsInt
		} else {
			err := fmt.Errorf("max_results must be a number")
			logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
			return mcp.NewToolResultError(
				"Parameter 'max_results' must be a number between 1 and 20. Received: " +
					fmt.Sprintf("%T", maxResultsValue),
			), nil
		}
	}

	results, err := search.NewFullTextSearcher(h.DB).Search(ctx, query, options)
	if err != nil {
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}

	resultJSON, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		logging.RequestEnd(ctx, "search", false, time.Since(startTime), err)
		return mcp.NewToolResultError("failed to serialize search results"), err
	}

	// Log request completion
	logging.RequestEnd(ctx, "search", true, time.Since(startTime), nil)

	return mcp.NewToolResultText(string(resultJSON)), nil
}
