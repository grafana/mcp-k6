// Package main provides a command for preparing the mcp-k6 server
// by collecting TypeScript type definitions.
//
//nolint:forbidigo
package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/grafana/mcp-k6/internal"
)

const (
	dirPermissions    = 0o700
	gitCommandTimeout = 15 * time.Minute
	distDir           = "dist"
)

func main() {
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	log.Println("Starting type definitions collection...")
	if err := runCollector(workDir); err != nil {
		log.Fatalf("Type definitions collection failed: %v", err)
	}
	log.Println("Preparation completed successfully")
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

// cloneTypesRepository clones the types repository and sets sparse checkout to k6 types
func cloneTypesRepository(repoURL, repoDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext( // #nosec G204
		ctx, "git", "clone", "--filter=blob:none", "--sparse", "--depth=1", repoURL, repoDir,
	)
	var cloneStderr bytes.Buffer
	cmd.Stderr = &cloneStderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to clone types repository; reason: %s", cloneStderr.String())
	}

	cmd = exec.CommandContext( // #nosec G204
		ctx, "git", "-C", repoDir, "sparse-checkout", "set", "types/k6",
	)
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
			if err := os.Remove(path); err != nil { // #nosec G122
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
