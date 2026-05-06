// Package k6extension registers mcp-k6 as a "mcp" subcommand in a custom k6 binary.
// Import this package with a blank identifier to activate the registration:
//
//	import _ "github.com/grafana/mcp-k6/k6extension"
package k6extension

import (
	"os" //nolint:forbidigo // subcommand must propagate the server exit code

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

	cmd.Flags().StringVarP(&cfg.Transport, "transport", "t", cfg.Transport, "Transport mode: stdio or http")
	cmd.Flags().StringVarP(&cfg.Addr, "addr", "a", cfg.Addr, "HTTP address to listen on")
	cmd.Flags().StringVarP(&cfg.Endpoint, "endpoint", "e", cfg.Endpoint, "Endpoint path for HTTP transport")
	cmd.Flags().BoolVarP(&cfg.Stateless, "stateless", "s", cfg.Stateless, "Run in stateless mode (no session tracking)")
	cmd.Flags().BoolVarP(&cfg.Preload, "preload", "p", cfg.Preload, "Download all documentation bundles at startup")

	return cmd
}
