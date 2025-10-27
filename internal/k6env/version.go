package k6env

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// Version executes "k6 version" using the resolved executable path.
// It returns only the semantic version (e.g., "1.3.0"). If no semver is found,
// the raw trimmed output is returned.
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

	raw := strings.TrimSpace(string(output))

	// Extract semantic version (e.g., 1.3.0) from outputs like:
	// "k6 v1.3.0 (commit/devel, go1.25.1, darwin/arm64)" or "k6 v0.0.0-test"
	// We prefer strict semver (MAJOR.MINOR.PATCH). If not found, fall back to the raw string.
	// The regex captures the first x.y.z sequence optionally prefixed by v.
	semverRe := regexp.MustCompile(`\bv?(\d+\.\d+\.\d+)\b`)
	if m := semverRe.FindStringSubmatch(raw); len(m) == 2 {
		return m[1], nil
	}

	return raw, nil
}
