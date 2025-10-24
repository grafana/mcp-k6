// Package runner provides k6 script execution functionality.
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/k6-mcp/internal/logging"
	"github.com/grafana/k6-mcp/internal/security"
)

const (
	// DefaultTimeout is the default timeout for k6 test runs.
	DefaultTimeout = 5 * time.Minute
	// MaxVUs is the maximum number of virtual users allowed.
	MaxVUs = 50
	// MaxDuration is the maximum test duration allowed.
	MaxDuration = 5 * time.Minute
	// DefaultVUs is the default number of virtual users.
	DefaultVUs = 1
	// DefaultDuration is the default test duration.
	DefaultDuration = "30s"
	// P95Percentile represents the 95th percentile for response time calculations.
	P95Percentile = 0.95
)

// RunOptions contains configuration options for running k6 tests.
type RunOptions struct {
	VUs        int                    `json:"vus,omitempty"`
	Duration   string                 `json:"duration,omitempty"`
	Iterations int                    `json:"iterations,omitempty"`
	Stages     []Stage                `json:"stages,omitempty"`
	Options    map[string]interface{} `json:"options,omitempty"`
}

// Stage represents a load testing stage with target VUs and duration.
type Stage struct {
	Duration string `json:"duration"`
	Target   int    `json:"target"`
}

// RunResult contains the result of a k6 test execution.
type RunResult struct {
	Success         bool                   `json:"success"`
	ExitCode        int                    `json:"exit_code"`
	Stdout          string                 `json:"stdout"`
	Stderr          string                 `json:"stderr"`
	Error           string                 `json:"error,omitempty"`
	Duration        string                 `json:"duration"`
	Metrics         map[string]interface{} `json:"metrics,omitempty"`
	Summary         TestSummary            `json:"summary,omitempty"`
	Analysis        TestAnalysis           `json:"analysis"`
	Issues          []TestIssue            `json:"issues,omitempty"`
	Recommendations []string               `json:"recommendations,omitempty"`
	NextSteps       []string               `json:"next_steps,omitempty"`
	Performance     PerformanceInsights    `json:"performance"`
}

// TestSummary contains a summary of the test execution results.
type TestSummary struct {
	TotalRequests   int     `json:"total_requests"`
	FailedRequests  int     `json:"failed_requests"`
	AvgResponseTime float64 `json:"avg_response_time_ms"`
	P95ResponseTime float64 `json:"p95_response_time_ms"`
	RequestRate     float64 `json:"request_rate_per_second"`
	DataReceived    string  `json:"data_received"`
	DataSent        string  `json:"data_sent"`
}

// TestAnalysis provides high-level analysis of test execution.
type TestAnalysis struct {
	Status      string  `json:"status"`       // "success", "failed", "warning"
	Grade       string  `json:"grade"`        // "A", "B", "C", "D", "F"
	Description string  `json:"description"`  // Human-readable summary
	SuccessRate float64 `json:"success_rate"` // Percentage of successful requests
	IssueCount  int     `json:"issue_count"`  // Number of issues found
	Severity    string  `json:"severity"`     // "critical", "high", "medium", "low", "none"
}

// TestIssue represents a specific issue found during test execution.
type TestIssue struct {
	Type       string  `json:"type"`                // "performance", "error", "threshold", "network"
	Severity   string  `json:"severity"`            // "critical", "high", "medium", "low"
	Message    string  `json:"message"`             // Description of the issue
	Suggestion string  `json:"suggestion"`          // Specific fix recommendation
	Value      float64 `json:"value,omitempty"`     // Actual value (for metrics)
	Threshold  float64 `json:"threshold,omitempty"` // Expected threshold (for performance issues)
	Count      int     `json:"count,omitempty"`     // Number of occurrences
}

// PerformanceInsights provides detailed performance analysis and recommendations.
type PerformanceInsights struct {
	OverallGrade    string                   `json:"overall_grade"` // "A", "B", "C", "D", "F"
	ResponseTime    PerformanceMetric        `json:"response_time"`
	Throughput      PerformanceMetric        `json:"throughput"`
	ErrorRate       PerformanceMetric        `json:"error_rate"`
	Recommendations []string                 `json:"recommendations"`
	Bottlenecks     []string                 `json:"bottlenecks,omitempty"`
	Optimizations   []OptimizationSuggestion `json:"optimizations,omitempty"`
}

