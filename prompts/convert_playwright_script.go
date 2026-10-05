// Package prompts provides MCP prompt definitions for the mcp-k6 server.
package prompts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

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
			"into a k6 browser script. Accepts the script text, or a file reference prefixed with '@' "+
			"(for example '@tests/login.spec.ts'). File references must point to a "+
			".js, .mjs, .cjs, .ts, .mts or .cts file inside the server's working directory; "+
			"'~' expands to the home directory."),
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

	templateContent, err := promptFiles.ReadFile("convert_playwright_script.md")
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

	if !strings.HasPrefix(trimmed, "@") {
		return value, nil
	}

	path := strings.TrimSpace(strings.TrimPrefix(trimmed, "@"))
	if path == "" {
		return "", fmt.Errorf("file reference prefixed with '@' must include a path")
	}

	return readScriptFileInWorkingDirectory(ctx, path)
}

// allowedScriptExtensions lists the file extensions that may be read through
// an '@' file reference. Restricting reads to script files keeps credentials
// and other secrets out of reach when the working directory contains them,
// for example when the server runs from the user's home directory.
//
//nolint:gochecknoglobals // Read-only lookup table.
var allowedScriptExtensions = map[string]bool{
	".js": true, ".mjs": true, ".cjs": true,
	".ts": true, ".mts": true, ".cts": true,
}

func hasScriptExtension(path string) bool {
	return allowedScriptExtensions[strings.ToLower(filepath.Ext(path))]
}

// readScriptFileInWorkingDirectory reads a script file referenced by path.
// The file must be inside the current working directory, and both the
// requested name and the name of the final symlink target must have a script
// extension.
//
// Containment is enforced by os.Root, which resolves the path relative to a
// handle on the working directory and refuses to leave it, including through
// symlinks.
//
//nolint:forbidigo // Controlled file access required for prompt inputs.
func readScriptFileInWorkingDirectory(ctx context.Context, path string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	rel, err := workingDirectoryRelativePath(cwd, path)
	if err != nil {
		return "", fmt.Errorf("invalid file reference %q: %w", path, err)
	}

	if !hasScriptExtension(rel) {
		return "", fmt.Errorf("invalid file reference %q: %w", path, errNotAScriptFile)
	}

	root, err := os.OpenRoot(cwd)
	if err != nil {
		return "", fmt.Errorf("failed to open current working directory: %w", err)
	}
	defer func() { _ = root.Close() }()

	// Opening through the root first means paths that escape the working
	// directory always fail here, regardless of what they point to.
	file, err := root.Open(rel)
	if err != nil {
		return "", fmt.Errorf("failed to read Playwright script file %q: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	// Check the extension of the final symlink target as well, so that a
	// script-named symlink cannot be used to read another file inside the
	// working directory. Containment does not depend on this check.
	resolved, err := filepath.EvalSymlinks(filepath.Join(cwd, rel))
	if err != nil {
		return "", fmt.Errorf("failed to read Playwright script file %q: %w", path, err)
	}
	if !hasScriptExtension(resolved) {
		return "", fmt.Errorf("invalid file reference %q: %w", path, errNotAScriptFile)
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read Playwright script file %q: %w", path, err)
	}

	logger := logging.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Loaded Playwright script from file",
		slog.String("path", rel),
		slog.Int("bytes", len(data)))

	return string(data), nil
}

var (
	errOutsideWorkingDirectory = errors.New("file path must be within current working directory")
	errNotAScriptFile          = errors.New("file must have a .js, .mjs, .cjs, .ts, .mts or .cts extension")
)

// workingDirectoryRelativePath normalizes path and converts absolute paths to
// paths relative to cwd, because os.Root only accepts relative names. Whether
// the result stays inside cwd is enforced by os.Root when the file is opened.
func workingDirectoryRelativePath(cwd, path string) (string, error) {
	normalized, err := normalizeFilePath(path)
	if err != nil {
		return "", err
	}

	if filepath.IsAbs(normalized) {
		rel, err := filepath.Rel(cwd, normalized)
		if err != nil {
			return "", errOutsideWorkingDirectory
		}
		normalized = rel
	}

	return normalized, nil
}

func normalizeFilePath(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	trimmed = strings.Trim(trimmed, "\"'")

	if trimmed == "" {
		return "", fmt.Errorf("file path cannot be empty")
	}

	//nolint:forbidigo // os.IsPathSeparator is a pure helper and knows the platform's separators.
	if trimmed == "~" || (len(trimmed) > 1 && trimmed[0] == '~' && os.IsPathSeparator(trimmed[1])) {
		home, err := resolveHomeDir()
		if err != nil {
			return "", fmt.Errorf("unable to resolve home directory: %w", err)
		}

		trimmed = filepath.Join(home, trimmed[1:])
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
