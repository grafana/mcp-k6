package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/mcp-k6/internal/helpers"

	"github.com/grafana/mcp-k6/internal/logging"
	"github.com/grafana/mcp-k6/internal/security"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RunTool exposes a tool for running k6 test scripts.
//
//nolint:gochecknoglobals // Shared tool definition registered at startup.
var RunTool = mcp.NewTool(
	"run_script",
	mcp.WithDescription(
		"Run a k6 test script with configurable parameters. "+
			"Returns execution results including stdout, stderr, exit code, and raw metrics from k6.",
	),
	mcp.WithString(
		"script",
		mcp.Required(),
		mcp.Description(
			"The k6 script content to run (JavaScript/TypeScript). "+
				"Should be a valid k6 script with proper imports and default function.",
		),
	),
	mcp.WithNumber(
		"vus",
		mcp.Description(
			"Number of virtual users (default: 1, max: 50). "+
				"Examples: 1 for basic test, 10 for moderate load, 50 for stress test.",
		),
	),
	mcp.WithString(
		"duration",
		mcp.Description(
			"Test duration (default: '30s', max: '5m'). "+
				"Examples: '30s', '2m', '5m'. Overridden by iterations if specified.",
		),
	),
	mcp.WithNumber(
		"iterations",
		mcp.Description(
			"Number of iterations per VU (overrides duration). "+
				"Examples: 1 for single run, 100 for throughput test.",
		),
	),
)

// RegisterRunTool registers the run tool with the MCP server.
func RegisterRunTool(s *server.MCPServer) {
	s.AddTool(RunTool, withToolLogger("run_script", run))
}

func run(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	script, err := request.RequireString("script")
	if err != nil {
		return nil, err
	}

	vus := request.GetInt("vus", 1)
	duration := request.GetString("duration", "30s")
	iterations := request.GetInt("iterations", 0)

	result, err := RunK6Test(ctx, script, &RunOptions{
		VUs:        vus,
		Duration:   duration,
		Iterations: iterations,
	})
	if err != nil {
		return nil, err
	}

	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}

	return mcp.NewToolResultText(string(resultJSON)), nil
}

const (
	// DefaultVUs is the default number of virtual users.
	DefaultVUs = 1

	// DefaultDuration is the default test duration.
	DefaultDuration = "30s"

	// MaxVUs is the maximum number of virtual users allowed.
	MaxVUs = 50

	// MaxDuration is the maximum test duration allowed.
	MaxDuration = 5 * time.Minute
)

// RunOptions contains configuration options for running k6 tests.
type RunOptions struct {
	VUs        int    `json:"vus,omitempty"`
	Duration   string `json:"duration,omitempty"`
	Iterations int    `json:"iterations,omitempty"`
}

