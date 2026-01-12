// Package main provides a unified command for preparing the mcp-k6 server
// by performing documentation preparation and type definitions collection.
//
//nolint:forbidigo
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/mcp-k6/internal"
	"github.com/grafana/mcp-k6/internal/sections"
)

const (
	dirPermissions    = 0o700
	gitCommandTimeout = 15 * time.Minute
	distDir           = "dist"
)

func main() {
	var (
		docsOnly  = flag.Bool("docs-only", false, "Only prepare documentation assets")
		typesOnly = flag.Bool("types-only", false, "Only collect type definitions")
	)
	flag.Parse()

	// Validate flags
	if *docsOnly && *typesOnly {
		log.Fatal("Cannot specify both --docs-only and --types-only flags")
	}

	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// Determine what operations to run
	runDocs := *docsOnly
	runTypes := *typesOnly
	if !*docsOnly && !*typesOnly {
		runDocs = true
		runTypes = true
	}

	if runDocs {
		log.Println("Starting documentation preparation...")
		if err := runDocsPreparation(workDir); err != nil {
			log.Fatalf("Documentation preparation failed: %v", err)
		}
		log.Println("Documentation preparation completed successfully")
	}

	if runTypes {
		log.Println("Starting type definitions collection...")
		if err := runCollector(workDir); err != nil {
			log.Fatalf("Type definitions collection failed: %v", err)
		}
		log.Println("Type definitions collection completed successfully")
	}

	log.Println("Preparation completed successfully")
}

// runDocsPreparation downloads the k6 documentation, builds sections.json,
// and copies markdown content into dist/markdown.
func runDocsPreparation(workDir string) error {
	const (
		k6DocsRepo     = "https://github.com/grafana/k6-docs.git"
		docsSourcePath = "docs/sources/k6"
		sectionsName   = "sections.json"
		markdownDir    = "markdown"
	)

	tempDir, err := os.MkdirTemp("", "k6-docs-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(tempDir); removeErr != nil {
			log.Printf("Warning: Failed to clean up temporary directory %s: %v", tempDir, removeErr)
		}
	}()

	log.Printf("Cloning k6 documentation repository...")
	if err := cloneRepository(k6DocsRepo, tempDir); err != nil {
		return fmt.Errorf("failed to clone k6-docs repository: %w", err)
	}

	docsDir := filepath.Join(tempDir, docsSourcePath)
	versions, err := findAvailableVersions(docsDir)
	if err != nil {
		return fmt.Errorf("failed to find documentation versions: %w", err)
	}
	latestVersion := versions[0]

	log.Printf("Using k6 documentation version: %s", latestVersion)

	distPath := filepath.Join(workDir, distDir)
	if err := os.MkdirAll(distPath, dirPermissions); err != nil {
		return fmt.Errorf("failed to create dist directory: %w", err)
	}

	sectionsIndexPath := filepath.Join(distPath, sectionsName)
	log.Printf("Building sections index at: %s", sectionsIndexPath)
	index, err := sections.BuildMultiVersionIndex(docsDir, versions)
	if err != nil {
		return fmt.Errorf("failed to build sections index: %w", err)
	}
	if err := index.WriteJSON(sectionsIndexPath); err != nil {
		return fmt.Errorf("failed to write sections index: %w", err)
	}

	markdownPath := filepath.Join(distPath, markdownDir)
	log.Printf("Copying markdown content to: %s", markdownPath)
	if err := os.RemoveAll(markdownPath); err != nil {
		return fmt.Errorf("failed to clean markdown directory: %w", err)
	}
	if err := copyMarkdownDocs(docsDir, markdownPath, versions); err != nil {
		return fmt.Errorf("failed to copy markdown documentation: %w", err)
	}

	log.Printf("Successfully prepared documentation for %d versions", len(versions))
	return nil
}

// runCollector performs the type definitions collection operation
func runCollector(workDir string) error {
	const typesRepo = "https://github.com/DefinitelyTyped/DefinitelyTyped.git"

	destDir := filepath.Join(workDir,
		internal.DistFolderName,
		internal.DistDefinitionsFolderName,
		internal.DistTypesFolderName,
		internal.DistK6FolderName)

	if _, err := os.Stat(destDir); !os.IsNotExist(err) {
		log.Printf("Removing existing dist definitions directory: %s", destDir)
		if err := os.RemoveAll(destDir); err != nil {
			return fmt.Errorf("failed to remove existing directory: %w", err)
		}
	}

	log.Printf("Cloning types repository...")
	if err := cloneTypesRepository(typesRepo, destDir); err != nil {
		return fmt.Errorf("failed to clone types repository: %w", err)
	}

	if err := cleanUpTypesRepository(destDir); err != nil {
		return fmt.Errorf("failed to clean up types repository: %w", err)
	}

	log.Printf("Successfully collected type definitions to: %s", destDir)
	return nil
}

