// Package logging provides helper functions for common logging patterns.
package logging

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
)

// RequestStart logs the beginning of an MCP request
func RequestStart(ctx context.Context, toolName string, params map[string]interface{}) {
	logger := WithTool(toolName)

	// Sanitize parameters for logging (exclude large script content)
	sanitizedParams := sanitizeParams(params)

	logger.InfoContext(ctx, "MCP request started",
		slog.Any("params", sanitizedParams),
		slog.Time("start_time", time.Now()),
	)
}

// RequestEnd logs the completion of an MCP request
func RequestEnd(ctx context.Context, toolName string, success bool, duration time.Duration, err error) {
	logger := WithTool(toolName)

	if err != nil {
		logger.ErrorContext(ctx, "MCP request failed",
			slog.Bool("success", success),
			slog.Duration("duration", duration),
			slog.Int64("duration_ms", duration.Milliseconds()),
			slog.String("error", err.Error()),
			slog.String("error_type", getErrorType(err)),
		)
	} else {
		logger.InfoContext(ctx, "MCP request completed",
			slog.Bool("success", success),
			slog.Duration("duration", duration),
			slog.Int64("duration_ms", duration.Milliseconds()),
		)
	}
}

// ValidationEvent logs validation-related events
func ValidationEvent(ctx context.Context, event string, success bool, details map[string]interface{}) {
	logger := WithComponent("validator")

	if success {
		if details != nil {
			logger.DebugContext(ctx, "Validation event",
				slog.String("event", event),
				slog.Bool("success", success),
				slog.Any("details", details),
			)
		} else {
			logger.DebugContext(ctx, "Validation event",
				slog.String("event", event),
				slog.Bool("success", success),
			)
		}
	} else {
		if details != nil {
			logger.WarnContext(ctx, "Validation event",
				slog.String("event", event),
				slog.Bool("success", success),
				slog.Any("details", details),
			)
		} else {
			logger.WarnContext(ctx, "Validation event",
				slog.String("event", event),
				slog.Bool("success", success),
			)
		}
	}
}

// SecurityEvent logs security-related events
func SecurityEvent(ctx context.Context, eventType string, severity string, message string, details map[string]interface{}) {
	logger := WithComponent("security")

	switch severity {
	case "critical", "high":
		if details != nil {
			logger.ErrorContext(ctx, message,
				slog.String("event_type", eventType),
				slog.String("severity", severity),
				slog.Any("details", details),
			)
		} else {
			logger.ErrorContext(ctx, message,
				slog.String("event_type", eventType),
				slog.String("severity", severity),
			)
		}
	case "medium":
		if details != nil {
			logger.WarnContext(ctx, message,
				slog.String("event_type", eventType),
				slog.String("severity", severity),
				slog.Any("details", details),
			)
		} else {
			logger.WarnContext(ctx, message,
				slog.String("event_type", eventType),
				slog.String("severity", severity),
			)
		}
	default:
		if details != nil {
			logger.InfoContext(ctx, message,
				slog.String("event_type", eventType),
				slog.String("severity", severity),
				slog.Any("details", details),
			)
		} else {
			logger.InfoContext(ctx, message,
				slog.String("event_type", eventType),
				slog.String("severity", severity),
			)
		}
	}
}

// ExecutionEvent logs command execution events
func ExecutionEvent(ctx context.Context, component string, command string, duration time.Duration, exitCode int, err error) {
	logger := WithComponent(component)

	if err != nil {
		logger.ErrorContext(ctx, "Command execution failed",
			slog.String("command", command),
			slog.Duration("duration", duration),
			slog.Int64("duration_ms", duration.Milliseconds()),
			slog.Int("exit_code", exitCode),
			slog.String("error", err.Error()),
			slog.String("error_type", getErrorType(err)),
		)
	} else {
		logger.DebugContext(ctx, "Command executed",
			slog.String("command", command),
			slog.Duration("duration", duration),
			slog.Int64("duration_ms", duration.Milliseconds()),
			slog.Int("exit_code", exitCode),
		)
	}
}

// FileOperation logs file-related operations
func FileOperation(ctx context.Context, component string, operation string, path string, err error) {
	logger := WithComponent(component)

	if err != nil {
		logger.ErrorContext(ctx, "File operation failed",
			slog.String("operation", operation),
			slog.String("path_type", getPathType(path)), // Avoid logging full paths
			slog.String("error", err.Error()),
			slog.String("error_type", getErrorType(err)),
		)
	} else {
		logger.DebugContext(ctx, "File operation completed",
			slog.String("operation", operation),
			slog.String("path_type", getPathType(path)), // Avoid logging full paths
		)
	}
}

// sanitizeParams removes or truncates large parameters for logging
func sanitizeParams(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return nil
	}

	sanitized := make(map[string]interface{})

	for key, value := range params {
		switch key {
		case "script":
			// For script content, only log metadata
			if str, ok := value.(string); ok {
				sanitized[key] = map[string]interface{}{
					"length":      len(str),
					"has_content": len(str) > 0,
				}
			}
		default:
			sanitized[key] = value
		}
	}

	return sanitized
}

// getErrorType extracts error type for classification
func getErrorType(err error) string {
	if err == nil {
		return ""
	}

	// Try to extract error type from custom error types
	{
		var e interface{ Error() string }
		switch {
		case errors.As(err, &e):
			errStr := e.Error()
			if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline") {
				return "timeout"
			} else if strings.Contains(errStr, "security") {
				return "security"
			} else if strings.Contains(errStr, "validation") {
				return "validation"
			} else if strings.Contains(errStr, "execution") {
				return "execution"
			}
		}
	}

	return "unknown"
}

// getPathType returns a safe representation of file paths
func getPathType(path string) string {
	if strings.Contains(path, "temp") || strings.Contains(path, "tmp") {
		return "temporary"
	} else if strings.HasSuffix(path, ".js") {
		return "javascript"
	} else if strings.HasSuffix(path, ".ts") {
		return "typescript"
	}
	return "other"
}
