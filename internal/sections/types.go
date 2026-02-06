// Package sections provides types and utilities for managing k6 documentation sections.
package sections

// Section represents a documentation section with metadata extracted from frontmatter.
type Section struct {
	// Slug is the hierarchical identifier for the section (e.g., "using-k6/scenarios")
	Slug string `json:"slug"`

	// Path is the full file path on disk (used internally during indexing)
	Path string `json:"-"`

	// RelPath is the relative path from the docs root
	RelPath string `json:"rel_path"`

	// Title is the section title from frontmatter
	Title string `json:"title"`

	// Description is the section description from frontmatter
	Description string `json:"description"`

	// Weight is used for sorting sections (lower values appear first)
	Weight int `json:"weight"`

	// Aliases are alternative slugs that can be used to reference this section
	Aliases []string `json:"aliases,omitempty"`

	// Category is the top-level category (e.g., "using-k6", "javascript-api")
	Category string `json:"category"`

	// Hierarchy is the full hierarchical path (e.g., ["using-k6", "scenarios"])
	Hierarchy []string `json:"hierarchy"`

	// IsIndex indicates if this is an _index.md file (directory-level documentation)
	IsIndex bool `json:"is_index"`
}

// SectionIndex is the root structure containing all documentation sections across versions.
type SectionIndex struct {
	// Versions is the list of all embedded k6 versions (e.g., ["v1.4.x", "v1.3.x", ...])
	Versions []string `json:"versions"`

	// Latest is the latest version string (e.g., "v1.4.x")
	Latest string `json:"latest"`

	// Sections maps version strings to their sections
	Sections map[string][]Section `json:"sections"`

	// BySlug is a runtime index for fast slug lookups (not serialized to JSON)
	// Structure: version -> slug -> *Section
	BySlug map[string]map[string]*Section `json:"-"`

	// ByPath is a runtime index for fast path lookups (not serialized to JSON)
	// Structure: version -> relative_path -> *Section
	ByPath map[string]map[string]*Section `json:"-"`
}

// GetVersion returns all sections for a specific version.
// Returns nil if the version doesn't exist.
func (idx *SectionIndex) GetVersion(version string) []Section {
	if sections, ok := idx.Sections[version]; ok {
		return sections
	}
	return nil
}

// ListVersions returns a list of all available versions.
func (idx *SectionIndex) ListVersions() []string {
	return idx.Versions
}

// GetLatestVersion returns the latest version string.
func (idx *SectionIndex) GetLatestVersion() string {
	return idx.Latest
}

// HasVersion checks if a version exists in the index.
func (idx *SectionIndex) HasVersion(version string) bool {
	_, ok := idx.Sections[version]
	return ok
}

// Frontmatter represents YAML frontmatter in markdown files.
type Frontmatter struct {
	// Title is the page title
	Title string `yaml:"title"`

	// Description is the page description
	Description string `yaml:"description"`

	// Weight is used for sorting (lower values appear first)
	Weight int `yaml:"weight"`

	// Aliases are alternative paths/slugs for this page
	Aliases []string `yaml:"aliases"`

	// MenuTitle is an optional alternative title for navigation menus
	MenuTitle string `yaml:"menuTitle"`
}
