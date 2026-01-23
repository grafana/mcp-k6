// Package prompts provides MCP prompt definitions for the mcp-k6 server.
package prompts

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	k6mcp "github.com/grafana/mcp-k6"
	"github.com/grafana/mcp-k6/internal/logging"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ConvertPlaywrightScriptPrompt is the MCP prompt definition for Playwright to k6 conversion.
//
//nolint:gochecknoglobals // Shared prompt definition registered at startup.
var ConvertPlaywrightScriptPrompt = mcp.NewPrompt(
	"convert_playwright_script",
	mcp.WithPromptDescription("Convert a Playwright script to its equivalent k6 script leveraging the browser module."),
	mcp.WithArgument(
		"playwright_script",
		mcp.ArgumentDescription("The Playwright script to convert (JavaScript or TypeScript) "+
			"into a k6 browser script. Accepts raw text or a file path."),
	),
)

// RegisterConvertPlaywrightScriptPrompt registers the convert_playwright_script prompt with the MCP server.
func RegisterConvertPlaywrightScriptPrompt(s *server.MCPServer) {
	s.AddPrompt(ConvertPlaywrightScriptPrompt, withPromptLogger("convert_playwright_script", convertPlaywrightScript))
}

// convertPlaywrightScript handles prompt requests to convert Playwright scripts to k6/browser scripts.
func convertPlaywrightScript(
	ctx context.Context,
	request mcp.GetPromptRequest,
) (*mcp.GetPromptResult, error) {
	logger := logging.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Starting playwright script conversion prompt")

	playwrightScript, err := extractPlaywrightScript(ctx, request)
	if err != nil {
		return nil, err
	}

	templateContent, err := k6mcp.Prompts.ReadFile("prompts/convert_playwright_script.md")
	if err != nil {
		logger.ErrorContext(ctx, "Failed to read embedded prompt template",
			slog.String("error", err.Error()))
		return nil, fmt.Errorf("failed to read embedded prompt template: %w", err)
	}

	promptText := strings.Replace(string(templateContent), "{{.PlaywrightScript}}", playwrightScript, 1)

	result := mcp.NewGetPromptResult(
		"A Playwright script converted to a k6 script",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleAssistant,
				mcp.NewTextContent(promptText),
			),
		},
	)

	logger.InfoContext(ctx, "Playwright script conversion prompt completed successfully",
		slog.Int("prompt_length", len(promptText)))

	return result, nil
}

func extractPlaywrightScript(
	ctx context.Context,
	request mcp.GetPromptRequest,
) (string, error) {
	logger := logging.LoggerFromContext(ctx)

	playwrightScript, exists := request.Params.Arguments["playwright_script"]
	if !exists {
		logger.WarnContext(ctx, "Missing required parameter 'playwright_script'")
		return "", fmt.Errorf(
			"missing required parameter 'playwright_script'. " +
				"Provide the script text directly or reference a file path prefixed with '@'",
		)
	}

	if strings.TrimSpace(playwrightScript) == "" {
		logger.WarnContext(ctx, "Empty playwright script parameter")
		return "", fmt.Errorf(
			"'playwright_script' parameter cannot be empty. " +
				"Provide the script text directly or reference a file path prefixed with '@'",
		)
	}

	resolvedScript, err := resolvePlaywrightScriptArgument(ctx, playwrightScript)
	if err != nil {
		logger.WarnContext(ctx, "Failed to resolve playwright script argument",
			slog.String("error", err.Error()))
		return "", err
	}

	if strings.TrimSpace(resolvedScript) == "" {
		return "", fmt.Errorf("resolved Playwright script content is empty")
	}

	return resolvedScript, nil
}

func resolvePlaywrightScriptArgument(ctx context.Context, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}

	if strings.HasPrefix(trimmed, "@") {
		path := strings.TrimSpace(strings.TrimPrefix(trimmed, "@"))
		if path == "" {
			return "", fmt.Errorf("file reference prefixed with '@' must include a path")
		}
		return readPlaywrightScriptFromFile(ctx, path)
	}

	if !strings.ContainsAny(trimmed, "\r\n") {
		script, ok, err := tryReadPlaywrightScriptFromFile(ctx, trimmed)
		if err != nil {
			return "", err
		}
		if ok {
			return script, nil
		}
	}

	return value, nil
}

//nolint:forbidigo // Controlled file access required for prompt inputs.
func readPlaywrightScriptFromFile(ctx context.Context, path string) (string, error) {
	normalizedPath, err := normalizeFilePath(path)
	if err != nil {
		return "", fmt.Errorf("invalid file path %q: %w", path, err)
	}

	// Resolve to absolute path and ensure it doesn't escape working directory
	absPath, err := filepath.Abs(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path for %q: %w", normalizedPath, err)
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Ensure the resolved path is within allowed directory, to prevent path traversal attacks
	if !strings.HasPrefix(absPath, cwd+string(filepath.Separator)) && absPath != cwd {
		return "", fmt.Errorf("file path must be within current working directory")
	}

	// #nosec G304 -- normalizedPath is sanitized before file access.
	data, err := os.ReadFile(normalizedPath)
	if err != nil {
		return "", fmt.Errorf("failed to read Playwright script file %q: %w", normalizedPath, err)
	}

	logger := logging.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Loaded Playwright script from file",
		slog.String("path", normalizedPath),
		slog.Int("bytes", len(data)))

	return string(data), nil
}

//nolint:forbidigo
func tryReadPlaywrightScriptFromFile(ctx context.Context, candidate string) (string, bool, error) {
	normalizedPath, err := normalizeFilePath(candidate)
	if err != nil {
		return "", false, fmt.Errorf("invalid candidate path %q: %w", candidate, err)
	}

	info, err := os.Stat(normalizedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to inspect candidate file %q: %w", normalizedPath, err)
	}

	if info.IsDir() {
		return "", false, nil
	}

	// #nosec G304 -- normalizedPath is sanitized before file access.
	data, err := os.ReadFile(normalizedPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to read candidate script file %q: %w", normalizedPath, err)
	}

	logger := logging.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Loaded Playwright script from implicit file reference",
		slog.String("path", normalizedPath),
		slog.Int("bytes", len(data)))

	return string(data), true, nil
}

func normalizeFilePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	trimmed = strings.Trim(trimmed, "\"'")

	if trimmed == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}

	if strings.HasPrefix(trimmed, "~") {
		home, err := resolveHomeDir()
		if err != nil {
			return "", fmt.Errorf("unable to resolve home directory: %w", err)
		}

		trimmed = filepath.Join(home, strings.TrimPrefix(trimmed, "~"))
	}

	return filepath.Clean(trimmed), nil
}

//nolint:forbidigo // HOME resolution relies on environment variables.
func resolveHomeDir() (string, error) {
	if home := os.Getenv("HOME"); home != "" {
		return home, nil
	}

	if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
		return userProfile, nil
	}

	drive := os.Getenv("HOMEDRIVE")
	path := os.Getenv("HOMEPATH")
	if drive != "" && path != "" {
		return filepath.Join(drive, path), nil
	}

	return "", fmt.Errorf("home directory not set in environment")
}
