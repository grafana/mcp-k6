package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
	"go.k6.io/k6/v2/lib/types"
	"gopkg.in/guregu/null.v3"
)

func TestBuildK6Args(t *testing.T) {
	t.Parallel()

	scriptPath := "script.js"

	tests := []struct {
		name    string
		options *RunOptions
		want    []string
	}{
		{
			name:    "nil options only pass script",
			options: nil,
			want:    []string{"run", scriptPath},
		},
		{
			name:    "empty options only pass script",
			options: &RunOptions{},
			want:    []string{"run", scriptPath},
		},
		{
			name: "zero numeric values are treated as unset",
			options: &RunOptions{
				VUs:        null.IntFrom(0),
				Iterations: null.IntFrom(0),
			},
			want: []string{"run", scriptPath},
		},
		{
			name: "explicit vus and duration are passed",
			options: &RunOptions{
				VUs:      null.IntFrom(10),
				Duration: types.NullDurationFrom(30 * time.Second),
			},
			want: []string{"run", "--vus", "10", "--duration", "30s", scriptPath},
		},
		{
			name: "iterations take precedence over duration",
			options: &RunOptions{
				VUs:        null.IntFrom(2),
				Duration:   types.NullDurationFrom(30 * time.Second),
				Iterations: null.IntFrom(5),
			},
			want: []string{"run", "--vus", "2", "--iterations", "5", scriptPath},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, buildK6Args(scriptPath, tt.options))
		})
	}
}

const emptyDefaultExportScript = "export default function () {}"

func TestParseRunOptions(t *testing.T) {
	t.Parallel()

	t.Run("omitted args are unset", func(t *testing.T) {
		t.Parallel()

		options, err := parseRunOptions(newRunCallRequest(map[string]any{
			"script": emptyDefaultExportScript,
		}))
		require.NoError(t, err)
		require.False(t, options.VUs.Valid)
		require.False(t, options.Duration.Valid)
		require.False(t, options.Iterations.Valid)
	})

	t.Run("provided args are valid", func(t *testing.T) {
		t.Parallel()

		options, err := parseRunOptions(newRunCallRequest(map[string]any{
			"script":     emptyDefaultExportScript,
			"vus":        float64(10),
			"duration":   "1m",
			"iterations": float64(3),
		}))
		require.NoError(t, err)
		require.True(t, options.VUs.Valid)
		require.EqualValues(t, 10, options.VUs.Int64)
		require.True(t, options.Duration.Valid)
		require.Equal(t, time.Minute, options.Duration.TimeDuration())
		require.True(t, options.Iterations.Valid)
		require.EqualValues(t, 3, options.Iterations.Int64)
	})

	t.Run("null zero and blank args are unset", func(t *testing.T) {
		t.Parallel()

		options, err := parseRunOptions(newRunCallRequest(map[string]any{
			"script":     emptyDefaultExportScript,
			"vus":        nil,
			"duration":   " ",
			"iterations": float64(0),
		}))
		require.NoError(t, err)
		require.False(t, options.VUs.Valid)
		require.False(t, options.Duration.Valid)
		require.False(t, options.Iterations.Valid)
	})

	t.Run("invalid duration is rejected", func(t *testing.T) {
		t.Parallel()

		_, err := parseRunOptions(newRunCallRequest(map[string]any{
			"script":   emptyDefaultExportScript,
			"duration": "not-a-duration",
		}))
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid duration format")
	})
}

func TestValidateRunOptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options *RunOptions
		wantErr string
	}{
		{
			name: "negative vus",
			options: &RunOptions{
				VUs: null.IntFrom(-1),
			},
			wantErr: "vus cannot be negative",
		},
		{
			name: "negative iterations",
			options: &RunOptions{
				Iterations: null.IntFrom(-1),
			},
			wantErr: "iterations cannot be negative",
		},
		{
			name: "duration cap",
			options: &RunOptions{
				Duration: types.NullDurationFrom(MaxDuration + time.Second),
			},
			wantErr: "duration cannot exceed 5m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateRunOptions(tt.options)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestRunK6TestDoesNotInjectUnsetWorkloadArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("recording shell stub is Unix-only")
	}

	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args.txt")
	createRecordingK6Stub(t, dir, argsPath)
	t.Setenv("PATH", dir)

	result, err := RunK6Test(context.Background(), scenarioScript(), &RunOptions{})
	require.NoError(t, err)
	require.True(t, result.Success)

	args := readRecordedArgs(t, argsPath)
	require.Equal(t, "run", args[0])
	require.NotContains(t, args, "--vus")
	require.NotContains(t, args, "--duration")
	require.NotContains(t, args, "--iterations")
}

func TestRunK6TestPassesExplicitWorkloadArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("recording shell stub is Unix-only")
	}

	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args.txt")
	createRecordingK6Stub(t, dir, argsPath)
	t.Setenv("PATH", dir)

	result, err := RunK6Test(context.Background(), scenarioScript(), &RunOptions{
		VUs:        null.IntFrom(3),
		Duration:   types.NullDurationFrom(2 * time.Second),
		Iterations: null.IntFrom(1),
	})
	require.NoError(t, err)
	require.True(t, result.Success)

	args := readRecordedArgs(t, argsPath)
	require.Equal(t, []string{"run", "--vus", "3", "--iterations", "1"}, args[:5])
	require.NotContains(t, args, "--duration")
}

func newRunCallRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      "run_script",
			Arguments: args,
		},
	}
}

func createRecordingK6Stub(t *testing.T, dir, argsPath string) {
	t.Helper()

	content := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %s\nexit 0\n", shellQuote(argsPath))
	path := filepath.Join(dir, "k6")
	//nolint:forbidigo // Test helper requires writing stub executable
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	//nolint:forbidigo // Adjust permissions for executable stub
	// #nosec G302 -- Stub executable must be runnable during tests
	require.NoError(t, os.Chmod(path, 0o700))
}

func readRecordedArgs(t *testing.T, argsPath string) []string {
	t.Helper()

	//nolint:forbidigo // Test helper reads recording produced by stub executable
	// #nosec G304 -- argsPath is generated from t.TempDir() under test control
	content, err := os.ReadFile(argsPath)
	require.NoError(t, err)

	return strings.Split(strings.TrimSpace(string(content)), "\n")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func scenarioScript() string {
	return `export const options = {
  scenarios: {
    one: {
      executor: 'shared-iterations',
      vus: 1,
      iterations: 1,
    },
  },
};

export default function () {}
`
}
