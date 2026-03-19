// Package main provides the k6 MCP server.
package main

import (
	"context"
	"flag"
	"os"

	"github.com/grafana/mcp-k6/internal/logging"
	"github.com/grafana/mcp-k6/mcpserver"
)

func main() {
	logger := logging.Default()

	cfg := mcpserver.DefaultConfig()

	fs := flag.NewFlagSet("mcp-k6", flag.ContinueOnError)
	//nolint:forbidigo // main must write to stderr.
	fs.SetOutput(os.Stderr)

	fs.StringVar(&cfg.Transport, "transport", cfg.Transport, "Transport mode: stdio or http")
	fs.StringVar(&cfg.Addr, "addr", cfg.Addr, "HTTP address to listen on")
	fs.StringVar(&cfg.Endpoint, "endpoint", cfg.Endpoint, "Endpoint path for HTTP transport")
	fs.BoolVar(&cfg.Stateless, "stateless", cfg.Stateless, "Run in stateless mode (no session tracking)")

	if err := fs.Parse(os.Args[1:]); err != nil {
		//nolint:forbidigo // main must exit with the server status code.
		os.Exit(1)
	}

	//nolint:forbidigo // main must exit with the server status code.
	os.Exit(mcpserver.Run(context.Background(), logger, os.Stderr, cfg))
}
