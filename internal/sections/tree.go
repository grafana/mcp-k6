package sections

import (
	"fmt"
	"strings"
)

// SectionNode represents a Section with its nested children.
type SectionNode struct {
	Section

	Children        []*SectionNode `json:"children,omitempty"`
	ChildCount      int            `json:"child_count"`
	HasChildren     bool           `json:"has_children"`
	HasMoreChildren bool           `json:"has_more_children,omitempty"`
}

// SectionDTO is a lean representation optimized for agent consumption.
type SectionDTO struct {
	Slug        string        `json:"slug"`
	Title       string        `json:"title"`
	Description string        `json:"description,omitempty"`
	ChildCount  int           `json:"child_count"`
	HasMore     bool          `json:"has_more,omitempty"`
	Children    []*SectionDTO `json:"children,omitempty"`
}

// BuildSectionTree organizes sections into a depth-limited tree.
// When rootSlug is empty the tree starts at top-level sections.
// Depth counts how many levels (including the root) are returned.
func BuildSectionTree(sections []Section, rootSlug string, depth int) ([]*SectionNode, error) {
	if depth < 1 {
		return nil, fmt.Errorf("depth must be at least 1")
	}

	slugToSection := make(map[string]Section, len(sections))
	childrenByParent := make(map[string][]Section)

	for _, section := range sections {
		slugToSection[section.Slug] = section

		parent := parentSlug(section.Slug)
		childrenByParent[parent] = append(childrenByParent[parent], section)
	}

	var roots []Section
	if rootSlug == "" {
		roots = childrenByParent[""]
	} else {
		if _, ok := slugToSection[rootSlug]; !ok {
			return nil, fmt.Errorf("root slug not found: %s", rootSlug)
		}
		roots = childrenByParent[rootSlug]
	}

	nodes := make([]*SectionNode, 0, len(roots))
	for _, section := range roots {
		node := buildSectionNode(section, childrenByParent, depth, 1)
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func buildSectionNode(section Section, childrenByParent map[string][]Section, maxDepth, currentDepth int) *SectionNode {
	childSections := childrenByParent[section.Slug]

	node := &SectionNode{
		Section:     section,
		ChildCount:  len(childSections),
		HasChildren: len(childSections) > 0,
	}

	if len(childSections) > 0 && currentDepth < maxDepth {
		node.Children = make([]*SectionNode, 0, len(childSections))
		for _, child := range childSections {
			node.Children = append(node.Children, buildSectionNode(child, childrenByParent, maxDepth, currentDepth+1))
		}
	}

	if len(childSections) > 0 && currentDepth >= maxDepth {
		node.HasMoreChildren = true
	}

	return node
}

func parentSlug(slug string) string {
	idx := strings.LastIndex(slug, "/")
	if idx == -1 {
		return ""
	}
	return slug[:idx]
}

// NodesToDTO converts SectionNode entries into lightweight DTOs.
func NodesToDTO(nodes []*SectionNode) []*SectionDTO {
	if len(nodes) == 0 {
		return []*SectionDTO{}
	}

	dtos := make([]*SectionDTO, 0, len(nodes))
	for _, node := range nodes {
		if node == nil {
			continue
		}
		dtos = append(dtos, node.toDTO())
	}

	return dtos
}

func (n *SectionNode) toDTO() *SectionDTO {
	if n == nil {
		return nil
	}

	dto := &SectionDTO{
		Slug:       n.Slug,
		Title:      n.Title,
		ChildCount: n.ChildCount,
		HasMore:    n.HasMoreChildren,
	}

	if n.Description != "" {
		dto.Description = n.Description
	}

	if len(n.Children) > 0 {
		dto.Children = make([]*SectionDTO, 0, len(n.Children))
		for _, child := range n.Children {
			dto.Children = append(dto.Children, child.toDTO())
		}
	}

	return dto
}
