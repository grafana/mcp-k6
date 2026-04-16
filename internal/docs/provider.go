// Package docs provides a documentation provider backed by k6-docs-lib.
// It downloads, caches, and indexes k6 documentation at runtime.
package docs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	k6docslib "github.com/grafana/k6-docs-lib"
	"go.k6.io/k6/lib/fsext"
)

// Provider wraps k6-docs-lib's MultiIndex and cache directory to serve
// documentation sections and markdown content at runtime.
type Provider struct {
	index    *k6docslib.MultiIndex
	cacheDir string
	afs      fsext.Fs
}

var errInvalidRelPath = errors.New("invalid documentation relative path")

// New initialises the documentation provider for the given k6 version.
// It downloads (or loads from cache) the docs bundle, builds the index,
// and returns a ready-to-use Provider.
func New(ctx context.Context, logger *slog.Logger, k6Version string) (*Provider, error) {
	version := k6docslib.MapToWildcard(k6Version)
	if version == "" {
		return nil, fmt.Errorf("failed to map k6 version %q to a docs version", k6Version)
	}

	logger.Info("Ensuring documentation bundle",
		slog.String("k6_version", k6Version),
		slog.String("docs_version", version))

	afs := fsext.NewOsFs()
	env := buildEnv()

	const httpTimeout = 60 * time.Second
	client := &http.Client{Timeout: httpTimeout}

	cacheDir, err := k6docslib.EnsureDocs(ctx, afs, env, version, client)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure docs for version %s: %w", version, err)
	}

	logger.Info("Documentation bundle ready", slog.String("cache_dir", cacheDir))

	idx, err := k6docslib.LoadIndex(afs, cacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load docs index for version %s: %w", version, err)
	}

	mi := k6docslib.NewMultiIndex()
	mi.Add(version, idx)
	mi.SetLatest(version)

	logger.Info("Documentation index loaded",
		slog.String("version", version),
		slog.Int("section_count", len(idx.Sections)))

	return &Provider{
		index:    mi,
		cacheDir: cacheDir,
		afs:      afs,
	}, nil
}

// NewFromMultiIndex creates a Provider from an existing MultiIndex and cache
// directory. This is intended for testing.
func NewFromMultiIndex(mi *k6docslib.MultiIndex, cacheDir string, afs fsext.Fs) *Provider {
	return &Provider{
		index:    mi,
		cacheDir: cacheDir,
		afs:      afs,
	}
}

// Lookup returns the section with the given slug for a specific version.
// Empty version uses latest.
func (p *Provider) Lookup(slug, version string) (*k6docslib.Section, bool) {
	return p.index.Lookup(slug, version)
}

// GetAll returns all sections for a version. Empty version uses latest.
func (p *Provider) GetAll(version string) []k6docslib.Section {
	return p.index.GetAll(version)
}

// GetByCategory returns sections in a category for a version.
// Empty version uses latest.
func (p *Provider) GetByCategory(category, version string) []k6docslib.Section {
	return p.index.GetByCategory(category, version)
}

// GetVersions returns all registered versions.
func (p *Provider) GetVersions() []string {
	return p.index.GetVersions()
}

// GetLatestVersion returns the latest version string.
func (p *Provider) GetLatestVersion() string {
	return p.index.GetLatestVersion()
}

// ReadContent reads the markdown content for a section from the cache directory.
func (p *Provider) ReadContent(section *k6docslib.Section, version string) ([]byte, error) {
	mdPath, err := p.markdownPath(section.RelPath)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read documentation content for %s (version %s): %w",
			section.Slug, version, err,
		)
	}

	content, err := fsext.ReadFile(p.afs, mdPath)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read documentation content for %s (version %s): %w",
			section.Slug, version, err,
		)
	}

	return content, nil
}

func (p *Provider) markdownPath(relPath string) (string, error) {
	if relPath == "" {
		return "", errInvalidRelPath
	}

	cleanRelPath := filepath.Clean(relPath)
	if cleanRelPath == "." || filepath.IsAbs(cleanRelPath) {
		return "", fmt.Errorf("%w: %q", errInvalidRelPath, relPath)
	}

	markdownRoot := filepath.Join(p.cacheDir, "markdown")
	fullPath := filepath.Join(markdownRoot, cleanRelPath)
	relativeToRoot, err := filepath.Rel(markdownRoot, fullPath)
	if err != nil {
		return "", fmt.Errorf("%w: %q", errInvalidRelPath, relPath)
	}

	if relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", errInvalidRelPath, relPath)
	}

	return fullPath, nil
}

// buildEnv returns a map of relevant environment variables for k6-docs-lib.
func buildEnv() map[string]string {
	env := make(map[string]string)
	for _, key := range []string{
		"HOME",
		"USERPROFILE",
		"K6_DOCS_CACHE_DIR",
		"K6_DOCS_BUNDLE_URL",
		"K6_DOCS_REFRESH_TIMEOUT",
	} {
		//nolint:forbidigo // Need to read env vars for k6-docs-lib configuration.
		if v := os.Getenv(key); v != "" {
			env[key] = v
		}
	}
	return env
}

// IsInvalidRelPath reports whether err was caused by rejected documentation metadata.
func IsInvalidRelPath(err error) bool {
	return errors.Is(err, errInvalidRelPath) || errors.Is(err, fs.ErrInvalid)
}
