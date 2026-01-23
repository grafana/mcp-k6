package tools

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/grafana/mcp-k6/internal/logging"
)

const (
	// DefaultTimeout is the default timeout for k6 operations.
	DefaultTimeout = 5 * time.Minute
)

// createSecureTempFile creates a secure temporary file with the script content.
func createSecureTempFile(script string) (string, func(), error) {
	//nolint:forbidigo // Temporary file creation required for k6 execution
	tmpFile, err := os.CreateTemp("", "k6-run-*.js")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temporary file: %w", err)
	}

	filename := tmpFile.Name()
	cleanup := func() {
		//nolint:forbidigo // Cleanup of temporary file required
		if removeErr := os.Remove(filename); removeErr != nil {
			logging.WithComponent("runner").Warn("Failed to remove temporary file",
				slog.String("operation", "cleanup"),
				slog.String("error", removeErr.Error()),
			)
		}
	}

	if err := setupTempFile(tmpFile, script); err != nil {
		cleanupTempFile(tmpFile)
		return "", nil, err
	}

	return filename, cleanup, nil
}

// setupTempFile configures and writes to the temporary file.
//
//nolint:forbidigo // Function parameter os.File required for temp file operations
func setupTempFile(tmpFile *os.File, script string) error {
	// Set secure permissions (owner read/write only)
	const secureFileMode = 0o600
	if err := tmpFile.Chmod(secureFileMode); err != nil {
		return fmt.Errorf("failed to set secure file permissions: %w", err)
	}

	// Write script content
	if _, err := tmpFile.WriteString(script); err != nil {
		return fmt.Errorf("failed to write script to temporary file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}

	return nil
}

// cleanupTempFile safely cleans up a temporary file.
//
//nolint:forbidigo // Function parameter os.File required for temp file operations
func cleanupTempFile(tmpFile *os.File) {
	logger := logging.WithComponent("runner")

	if closeErr := tmpFile.Close(); closeErr != nil {
		logger.Warn("Failed to close temp file",
			slog.String("operation", "cleanup"),
			slog.String("error", closeErr.Error()),
		)
	}
	//nolint:forbidigo // Cleanup of temporary file required
	if removeErr := os.Remove(tmpFile.Name()); removeErr != nil {
		logger.Warn("Failed to remove temp file",
			slog.String("operation", "cleanup"),
			slog.String("error", removeErr.Error()),
		)
	}
}

// executeCommand executes a command and returns stdout, stderr, exit code, and error.
func executeCommand(cmd *exec.Cmd) (stdout, stderr string, exitCode int, err error) {
	var stdoutBuf, stderrBuf strings.Builder
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	if err != nil {
		return stdout, stderr, exitCode, fmt.Errorf("command execution failed: %w", err)
	}
	return stdout, stderr, exitCode, nil
}
