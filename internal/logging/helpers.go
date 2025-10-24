// Package logging provides helper functions for common logging patterns.
package logging

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
)

// RequestStart logs the beginning of an MCP request.
func RequestStart(ctx context.Context, toolName string, params map[string]any) {
	logger := WithTool(toolName)

	// Sanitize parameters for logging (exclude large script content)
	sanitizedParams := sanitizeParams(params)

	logger.InfoContext(ctx, "MCP request started",
		slog.Any("params", sanitizedParams),
		slog.Time("start_time", time.Now()),
	)
}

// RequestEnd logs the completion of an MCP request.
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

// ValidationEvent logs validation-related events.
func ValidationEvent(ctx context.Context, event string, success bool, details map[string]any) {
	logger := WithComponent("validator")

	attrs := []any{
		slog.String("event", event),
		slog.Bool("success", success),
	}
	if details != nil {
		attrs = append(attrs, slog.Any("details", details))
	}

	if success {
		logger.DebugContext(ctx, "Validation event", attrs...)
		return
	}
	logger.WarnContext(ctx, "Validation event", attrs...)
}

// SecurityEvent logs security-related events.
func SecurityEvent(ctx context.Context, eventType string, severity string, message string, details map[string]any) {
	logger := WithComponent("security")

	attrs := []any{
		slog.String("event_type", eventType),
		slog.String("severity", severity),
	}
	if details != nil {
		attrs = append(attrs, slog.Any("details", details))
	}

	switch severity {
	case "critical", "high":
		logger.ErrorContext(ctx, message, attrs...)
	case "medium":
		logger.WarnContext(ctx, message, attrs...)
	default:
		logger.InfoContext(ctx, message, attrs...)
	}
}

// ExecutionEvent logs command execution events.
func ExecutionEvent(
	ctx context.Context,
	component string,
	command string,
	duration time.Duration,
	exitCode int,
	err error,
) {
	logger := WithComponent(component)

	attrs := []any{
		slog.String("command", command),
		slog.Duration("duration", duration),
		slog.Int64("duration_ms", duration.Milliseconds()),
		slog.Int("exit_code", exitCode),
	}
	if err != nil {
		attrs = append(attrs,
			slog.String("error", err.Error()),
			slog.String("error_type", getErrorType(err)),
		)
		logger.ErrorContext(ctx, "Command execution failed", attrs...)
		return
	}
	logger.DebugContext(ctx, "Command executed", attrs...)
}

// FileOperation logs file-related operations.
func FileOperation(ctx context.Context, component string, operation string, path string, err error) {
	logger := WithComponent(component)

	attrs := []any{
		slog.String("operation", operation),
		slog.String("path_type", getPathType(path)), // Avoid logging full paths
	}
	if err != nil {
		attrs = append(attrs,
			slog.String("error", err.Error()),
			slog.String("error_type", getErrorType(err)),
		)
		logger.ErrorContext(ctx, "File operation failed", attrs...)
		return
	}
	logger.DebugContext(ctx, "File operation completed", attrs...)
}

// sanitizeParams removes or truncates large parameters for logging.
func sanitizeParams(params map[string]any) map[string]any {
	if params == nil {
		return nil
	}

	sanitized := make(map[string]any)

	for key, value := range params {
		switch key {
		case "script":
			// For script content, only log metadata
			if str, ok := value.(string); ok {
				sanitized[key] = map[string]any{
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

// getErrorType extracts error type for classification.
func getErrorType(err error) string {
	if err == nil {
		return ""
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}

	errStr := err.Error()
	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline") {
		return "timeout"
	}
	if strings.Contains(errStr, "security") {
		return "security"
	}
	if strings.Contains(errStr, "validation") {
		return "validation"
	}
	if strings.Contains(errStr, "execution") {
		return "execution"
	}

	return "unknown"
}

// getPathType returns a safe representation of file paths.
func getPathType(path string) string {
	if strings.Contains(path, "temp") || strings.Contains(path, "tmp") {
		return "temporary"
	}
	if strings.HasSuffix(path, ".js") {
		return "javascript"
	}
	if strings.HasSuffix(path, ".ts") {
		return "typescript"
	}
	return "other"
}
