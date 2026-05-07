// Package k6mcp registers mcp-k6 as a k6 subcommand extension ("k6 x mcp").
// Import this package via xk6 to activate the subcommand.
package k6mcp

import (
	"os"

	"github.com/spf13/cobra"
	"go.k6.io/k6/v2/cmd/state"
	"go.k6.io/k6/v2/subcommand"

	"github.com/grafana/mcp-k6/internal/logging"
	"github.com/grafana/mcp-k6/mcpserver"
)

func init() {
	subcommand.RegisterExtension("mcp", newCommand)
}

func newCommand(gs *state.GlobalState) *cobra.Command {
	cfg := mcpserver.DefaultConfig()
	logger := logging.NewLogrusLogger(gs.Logger)
	logging.SetDefault(logger)

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server",
		Long: `An experimental MCP server for k6.

The Model Context Protocol server offers script validation, test execution,
documentation browsing, and guided script generation.`,
		Run: func(cmd *cobra.Command, _ []string) {
			//nolint:forbidigo // subcommand must propagate the server exit code
			os.Exit(mcpserver.Run(cmd.Context(), logger, gs.Stderr, cfg))
		},
	}

	cmd.Flags().StringVar(&cfg.Transport, "transport", cfg.Transport, "Transport mode: stdio or http")
	cmd.Flags().StringVar(&cfg.Addr, "addr", cfg.Addr, "HTTP address to listen on")
	cmd.Flags().StringVar(&cfg.Endpoint, "endpoint", cfg.Endpoint, "Endpoint path for HTTP transport")
	cmd.Flags().BoolVar(&cfg.Stateless, "stateless", cfg.Stateless, "Run in stateless mode (no session tracking)")
	cmd.Flags().BoolVar(&cfg.Preload, "preload", cfg.Preload, "Download all documentation bundles at startup")

	return cmd
}
