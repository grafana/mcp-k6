package sections

import (
	"fmt"
	"strings"
)

// Finder provides lookup operations on a section index.
type Finder struct {
	index *SectionIndex
}

// NewFinder creates a new section finder from an index.
func NewFinder(index *SectionIndex) *Finder {
	return &Finder{index: index}
}

// GetAll returns all sections for a specific version.
// If version is empty, returns sections for the latest version.
func (f *Finder) GetAll(version string) ([]Section, error) {
	version = f.resolveVersion(version)

	if !f.index.HasVersion(version) {
		return nil, fmt.Errorf("version not found: %s", version)
	}

	sections := f.index.GetVersion(version)
	return sections, nil
}

// GetBySlug finds a section by slug for a specific version.
// Handles aliases automatically.
// If version is empty, uses the latest version.
func (f *Finder) GetBySlug(slug, version string) (*Section, error) {
	version = f.resolveVersion(version)

	if !f.index.HasVersion(version) {
		return nil, fmt.Errorf("version not found: %s", version)
	}

	versionIndex, ok := f.index.BySlug[version]
	if !ok {
		return nil, fmt.Errorf("no slug index for version: %s", version)
	}

	section, ok := versionIndex[slug]
	if !ok {
		return nil, fmt.Errorf("section not found: %s", slug)
	}

	return section, nil
}

// GetByCategory returns all sections in a specific category for a version.
// If version is empty, uses the latest version.
func (f *Finder) GetByCategory(category, version string) ([]Section, error) {
	version = f.resolveVersion(version)

	if !f.index.HasVersion(version) {
		return nil, fmt.Errorf("version not found: %s", version)
	}

	allSections := f.index.GetVersion(version)
	var results []Section

	for _, section := range allSections {
		if section.Category == category {
			results = append(results, section)
		}
	}

	return results, nil
}

// Search performs a simple text search across titles, descriptions, and slugs for a version.
// If version is empty, uses the latest version.
func (f *Finder) Search(query, version string) ([]Section, error) {
	version = f.resolveVersion(version)

	if !f.index.HasVersion(version) {
		return nil, fmt.Errorf("version not found: %s", version)
	}

	query = strings.ToLower(query)
	allSections := f.index.GetVersion(version)
	var results []Section

	for _, section := range allSections {
		titleMatch := strings.Contains(strings.ToLower(section.Title), query)
		descMatch := strings.Contains(strings.ToLower(section.Description), query)
		slugMatch := strings.Contains(strings.ToLower(section.Slug), query)

		if titleMatch || descMatch || slugMatch {
			results = append(results, section)
		}
	}

	return results, nil
}

// GetCategories returns unique top-level categories for a version.
// If version is empty, uses the latest version.
func (f *Finder) GetCategories(version string) ([]string, error) {
	version = f.resolveVersion(version)

	if !f.index.HasVersion(version) {
		return nil, fmt.Errorf("version not found: %s", version)
	}

	allSections := f.index.GetVersion(version)
	seen := make(map[string]bool)
	var categories []string

	for _, section := range allSections {
		if section.Category != "" && !seen[section.Category] {
			seen[section.Category] = true
			categories = append(categories, section.Category)
		}
	}

	return categories, nil
}

// GetVersions returns the list of all available versions.
func (f *Finder) GetVersions() []string {
	return f.index.ListVersions()
}

// GetLatestVersion returns the latest version string.
func (f *Finder) GetLatestVersion() string {
	return f.index.GetLatestVersion()
}

// resolveVersion returns the latest version if version is empty, otherwise returns the provided version.
func (f *Finder) resolveVersion(version string) string {
	if version == "" {
		return f.index.GetLatestVersion()
	}
	return version
}

// MatchVersion attempts to match a user's k6 version (e.g., "v1.4.0") to an available docs version (e.g., "v1.4.x").
// Returns the best matching version or an error if no match is found.
func (f *Finder) MatchVersion(userVersion string) (string, error) {
	if userVersion == "" {
		return f.index.GetLatestVersion(), nil
	}

	// Direct match (e.g., "v1.4.x")
	if f.index.HasVersion(userVersion) {
		return userVersion, nil
	}

	// Try to extract major.minor from user version and match to .x version
	// Example: "v1.4.0" -> "v1.4.x"
	parts := strings.Split(userVersion, ".")
	if len(parts) >= 2 {
		majorMinor := parts[0] + "." + parts[1] + ".x"
		if f.index.HasVersion(majorMinor) {
			return majorMinor, nil
		}
	}

	// No match found - return latest as fallback
	return f.index.GetLatestVersion(), fmt.Errorf("no exact match for version %s, using latest", userVersion)
}
