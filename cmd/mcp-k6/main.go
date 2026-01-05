//go:build fts5

// Package main provides the k6 MCP server.
package main

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/server"

	k6mcp "github.com/grafana/mcp-k6"
	"github.com/grafana/mcp-k6/internal/buildinfo"
	"github.com/grafana/mcp-k6/internal/k6env"
	"github.com/grafana/mcp-k6/internal/logging"
	"github.com/grafana/mcp-k6/prompts"
	"github.com/grafana/mcp-k6/resources"
	"github.com/grafana/mcp-k6/tools"
)

// Server instructions are a good opportunity to give the agent a high-level overview of the tools
// and resources that will be made available. However, it should be kept as brief as possible, as
// to not waste conversation tokens.
const instructions = `
Use the provided tools for running or validating k6 scripts, or for searching through the k6 OSS docs.
Use the provided resources for understanding the k6 script authoring best practices and for consulting
type definitions.
List the resources at least once before trying to access one of them.
Use the provided prompts as a good starting point for authoring complex k6 scripts.
`

var serveStdio = server.ServeStdio

func main() {
	logger := logging.Default()
	os.Exit(run(context.Background(), logger, os.Stderr))
}

func run(ctx context.Context, logger *slog.Logger, stderr io.Writer) int {
	logger.Info("Starting k6 MCP server",
		slog.String("version", buildinfo.Version),
		slog.String("commit", buildinfo.Commit),
		slog.String("built_at", buildinfo.Date),
		slog.Bool("resource_capabilities", true),
	)

	k6Info, err := k6env.Locate(ctx)
	if err != nil {
		return handleK6LookupError(logger, stderr, err)
	}

	logger.Info("Detected k6 executable", slog.String("path", k6Info.Path))

	// Open the embedded database SQLite file
	db, dbFile, err := openDB(k6mcp.EmbeddedDB)
	if err != nil {
		logger.Error("Error opening database", "error", err)
		fmt.Fprintf(stderr, "Failed to open embedded database: %v\n", err)
		return 1
	}
	defer closeDB(logger, db)
	defer removeDBFile(logger, dbFile)

	s := server.NewMCPServer(
		"k6",
		buildinfo.Version,
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
		server.WithRecovery(),
		server.WithInstructions(instructions),
	)

	// Register tools
	tools.RegisterInfoTool(s)
	tools.RegisterValidateTool(s)
	tools.RegisterSearchDocumentationTool(s, db)
	tools.RegisterRunTool(s)

	// Register resources
	resources.RegisterBestPracticesResource(s)
	resources.RegisterTypeDefinitionsResources(s)

	// Register prompts
	prompts.RegisterGenerateScriptPrompt(s)
	prompts.RegisterConvertPlaywrightScriptPrompt(s)

	logger.Info("Starting MCP server on stdio")
	if err := serveStdio(s); err != nil {
		logger.Error("Server error", slog.String("error", err.Error()))
		fmt.Fprintf(stderr, "MCP server exited with error: %v\n", err)
		return 1
	}

	return 0
}

func handleK6LookupError(logger *slog.Logger, stderr io.Writer, err error) int {
	if errors.Is(err, k6env.ErrNotFound) {
		message := "mcp-k6 requires the `k6` executable on your PATH. Install k6 (https://grafana.com/docs/k6/latest/get-started/installation/) and ensure it is accessible before retrying."
		logger.Error("k6 executable not found on PATH", slog.String("hint", message))
		fmt.Fprintln(stderr, message)
	} else {
		logger.Error("Failed to locate k6 executable", slog.String("error", err.Error()))
		fmt.Fprintf(stderr, "Failed to locate k6 executable: %v\n", err)
	}

	return 1
}

// openDB loads the database file from the embedded data, writes it to a temporary file,
// and returns the file handle and a database connection.
//
// The caller is responsible for closing the database connection and removing the temporary file.
func openDB(dbData []byte) (db *sql.DB, dbFile *os.File, err error) {
	// Load the search index database file from the embedded data
	dbFile, err = os.CreateTemp("", "mcp-k6-index-*.db")
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
