package tools

import (
	"context"
	"testing"

	k6docslib "github.com/grafana/k6-docs-lib"
	"github.com/grafana/mcp-k6/internal/docs"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/lib/fsext"
)

func TestGetDocumentationHandlerRejectsInvalidRelPath(t *testing.T) {
	t.Parallel()

	afs := fsext.NewMemMapFs()
	idx := &k6docslib.Index{
		Version: "vtest",
		Sections: []k6docslib.Section{
			{
				Slug:    "escape",
				RelPath: "../secret.txt",
				Title:   "Escape",
			},
		},
	}

	mi := k6docslib.NewMultiIndex()
	mi.Add("vtest", idx)
	mi.SetLatest("vtest")

	provider := docs.NewFromMultiIndex(mi, "/cache", afs)
	handler := newGetDocumentationHandlerFunc(provider)

	result, err := handler(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "get_documentation",
			Arguments: map[string]any{
				"slug": "escape",
			},
		},
	})
	require.NoError(t, err)
	require.True(t, result.IsError)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)
	require.Contains(t, textContent.Text, "contains an invalid content path")
	require.NotContains(t, textContent.Text, "/cache")
}
