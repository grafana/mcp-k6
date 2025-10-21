//go:build fts5

// Package main provides the k6 MCP server.
package main

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	k6mcp "github.com/grafana/k6-mcp"
	"github.com/grafana/k6-mcp/internal"
	"github.com/grafana/k6-mcp/internal/buildinfo"
	"github.com/grafana/k6-mcp/internal/handlers"
	"github.com/grafana/k6-mcp/internal/logging"
)

func main() {
	logger := logging.Default()

	logger.Info("Starting k6 MCP server",
		slog.String("version", buildinfo.Version),
		slog.String("commit", buildinfo.Commit),
		slog.String("built_at", buildinfo.Date),
		slog.Bool("resource_capabilities", true),
	)

	// Open the embedded database SQLite file
	db, dbFile, err := openDB(k6mcp.EmbeddedDB)
	if err != nil {
		logger.Error("Error opening database", "error", err)
		panic(err)
	}
	defer closeDB(logger, db)
	defer removeDBFile(logger, dbFile)

	s := server.NewMCPServer(
		"k6",
		buildinfo.Version,
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
		server.WithRecovery(),
	)

	// Register tools
	registerRunTool(s, handlers.WithToolMiddleware("run_k6_script", handlers.NewRunHandler()))
	registerDocumentationTools(s, handlers.WithToolMiddleware("search_k6_documentation", handlers.NewFullTextSearchHandler(db)))
	registerValidationTool(s, handlers.WithToolMiddleware("validate_k6_script", handlers.NewValidationHandler()))
	registerTerraformTool(s, handlers.WithToolMiddleware("generate_k6_cloud_terraform_load_test_resource", handlers.NewTerraformHandler()))

	// Register resources
	registerBestPracticesResource(s)
	registerTypeDefinitionsResource(s)

	// Register prompts
	registerGenerateScriptPrompt(s, handlers.WithPromptMiddleware("generate_k6_script", handlers.NewScriptGenerator()))

	logger.Info("Starting MCP server on stdio")
	if err := server.ServeStdio(s); err != nil {
		logger.Error("Server error", slog.String("error", err.Error()))
		return
	}
}

func registerValidationTool(s *server.MCPServer, h handlers.ToolHandler) {
	validateTool := mcp.NewTool(
		"validate_k6_script",
		mcp.WithDescription("Validate a k6 script by running it with minimal configuration (1 VU, 1 iteration). Returns detailed validation results with syntax errors, runtime issues, and actionable recommendations for fixing problems."),
		mcp.WithString(
			"script",
			mcp.Required(),
			mcp.Description("The k6 script content to validate (JavaScript/TypeScript). Example: 'import http from \"k6/http\"; export default function() { http.get(\"https://httpbin.org/get\"); }'"),
		),
	)

	s.AddTool(validateTool, h.Handle)
}

func registerDocumentationTools(s *server.MCPServer, h handlers.ToolHandler) {
	// Register the search tool
	searchTool := mcp.NewTool(
		"search_k6_documentation",
		mcp.WithDescription("Search up-to-date k6 documentation using SQLite FTS5 full-text search. Use proactively while authoring or validating scripts to find best practices, troubleshoot errors, discover examples/templates, and learn idiomatic k6 usage. Query semantics: space-separated terms are ANDed by default; use quotes for exact phrases; FTS5 operators (AND, OR, NEAR, parentheses) and prefix wildcards (e.g., http*) are supported. Returns structured results with title, content, and path."),
		mcp.WithString(
			"keywords",
			mcp.Required(),
			mcp.Description("FTS5 query string. Use space-separated terms (implicit AND), quotes for exact phrases, and optional FTS5 operators. Examples: 'load' → matches load; 'load testing' → matches load AND testing; '\"load testing\"' → exact phrase; 'thresholds OR checks'; 'stages NEAR/5 ramping'; 'http*' for prefix."),
		),
		mcp.WithNumber(
			"max_results",
			mcp.Description("Maximum number of results to return (default: 10, max: 20). Use 5–10 for focused results, 15–20 for broader coverage."),
		),
	)

	s.AddTool(searchTool, h.Handle)
}

func registerRunTool(s *server.MCPServer, h handlers.ToolHandler) {
	// Register the run tool
	runTool := mcp.NewTool(
		"run_k6_script",
		mcp.WithDescription("Run a k6 test script with configurable parameters. Returns detailed execution results including performance metrics, failure analysis, and optimization recommendations."),
		mcp.WithString(
			"script",
			mcp.Required(),
			mcp.Description("The k6 script content to run (JavaScript/TypeScript). Should be a valid k6 script with proper imports and default function."),
		),
		mcp.WithNumber(
			"vus",
			mcp.Description("Number of virtual users (default: 1, max: 50). Examples: 1 for basic test, 10 for moderate load, 50 for stress test."),
		),
		mcp.WithString(
			"duration",
			mcp.Description("Test duration (default: '30s', max: '5m'). Examples: '30s', '2m', '5m'. Overridden by iterations if specified."),
		),
		mcp.WithNumber(
			"iterations",
			mcp.Description("Number of iterations per VU (overrides duration). Examples: 1 for single run, 100 for throughput test."),
		),
		mcp.WithObject(
			"stages",
			mcp.Description("Load profile stages for ramping (array of {duration, target}). Example: [{\"duration\": \"30s\", \"target\": 10}, {\"duration\": \"1m\", \"target\": 20}]"),
		),
		mcp.WithObject(
			"options",
			mcp.Description("Additional k6 options as JSON object. Example: {\"thresholds\": {\"http_req_duration\": [\"p(95)<500\"]}}"),
		),
	)

	s.AddTool(runTool, h.Handle)
}

