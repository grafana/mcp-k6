// Package logging provides structured logging functionality for the k6 MCP server.
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/grafana/k6-mcp/internal/buildinfo"
)

const (
	// ServiceName is the service identifier for Loki labels
	ServiceName = "k6-mcp"
)

type contextKey string

const (
	// requestIDKey is the context key for request correlation
	requestIDKey contextKey = "request_id"
)

// defaultLogger is the package-level logger instance
//
//nolint:gochecknoglobals // Package-level logger singleton initialized in init()
var defaultLogger *slog.Logger

// LogConfig holds logging configuration
type LogConfig struct {
	Level  slog.Level
	Format string // "json" or "text"
}

// init initializes the default logger based on environment variables
func init() {
	config := getConfigFromEnv()
	defaultLogger = newLogger(config)
}

// getConfigFromEnv reads logging configuration from environment variables
func getConfigFromEnv() LogConfig {
	config := LogConfig{
		Level:  slog.LevelInfo, // Default level
		Format: "json",         // Default to JSON for Loki compatibility
	}

	// Parse LOG_LEVEL environment variable
	//nolint:forbidigo // Logger configuration requires reading environment variables
	if levelStr := os.Getenv("LOG_LEVEL"); levelStr != "" {
		switch strings.ToUpper(levelStr) {
		case "DEBUG":
			config.Level = slog.LevelDebug
		case "INFO":
			config.Level = slog.LevelInfo
		case "WARN", "WARNING":
			config.Level = slog.LevelWarn
		case "ERROR":
			config.Level = slog.LevelError
		}
	}

	// Parse LOG_FORMAT environment variable
	//nolint:forbidigo // Logger configuration requires reading environment variables
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		if strings.ToLower(format) == "text" {
			config.Format = "text"
		}
	}

	return config
}

// newLogger creates a new slog.Logger with the given configuration
func newLogger(config LogConfig) *slog.Logger {
	var handler slog.Handler

	handlerOpts := &slog.HandlerOptions{
		Level: config.Level,
	}

	//nolint:forbidigo // Logger needs to write to stderr
	if config.Format == "text" {
		handler = slog.NewTextHandler(os.Stderr, handlerOpts)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, handlerOpts)
	}

	// Create logger with service-level attributes
	return slog.New(handler).With(
		slog.String("service", ServiceName),
		slog.String("version", buildinfo.Version),
	)
}

// Default returns the default logger instance
func Default() *slog.Logger {
	return defaultLogger
}

// WithContext returns a logger with context-specific attributes
func WithContext(ctx context.Context) *slog.Logger {
	logger := defaultLogger

	// Add request ID if available in context
	if requestID := ctx.Value(requestIDKey); requestID != nil {
		if id, ok := requestID.(string); ok {
			logger = logger.With(slog.String("request_id", id))
		}
	}

	return logger
}

// WithComponent returns a logger with component-specific attributes
func WithComponent(component string) *slog.Logger {
	return defaultLogger.With(slog.String("component", component))
}

// WithTool returns a logger with tool-specific attributes for MCP requests
func WithTool(toolName string) *slog.Logger {
	return defaultLogger.With(
		slog.String("tool", toolName),
		slog.String("component", "mcp"),
	)
}

// ContextWithRequestID adds a request ID to the context for log correlation
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}
