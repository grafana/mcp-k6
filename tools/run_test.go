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
	"go.k6.io/k6/v2/errext/exitcodes"
	"go.k6.io/k6/v2/lib/types"
	"gopkg.in/guregu/null.v3"
)

const summaryFixturePath = "testdata/summary_export.json"

func TestBuildK6Args(t *testing.T) {
	t.Parallel()

	scriptPath := "script.js"
	summaryPath := "summary.json"
	summaryFlag := "--summary-export=" + summaryPath

	tests := []struct {
		name    string
		options *RunOptions
		want    []string
	}{
		{
			name:    "nil options only pass script",
			options: nil,
			want:    []string{"run", summaryFlag, scriptPath},
		},
		{
			name:    "empty options only pass script",
			options: &RunOptions{},
			want:    []string{"run", summaryFlag, scriptPath},
		},
		{
			name: "zero numeric values are treated as unset",
			options: &RunOptions{
				VUs:        null.IntFrom(0),
				Iterations: null.IntFrom(0),
			},
			want: []string{"run", summaryFlag, scriptPath},
		},
		{
			name: "explicit vus and duration are passed",
			options: &RunOptions{
				VUs:      null.IntFrom(10),
				Duration: types.NullDurationFrom(30 * time.Second),
			},
			want: []string{"run", summaryFlag, "--vus", "10", "--duration", "30s", scriptPath},
		},
		{
			name: "iterations take precedence over duration",
			options: &RunOptions{
				VUs:        null.IntFrom(2),
				Duration:   types.NullDurationFrom(30 * time.Second),
				Iterations: null.IntFrom(5),
			},
			want: []string{"run", summaryFlag, "--vus", "2", "--iterations", "5", scriptPath},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, buildK6Args(scriptPath, summaryPath, tt.options))
		})
	}
}

func TestParseSummaryExport(t *testing.T) {
	t.Parallel()

	//nolint:forbidigo // Test reads a checked-in fixture
	content, err := os.ReadFile(summaryFixturePath)
	require.NoError(t, err)

	summary, err := parseSummaryExport(content)
	require.NoError(t, err)

	require.Len(t, summary.Metrics, 7)
	require.Equal(t, "trend", summary.Metrics["iteration_duration"].Type)
	require.InDelta(t, 0.718042, summary.Metrics["iteration_duration"].Values["p(95)"], 1e-9)
	require.Equal(t, "counter", summary.Metrics["iterations"].Type)
	require.InDelta(t, 1, summary.Metrics["iterations"].Values["count"], 0)
	require.Equal(t, "rate", summary.Metrics["checks"].Type)
	require.InDelta(t, 0.5, summary.Metrics["checks"].Values["value"], 0)
	require.Equal(t, "gauge", summary.Metrics["vus"].Type)
	require.NotContains(t, summary.Metrics["checks"].Values, "thresholds")

	require.Equal(t, []ThresholdResult{
		{Metric: "checks", Expression: "rate>0.5", Passed: false},
		{Metric: "iterations", Expression: "count>=1", Passed: true},
		{Metric: "my_counter", Expression: "count<1", Passed: false},
	}, summary.Thresholds)

	require.Equal(t, &CheckSummary{Passes: 1, Fails: 1}, summary.Checks)
}

func TestParseSummaryExportWithoutChecksOrThresholds(t *testing.T) {
	t.Parallel()

	summary, err := parseSummaryExport([]byte(`{"metrics":{"iterations":{"count":2,"rate":4}}}`))
	require.NoError(t, err)
	require.Nil(t, summary.Checks)
	require.Empty(t, summary.Thresholds)
	require.Equal(t, "counter", summary.Metrics["iterations"].Type)
}

func TestReadSummaryExport(t *testing.T) {
	t.Parallel()

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()

		summary, err := readSummaryExport(filepath.Join(t.TempDir(), "missing.json"))
		require.Nil(t, summary)
		require.ErrorContains(t, err, "failed to read summary export")
	})

	t.Run("empty file", func(t *testing.T) {
		t.Parallel()

		summary, err := readSummaryExport(writeTempFile(t, ""))
		require.Nil(t, summary)
		require.ErrorContains(t, err, "did not write a summary export")
	})

	t.Run("malformed file", func(t *testing.T) {
		t.Parallel()

		summary, err := readSummaryExport(writeTempFile(t, "{not json"))
		require.Nil(t, summary)
		require.ErrorContains(t, err, "failed to parse summary export")
	})
}

