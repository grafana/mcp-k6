// Package mcpserver provides the k6 MCP server as a reusable library.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/server"

	"github.com/grafana/mcp-k6/internal/buildinfo"
	"github.com/grafana/mcp-k6/internal/k6env"
	"github.com/grafana/mcp-k6/prompts"
	"github.com/grafana/mcp-k6/resources"
	"github.com/grafana/mcp-k6/tools"
	"github.com/grafana/xk6-docs/docs"
)

// instructions is a high-level overview of the tools and resources available.
const instructions = `
Use the provided tools for running or validating k6 scripts, for browsing the k6 OSS docs, or
for searching for k6 Cloud-related Terraform resources in the Grafana Terraform provider.
Use the provided resources for understanding the k6 script authoring best practices and for consulting
type definitions.
List the resources at least once before trying to access one of them.
Use the provided prompts as a good starting point for authoring complex k6 scripts.
`

// Config holds the MCP server configuration.
type Config struct {
	Transport string // "stdio" or "http" (default: "stdio")
	Addr      string // HTTP listen address (default: ":8080")
	Endpoint  string // HTTP endpoint path (default: "/mcp")
	Stateless bool   // Stateless mode for HTTP
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		Transport: "stdio",
		Addr:      ":8080",
		Endpoint:  "/mcp",
	}
}

// Option configures the server runner.
type Option func(*runner)

// WithServeStdio overrides the function used to serve the MCP server over stdio.
// This is primarily useful for testing.
func WithServeStdio(fn func(*server.MCPServer, ...server.StdioOption) error) Option {
	return func(r *runner) {
		r.serveStdio = fn
	}
}

type runner struct {
	serveStdio func(*server.MCPServer, ...server.StdioOption) error
}

// Run starts the MCP server with the given configuration. It blocks until the
// server exits and returns the exit code (0 for success, 1 for failure).
//
// If logger is nil, a default logger is created. If stderr is nil, os.Stderr is used.
func Run(ctx context.Context, logger *slog.Logger, stderr io.Writer, cfg Config, opts ...Option) int {
	if logger == nil {
		logger = slog.Default()
	}
	if stderr == nil {
		//nolint:forbidigo // Need a default stderr writer.
		stderr = os.Stderr
	}

	r := &runner{
		serveStdio: server.ServeStdio,
	}
	for _, opt := range opts {
		opt(r)
	}

	switch cfg.Transport {
	case "stdio", "http":
	default:
		logger.Error("Unsupported transport", slog.String("transport", cfg.Transport))
		_, _ = fmt.Fprintf(stderr, "unsupported transport %q (must be \"stdio\" or \"http\")\n", cfg.Transport)
		return 1
	}

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

	catalog := docs.NewCatalog()

	s := createServer(catalog)

	if cfg.Transport == "http" {
		return r.serveHTTP(logger, stderr, s, cfg)
	}

	logger.Info("Starting MCP server on stdio")
	if err := r.serveStdio(s); err != nil {
		logger.Error("Server error", slog.String("error", err.Error()))
		_, _ = fmt.Fprintf(stderr, "MCP server exited with error: %v\n", err)
		return 1
	}

	return 0
}

func (r *runner) serveHTTP(logger *slog.Logger, stderr io.Writer, s *server.MCPServer, cfg Config) int {
	httpOpts := []server.StreamableHTTPOption{
		server.WithEndpointPath(cfg.Endpoint),
	}

	if cfg.Stateless {
		httpOpts = append(httpOpts, server.WithStateLess(true))
	}

	httpServer := server.NewStreamableHTTPServer(s, httpOpts...)

	logger.Info("Starting MCP server with Streamable HTTP",
		slog.String("addr", cfg.Addr),
		slog.String("endpoint", cfg.Endpoint),
		slog.Bool("stateless", cfg.Stateless),
	)

	if err := httpServer.Start(cfg.Addr); err != nil {
		logger.Error("Server error", slog.String("error", err.Error()))
		_, _ = fmt.Fprintf(stderr, "MCP server exited with error: %v\n", err)
		return 1
	}
	return 0
}

func createServer(catalog *docs.Catalog) *server.MCPServer {
	s := server.NewMCPServer(
		"k6",
		buildinfo.Version,
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
		server.WithRecovery(),
		server.WithInstructions(instructions),
	)

	tools.RegisterInfoTool(s)
	tools.RegisterValidateTool(s)
	tools.RegisterRunTool(s)
	tools.RegisterSearchTerraformTool(s)
	tools.RegisterListSectionsTool(s, catalog)
	tools.RegisterGetDocumentationTool(s, catalog)

	resources.RegisterBestPracticesResource(s)
	resources.RegisterTypeDefinitionsResources(s)

	prompts.RegisterGenerateScriptPrompt(s)
	prompts.RegisterConvertPlaywrightScriptPrompt(s)

	return s
}

func handleK6LookupError(logger *slog.Logger, stderr io.Writer, err error) int {
	if errors.Is(err, k6env.ErrNotFound) {
		message := "mcp-k6 requires the `k6` executable on your PATH. Install k6 " +
			"(https://grafana.com/docs/k6/latest/get-started/installation/) " +
			"and ensure it is accessible before retrying."
		logger.Error("k6 executable not found on PATH", slog.String("hint", message))
		_, _ = fmt.Fprintln(stderr, message)
		return 1
	}

	logger.Error("Failed to locate k6 executable", slog.String("error", err.Error()))
	_, _ = fmt.Fprintf(stderr, "Failed to locate k6 executable: %v\n", err)
	return 1
}
