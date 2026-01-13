package sections

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildSectionTreeDepthLimit(t *testing.T) {
	t.Parallel()

	sections := []Section{
		{Slug: "root", Title: "Root"},
		{Slug: "root/child", Title: "Child"},
		{Slug: "root/child/grand", Title: "Grandchild"},
	}

	nodes, err := BuildSectionTree(sections, "", 1)
	require.NoError(t, err)
	require.Len(t, nodes, 1)

	root := nodes[0]
	require.Equal(t, "root", root.Slug)
	require.True(t, root.HasChildren)
	require.True(t, root.HasMoreChildren)
	require.Nil(t, root.Children)
}

func TestBuildSectionTreeRootFilter(t *testing.T) {
	t.Parallel()

	sections := []Section{
		{Slug: "root", Title: "Root"},
		{Slug: "root/child", Title: "Child"},
		{Slug: "root/child/grand", Title: "Grandchild"},
	}

	nodes, err := BuildSectionTree(sections, "root/child", 2)
	require.NoError(t, err)
	require.Len(t, nodes, 1)

	grands := nodes[0]
	require.Equal(t, "root/child/grand", grands.Slug)
	require.Nil(t, grands.Children)
}

func TestBuildSectionTreeInvalidRoot(t *testing.T) {
	t.Parallel()

	sections := []Section{{Slug: "root", Title: "Root"}}

	_, err := BuildSectionTree(sections, "missing", 1)
	require.Error(t, err)
}

func TestBuildSectionTreeDepthValidation(t *testing.T) {
	t.Parallel()

	sections := []Section{{Slug: "root", Title: "Root"}}

	_, err := BuildSectionTree(sections, "", 0)
	require.Error(t, err)
}

func TestNodesToDTO(t *testing.T) {
	t.Parallel()

	node := &SectionNode{
		Section: Section{
			Slug:        "root",
			Title:       "Root",
			Description: "desc",
		},
		ChildCount:      1,
		HasMoreChildren: true,
		Children: []*SectionNode{
			{
				Section: Section{
					Slug:  "root/child",
					Title: "Child",
				},
				ChildCount: 0,
			},
		},
	}

	dtos := NodesToDTO([]*SectionNode{node})
	require.Len(t, dtos, 1)

	root := dtos[0]
	require.Equal(t, "root", root.Slug)
	require.Equal(t, "Root", root.Title)
	require.Equal(t, "desc", root.Description)
	require.Equal(t, 1, root.ChildCount)
	require.True(t, root.HasMore)
	require.Len(t, root.Children, 1)
	require.Equal(t, "root/child", root.Children[0].Slug)
	require.Empty(t, root.Children[0].Description)
}
