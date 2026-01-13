package sections

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BuildSectionIndex walks a documentation directory and creates a section index for a single version.
func BuildSectionIndex(docsPath, version string) (*SectionIndex, error) {
	var sections []Section

	err := filepath.WalkDir(docsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Only process markdown files
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		section, err := ExtractSection(path, docsPath)
		if err != nil {
			// Log warning but continue indexing
			fmt.Printf("Warning: failed to parse %s: %v\n", path, err)
			return nil
		}

		sections = append(sections, *section)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", docsPath, err)
	}

	// Sort sections by weight, then by title
	sort.Slice(sections, func(i, j int) bool {
		if sections[i].Weight != sections[j].Weight {
			return sections[i].Weight < sections[j].Weight
		}
		return sections[i].Title < sections[j].Title
	})

	// Create index with single version
	index := &SectionIndex{
		Versions: []string{version},
		Latest:   version,
		Sections: map[string][]Section{
			version: sections,
		},
		BySlug: make(map[string]map[string]*Section),
		ByPath: make(map[string]map[string]*Section),
	}

	// Build runtime indexes
	index.buildRuntimeIndexes()

	return index, nil
}

// BuildMultiVersionIndex builds a section index for multiple k6 versions.
// The docsRootPath should contain subdirectories for each version (e.g., v1.4.x, v1.3.x).
func BuildMultiVersionIndex(docsRootPath string, versions []string) (*SectionIndex, error) {
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions specified")
	}

	index := &SectionIndex{
		Versions: versions,
		Latest:   versions[0], // First version is assumed to be latest
		Sections: make(map[string][]Section),
		BySlug:   make(map[string]map[string]*Section),
		ByPath:   make(map[string]map[string]*Section),
	}

	// Build sections for each version
	for _, version := range versions {
		versionPath := filepath.Join(docsRootPath, version)

		// Check if version directory exists
		//nolint:forbidigo // File system check necessary for version validation
		if _, err := os.Stat(versionPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("version directory not found: %s", versionPath)
		}

		var sections []Section

		err := filepath.WalkDir(versionPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Only process markdown files
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
				return nil
			}

			section, err := ExtractSection(path, versionPath)
			if err != nil {
				// Log warning but continue indexing
				fmt.Printf("Warning: failed to parse %s (version %s): %v\n", path, version, err)
				return nil
			}

			sections = append(sections, *section)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk directory for version %s: %w", version, err)
		}

		// Sort sections by weight, then by title
		sort.Slice(sections, func(i, j int) bool {
			if sections[i].Weight != sections[j].Weight {
				return sections[i].Weight < sections[j].Weight
			}
			return sections[i].Title < sections[j].Title
		})

		index.Sections[version] = sections
		fmt.Printf("Indexed %d sections for version %s\n", len(sections), version)
	}

	// Build runtime indexes for all versions
	index.buildRuntimeIndexes()

	return index, nil
}

// buildRuntimeIndexes creates lookup maps for fast retrieval.
func (idx *SectionIndex) buildRuntimeIndexes() {
	for version, sections := range idx.Sections {
		// Initialize maps for this version if they don't exist
		if idx.BySlug[version] == nil {
			idx.BySlug[version] = make(map[string]*Section)
		}
		if idx.ByPath[version] == nil {
			idx.ByPath[version] = make(map[string]*Section)
		}

		// Index each section
		for i := range sections {
			section := &sections[i]

			// Index by slug (primary)
			idx.BySlug[version][section.Slug] = section

			// Index by relative path
			idx.ByPath[version][section.RelPath] = section

			// Index by aliases
			for _, alias := range section.Aliases {
				// Clean up alias (remove leading slashes, etc.)
				cleanAlias := strings.TrimPrefix(alias, "/")
				cleanAlias = strings.TrimPrefix(cleanAlias, "docs/k6/")

				if cleanAlias != "" {
					if _, exists := idx.BySlug[version][cleanAlias]; !exists {
						idx.BySlug[version][cleanAlias] = section
					}
				}
			}
		}
	}
}

// WriteJSON serializes the index to a JSON file.
func (idx *SectionIndex) WriteJSON(outputPath string) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	// Create parent directory if it doesn't exist
	dir := filepath.Dir(outputPath)
	//nolint:forbidigo // Directory creation necessary for writing index file
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	//nolint:forbidigo // File I/O necessary for writing sections index
	if err := os.WriteFile(outputPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write index file: %w", err)
	}

	return nil
}

// LoadJSON deserializes the index from JSON data and rebuilds runtime indexes.
func LoadJSON(data []byte) (*SectionIndex, error) {
	var index SectionIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to unmarshal index: %w", err)
	}

	// Initialize runtime index maps
	index.BySlug = make(map[string]map[string]*Section)
	index.ByPath = make(map[string]map[string]*Section)

	// Rebuild runtime indexes
	index.buildRuntimeIndexes()

	return &index, nil
}
