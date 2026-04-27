package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/grafana/xk6-docs/docs"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

func TestListSectionsHandlerDefault(t *testing.T) {
	t.Parallel()

	catalog := docs.NewCatalog()
	handler := newListSectionsHandlerFunc(catalog)

	result, err := handler(context.Background(), newCallRequest(nil))
	require.NoError(t, err)
	require.False(t, result.IsError, "tool returned error: %+v", result.Content)

	resp := decodeListSectionsResponse(t, result)
	require.Equal(t, 1, resp.Depth)
	require.Empty(t, resp.RootSlug)

	slugs := make(map[string]bool, len(resp.Tree))
	for _, item := range resp.Tree {
		slugs[item.Slug] = true
	}
	require.True(t, slugs["using-k6"], "expected using-k6 in tree, got %v", slugs)
	require.True(t, slugs["javascript-api"], "expected javascript-api in tree, got %v", slugs)
}

func TestListSectionsHandlerVersionAll(t *testing.T) {
	t.Parallel()

	catalog := docs.NewCatalog()
	handler := newListSectionsHandlerFunc(catalog)

	result, err := handler(context.Background(), newCallRequest(map[string]any{"version": "all"}))
	require.NoError(t, err)
	require.False(t, result.IsError, "tool returned error: %+v", result.Content)

	var resp versionsResponse
	decodeJSON(t, result, &resp)
	require.NotEmpty(t, resp.Versions)
	require.Equal(t, "v1.7.x", resp.Latest)
}

func TestListSectionsHandlerCategoryFilter(t *testing.T) {
	t.Parallel()

	catalog := docs.NewCatalog()
	handler := newListSectionsHandlerFunc(catalog)

	result, err := handler(context.Background(), newCallRequest(map[string]any{"category": "javascript-api"}))
	require.NoError(t, err)
	require.False(t, result.IsError, "tool returned error: %+v", result.Content)

	resp := decodeListSectionsResponse(t, result)
	require.Equal(t, 1, resp.Count)
	require.Len(t, resp.Tree, 1)
	require.Equal(t, "javascript-api", resp.Tree[0].Slug)
	require.True(t, resp.Tree[0].HasMore)
	require.Empty(t, resp.Tree[0].Children, "category root should have no children populated at default depth 1")
	require.Greater(t, resp.Total, resp.Count)
}

func TestListSectionsHandlerRootSlugDepth(t *testing.T) {
	t.Parallel()

	catalog := docs.NewCatalog()
	handler := newListSectionsHandlerFunc(catalog)

	result, err := handler(context.Background(), newCallRequest(map[string]any{
		"root_slug": "using-k6",
		"depth":     2,
	}))
	require.NoError(t, err)
	require.False(t, result.IsError, "tool returned error: %+v", result.Content)

	resp := decodeListSectionsResponse(t, result)
	require.Equal(t, 2, resp.Depth)
	require.Equal(t, "using-k6", resp.RootSlug)
	require.NotEmpty(t, resp.Tree)

	var scenarios *treeItem
	for _, item := range resp.Tree {
		require.NotEqual(t, "using-k6", item.Slug, "root using-k6 should not appear; tree should contain its children")
		if item.Slug == "using-k6/scenarios" {
			scenarios = item
		}
	}
	require.NotNil(t, scenarios, "expected using-k6/scenarios as a returned root")
	require.NotEmpty(t, scenarios.Children, "expected using-k6/scenarios to have children at depth 2")

	foundConcepts := false
	for _, child := range scenarios.Children {
		if child.Slug == "using-k6/scenarios/concepts" {
			foundConcepts = true
			break
		}
	}
	require.True(t, foundConcepts, "expected using-k6/scenarios/concepts under using-k6/scenarios")
}

func TestListSectionsHandlerMissingRootSlug(t *testing.T) {
	t.Parallel()

	catalog := docs.NewCatalog()
	handler := newListSectionsHandlerFunc(catalog)

	result, err := handler(context.Background(), newCallRequest(map[string]any{
		"root_slug": "does-not-exist-xyz",
	}))
	require.NoError(t, err)
	require.True(t, result.IsError, "expected tool error result for unknown root_slug")
}

func newCallRequest(args map[string]any) mcp.CallToolRequest {
	if args == nil {
		args = map[string]any{}
	}
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "list_sections",
			Arguments: args,
		},
	}
}

func decodeListSectionsResponse(t *testing.T, result *mcp.CallToolResult) listSectionsResponse {
	t.Helper()
	var resp listSectionsResponse
	decodeJSON(t, result, &resp)
	return resp
}

func decodeJSON(t *testing.T, result *mcp.CallToolResult, v any) {
	t.Helper()
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected TextContent")

	require.NoError(t, json.Unmarshal([]byte(textContent.Text), v),
		"failed to decode response: %s", textContent.Text)
}