// RunResult contains the result of a k6 test execution.
type RunResult struct {
	Success   bool                   `json:"success"`
	ExitCode  int                    `json:"exit_code"`
	Stdout    string                 `json:"stdout"`
	Stderr    string                 `json:"stderr"`
	Error     string                 `json:"error,omitempty"`
	Duration  string                 `json:"duration"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
	NextSteps []string               `json:"next_steps,omitempty"`
}

// RunError represents errors that occur during k6 test execution.
type RunError struct {
	Type    string
	Message string
	Cause   error
}

func (e *RunError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *RunError) Unwrap() error {
	return e.Cause
}

// RunK6Test executes a k6 script with the specified options.
func RunK6Test(ctx context.Context, script string, options *RunOptions) (*RunResult, error) {
	startTime := time.Now()
	logger := logging.LoggerFromContext(ctx)

	// Log test configuration
	logger.DebugContext(ctx, "Starting k6 test execution",
		slog.Int("script_size", len(script)),
		slog.Any("options", sanitizeRunOptions(options)),
	)

	// Input validation
	if err := validateRunInput(ctx, script, options); err != nil {
		logger.WarnContext(ctx, "Test input validation failed",
			slog.String("error", err.Error()),
		)
		return &RunResult{
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(startTime).String(),
		}, err
	}

	logger.DebugContext(ctx, "Test input validation passed")

	// Create secure temporary file
	tempFile, cleanup, err := createSecureTempFile(script)
	if err != nil {
		logging.FileOperation(ctx, "runner", "create_temp_file", tempFile, err)
		return &RunResult{
			Success:  false,
			Error:    fmt.Sprintf("failed to create temporary file: %v", err),
			Duration: time.Since(startTime).String(),
		}, err
	}
	defer cleanup()

	logging.FileOperation(ctx, "runner", "create_temp_file", tempFile, nil)

	// Execute k6 test
	logger.DebugContext(ctx, "Starting k6 test execution",
		slog.String("script_path", helpers.GetPathType(tempFile)),
		slog.Any("options", sanitizeRunOptions(options)))
	result, err := executeK6Test(ctx, tempFile, options)
	if err != nil {
		return nil, fmt.Errorf("executing k6 script failed; reason: %w", err)
	}

	result.Duration = time.Since(startTime).String()
	result.NextSteps = generateRunNextSteps(result, options)

	logger.InfoContext(ctx, "k6 test execution completed",
		slog.Bool("success", result.Success),
		slog.Int("exit_code", result.ExitCode),
		slog.Duration("duration", time.Since(startTime)),
	)

	return result, err
}

// validateRunInput performs input validation on the script and options.
func validateRunInput(ctx context.Context, script string, options *RunOptions) error {
	logger := logging.LoggerFromContext(ctx)
	logger.DebugContext(ctx, "Validating run input",
		slog.Int("script_size", len(script)),
		slog.Any("options", sanitizeRunOptions(options)))

	// Validate script content using existing security module
	if err := security.ValidateScriptContent(ctx, script); err != nil {
		logger.WarnContext(ctx, "Script content validation failed",
			slog.String("error", err.Error()))
		return &RunError{
			Type:    "INPUT_VALIDATION",
			Message: "script validation failed",
			Cause:   err,
		}
	}

	// Set defaults if options is nil
	if options == nil {
		logger.DebugContext(ctx, "Run input validation passed")
		return nil
	}

	if err := validateRunOptions(options); err != nil {
		logger.WarnContext(ctx, "Run options validation failed",
			slog.String("error", err.Error()),
			slog.Any("options", sanitizeRunOptions(options)))
		return err
	}

	logger.DebugContext(ctx, "Run input validation passed")
	return nil
}

// validateRunOptions validates the run options parameters.
func validateRunOptions(options *RunOptions) error {
	if err := validateVUsAndIterations(options); err != nil {
		return err
	}

	return validateDuration(options)
}

// validateVUsAndIterations validates VUs and iterations parameters.
func validateVUsAndIterations(options *RunOptions) error {
	// Validate VUs
	if options.VUs < 0 {
		return &RunError{
			Type:    "PARAMETER_VALIDATION",
			Message: "vus cannot be negative",
		}
	}
	if options.VUs > MaxVUs {
		return &RunError{
			Type:    "PARAMETER_VALIDATION",
			Message: fmt.Sprintf("vus cannot exceed %d", MaxVUs),
		}
	}

	// Validate iterations
	if options.Iterations < 0 {
		return &RunError{
			Type:    "PARAMETER_VALIDATION",
			Message: "iterations cannot be negative",
		}
	}

	return nil
}

// validateDuration validates the duration parameter.
func validateDuration(options *RunOptions) error {
	if options.Duration == "" {
		return nil
	}

	duration, err := time.ParseDuration(options.Duration)
	if err != nil {
		return &RunError{
			Type:    "PARAMETER_VALIDATION",
			Message: fmt.Sprintf("invalid duration format: %s", options.Duration),
			Cause:   err,
		}
	}
	if duration > MaxDuration {
		return &RunError{
			Type:    "PARAMETER_VALIDATION",
			Message: fmt.Sprintf("duration cannot exceed %v", MaxDuration),
		}
	}

	return nil
}

// executeK6Test executes k6 with the given script file and options.
//
//nolint:funlen // Function length slightly exceeds limit due to comprehensive logging
func executeK6Test(ctx context.Context, scriptPath string, options *RunOptions) (*RunResult, error) {
	logger := logging.LoggerFromContext(ctx)
	startTime := time.Now()

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	// Check if k6 is available
	if err := security.ValidateEnvironment(cmdCtx); err != nil {
		logger.ErrorContext(ctx, "Environment validation failed",
			slog.String("error", err.Error()),
		)
		return &RunResult{
				Success: false,
				Error:   "k6 executable not found in PATH",
			}, &RunError{
				Type:    "K6_NOT_FOUND",
				Message: "k6 executable not found in PATH",
				Cause:   err,
			}
	}
	logger.DebugContext(ctx, "Environment validation passed")

	// Build k6 command arguments
	args := buildK6Args(scriptPath, options)

	logger.DebugContext(ctx, "Executing k6 test command",
		slog.Any("args", args),
		slog.String("script_path", helpers.GetPathType(scriptPath)),
	)

	// Prepare k6 command
	// #nosec G204 - k6 binary is validated to exist, args are sanitized
	cmd := exec.CommandContext(cmdCtx, "k6", args...)

	// Set secure environment
	cmd.Env = security.SecureEnvironment()

	// Execute command and capture output
	stdout, stderr, exitCode, err := executeCommand(cmd)

	// Log execution results
	logging.ExecutionEvent(ctx, "runner", "k6 run", time.Since(startTime), exitCode, err)

	// Sanitize output to prevent information leakage
	stdout = security.SanitizeOutput(stdout)
	stderr = security.SanitizeOutput(stderr)

	result := &RunResult{
		Success:  exitCode == 0,
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
	}

	// Parse metrics from output
	if result.Success {
		logger.DebugContext(ctx, "Parsing k6 output for metrics")
		result.Metrics = parseK6Output(stdout)
		logger.DebugContext(ctx, "Metrics parsed",
			slog.Int("metric_count", len(result.Metrics)))
	}

	// Handle different types of errors
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			// Command timed out
			logger.WarnContext(ctx, "k6 test timed out",
				slog.Duration("timeout", DefaultTimeout))
			result.Error = fmt.Sprintf("k6 test timed out after %v", DefaultTimeout)
			return result, &RunError{
				Type:    "TIMEOUT",
				Message: fmt.Sprintf("k6 test timed out after %v", DefaultTimeout),
				Cause:   err,
			}
		default:
			var exitError *exec.ExitError
			if errors.As(err, &exitError) {
				// Command executed but returned non-zero exit code
				stderrPreview := stderr
				if len(stderr) > 200 {
					stderrPreview = stderr[:200]
				}
				logger.WarnContext(ctx, "k6 test failed with non-zero exit code",
					slog.Int("exit_code", exitCode),
					slog.String("stderr_preview", stderrPreview))
				result.Error = fmt.Sprintf("k6 test failed with exit code %d", exitCode)
			} else {
				// Other execution errors
				logger.ErrorContext(ctx, "k6 execution error",
					slog.String("error", err.Error()))
				result.Error = fmt.Sprintf("failed to execute k6: %v", err)
				return result, &RunError{
					Type:    "EXECUTION_ERROR",
					Message: "failed to execute k6 command",
					Cause:   err,
				}
			}
		}
	}

	return result, nil
}

// buildK6Args builds the command line arguments for k6 based on the provided options.
func buildK6Args(scriptPath string, options *RunOptions) []string {
	args := []string{"run"}

	// Set defaults if options is nil
	if options == nil {
		options = &RunOptions{
			VUs:      DefaultVUs,
			Duration: DefaultDuration,
		}
	}

	// Set VUs (default to 1 if not specified)
	vus := options.VUs
	if vus == 0 {
		vus = DefaultVUs
	}
	args = append(args, "--vus", strconv.Itoa(vus))

	// Handle duration vs iterations
	if options.Iterations > 0 {
		args = append(args, "--iterations", strconv.Itoa(options.Iterations))
	} else {
		duration := options.Duration
		if duration == "" {
			duration = DefaultDuration
		}
		args = append(args, "--duration", duration)
	}

	// Add script path
	args = append(args, scriptPath)

	return args
}

// parseK6Output parses k6 output to extract raw metrics.
func parseK6Output(output string) map[string]interface{} {
	metrics := make(map[string]interface{})

	// Split output into lines and parse JSON metrics
	lines := strings.Split(output, "\n")
	var jsonMetrics []map[string]interface{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Try to parse as JSON metric
		var metric map[string]interface{}
		if err := json.Unmarshal([]byte(line), &metric); err == nil {
			jsonMetrics = append(jsonMetrics, metric)
		}
	}

	// Store raw metrics from k6
	if len(jsonMetrics) > 0 {
		metrics["raw_metrics"] = jsonMetrics
		metrics["metrics_count"] = len(jsonMetrics)
	}

	return metrics
}

// sanitizeRunOptions removes sensitive information from run options for logging
func sanitizeRunOptions(options *RunOptions) interface{} {
	if options == nil {
		return nil
	}

	return map[string]interface{}{
		"vus":        options.VUs,
		"duration":   options.Duration,
		"iterations": options.Iterations,
	}
}

// generateRunNextSteps provides actionable next steps based on test results
func generateRunNextSteps(result *RunResult, options *RunOptions) []string {
	if result == nil {
		return nil
	}

	var steps []string

	// Handle test failures
	if !result.Success || result.ExitCode != 0 {
		steps = append(steps, "Use validate_k6_script to check for syntax errors and script validity")
		steps = append(steps, "Use stderr output above to identify specific error messages")

		if result.ExitCode != 0 {
			steps = append(steps, "Use network debugging tools to verify target server availability")
		}

		if options != nil && (options.VUs > 1 || options.Iterations > 1) {
			steps = append(steps, "Use run_k6_script with 1 VU and 1 iteration to isolate the issue")
		}

		return steps
	}

	// Successful execution
	steps = append(steps, "Use the metrics data above to analyze test performance and results")

	// Suggest scaling if using minimal configuration
	if options != nil && options.VUs == 1 && options.Iterations <= 1 {
		steps = append(steps, "Use run_k6_script with higher VUs or iterations for comprehensive load testing")
		steps = append(steps, "Use search_k6_docs to learn about advanced testing patterns and scenarios")
	} else {
		steps = append(steps, "Use search_k6_docs to explore advanced k6 features and optimization techniques")
	}

	steps = append(steps, "Use k6_info to discover additional k6 capabilities and features")

	return steps
}
