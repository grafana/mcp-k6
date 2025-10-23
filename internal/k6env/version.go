package k6env

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Version executes "k6 version" using the resolved executable path.
// It returns the trimmed stdout string (e.g., "k6 v0.47.0 (...)").
func (i Info) Version(ctx context.Context) (string, error) {
	if i.Path == "" {
		return "", errors.New("k6 executable path is empty")
	}

	// #nosec G204 -- i.Path is obtained from Locate and points to a trusted executable
	cmd := exec.CommandContext(ctx, i.Path, "version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get k6 version: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}
