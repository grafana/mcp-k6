package prompts

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/grafana/mcp-k6/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// promptHandler is the function signature for MCP prompt handlers.
type promptHandler func(context.Context, mcp.GetPromptRequest) (*mcp.GetPromptResult, error)

// withPromptLogger wraps a prompt handler to inject a logger into context and provide panic recovery.
// The logger is configured with the prompt name and made available via logging.LoggerFromContext.
func withPromptLogger(promptName string, handler promptHandler) server.PromptHandlerFunc {
	return func(ctx context.Context, request mcp.GetPromptRequest) (result *mcp.GetPromptResult, err error) {
		// Create prompt-specific logger and add to context
		logger := logging.WithPrompt(promptName)
		ctx = logging.ContextWithLogger(ctx, logger)

		// Panic recovery with logging
		defer func() {
			if r := recover(); r != nil {
				logger.ErrorContext(ctx, "panic in prompt execution",
					slog.String("prompt", promptName),
					slog.Any("panic", r))
				result = nil
				err = fmt.Errorf("internal error in prompt execution: %s", r)
			}
		}()

		return handler(ctx, request)
	}
}
