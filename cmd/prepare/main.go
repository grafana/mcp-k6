// Package main provides a unified command for preparing the k6-mcp server
// by performing documentation indexing and type definitions collection.
//
//nolint:forbidigo
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/grafana/k6-mcp/internal"
	"github.com/grafana/k6-mcp/internal/search"
)

const (
	dirPermissions    = 0o700
	gitCommandTimeout = 5 * time.Minute

	tfSchemaURL          = "https://raw.githubusercontent.com/grafana/terraform-provider-grafana/refs/heads/main/current_schema.json"
	tfGrafanaProviderURI = "registry.terraform.io/grafana/grafana"
)

var tfK6CloudResources []string = []string{
	"grafana_k6_installation",
	"grafana_k6_load_test",
	"grafana_k6_project",
	"grafana_k6_project_allowed_load_zones",
	"grafana_k6_project_limits",
	"grafana_k6_schedule",
}

var tfK6CloudDataSources = []string{
	"grafana_k6_load_test",
	"grafana_k6_load_tests",
	"grafana_k6_project",
	"grafana_k6_project_allowed_load_zones",
	"grafana_k6_project_limits",
	"grafana_k6_projects",
	"grafana_k6_schedule",
	"grafana_k6_schedules",
}

var tfAttributeProperties = []string{
	"type", "description", "computed", "optional", "required",
}

func main() {
	var (
		indexOnly   = flag.Bool("index-only", false, "Only perform documentation indexing")
		collectOnly = flag.Bool("collect-only", false, "Only collect type definitions")
		tfOnly      = flag.Bool("terraform-only", false, "Only collect Grafana Terraform provider resource definitions")
		recreateDB  = flag.Bool("recreate-db", true, "Drop and recreate the FTS5 table before indexing")
	)
	flag.Parse()

	// Validate flags
	onlyOptions := 0
	for _, opt := range []*bool{indexOnly, collectOnly, tfOnly} {
		if *opt {
			onlyOptions++
		}
	}
	if onlyOptions > 1 {
		log.Fatal("Cannot specify more than one --[operation]-only flag")
	}

	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// Determine what operations to run
	runIndex := *indexOnly
	runCollect := *collectOnly
	runTf := *tfOnly
	if onlyOptions == 0 {
		// Run all operations
		runIndex = true
		runCollect = true
		runTf = true
	}

	if runIndex {
		log.Println("Starting documentation indexing...")
		if err := runIndexer(workDir, *recreateDB); err != nil {
			log.Fatalf("Documentation indexing failed: %v", err)
		}
		log.Println("Documentation indexing completed successfully")
	}

	if runCollect {
		log.Println("Starting type definitions collection...")
		if err := runCollector(workDir); err != nil {
			log.Fatalf("Type definitions collection failed: %v", err)
		}
		log.Println("Type definitions collection completed successfully")
	}

	if runTf {
		log.Println("Starting Terraform provider resources extraction...")
		if err := runTerraformExtractor(); err != nil {
			log.Fatalf("Terraform resources extraction failed: %v", err)
		}
		log.Println("Terraform resources extraction completed successfully")
	}

	log.Println("Preparation completed successfully")
}