// cloneRepository clones a git repository to the target directory
func cloneRepository(repoURL, targetDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", repoURL, targetDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git command failed: %w", err)
	}
	return nil
}

// findAvailableVersions finds k6 version directories in the docs sorted latest-first.
func findAvailableVersions(docsDir string) ([]string, error) {
	type Version struct {
		Original string
		Major    int
		Minor    int
	}

	entries, err := os.ReadDir(docsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read docs directory: %w", err)
	}

	versions := make([]Version, 0, len(entries))
	versionRegex := regexp.MustCompile(`^v(\d+)\.(\d+)\.x$`)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		if name == "next" {
			continue
		}

		matches := versionRegex.FindStringSubmatch(name)
		if matches == nil {
			continue
		}

		major, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}

		minor, err := strconv.Atoi(matches[2])
		if err != nil {
			continue
		}

		versions = append(versions, Version{
			Original: name,
			Major:    major,
			Minor:    minor,
		})
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no valid version directories found")
	}

	sort.Slice(versions, func(i, j int) bool {
		if versions[i].Major != versions[j].Major {
			return versions[i].Major > versions[j].Major
		}
		return versions[i].Minor > versions[j].Minor
	})

	results := make([]string, 0, len(versions))
	for _, v := range versions {
		results = append(results, v.Original)
	}

	return results, nil
}

// cloneTypesRepository clones the types repository and sets sparse checkout to k6 types
func cloneTypesRepository(repoURL, repoDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "clone", "--filter=blob:none", "--sparse", "--depth=1", repoURL, repoDir)
	var cloneStderr bytes.Buffer
	cmd.Stderr = &cloneStderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to clone types repository; reason: %s", cloneStderr.String())
	}

	cmd = exec.CommandContext(ctx, "git", "-C", repoDir, "sparse-checkout", "set", "types/k6")
	var sparseStderr bytes.Buffer
	cmd.Stderr = &sparseStderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to set sparse checkout; reason: %s", sparseStderr.String())
	}

	// Move the checked-out subtree (types/k6) up to repoDir so that repoDir mirrors the k6 types folder
	srcDir := filepath.Join(repoDir, "types", "k6")
	tmpDir := repoDir + ".tmp"
	if err := os.Rename(srcDir, tmpDir); err != nil {
		return fmt.Errorf("failed to move %s to temporary location %s: %w", srcDir, tmpDir, err)
	}
	if err := os.RemoveAll(repoDir); err != nil {
		return fmt.Errorf("failed to clear repository directory %s: %w", repoDir, err)
	}
	if err := os.Rename(tmpDir, repoDir); err != nil {
		return fmt.Errorf("failed to move temporary directory back to %s: %w", repoDir, err)
	}

	return nil
}

// cleanUpTypesRepository removes non-.d.ts files and empty directories
func cleanUpTypesRepository(repoDir string) error {
	// First pass: remove any file that does not end with .d.ts
	removeNonDTS := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), internal.DistDTSFileSuffix) {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("failed to remove file %s: %w", path, err)
			}
		}
		return nil
	}

	if err := filepath.WalkDir(repoDir, removeNonDTS); err != nil {
		return fmt.Errorf("failed to walk directory for cleanup: %w", err)
	}

	// Second pass: gather directories and prune empty ones from deepest to root
	var directories []string
	collectDirs := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			directories = append(directories, path)
		}
		return nil
	}

	if err := filepath.WalkDir(repoDir, collectDirs); err != nil {
		return fmt.Errorf("failed to collect directories: %w", err)
	}

	sort.Slice(directories, func(i, j int) bool { return len(directories[i]) > len(directories[j]) })
	for _, dir := range directories {
		_ = os.Remove(dir) // remove only if empty
	}

	return nil
}

func copyMarkdownDocs(docsRoot, destRoot string, versions []string) error {
	for _, version := range versions {
		sourceRoot := filepath.Join(docsRoot, version)
		targetRoot := filepath.Join(destRoot, version)

		err := filepath.WalkDir(sourceRoot, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
				return nil
			}

			relPath, err := filepath.Rel(sourceRoot, path)
			if err != nil {
				return fmt.Errorf("failed to compute relative path: %w", err)
			}

			targetPath := filepath.Join(targetRoot, relPath)
			if err := os.MkdirAll(filepath.Dir(targetPath), dirPermissions); err != nil {
				return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
			}

			if err := copyFile(path, targetPath); err != nil {
				return fmt.Errorf("failed to copy %s: %w", path, err)
			}

			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to copy markdown for version %s: %w", version, err)
		}
	}

	return nil
}

func copyFile(source, dest string) (retErr error) {
	// #nosec G304 -- source path is derived from a controlled docs tree walk.
	srcFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil && retErr == nil {
			retErr = closeErr
		}
	}()

	// #nosec G304 -- destination path is derived from a controlled docs tree walk.
	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil && retErr == nil {
			retErr = closeErr
		}
	}()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return err
	}

	return nil
}