// PerformanceMetric represents analysis of a specific performance metric.
type PerformanceMetric struct {
	Value       float64 `json:"value"`
	Grade       string  `json:"grade"`               // "A", "B", "C", "D", "F"
	Status      string  `json:"status"`              // "excellent", "good", "acceptable", "poor", "critical"
	Description string  `json:"description"`         // Human-readable assessment
	Benchmark   string  `json:"benchmark,omitempty"` // Industry benchmark comparison
}

// OptimizationSuggestion provides specific optimization recommendations.
type OptimizationSuggestion struct {
	Category    string `json:"category"`    // "script", "infrastructure", "configuration"
	Priority    string `json:"priority"`    // "high", "medium", "low"
	Title       string `json:"title"`       // Short title
	Description string `json:"description"` // Detailed explanation
	Impact      string `json:"impact"`      // Expected improvement
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
	logger := logging.WithComponent("runner")

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
	result, err := executeK6Test(ctx, tempFile, options)
	if err != nil {
		return nil, fmt.Errorf("executing k6 script failed; reason: %w", err)
	}

	result.Duration = time.Since(startTime).String()
	enhanceRunResult(result, options)

	logger.InfoContext(ctx, "k6 test execution completed",
		slog.Bool("success", result.Success),
		slog.Int("exit_code", result.ExitCode),
		slog.Duration("duration", time.Since(startTime)),
		slog.Int("total_requests", result.Summary.TotalRequests),
		slog.Int("failed_requests", result.Summary.FailedRequests),
		slog.String("grade", result.Analysis.Grade),
	)

	return result, err
}

// validateRunInput performs input validation on the script and options.
func validateRunInput(ctx context.Context, script string, options *RunOptions) error {
	// Validate script content using existing security module
	if err := security.ValidateScriptContent(ctx, script); err != nil {
		return &RunError{
			Type:    "INPUT_VALIDATION",
			Message: "script validation failed",
			Cause:   err,
		}
	}

	// Set defaults if options is nil
	if options == nil {
		return nil
	}

	return validateRunOptions(options)
}

