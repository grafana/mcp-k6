package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/grafana/mcp-k6/internal/sections"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

func TestListSectionsHandlerReturnsTopLevelTree(t *testing.T) {
	t.Parallel()

	handler := newListSectionsHandlerFunc(newTestFinder(t, sampleSections()))

	result, err := handler(context.Background(), newCallRequest(nil))
	require.NoError(t, err)

	resp := decodeListSectionsResponse(t, result)
	require.Equal(t, 1, resp.Depth)
	require.Empty(t, resp.RootSlug)
	require.Len(t, resp.Tree, 2)

	first := resp.Tree[0]
	require.Equal(t, "using-k6", first.Slug)
	require.Equal(t, 1, first.ChildCount)
	require.True(t, first.HasMore)
	require.Nil(t, first.Children)
}

func TestListSectionsHandlerDepthAndRoot(t *testing.T) {
	t.Parallel()

	handler := newListSectionsHandlerFunc(newTestFinder(t, sampleSections()))

	args := map[string]any{
		"root_slug": "using-k6",
		"depth":     2,
	}
	result, err := handler(context.Background(), newCallRequest(args))
	require.NoError(t, err)

	resp := decodeListSectionsResponse(t, result)
	require.Equal(t, 2, resp.Depth)
	require.Equal(t, "using-k6", resp.RootSlug)
	require.Len(t, resp.Tree, 1)

	child := resp.Tree[0]
	require.Equal(t, "using-k6/get-started", child.Slug)
	require.Len(t, child.Children, 1)
	require.Equal(t, "using-k6/get-started/install", child.Children[0].Slug)
	require.False(t, child.Children[0].HasMore)
}

func sampleSections() []sections.Section {
	return []sections.Section{
		{
			Slug:        "using-k6",
			RelPath:     "using-k6/_index.md",
			Title:       "Using k6",
			Category:    "using-k6",
			Hierarchy:   []string{"using-k6"},
			IsIndex:     true,
			Weight:      0,
			Description: "Overview of using k6",
		},
		{
			Slug:        "using-k6/get-started",
			RelPath:     "using-k6/get-started.md",
			Title:       "Get Started",
			Category:    "using-k6",
			Hierarchy:   []string{"using-k6"},
			Weight:      10,
			Description: "Intro guide",
		},
		{
			Slug:        "using-k6/get-started/install",
			RelPath:     "using-k6/get-started/install.md",
			Title:       "Install",
			Category:    "using-k6",
			Hierarchy:   []string{"using-k6", "get-started"},
			Weight:      20,
			Description: "Install guide",
		},
		{
			Slug:        "javascript-api",
			RelPath:     "javascript-api/_index.md",
			Title:       "JavaScript API",
			Category:    "javascript-api",
			Hierarchy:   []string{"javascript-api"},
			Weight:      5,
			Description: "API ref",
		},
	}
}

func newTestFinder(t *testing.T, sectionData []sections.Section) *sections.Finder {
	t.Helper()

	sectionsCopy := append([]sections.Section(nil), sectionData...)

	index := &sections.SectionIndex{
		Versions: []string{"vtest"},
		Latest:   "vtest",
		Sections: map[string][]sections.Section{
			"vtest": sectionsCopy,
		},
		BySlug: map[string]map[string]*sections.Section{
			"vtest": {},
		},
		ByPath: map[string]map[string]*sections.Section{
			"vtest": {},
		},
	}

	for i := range sectionsCopy {
		section := &sectionsCopy[i]
		index.BySlug["vtest"][section.Slug] = section
		index.ByPath["vtest"][section.RelPath] = section
	}

	return sections.NewFinder(index)
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
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content)

	textContent, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok)

	var resp listSectionsResponse
	require.NoError(t, json.Unmarshal([]byte(textContent.Text), &resp))
	return resp
}