func TestExitReason(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code int
		want string
	}{
		{code: 0, want: "success"},
		{code: int(exitcodes.ThresholdsHaveFailed), want: "thresholds_failed"},
		{code: int(exitcodes.SetupTimeout), want: "setup_timeout"},
		{code: int(exitcodes.TeardownTimeout), want: "teardown_timeout"},
		{code: int(exitcodes.ScriptException), want: "script_exception"},
		{code: int(exitcodes.ScriptAborted), want: "script_aborted"},
		{code: int(exitcodes.InvalidConfig), want: "invalid_config"},
		{code: 1, want: "unknown"},
		{code: -1, want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, exitReason(tt.code))
		})
	}
}

func TestTruncateStdout(t *testing.T) {
	t.Parallel()

	short := strings.Repeat("a", MaxStdoutPreviewBytes)
	require.Equal(t, short, truncateStdout(short))

	long := strings.Repeat("a", MaxStdoutPreviewBytes-1) + "µ" + strings.Repeat("b", 100)
	truncated := truncateStdout(long)
	require.True(t, strings.HasPrefix(truncated, strings.Repeat("a", MaxStdoutPreviewBytes-1)))
	require.Contains(t, truncated, "[stdout truncated to 4096 bytes")
	require.NotContains(t, truncated, "µ")
	require.NotContains(t, truncated, "bbb")
}

func TestGenerateRunNextStepsForThresholdFailure(t *testing.T) {
	t.Parallel()

	steps := generateRunNextSteps(&RunResult{
		Success:          false,
		ExitCode:         int(exitcodes.ThresholdsHaveFailed),
		ThresholdsFailed: true,
	}, &RunOptions{VUs: null.IntFrom(5)})

	require.Contains(t, steps[0], "summary.thresholds")
	require.Contains(t, strings.Join(steps, "\n"), "run_script with 1 VU and 1 iteration")
	require.NotContains(t, strings.Join(steps, "\n"), "validate_script")
}

const emptyDefaultExportScript = "export default function() {}"

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
	require.Equal(t, "success", result.ExitReason)
	require.Nil(t, result.Summary)
	require.Equal(t, []string{"summary unavailable: k6 did not write a summary export"}, result.Warnings)

	args := readRecordedArgs(t, argsPath)
	require.Equal(t, "run", args[0])
	require.NotContains(t, args, "--vus")
	require.NotContains(t, args, "--duration")
	require.NotContains(t, args, "--iterations")
}

func TestRunK6TestReportsThresholdFailureWithSummary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("recording shell stub is Unix-only")
	}

	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args.txt")
	fixturePath, err := filepath.Abs(summaryFixturePath)
	require.NoError(t, err)
	createSummaryWritingK6Stub(t, dir, argsPath, fixturePath, int(exitcodes.ThresholdsHaveFailed))
	t.Setenv("PATH", dir)

	result, err := RunK6Test(context.Background(), scenarioScript(), &RunOptions{})
	require.NoError(t, err)
	require.False(t, result.Success)
	require.True(t, result.ThresholdsFailed)
	require.Equal(t, "thresholds_failed", result.ExitReason)
	require.Equal(t, "k6 test failed with exit code 99", result.Error)
	require.Empty(t, result.Warnings)
	require.NotNil(t, result.Summary)
	require.Len(t, result.Summary.Thresholds, 3)
	require.Contains(t, result.NextSteps[0], "summary.thresholds")
	require.Contains(t, result.Stdout, "[stdout truncated to")

	args := readRecordedArgs(t, argsPath)
	require.True(t, strings.HasPrefix(args[1], "--summary-export="))
	require.NoFileExists(t, strings.TrimPrefix(args[1], "--summary-export="))
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
	require.Equal(t, "run", args[0])
	require.True(t, strings.HasPrefix(args[1], "--summary-export="))
	require.Equal(t, []string{"--vus", "3", "--iterations", "1"}, args[2:6])
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

func createSummaryWritingK6Stub(t *testing.T, dir, argsPath, fixturePath string, exitCode int) {
	t.Helper()

	content := fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$@" > %s
for arg in "$@"; do
  case "$arg" in
    --summary-export=*) /bin/cp %s "${arg#--summary-export=}" ;;
  esac
done
i=0
while [ $i -le %d ]; do printf 'x'; i=$((i+1)); done
exit %d
`, shellQuote(argsPath), shellQuote(fixturePath), MaxStdoutPreviewBytes, exitCode)
	path := filepath.Join(dir, "k6")
	//nolint:forbidigo // Test helper requires writing stub executable
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	//nolint:forbidigo // Adjust permissions for executable stub
	// #nosec G302 -- Stub executable must be runnable during tests
	require.NoError(t, os.Chmod(path, 0o700))
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "summary.json")
	//nolint:forbidigo // Test helper writes a fixture file
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
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

export default function() {}
	`
}
