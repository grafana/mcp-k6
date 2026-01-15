// Package main provides the k6 MCP server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/server"

	k6mcp "github.com/grafana/mcp-k6"
	"github.com/grafana/mcp-k6/internal/buildinfo"
	"github.com/grafana/mcp-k6/internal/k6env"
	"github.com/grafana/mcp-k6/internal/logging"
	"github.com/grafana/mcp-k6/internal/sections"
	"github.com/grafana/mcp-k6/prompts"
	"github.com/grafana/mcp-k6/resources"
	"github.com/grafana/mcp-k6/tools"
)

// Server instructions are a good opportunity to give the agent a high-level overview of the tools
// and resources that will be made available. However, it should be kept as brief as possible, as
// to not waste conversation tokens.
const instructions = `
Use the provided tools for running or validating k6 scripts, for browsing the k6 OSS docs, or
for searching for k6 Cloud-related Terraform resources in the Grafana Terraform provider.
Use the provided resources for understanding the k6 script authoring best practices and for consulting
type definitions.
List the resources at least once before trying to access one of them.
Use the provided prompts as a good starting point for authoring complex k6 scripts.
`

//nolint:gochecknoglobals // Allows test override for stdio server.
var serveStdio = server.ServeStdio

func main() {
	logger := logging.Default()
	//nolint:forbidigo // main must exit with the server status code.
	os.Exit(run(context.Background(), logger, os.Stderr))
}

func run(ctx context.Context, logger *slog.Logger, stderr io.Writer) int {
	var (
		transport    = flag.String("transport", "stdio", "Transport mode: stdio or http")
		addr         = flag.String("addr", ":8080", "HTTP address to listen on")
		ssePath      = flag.String("sse-path", "/sse", "Path for SSE endpoint")
		messagesPath = flag.String("messages-path", "/messages", "Path for message posting")
	)
	flag.Parse()

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

	// Load sections index
	logger.Info("Loading sections index")
	sectionsIdx, err := sections.LoadJSON(k6mcp.SectionsIndex)
	if err != nil {
		logger.Error("Error loading sections index", "error", err)
		_, _ = fmt.Fprintf(stderr, "Failed to load sections index: %v\n", err)
		return 1
	}
	finder := sections.NewFinder(sectionsIdx)

	totalSections := 0
	for _, secs := range sectionsIdx.Sections {
		totalSections += len(secs)
	}
	logger.Info("Loaded sections index",
		slog.Int("version_count", len(sectionsIdx.Versions)),
		slog.Int("total_sections", totalSections),
		slog.String("latest_version", sectionsIdx.Latest))

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
	tools.RegisterRunTool(s)
	tools.RegisterSearchTerraformTool(s)
	tools.RegisterListSectionsTool(s, finder)
	tools.RegisterGetDocumentationTool(s, finder)

	// Register resources
	resources.RegisterBestPracticesResource(s)
	resources.RegisterTypeDefinitionsResources(s)

	// Register prompts
	prompts.RegisterGenerateScriptPrompt(s)
	prompts.RegisterConvertPlaywrightScriptPrompt(s)

	if *transport == "http" {
		// Construct BaseURL from the address
		baseURL := "http://localhost:8080" // Default fallback
		if *addr != "" {
			if (*addr)[0] == ':' {
				baseURL = "http://localhost" + *addr
			} else {
				baseURL = "http://" + *addr
			}
		}

		sseServer := server.NewSSEServer(s,
			server.WithBaseURL(baseURL),
			server.WithSSEEndpoint(*ssePath),
			server.WithMessageEndpoint(*messagesPath),
		)
		mux := http.NewServeMux()
		mux.Handle(*ssePath, sseServer)
		mux.Handle(*messagesPath, sseServer)

		logger.Info("Starting MCP server on HTTP",
			slog.String("addr", *addr),
			slog.String("sse_path", *ssePath),
			slog.String("messages_path", *messagesPath),
			slog.String("base_url", baseURL),
		)

		if err := http.ListenAndServe(*addr, mux); err != nil {
			logger.Error("Server error", slog.String("error", err.Error()))
			_, _ = fmt.Fprintf(stderr, "MCP server exited with error: %v\n", err)
			return 1
		}
		return 0
	}

	logger.Info("Starting MCP server on stdio")
	if err := serveStdio(s); err != nil {
		logger.Error("Server error", slog.String("error", err.Error()))
		_, _ = fmt.Fprintf(stderr, "MCP server exited with error: %v\n", err)
		return 1
	}

	return 0
}

func handleK6LookupError(logger *slog.Logger, stderr io.Writer, err error) int {
	if errors.Is(err, k6env.ErrNotFound) {
		message := "mcp-k6 requires the `k6` executable on your PATH. Install k6 " +
			"(https://grafana.com/docs/k6/latest/get-started/installation/) " +
			"and ensure it is accessible before retrying."
		logger.Error("k6 executable not found on PATH", slog.String("hint", message))
		_, _ = fmt.Fprintln(stderr, message)
	} else {
		logger.Error("Failed to locate k6 executable", slog.String("error", err.Error()))
		_, _ = fmt.Fprintf(stderr, "Failed to locate k6 executable: %v\n", err)
	}

	return 1
}
