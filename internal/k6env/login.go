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
		return false, fmt.Errorf("failed to check k6 cloud login status: %w", err)
	}

	return parseCloudLoginStatus(string(output))
}

// tokenLineRE captures the value of the "token:" line printed by
// `k6 cloud login --show` (e.g. "  token: <not set>" or "  token: <value>").
var tokenLineRE = regexp.MustCompile(`(?m)^\s*token:\s*(.*?)\s*$`)

// parseCloudLoginStatus interprets the output of `k6 cloud login --show`.
//
// Being logged out is a valid state, not an error: k6 prints "token: <not set>"
// (and exits 0) when no token is stored. Only genuinely unrecognizable output
// (no token line at all) is treated as an error.
func parseCloudLoginStatus(output string) (bool, error) {
	raw := strings.TrimSpace(output)

	match := tokenLineRE.FindStringSubmatch(raw)
	if match == nil {
		return false, errors.New("unable to determine k6 cloud login status: unexpected output format")
	}

	token := strings.TrimSpace(match[1])
	if token == "" || token == "<not set>" {
		return false, nil
	}

	return true, nil
}