// runIndexer performs the documentation indexing operation
func runIndexer(workDir string, recreate bool) error {
	const (
		k6DocsRepo     = "https://github.com/grafana/k6-docs.git"
		docsSourcePath = "docs/sources/k6"
		databaseName   = "index.db"
		distDir        = "dist"
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
	latestVersion, err := findLatestVersion(docsDir)
	if err != nil {
		return fmt.Errorf("failed to find latest version: %w", err)
	}

	log.Printf("Using k6 documentation version: %s", latestVersion)
	docsPath := filepath.Join(docsDir, latestVersion)

	distPath := filepath.Join(workDir, distDir)
	if err := os.MkdirAll(distPath, dirPermissions); err != nil {
		return fmt.Errorf("failed to create dist directory: %w", err)
	}

	databasePath := filepath.Join(distPath, databaseName)
	log.Printf("Generating SQLite database at: %s", databasePath)

	db, err := search.InitSQLiteDB(databasePath, recreate)
	if err != nil {
		return fmt.Errorf("failed to initialize SQLite database: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Warning: Failed to close database: %v", closeErr)
		}
	}()

	indexer := search.NewSQLiteIndexer(db)
	count, err := indexer.IndexDirectory(docsPath)
	if err != nil {
		return fmt.Errorf("failed to index documents: %w", err)
	}

	log.Printf("Successfully generated database with %d documents at: %s", count, databasePath)
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

// findLatestVersion finds the latest k6 version directory in the docs
func findLatestVersion(docsDir string) (string, error) {
	type Version struct {
		Original string
		Major    int
		Minor    int
	}

	entries, err := os.ReadDir(docsDir)
	if err != nil {
		return "", fmt.Errorf("failed to read docs directory: %w", err)
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
		return "", fmt.Errorf("no valid version directories found")
	}

	sort.Slice(versions, func(i, j int) bool {
		if versions[i].Major != versions[j].Major {
			return versions[i].Major > versions[j].Major
		}
		return versions[i].Minor > versions[j].Minor
	})

	return versions[0].Original, nil
}

// cloneTypesRepository clones the types repository and sets sparse checkout to k6 types
func cloneTypesRepository(repoURL, repoDir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "clone", "--filter=blob:none", "--sparse", repoURL, repoDir)
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

type tfAttribute struct {
	Type        json.RawMessage `json:"type"`
	Description string          `json:"description"`
	Optional    bool            `json:"optional,omitempty"`
	Computed    bool            `json:"computed,omitempty"`
	Required    bool            `json:"required,omitempty"`
}

type tfBlock struct {
	Description string                 `json:"description"`
	Attributes  map[string]tfAttribute `json:"attributes"`
}

type tfSchemaObject struct {
	Block tfBlock `json:"block"`
}

type tfProviderSchema struct {
	ResourceSchemas   map[string]tfSchemaObject `json:"resource_schemas"`
	DataSourceSchemas map[string]tfSchemaObject `json:"data_source_schemas"`
}

type tfJson struct {
	ProviderSchemas map[string]tfProviderSchema `json:"provider_schemas"`
}

type templateResource struct {
	Name        string
	Description string
	JSON        string
}

type templateData struct {
	Resources   []templateResource
	DataSources []templateResource
}

// schemaObjectToTemplateResource converts a tfSchemaObject to a templateResource
func schemaObjectToTemplateResource(name string, schema tfSchemaObject) (templateResource, error) {
	// Pretty print only the attributes
	attributesJSON, err := json.MarshalIndent(schema.Block.Attributes, "", "  ")
	if err != nil {
		return templateResource{}, fmt.Errorf("failed to format JSON for %s: %w", name, err)
	}

	return templateResource{
		Name:        name,
		Description: strings.TrimSpace(schema.Block.Description),
		JSON:        string(attributesJSON),
	}, nil
}

func runTerraformExtractor() error {
	log.Printf("Fetching Terraform provider schema from: %s", tfSchemaURL)

	resp, err := http.Get(tfSchemaURL)
	if err != nil {
		return fmt.Errorf("failed to fetch schema: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch schema: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	var schema tfJson
	if err := json.Unmarshal(body, &schema); err != nil {
		return fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	log.Printf("Successfully parsed Terraform provider schema with %d provider(s)", len(schema.ProviderSchemas))

	grafanaProvider, ok := schema.ProviderSchemas[tfGrafanaProviderURI]
	if !ok {
		return fmt.Errorf("provider for Grafana not found in schema: %s", tfGrafanaProviderURI)
	}

	// Collect resources
	var templateResources []templateResource
	for _, resName := range tfK6CloudResources {
		resSchema, ok := grafanaProvider.ResourceSchemas[resName]
		if !ok {
			log.Printf("Warning: Resource schema for '%s' not found\n", resName)
			continue
		}

		tmplRes, err := schemaObjectToTemplateResource(resName, resSchema)
		if err != nil {
			return err
		}
		templateResources = append(templateResources, tmplRes)
	}

	// Collect data sources
	var templateDataSources []templateResource
	for _, dsName := range tfK6CloudDataSources {
		dsSchema, ok := grafanaProvider.DataSourceSchemas[dsName]
		if !ok {
			log.Printf("Warning: Data source schema for '%s' not found\n", dsName)
			continue
		}

		tmplDS, err := schemaObjectToTemplateResource(dsName, dsSchema)
		if err != nil {
			return err
		}
		templateDataSources = append(templateDataSources, tmplDS)
	}

	// Load and parse the template
	tmplContent, err := os.ReadFile("cmd/prepare/resources/TERRAFORM.md.tpl")
	if err != nil {
		return fmt.Errorf("failed to read template file: %w", err)
	}

	tmpl, err := template.New("terraform").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute the template
	var output bytes.Buffer
	data := templateData{
		Resources:   templateResources,
		DataSources: templateDataSources,
	}
	if err := tmpl.Execute(&output, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Write output to the destination file
	outputPath := filepath.Join("dist", "TERRAFORM.md")
	if err := os.WriteFile(outputPath, output.Bytes(), 0o600); err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	log.Printf("Successfully generated Terraform documentation at: %s", outputPath)

	return nil
}