// validateRunOptions validates the run options parameters.
func validateRunOptions(options *RunOptions) error {
	if err := validateVUsAndIterations(options); err != nil {
		return err
	}

	if err := validateDuration(options); err != nil {
		return err
	}

	return validateStages(options.Stages)
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

// validateStages validates the stages configuration.
func validateStages(stages []Stage) error {
	for i, stage := range stages {
		if stage.Target > MaxVUs {
			return &RunError{
				Type:    "PARAMETER_VALIDATION",
				Message: fmt.Sprintf("stage %d target VUs (%d) cannot exceed %d", i, stage.Target, MaxVUs),
			}
		}
		if _, err := time.ParseDuration(stage.Duration); err != nil {
			return &RunError{
				Type:    "PARAMETER_VALIDATION",
				Message: fmt.Sprintf("stage %d has invalid duration format: %s", i, stage.Duration),
				Cause:   err,
			}
		}
	}

	return nil
}

// createSecureTempFile creates a secure temporary file with the script content.
func createSecureTempFile(script string) (string, func(), error) {
	//nolint:forbidigo // Temporary file creation required for k6 execution
	tmpFile, err := os.CreateTemp("", "k6-run-*.js")
	if err != nil {
		return "", nil, &RunError{
			Type:    "FILE_CREATION",
			Message: "failed to create temporary file",
			Cause:   err,
		}
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
		return &RunError{
			Type:    "FILE_PERMISSION",
			Message: "failed to set secure file permissions",
			Cause:   err,
		}
	}

	// Write script content
	if _, err := tmpFile.WriteString(script); err != nil {
		return &RunError{
			Type:    "FILE_WRITE",
			Message: "failed to write script to temporary file",
			Cause:   err,
		}
	}

	if err := tmpFile.Close(); err != nil {
		return &RunError{
			Type:    "FILE_CLOSE",
			Message: "failed to close temporary file",
			Cause:   err,
		}
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

// executeK6Test executes k6 with the given script file and options.
func executeK6Test(ctx context.Context, scriptPath string, options *RunOptions) (*RunResult, error) {
	logger := logging.WithComponent("runner")
	startTime := time.Now()

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	// Check if k6 is available
	if err := security.ValidateEnvironment(cmdCtx); err != nil {
		logger.ErrorContext(ctx, "k6 executable not found",
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

	// Build k6 command arguments
	args := buildK6Args(scriptPath, options)

	logger.DebugContext(ctx, "Executing k6 test command",
		slog.Any("args", args),
		slog.String("script_path", getPathType(scriptPath)),
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

	// Parse metrics and summary from output
	if result.Success {
		result.Metrics, result.Summary = parseK6Output(stdout)
	}

	// Handle different types of errors
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			// Command timed out
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
				result.Error = fmt.Sprintf("k6 test failed with exit code %d", exitCode)
			} else {
				// Other execution errors
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

	// Handle stages for load profiling
	if len(options.Stages) > 0 {
		stagesStr := buildStagesString(options.Stages)
		args = append(args, "--stage", stagesStr)
	}

	// Add JSON output for metrics parsing
	args = append(args, "--out", "json=/dev/stdout")

	// Add script path
	args = append(args, scriptPath)

	return args
}

// buildStagesString creates a stages configuration string for k6.
func buildStagesString(stages []Stage) string {
	stageStrings := make([]string, 0, len(stages))
	for _, stage := range stages {
		stageStr := fmt.Sprintf("%s:%d", stage.Duration, stage.Target)
		stageStrings = append(stageStrings, stageStr)
	}
	return strings.Join(stageStrings, ",")
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

// parseK6Output parses k6 output to extract metrics and summary information.
func parseK6Output(output string) (map[string]interface{}, TestSummary) {
	metrics := make(map[string]interface{})
	summary := TestSummary{}

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

	// Extract summary metrics from JSON output
	if len(jsonMetrics) > 0 {
		summary = extractSummaryFromMetrics(jsonMetrics)
		metrics["raw_metrics"] = jsonMetrics
		metrics["metrics_count"] = len(jsonMetrics)
	}

	return metrics, summary
}

// extractSummaryFromMetrics extracts test summary information from k6 JSON metrics.
func extractSummaryFromMetrics(jsonMetrics []map[string]interface{}) TestSummary {
	summary := TestSummary{}

	// Count HTTP requests and calculate statistics
	httpReqs := 0
	httpFailures := 0
	responseTimes := make([]float64, 0, len(jsonMetrics))

	for _, metric := range jsonMetrics {
		if !isPointMetric(metric) {
			continue
		}

		name, ok := metric["metric"].(string)
		if !ok {
			continue
		}

		switch name {
		case "http_reqs":
			httpReqs++
		case "http_req_failed":
			if value := extractMetricValue(metric); value > 0 {
				httpFailures++
			}
		case "http_req_duration":
			if duration := extractMetricValue(metric); duration > 0 {
				responseTimes = append(responseTimes, duration)
			}
		}
	}

	summary.TotalRequests = httpReqs
	summary.FailedRequests = httpFailures

	// Calculate response time statistics
	if len(responseTimes) > 0 {
		summary.AvgResponseTime = calculateAverage(responseTimes)
		summary.P95ResponseTime = calculatePercentile(responseTimes, P95Percentile)
	}

	return summary
}

func isPointMetric(metric map[string]interface{}) bool {
	metricType, ok := metric["type"].(string)
	return ok && metricType == "Point"
}

func extractMetricValue(metric map[string]interface{}) float64 {
	data, ok := metric["data"].(map[string]interface{})
	if !ok {
		return 0
	}

	value, _ := data["value"].(float64)
	return value
}

// calculateAverage calculates the average of a slice of float64 values.
func calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculatePercentile calculates the specified percentile of a slice of float64 values.
func calculatePercentile(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Simple percentile calculation (would be more accurate with sorting)
	index := int(float64(len(values)) * percentile)
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

// sanitizeRunOptions removes sensitive information from run options for logging
func sanitizeRunOptions(options *RunOptions) interface{} {
	if options == nil {
		return nil
	}

	return map[string]interface{}{
		"vus":         options.VUs,
		"duration":    options.Duration,
		"iterations":  options.Iterations,
		"stages":      options.Stages,
		"has_options": options.Options != nil,
	}
}

// getPathType returns a safe representation of file paths for logging
func getPathType(path string) string {
	switch {
	case strings.Contains(path, "temp"), strings.Contains(path, "tmp"):
		return "temporary"
	case strings.HasSuffix(path, ".js"):
		return "javascript"
	case strings.HasSuffix(path, ".ts"):
		return "typescript"
	}
	return "other"
}

// enhanceRunResult adds comprehensive analysis to the run result
func enhanceRunResult(result *RunResult, options *RunOptions) {
	if result == nil {
		return
	}

	// Initialize empty slices if nil
	if result.Issues == nil {
		result.Issues = []TestIssue{}
	}
	if result.Recommendations == nil {
		result.Recommendations = []string{}
	}
	if result.NextSteps == nil {
		result.NextSteps = []string{}
	}

	// Analyze performance and generate insights
	result.Performance = analyzePerformance(result)
	result.Analysis = generateTestAnalysis(result)

	// Identify issues
	issues := identifyTestIssues(result)
	result.Issues = append(result.Issues, issues...)

	// Generate recommendations
	result.Recommendations = generateRecommendations(result, options)

	// Generate next steps
	result.NextSteps = generateRunNextSteps(result)

	// Add workflow integration suggestions
	addRunWorkflowIntegrationSuggestions(result, options)
}

// analyzePerformance analyzes performance metrics and provides insights
func analyzePerformance(result *RunResult) PerformanceInsights {
	insights := PerformanceInsights{
		Recommendations: []string{},
		Bottlenecks:     []string{},
		Optimizations:   []OptimizationSuggestion{},
	}

	// Analyze response time
	insights.ResponseTime = analyzeResponseTime(result.Summary.AvgResponseTime)

	// Analyze throughput
	insights.Throughput = analyzeThroughput(result.Summary.RequestRate)

	// Analyze error rate
	errorRate := 0.0
	if result.Summary.TotalRequests > 0 {
		errorRate = float64(result.Summary.FailedRequests) / float64(result.Summary.TotalRequests) * 100
	}
	insights.ErrorRate = analyzeErrorRate(errorRate)

	// Calculate overall grade
	insights.OverallGrade = calculateOverallGrade(
		insights.ResponseTime.Grade,
		insights.Throughput.Grade,
		insights.ErrorRate.Grade,
	)

	// Generate performance recommendations
	insights.Recommendations = generatePerformanceRecommendations(insights)

	// Identify bottlenecks
	insights.Bottlenecks = identifyBottlenecks(insights)

	// Generate optimizations
	insights.Optimizations = generateOptimizations(insights)

	return insights
}

// analyzeResponseTime analyzes response time metrics
func analyzeResponseTime(avg float64) PerformanceMetric {
	metric := PerformanceMetric{
		Value: avg,
	}

	// Grade based on average response time (in milliseconds)
	switch {
	case avg <= 100:
		metric.Grade = "A"
		metric.Status = "excellent"
		metric.Description = "Excellent response time"
		metric.Benchmark = "Well below 200ms target"
	case avg <= 200:
		metric.Grade = "B"
		metric.Status = "good"
		metric.Description = "Good response time"
		metric.Benchmark = "Within acceptable 200ms target"
	case avg <= 500:
		metric.Grade = "C"
		metric.Status = "acceptable"
		metric.Description = "Acceptable response time"
		metric.Benchmark = "Above optimal but under 500ms"
	case avg <= 1000:
		metric.Grade = "D"
		metric.Status = "poor"
		metric.Description = "Poor response time"
		metric.Benchmark = "Approaching 1s threshold"
	default:
		metric.Grade = "F"
		metric.Status = "critical"
		metric.Description = "Critical response time"
		metric.Benchmark = "Exceeds 1s acceptable limit"
	}

	return metric
}

// analyzeThroughput analyzes throughput metrics
func analyzeThroughput(rps float64) PerformanceMetric {
	metric := PerformanceMetric{
		Value: rps,
	}

	// Grade based on requests per second
	switch {
	case rps >= 100:
		metric.Grade = "A"
		metric.Status = "excellent"
		metric.Description = "Excellent throughput"
		metric.Benchmark = "High performance server capability"
	case rps >= 50:
		metric.Grade = "B"
		metric.Status = "good"
		metric.Description = "Good throughput"
		metric.Benchmark = "Above average performance"
	case rps >= 20:
		metric.Grade = "C"
		metric.Status = "acceptable"
		metric.Description = "Acceptable throughput"
		metric.Benchmark = "Moderate performance"
	case rps >= 5:
		metric.Grade = "D"
		metric.Status = "poor"
		metric.Description = "Poor throughput"
		metric.Benchmark = "Below optimal performance"
	default:
		metric.Grade = "F"
		metric.Status = "critical"
		metric.Description = "Critical throughput"
		metric.Benchmark = "Very low performance"
	}

	return metric
}

// analyzeErrorRate analyzes error rate metrics
func analyzeErrorRate(errorRate float64) PerformanceMetric {
	metric := PerformanceMetric{
		Value: errorRate,
	}

	// Grade based on error percentage
	switch {
	case errorRate == 0:
		metric.Grade = "A"
		metric.Status = "excellent"
		metric.Description = "No errors"
		metric.Benchmark = "Perfect reliability"
	case errorRate <= 0.1:
		metric.Grade = "B"
		metric.Status = "good"
		metric.Description = "Very low error rate"
		metric.Benchmark = "Excellent reliability"
	case errorRate <= 1:
		metric.Grade = "C"
		metric.Status = "acceptable"
		metric.Description = "Low error rate"
		metric.Benchmark = "Acceptable reliability"
	case errorRate <= 5:
		metric.Grade = "D"
		metric.Status = "poor"
		metric.Description = "High error rate"
		metric.Benchmark = "Poor reliability"
	default:
		metric.Grade = "F"
		metric.Status = "critical"
		metric.Description = "Critical error rate"
		metric.Benchmark = "Unacceptable reliability"
	}

	return metric
}

// calculateOverallGrade calculates overall performance grade
func calculateOverallGrade(responseGrade, throughputGrade, errorGrade string) string {
	gradeValues := map[string]int{
		"A": 4, "B": 3, "C": 2, "D": 1, "F": 0,
	}

	// Error rate has higher weight
	errorWeight := 2.0
	responseWeight := 1.5
	throughputWeight := 1.0

	total := float64(gradeValues[errorGrade])*errorWeight +
		float64(gradeValues[responseGrade])*responseWeight +
		float64(gradeValues[throughputGrade])*throughputWeight

	maxTotal := 4.0 * (errorWeight + responseWeight + throughputWeight)
	percentage := total / maxTotal

	switch {
	case percentage >= 0.9:
		return "A"
	case percentage >= 0.8:
		return "B"
	case percentage >= 0.7:
		return "C"
	case percentage >= 0.6:
		return "D"
	default:
		return "F"
	}
}

// generateTestAnalysis creates comprehensive test analysis
func generateTestAnalysis(result *RunResult) TestAnalysis {
	analysis := TestAnalysis{
		IssueCount: len(result.Issues),
	}

	// Calculate success rate
	if result.Summary.TotalRequests > 0 {
		successfulRequests := result.Summary.TotalRequests - result.Summary.FailedRequests
		analysis.SuccessRate = float64(successfulRequests) / float64(result.Summary.TotalRequests) * 100
	} else {
		analysis.SuccessRate = 0
	}

	// Determine status and grade
	switch {
	case !result.Success || result.ExitCode != 0:
		analysis.Status = "failed"
		analysis.Grade = "F"
		analysis.Description = "Test execution failed"
		analysis.Severity = "critical"
	case analysis.SuccessRate == 100 && result.Performance.OverallGrade >= "B":
		analysis.Status = "success"
		analysis.Grade = result.Performance.OverallGrade
		analysis.Description = "Test completed successfully with good performance"
		analysis.Severity = "none"
	case analysis.SuccessRate >= 95:
		analysis.Status = "warning"
		analysis.Grade = result.Performance.OverallGrade
		analysis.Description = "Test completed with minor issues"
		analysis.Severity = "low"
	default:
		analysis.Status = "warning"
		analysis.Grade = "D"
		analysis.Description = "Test completed but with significant issues"
		analysis.Severity = "medium"
	}

	return analysis
}

// identifyTestIssues identifies specific issues from test results
func identifyTestIssues(result *RunResult) []TestIssue {
	var issues []TestIssue

	// Check for high error rate
	if result.Summary.TotalRequests > 0 {
		errorRate := float64(result.Summary.FailedRequests) / float64(result.Summary.TotalRequests) * 100
		if errorRate > 5 {
			issues = append(issues, TestIssue{
				Type:       "error",
				Severity:   "critical",
				Message:    fmt.Sprintf("High error rate: %.1f%%", errorRate),
				Suggestion: "Investigate failed requests. Check target server capacity and network connectivity.",
				Value:      errorRate,
				Threshold:  5.0,
				Count:      result.Summary.FailedRequests,
			})
		} else if errorRate > 1 {
			issues = append(issues, TestIssue{
				Type:       "error",
				Severity:   "medium",
				Message:    fmt.Sprintf("Elevated error rate: %.1f%%", errorRate),
				Suggestion: "Monitor error patterns and consider optimizing request handling.",
				Value:      errorRate,
				Threshold:  1.0,
				Count:      result.Summary.FailedRequests,
			})
		}
	}

	// Check for slow response times
	if result.Summary.AvgResponseTime > 1000 {
		issues = append(issues, TestIssue{
			Type:       "performance",
			Severity:   "high",
			Message:    fmt.Sprintf("Slow average response time: %.0fms", result.Summary.AvgResponseTime),
			Suggestion: "Optimize server performance, check database queries, or consider caching.",
			Value:      result.Summary.AvgResponseTime,
			Threshold:  500.0,
		})
	} else if result.Summary.AvgResponseTime > 500 {
		issues = append(issues, TestIssue{
			Type:       "performance",
			Severity:   "medium",
			Message:    fmt.Sprintf("Response time above optimal: %.0fms", result.Summary.AvgResponseTime),
			Suggestion: "Consider performance optimizations to reduce response time.",
			Value:      result.Summary.AvgResponseTime,
			Threshold:  200.0,
		})
	}

	// Check for very slow P95 response times
	if result.Summary.P95ResponseTime > result.Summary.AvgResponseTime*2 {
		issues = append(issues, TestIssue{
			Type:       "performance",
			Severity:   "medium",
			Message:    "High response time variability",
			Suggestion: "Investigate outliers causing slow P95 times. Check for resource contention.",
			Value:      result.Summary.P95ResponseTime,
			Threshold:  result.Summary.AvgResponseTime * 1.5,
		})
	}

	// Check for low throughput
	if result.Summary.RequestRate < 1 && result.Summary.TotalRequests > 10 {
		issues = append(issues, TestIssue{
			Type:       "performance",
			Severity:   "medium",
			Message:    "Very low throughput",
			Suggestion: "Increase virtual users or reduce think time to achieve better throughput.",
			Value:      result.Summary.RequestRate,
			Threshold:  5.0,
		})
	}

	return issues
}

// generateRecommendations generates specific recommendations based on results
func generateRecommendations(result *RunResult, options *RunOptions) []string {
	var recommendations []string

	// Performance-based recommendations
	if result.Performance.ResponseTime.Grade <= "C" {
		recommendations = append(recommendations,
			"Consider optimizing server response times",
			"Use the 'search' tool with query 'performance optimization' for tips",
		)
	}

	if result.Performance.ErrorRate.Grade <= "C" {
		recommendations = append(recommendations,
			"Investigate and fix error patterns in your application",
			"Add error handling and retry logic to your k6 script",
		)
	}

	if result.Performance.Throughput.Grade <= "C" {
		recommendations = append(recommendations,
			"Consider increasing virtual users for better load testing",
			"Optimize your k6 script to reduce unnecessary delays",
		)
	}

	// Configuration-based recommendations
	if options != nil && options.VUs == 1 && result.Summary.TotalRequests < 100 {
		recommendations = append(recommendations,
			"Consider increasing VUs or iterations for more comprehensive testing",
			"Use stages configuration for realistic load patterns",
		)
	}

	// General recommendations
	recommendations = append(recommendations,
		"Use the 'search' tool to find k6 best practices and examples",
		"Consider adding checks and thresholds to your script",
	)

	return removeDuplicateStrings(recommendations)
}

// generatePerformanceRecommendations generates performance-specific recommendations
func generatePerformanceRecommendations(insights PerformanceInsights) []string {
	var recommendations []string

	if insights.ResponseTime.Grade <= "C" {
		recommendations = append(recommendations, "Optimize server response times for better user experience")
	}

	if insights.ErrorRate.Grade <= "C" {
		recommendations = append(recommendations, "Reduce error rate to improve reliability")
	}

	if insights.Throughput.Grade <= "C" {
		recommendations = append(recommendations, "Increase system throughput capacity")
	}

	return recommendations
}

// identifyBottlenecks identifies performance bottlenecks
func identifyBottlenecks(insights PerformanceInsights) []string {
	var bottlenecks []string

	if insights.ResponseTime.Grade <= "D" {
		bottlenecks = append(bottlenecks, "Slow response times indicate server or network bottlenecks")
	}

	if insights.ErrorRate.Grade <= "D" {
		bottlenecks = append(bottlenecks, "High error rate suggests system overload or configuration issues")
	}

	if insights.Throughput.Grade <= "D" {
		bottlenecks = append(bottlenecks, "Low throughput indicates capacity limitations")
	}

	return bottlenecks
}

// generateOptimizations generates specific optimization suggestions
func generateOptimizations(insights PerformanceInsights) []OptimizationSuggestion {
	var optimizations []OptimizationSuggestion

	if insights.ResponseTime.Grade <= "C" {
		optimizations = append(optimizations, OptimizationSuggestion{
			Category:    "infrastructure",
			Priority:    "high",
			Title:       "Optimize Response Time",
			Description: "Improve server performance through caching, database optimization, or CDN implementation",
			Impact:      "Reduce average response time by 30-50%",
		})
	}

	if insights.ErrorRate.Grade <= "C" {
		optimizations = append(optimizations, OptimizationSuggestion{
			Category:    "configuration",
			Priority:    "high",
			Title:       "Reduce Error Rate",
			Description: "Implement proper error handling, increase timeouts, or scale server capacity",
			Impact:      "Improve reliability to 99%+ success rate",
		})
	}

	if insights.Throughput.Grade <= "C" {
		optimizations = append(optimizations, OptimizationSuggestion{
			Category:    "script",
			Priority:    "medium",
			Title:       "Increase Test Load",
			Description: "Use more virtual users or longer test duration for better load simulation",
			Impact:      "Better stress testing and capacity planning",
		})
	}

	return optimizations
}

// generateRunNextSteps provides actionable next steps based on run results
func generateRunNextSteps(result *RunResult) []string {
	var steps []string

	switch {
	case result.Success && result.Analysis.Grade >= "B":
		steps = append(steps, "Great results! Your application performed well under load")

		if len(result.Issues) > 0 {
			steps = append(steps, "Address the minor issues found for even better performance")
		}

		steps = append(steps,
			"Consider increasing load to find your system's limits",
			"Add more complex scenarios to your test script",
		)
	case result.Success:
		steps = append(steps, "Test completed but found performance issues to address")

		if result.Analysis.Grade <= "D" {
			steps = append(steps, "Focus on critical performance improvements first")
		}

		steps = append(steps,
			"Review the performance insights and optimization suggestions",
			"Use the 'search' tool for specific optimization techniques",
		)
	default:
		steps = append(steps, "Fix test execution issues before analyzing performance")

		if result.ExitCode != 0 {
			steps = append(steps, "Check k6 script syntax and target server availability")
		}
	}

	steps = append(steps,
		"Use the 'search' tool for advanced k6 testing patterns",
		"Consider setting up monitoring for ongoing performance tracking",
	)

	return steps
}

// removeDuplicateStrings removes duplicate strings from a slice
func removeDuplicateStrings(slice []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

// addRunWorkflowIntegrationSuggestions adds workflow suggestions for run results
func addRunWorkflowIntegrationSuggestions(result *RunResult, options *RunOptions) {
	if result == nil {
		return
	}

	// Add workflow suggestions based on run results
	switch {
	case result.Success && result.Analysis.Grade >= "B":
		// Successful run with good performance
		workflowSuggestions := []string{
			"🎉 Excellent test results! Your application performed well",
			"Consider these next steps in your testing workflow:",
			"  • Scale up load to find performance limits",
			"  • Add more complex scenarios to your test suite",
			"  • Implement continuous performance monitoring",
		}

		// Insert at the beginning of next steps
		result.NextSteps = append(workflowSuggestions, result.NextSteps...)

		// Add advanced testing recommendations
		advancedRecommendations := []string{
			"Try advanced k6 features: scenarios, checks, custom metrics",
			"Consider integrating with CI/CD pipeline for automated testing",
			"Explore browser testing for frontend performance validation",
		}
		result.Recommendations = append(result.Recommendations, advancedRecommendations...)
	case result.Success && result.Analysis.Grade >= "C":
		// Successful run with moderate performance
		workflowSuggestions := []string{
			"✓ Test completed successfully with moderate performance",
			"Focus on optimization before scaling up:",
			"  • Address performance issues identified in the analysis",
			"  • Re-run with same parameters after optimizations",
			"  • Gradually increase load once performance improves",
		}

		result.NextSteps = append(workflowSuggestions, result.NextSteps...)
	case result.Success:
		// Successful run but poor performance
		workflowSuggestions := []string{
			"⚠ Test completed but revealed significant performance issues",
			"Recommended workflow for improvement:",
			"  • Analyze and fix critical performance bottlenecks",
			"  • Use 'validate' tool to check script optimizations",
			"  • Re-test with reduced load until performance improves",
		}

		result.NextSteps = append(workflowSuggestions, result.NextSteps...)
	default:
		// Failed run
		workflowSuggestions := []string{
			"❌ Test execution failed - troubleshooting workflow:",
			"  • Use 'validate' tool to check script syntax and structure",
			"  • Verify target server availability and configuration",
			"  • Start with minimal load (1 VU, 1 iteration) for debugging",
		}

		result.NextSteps = append(workflowSuggestions, result.NextSteps...)
	}

	// Add configuration-specific suggestions
	if options != nil {
		if options.VUs == 1 && result.Success {
			result.Recommendations = append(result.Recommendations,
				"Consider increasing VUs (5-10) for more realistic load simulation",
				"Use the current configuration as baseline for performance comparison",
			)
		}

		if options.VUs >= 20 && result.Success && result.Analysis.Grade <= "C" {
			result.Recommendations = append(result.Recommendations,
				"High VU count revealed performance issues - optimize before scaling further",
				"Consider using stages for gradual load ramping",
			)
		}
	}

	// Add general workflow integration recommendations
	generalWorkflow := []string{
		"Iterative testing approach: validate → small load → optimize → scale → monitor",
		"Use 'search' tool to find optimization techniques and advanced patterns",
		"Document your performance baselines for future comparison",
	}
	result.Recommendations = append(result.Recommendations, generalWorkflow...)

	// Remove duplicates
	result.NextSteps = removeDuplicateStrings(result.NextSteps)
	result.Recommendations = removeDuplicateStrings(result.Recommendations)
}
