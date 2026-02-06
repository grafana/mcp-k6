package sections

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseFrontmatter extracts YAML frontmatter from a markdown file.
// Returns an empty Frontmatter struct if no frontmatter is present.
func ParseFrontmatter(path string) (*Frontmatter, error) {
	// #nosec G304 -- path is validated during indexing
	//nolint:forbidigo // File I/O necessary for parsing markdown frontmatter
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Check for frontmatter delimiters (---)
	if !bytes.HasPrefix(content, []byte("---\n")) && !bytes.HasPrefix(content, []byte("---\r\n")) {
		return &Frontmatter{}, nil // No frontmatter
	}

	// Find closing delimiter
	scanner := bufio.NewScanner(bytes.NewReader(content[4:]))
	var yamlLines []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "---" {
			break
		}
		yamlLines = append(yamlLines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan frontmatter: %w", err)
	}

	rawFrontmatter := strings.Join(yamlLines, "\n")

	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(rawFrontmatter), &fm); err == nil {
		return &fm, nil
	} else if !strings.Contains(err.Error(), "mapping key") {
		return nil, fmt.Errorf("failed to parse frontmatter YAML: %w", err)
	}

	sanitized := dedupeFrontmatter(rawFrontmatter)
	if sanitized == rawFrontmatter {
		return nil, fmt.Errorf("failed to parse frontmatter YAML: %w", err)
	}

	if err := yaml.Unmarshal([]byte(sanitized), &fm); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter YAML: %w", err)
	}

	return &fm, nil
}

func dedupeFrontmatter(raw string) string {
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		return raw
	}

	type keyLine struct {
		key   string
		index int
	}

	var keyLines []keyLine
	for i, line := range lines {
		if key, ok := topLevelKey(line); ok {
			keyLines = append(keyLines, keyLine{key: key, index: i})
		}
	}

	if len(keyLines) == 0 {
		return raw
	}

	type block struct {
		key   string
		start int
		end   int
	}

	blocks := make([]block, 0, len(keyLines)+1)
	if keyLines[0].index > 0 {
		blocks = append(blocks, block{
			key:   "",
			start: 0,
			end:   keyLines[0].index - 1,
		})
	}

	for i, entry := range keyLines {
		start := entry.index
		end := len(lines) - 1
		if i+1 < len(keyLines) {
			end = keyLines[i+1].index - 1
		}
		blocks = append(blocks, block{
			key:   entry.key,
			start: start,
			end:   end,
		})
	}

	lastBlock := make(map[string]int, len(blocks))
	for i, block := range blocks {
		if block.key == "" {
			continue
		}
		lastBlock[block.key] = i
	}

	var output []string
	for i, block := range blocks {
		if block.key != "" && lastBlock[block.key] != i {
			continue
		}
		output = append(output, lines[block.start:block.end+1]...)
	}

	return strings.Join(output, "\n")
}

func topLevelKey(line string) (string, bool) {
	if line == "" {
		return "", false
	}
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return "", false
	}
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "-") {
		return "", false
	}

	parts := strings.SplitN(trimmed, ":", 2)
	if len(parts) != 2 {
		return "", false
	}
	key := strings.TrimSpace(parts[0])
	if key == "" || strings.ContainsAny(key, " \t") {
		return "", false
	}
	return key, true
}

// ExtractSection creates a Section from a markdown file path.
// The docsRoot parameter should be the root directory of the documentation.
func ExtractSection(path, docsRoot string) (*Section, error) {
	fm, err := ParseFrontmatter(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter for %s: %w", path, err)
	}

	relPath, err := filepath.Rel(docsRoot, path)
	if err != nil {
		return nil, fmt.Errorf("failed to compute relative path: %w", err)
	}

	// Build hierarchy from directory structure
	hierarchy := buildHierarchy(relPath)

	// Generate slug from relative path
	slug := pathToSlug(relPath)

	// Determine category (top-level directory)
	category := ""
	if len(hierarchy) > 0 {
		category = hierarchy[0]
	}

	// Check if this is an _index.md file
	isIndex := filepath.Base(path) == "_index.md"

	return &Section{
		Slug:        slug,
		Path:        path,
		RelPath:     relPath,
		Title:       fm.Title,
		Description: fm.Description,
		Weight:      fm.Weight,
		Aliases:     fm.Aliases,
		Category:    category,
		Hierarchy:   hierarchy,
		IsIndex:     isIndex,
	}, nil
}

// buildHierarchy creates a hierarchy array from a relative path.
// Example: "using-k6/scenarios/_index.md" -> ["using-k6", "scenarios"]
func buildHierarchy(relPath string) []string {
	// Get the directory path (remove filename)
	dir := filepath.Dir(relPath)
	if dir == "." || dir == "" {
		return []string{}
	}

	// Split by path separator and return as hierarchy
	parts := strings.Split(dir, string(filepath.Separator))

	// Filter out empty parts
	var hierarchy []string
	for _, part := range parts {
		if part != "" && part != "." {
			hierarchy = append(hierarchy, part)
		}
	}

	return hierarchy
}

// pathToSlug converts a relative path to a slug.
// Examples:
//   - "using-k6/scenarios/_index.md" -> "using-k6/scenarios"
//   - "javascript-api/k6-http/request.md" -> "javascript-api/k6-http/request"
//   - "get-started.md" -> "get-started"
func pathToSlug(relPath string) string {
	// Remove .md extension
	slug := strings.TrimSuffix(relPath, ".md")

	// Remove _index suffix if present
	slug = strings.TrimSuffix(slug, string(filepath.Separator)+"_index")
	slug = strings.TrimSuffix(slug, "/_index")

	// Convert path separators to forward slashes for consistency
	slug = strings.ReplaceAll(slug, string(filepath.Separator), "/")

	return slug
}
