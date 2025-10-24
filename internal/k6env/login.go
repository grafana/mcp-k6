package k6env

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// IsLoggedIn checks whether the k6 executable has an active k6 Cloud login.
func (i Info) IsLoggedIn(ctx context.Context) (bool, error) {
	if i.Path == "" {
		return false, errors.New("k6 executable path is empty")
	}

	// #nosec G204 -- i.Path is obtained from Locate and points to a trusted executable
	cmd := exec.CommandContext(ctx, i.Path, "cloud", "login", "--show")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to get k6 version: %w", err)
	}

	raw := strings.TrimSpace(string(output))

	re := regexp.MustCompile(`(?m)^\s*token:\s*([0-9a-fA-F]{64})\s*$`)
	if !re.MatchString(raw) {
		return false, fmt.Errorf("missing token line in output:\n%s", raw)
	}
	token := re.FindStringSubmatch(raw)[1]

	return token != "", nil
}