func registerTerraformTool(s *server.MCPServer, h handlers.ToolHandler) {
	terraformTool := mcp.NewTool(
		"generate_k6_cloud_terraform_load_test_resource",
		mcp.WithDescription("Generate a Terraform resource for a k6 load test in Grafana Cloud. This tool will generate a Terraform resource returned as a string."),
		mcp.WithString(
			"load_test_name",
			mcp.Required(),
			mcp.Description("The human-readable name of the load test to prepare using Terraform. Example: 'My Load Test'"),
		),
		mcp.WithString(
			"load_test_resource_name",
			mcp.Required(),
			mcp.Description("The name of the Terraform resource to generate. This should be a valid Terraform resource name. Example: 'my_load_test'"),
		),
		mcp.WithString(
			"script",
			mcp.Required(),
			mcp.Description("The k6 script content to run (JavaScript/TypeScript). Should be a valid k6 script with proper imports and default function."),
		),
		mcp.WithString(
			"project_id",
			mcp.Required(),
			mcp.Description("The Grafana Cloud k6 project ID to use in the Terraform resource definition. Example: '3688954'"),
		),
	)

	s.AddTool(terraformTool, h.Handle)
}

func registerBestPracticesResource(s *server.MCPServer) {
	bestPracticesResource := mcp.NewResource(
		"docs://k6/best_practices",
		"k6 best practices",
		mcp.WithResourceDescription("Provides a list of best practices for writing k6 scripts."),
		mcp.WithMIMEType("text/markdown"),
	)

	s.AddResource(bestPracticesResource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		content, err := k6mcp.Resources.ReadFile("resources/practices/PRACTICES.md")
		if err != nil {
			return nil, fmt.Errorf("failed to read embedded best practices resource: %w", err)
		}

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      "docs://k6/best_practices",
				MIMEType: "text/markdown",
				Text:     string(content),
			},
		}, nil
	})
}

func registerTypeDefinitionsResource(s *server.MCPServer) {
	_ = fs.WalkDir(k6mcp.TypeDefinitions, ".", func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() && strings.HasSuffix(path, internal.DistDTSFileSuffix) {
			bytes, err := k6mcp.TypeDefinitions.ReadFile(path)
			if err != nil {
				return err
			}

			relPath := strings.TrimPrefix(path, internal.DefinitionsPath)
			uri := "types://k6/" + relPath
			displayName := relPath

			fileBytes := bytes
			fileURI := uri
			resource := mcp.NewResource(
				fileURI,
				displayName,
				mcp.WithResourceDescription("Provides type definitions for k6."),
				mcp.WithMIMEType("application/json"),
			)

			s.AddResource(resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
				return []mcp.ResourceContents{
					mcp.TextResourceContents{
						URI:      fileURI,
						MIMEType: "application/json",
						Text:     string(fileBytes),
					},
				}, nil
			})
		}
		return nil
	})
}

func registerGenerateScriptPrompt(s *server.MCPServer, h handlers.PromptHandler) {
	generateScriptPrompt := mcp.NewPrompt(
		"generate_script",
		mcp.WithPromptDescription("Generate a k6 script based on the user's request."),
		mcp.WithArgument("description", mcp.ArgumentDescription("The description of the script to generate.")),
	)

	s.AddPrompt(generateScriptPrompt, h.Handle)
}

// openDB loads the database file from the embedded data, writes it to a temporary file,
// and returns the file handle and a database connection.
//
// The caller is responsible for closing the database connection and removing the temporary file.
func openDB(dbData []byte) (db *sql.DB, dbFile *os.File, err error) {
	// Load the search index database file from the embedded data
	dbFile, err = os.CreateTemp("", "k6-mcp-index-*.db")
	if err != nil {
		return nil, nil, fmt.Errorf("error creating temporary database file: %w", err)
	}

	_, err = dbFile.Write(dbData)
	if err != nil {
		return nil, nil, fmt.Errorf("error writing index database to temporary file: %w", err)
	}
	err = dbFile.Close()
	if err != nil {
		return nil, nil, fmt.Errorf("error closing temporary database file: %w", err)
	}

	// Open SQLite connection
	db, err = sql.Open("sqlite3", dbFile.Name()+"?mode=ro")
	if err != nil {
		return nil, nil, fmt.Errorf("error opening temporary database file: %w", err)
	}

	return db, dbFile, nil
}

func closeDB(logger *slog.Logger, db *sql.DB) {
	err := db.Close()
	if err != nil {
		logger.Error("Error closing database connection", "error", err)
	}
}

func removeDBFile(logger *slog.Logger, dbFile *os.File) {
	err := os.Remove(dbFile.Name())
	if err != nil {
		logger.Error("Error removing temporary database file", "error", err)
	}
}
