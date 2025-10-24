// Package validator provides k6 script validation functionality.
package validator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/grafana/k6-mcp/internal/logging"
)

const (
	// DefaultTimeout is the default timeout for k6 validation runs.
	DefaultTimeout = 30 * time.Second
	// MaxScriptSize is the maximum allowed script size in bytes (1MB).
	MaxScriptSize = 1024 * 1024
)

// ValidationResult contains the result of a k6 script validation.
type ValidationResult struct {
	Valid           bool              `json:"valid"`
	ExitCode        int               `json:"exit_code"`
	Stdout          string            `json:"stdout"`
	Stderr          string            `json:"stderr"`
	Error           string            `json:"error,omitempty"`
	Duration        string            `json:"duration"`
	ScriptURL       string            `json:"script_url,omitempty"`
	Summary         ValidationSummary `json:"summary"`
	Issues          []ValidationIssue `json:"issues,omitempty"`
	Recommendations []string          `json:"recommendations,omitempty"`
	NextSteps       []string          `json:"next_steps,omitempty"`
}

// ValidationSummary provides a high-level overview of the validation results.
type ValidationSummary struct {
	Status      string `json:"status"`       // "success", "failed", "warning"
	Description string `json:"description"`  // Human-readable summary
	IssueCount  int    `json:"issue_count"`  // Number of issues found
	Severity    string `json:"severity"`     // "critical", "high", "medium", "low", "none"
	ReadyToRun  bool   `json:"ready_to_run"` // Whether script can be executed
}

// ValidationIssue represents a specific issue found during validation.
type ValidationIssue struct {
	Type       string `json:"type"`                  // "syntax", "import", "function", "security"
	Severity   string `json:"severity"`              // "critical", "high", "medium", "low"
	Message    string `json:"message"`               // Description of the issue
	Suggestion string `json:"suggestion"`            // Specific fix recommendation
	LineNumber int    `json:"line_number,omitempty"` // Line where issue occurs (if available)
}

// ValidationError represents errors that occur during validation.
type ValidationError struct {
	Type    string
	Message string
	Cause   error
}

func (e *ValidationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Cause
}

// ValidateK6Script validates a k6 script by executing it with minimal configuration.
func ValidateK6Script(ctx context.Context, script string) (*ValidationResult, error) {
	startTime := time.Now()
	logger := logging.WithComponent("validator")

	logger.DebugContext(ctx, "Starting script validation",
		slog.Int("script_size", len(script)),
	)

	// Input validation
	if err := validateInput(script); err != nil {
		logging.ValidationEvent(ctx, "input_validation", false, map[string]interface{}{
			"error":       err.Error(),
			"script_size": len(script),
		})

		issue := createValidationIssueFromError(err)
		return &ValidationResult{
			Valid:    false,
			Error:    err.Error(),
			Duration: time.Since(startTime).String(),
			Summary: ValidationSummary{
				Status:      "failed",
				Description: "Script validation failed during input validation",
				IssueCount:  1,
				Severity:    issue.Severity,
				ReadyToRun:  false,
			},
			Issues:          []ValidationIssue{issue},
			Recommendations: getRecommendationsForIssue(issue),
			NextSteps:       []string{"Fix the validation issue and try again", "Use the 'search' tool for k6 documentation"},
		}, err
	}

	logging.ValidationEvent(ctx, "input_validation", true, map[string]interface{}{
		"script_size": len(script),
	})

	// Create secure temporary file
	tempFile, cleanup, err := createSecureTempFile(script)
	if err != nil {
		logging.FileOperation(ctx, "validator", "create_temp_file", tempFile, err)
		return &ValidationResult{
			Valid:    false,
			Error:    fmt.Sprintf("failed to create temporary file: %v", err),
			Duration: time.Since(startTime).String(),
			Summary: ValidationSummary{
				Status:      "failed",
				Description: "Internal error: failed to create temporary file for validation",
				IssueCount:  1,
				Severity:    "critical",
				ReadyToRun:  false,
			},
			Issues: []ValidationIssue{{
				Type:       "system",
				Severity:   "critical",
				Message:    "Failed to create temporary file for validation",
				Suggestion: "This is an internal error. Please try again or contact support if the issue persists.",
			}},
			NextSteps: []string{"Try running the validation again", "Check system permissions and disk space"},
		}, err
	}
	defer cleanup()

	logging.FileOperation(ctx, "validator", "create_temp_file", tempFile, nil)

	// Execute k6 validation
	result, err := executeK6Validation(ctx, tempFile)
	if err != nil {
		return nil, fmt.Errorf("validating k6 script failed; reason: %w", err)
	}

	// Enhance result with analysis if validation completed
	result.Duration = time.Since(startTime).String()
	enhanceValidationResult(result, script)

	logger.DebugContext(ctx, "Validation completed",
		slog.Bool("valid", result.Valid),
		slog.Int("exit_code", result.ExitCode),
		slog.Duration("duration", time.Since(startTime)),
	)

	return result, err
}

// validateInput performs basic input validation on the script content.
func validateInput(script string) error {
	if len(script) == 0 {
		return &ValidationError{
			Type:    "INPUT_VALIDATION",
			Message: "script content cannot be empty",
		}
	}

	if len(script) > MaxScriptSize {
		return &ValidationError{
			Type:    "INPUT_VALIDATION",
			Message: fmt.Sprintf("script size exceeds maximum allowed size of %d bytes", MaxScriptSize),
		}
	}

	// Check for potentially dangerous patterns
	dangerousPatterns := []string{
		"require('child_process')",
		"require(\"child_process\")",
		"exec(",
		"spawn(",
		"fork(",
		"execSync(",
		"execFile(",
	}

	scriptLower := strings.ToLower(script)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(scriptLower, strings.ToLower(pattern)) {
			return &ValidationError{
				Type:    "SECURITY_VALIDATION",
				Message: fmt.Sprintf("script contains potentially dangerous pattern: %s", pattern),
			}
		}
	}

	return nil
}

// createSecureTempFile creates a secure temporary file with the script content.
func createSecureTempFile(script string) (string, func(), error) {
	//nolint:forbidigo // Temporary file creation required for k6 validation
	tmpFile, err := os.CreateTemp("", "k6-script-*.js")
	if err != nil {
		return "", nil, &ValidationError{
			Type:    "FILE_CREATION",
			Message: "failed to create temporary file",
			Cause:   err,
		}
	}

	filename := tmpFile.Name()
	cleanup := func() {
		//nolint:forbidigo // Cleanup of temporary file required
		if removeErr := os.Remove(filename); removeErr != nil {
			logging.WithComponent("validator").Warn("Failed to remove temporary file",
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
		return &ValidationError{
			Type:    "FILE_PERMISSION",
			Message: "failed to set secure file permissions",
			Cause:   err,
		}
	}

	// Write script content
	if _, err := tmpFile.WriteString(script); err != nil {
		return &ValidationError{
			Type:    "FILE_WRITE",
			Message: "failed to write script to temporary file",
			Cause:   err,
		}
	}

	if err := tmpFile.Close(); err != nil {
		return &ValidationError{
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
	logger := logging.WithComponent("validator")

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

// executeK6Validation executes k6 with the given script file.
func executeK6Validation(ctx context.Context, scriptPath string) (*ValidationResult, error) {
	logger := logging.WithComponent("validator")
	startTime := time.Now()

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	// Check if k6 is available
	if _, err := exec.LookPath("k6"); err != nil {
		logger.ErrorContext(ctx, "k6 executable not found",
			slog.String("error", err.Error()),
		)
		return &ValidationResult{
				Valid: false,
				Error: "k6 executable not found in PATH",
			}, &ValidationError{
				Type:    "K6_NOT_FOUND",
				Message: "k6 executable not found in PATH",
				Cause:   err,
			}
	}

	// Prepare k6 command with minimal configuration and additional validation flags
	cmd := exec.CommandContext(cmdCtx, "k6", "run",
		"--vus", "1",
		"--iterations", "1",
		"--quiet",
		"--insecure-skip-tls-verify",
		"--log-format=json",
		"--no-usage-report",
		scriptPath)

	// Set minimal environment
	//nolint:forbidigo // Environment variables required for k6 execution
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}

	logger.DebugContext(ctx, "Executing k6 validation command",
		slog.String("command", "k6 run"),
		slog.String("script_path", getPathType(scriptPath)),
	)

	// Execute command and capture output
	stdout, stderr, exitCode, err := executeCommand(cmd)

	// Log execution results
	logging.ExecutionEvent(ctx, "validator", "k6 run", time.Since(startTime), exitCode, err)

	result := &ValidationResult{
		Valid:    exitCode == 0,
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
	}

	if err == nil {
		return result, nil
	}

	// Handle different types of errors
	if errors.Is(err, context.DeadlineExceeded) {
		result.Error = fmt.Sprintf("k6 validation timed out after %v", DefaultTimeout)
		return result, &ValidationError{
			Type:    "TIMEOUT",
			Message: fmt.Sprintf("k6 validation timed out after %v", DefaultTimeout),
			Cause:   err,
		}
	}

	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		// Check if this is a threshold failure (which we should ignore for validation)
		if isThresholdFailure(stderr, stdout) {
			// Threshold failure - script syntax is valid, just performance criteria not met
			result.Valid = true
			result.Error = "" // Clear error since this is not a validation failure
		} else {
			// Actual validation failure
			result.Error = fmt.Sprintf("k6 validation failed with exit code %d", exitCode)
		}
		return result, nil
	}

	// Other execution errors
	result.Error = fmt.Sprintf("failed to execute k6: %v", err)
	return result, &ValidationError{
		Type:    "EXECUTION_ERROR",
		Message: "failed to execute k6 command",
		Cause:   err,
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

// getPathType returns a safe representation of file paths for logging
func getPathType(path string) string {
	switch {
	case strings.Contains(path, "temp"), strings.Contains(path, "tmp"):
		return "temporary"
	case strings.HasSuffix(path, ".js"):
		return "javascript"
	case strings.HasSuffix(path, ".ts"):
		return "typescript"
	default:
		return "other"
	}
}

// isThresholdFailure checks if a k6 run failure was due to threshold violations
// rather than syntax or runtime errors. For validation purposes, we only care
// about syntax correctness, not whether performance thresholds are met.
func isThresholdFailure(stderr, stdout string) bool {
	output := strings.ToLower(stderr + " " + stdout)

	// Check for threshold-specific error messages
	thresholdPatterns := []string{
		"some thresholds have failed",
		"thresholds have failed",
		"threshold failed",
		"threshold violation",
	}

	for _, pattern := range thresholdPatterns {
		if strings.Contains(output, pattern) {
			// Also check that there are no syntax errors
			if !hasSyntaxErrors(output) {
				return true
			}
		}
	}

	return false
}

// hasSyntaxErrors checks if the output contains syntax or runtime errors
// that would indicate the script itself is invalid
func hasSyntaxErrors(output string) bool {
	syntaxErrorPatterns := []string{
		"syntaxerror",
		"referenceerror",
		"typeerror",
		"cannot resolve module",
		"module not found",
		"unexpected token",
		"unexpected end of input",
		"invalid or unexpected token",
		"parsing error",
		"compilation error",
	}

	for _, pattern := range syntaxErrorPatterns {
		if strings.Contains(output, pattern) {
			return true
		}
	}

	return false
}

// createValidationIssueFromError converts a ValidationError to a ValidationIssue
func createValidationIssueFromError(err error) ValidationIssue {
	var valErr *ValidationError
	if errors.As(err, &valErr) {
		return ValidationIssue{
			Type:       mapErrorTypeToIssueType(valErr.Type),
			Severity:   mapErrorTypeToSeverity(valErr.Type),
			Message:    valErr.Message,
			Suggestion: getSuggestionForErrorType(valErr.Type, valErr.Message),
		}
	}

	return ValidationIssue{
		Type:       "unknown",
		Severity:   "medium",
		Message:    err.Error(),
		Suggestion: "Please check your script and try again",
	}
}

// mapErrorTypeToIssueType maps ValidationError types to ValidationIssue types
func mapErrorTypeToIssueType(errorType string) string {
	switch errorType {
	case "INPUT_VALIDATION":
		return "syntax"
	case "SECURITY_VALIDATION":
		return "security"
	case "FILE_CREATION", "FILE_PERMISSION", "FILE_WRITE", "FILE_CLOSE":
		return "system"
	case "K6_NOT_FOUND", "EXECUTION_ERROR":
		return "environment"
	case "TIMEOUT":
		return "performance"
	default:
		return "unknown"
	}
}

// mapErrorTypeToSeverity maps ValidationError types to severity levels
func mapErrorTypeToSeverity(errorType string) string {
	switch errorType {
	case "SECURITY_VALIDATION":
		return "critical"
	case "K6_NOT_FOUND", "EXECUTION_ERROR":
		return "critical"
	case "INPUT_VALIDATION":
		return "high"
	case "TIMEOUT":
		return "medium"
	case "FILE_CREATION", "FILE_PERMISSION", "FILE_WRITE", "FILE_CLOSE":
		return "high"
	default:
		return "medium"
	}
}

// getSuggestionForErrorType provides specific suggestions based on error type
func getSuggestionForErrorType(errorType, message string) string {
	switch errorType {
	case "INPUT_VALIDATION":
		if strings.Contains(message, "empty") {
			return "Provide a valid k6 script with at least an import and default function. " +
				"Example: import http from 'k6/http'; export default function() { http.get('https://httpbin.org/get'); }"
		}
		if strings.Contains(message, "size") {
			return "Reduce your script size. Consider splitting large scripts into modules or removing unnecessary code."
		}
		return "Check your script syntax and ensure it follows k6 script structure"
	case "SECURITY_VALIDATION":
		return "Remove dangerous patterns from your script. k6 scripts should only use k6 APIs, not Node.js system functions."
	case "K6_NOT_FOUND":
		return "Install k6 on your system. Visit https://k6.io/docs/getting-started/" +
			"installation/ for installation instructions."
	case "TIMEOUT":
		return "Your script may have infinite loops or very slow operations. " +
			"Check for blocking code and optimize performance."
	default:
		return "Review your script and ensure it follows k6 best practices"
	}
}

// getRecommendationsForIssue provides general recommendations based on issue type
func getRecommendationsForIssue(issue ValidationIssue) []string {
	switch issue.Type {
	case "syntax":
		return []string{
			"Use the 'search' tool with query 'getting started' for basic k6 syntax",
			"Ensure your script has proper import statements and a default function",
			"Check for missing semicolons, brackets, or quotes",
		}
	case "security":
		return []string{
			"Use only k6 built-in modules and APIs",
			"Remove any Node.js system calls or file system access",
			"Use the 'search' tool with query 'k6 modules' to see available APIs",
		}
	case "environment":
		return []string{
			"Ensure k6 is installed and available in your PATH",
			"Use the 'search' tool with query 'installation' for setup help",
		}
	default:
		return []string{
			"Use the 'search' tool to find relevant k6 documentation",
			"Start with a simple script and gradually add complexity",
		}
	}
}

// enhanceValidationResult adds comprehensive analysis to the validation result
func enhanceValidationResult(result *ValidationResult, script string) {
	if result == nil {
		return
	}

	// Initialize empty slices if nil
	if result.Issues == nil {
		result.Issues = []ValidationIssue{}
	}
	if result.Recommendations == nil {
		result.Recommendations = []string{}
	}
	if result.NextSteps == nil {
		result.NextSteps = []string{}
	}

	// Analyze the script and k6 output for additional insights
	issues := analyzeScriptContent(script)
	if result.Stderr != "" || result.ExitCode != 0 {
		issues = append(issues, analyzeK6Output(result.Stderr, result.Stdout)...)
	}

	result.Issues = append(result.Issues, issues...)

	// Generate summary
	result.Summary = generateValidationSummary(result)

	// Add recommendations based on issues
	for _, issue := range result.Issues {
		result.Recommendations = append(result.Recommendations, getRecommendationsForIssue(issue)...)
	}

	// Remove duplicate recommendations
	result.Recommendations = removeDuplicates(result.Recommendations)

	// Generate next steps
	result.NextSteps = generateNextSteps(result)

	// Add workflow integration suggestions
	addWorkflowIntegrationSuggestions(result)
}

// analyzeScriptContent performs static analysis of the script content
func analyzeScriptContent(script string) []ValidationIssue {
	var issues []ValidationIssue

	lines := strings.Split(script, "\n")

	// Check for common k6 patterns
	hasImport := false
	hasDefaultFunction := false
	hasHTTPCall := false

	for i, line := range lines {
		line = strings.TrimSpace(line)
		lineNum := i + 1

		// Check for imports
		if strings.HasPrefix(line, "import") && strings.Contains(line, "k6/") {
			hasImport = true
		}

		// Check for default function
		if strings.Contains(line, "export default function") {
			hasDefaultFunction = true
		}

		// Check for HTTP calls
		if strings.Contains(line, "http.get") || strings.Contains(line, "http.post") ||
			strings.Contains(line, "http.put") || strings.Contains(line, "http.delete") {
			hasHTTPCall = true
		}

		// Check for common issues
		if strings.Contains(line, "console.log") {
			issues = append(issues, ValidationIssue{
				Type:       "syntax",
				Severity:   "low",
				Message:    "Using console.log in k6 script",
				Suggestion: "Use console.log sparingly in k6. Consider using k6's built-in metrics instead for better performance.",
				LineNumber: lineNum,
			})
		}

		if strings.Contains(line, "sleep(") && !strings.Contains(line, "import") {
			if !strings.Contains(script, "import { sleep }") && !strings.Contains(script, "from 'k6'") {
				issues = append(issues, ValidationIssue{
					Type:       "syntax",
					Severity:   "medium",
					Message:    "Using sleep without proper import",
					Suggestion: "Import sleep from k6: import { sleep } from 'k6';",
					LineNumber: lineNum,
				})
			}
		}
	}

	// Check for missing essential components
	if !hasImport {
		issues = append(issues, ValidationIssue{
			Type:       "syntax",
			Severity:   "high",
			Message:    "Missing k6 module imports",
			Suggestion: "Add import statements for k6 modules. Example: import http from 'k6/http';",
		})
	}

	if !hasDefaultFunction {
		issues = append(issues, ValidationIssue{
			Type:       "syntax",
			Severity:   "critical",
			Message:    "Missing default export function",
			Suggestion: "Add a default export function: export default function() { /* your test code */ }",
		})
	}

	if hasDefaultFunction && !hasHTTPCall && !strings.Contains(script, "check(") {
		issues = append(issues, ValidationIssue{
			Type:     "syntax",
			Severity: "low",
			Message:  "Script doesn't appear to make any HTTP requests or checks",
			Suggestion: "Add HTTP requests or checks to make your test meaningful. " +
				"Example: http.get('https://httpbin.org/get');",
		})
	}

	return issues
}

// analyzeK6Output analyzes k6 stderr and stdout for specific error patterns
func analyzeK6Output(stderr, stdout string) []ValidationIssue {
	var issues []ValidationIssue

	output := stderr + " " + stdout
	outputLower := strings.ToLower(output)

	// Common k6 error patterns
	if strings.Contains(outputLower, "syntaxerror") {
		issues = append(issues, ValidationIssue{
			Type:       "syntax",
			Severity:   "critical",
			Message:    "JavaScript syntax error in script",
			Suggestion: "Check your JavaScript syntax. Look for missing brackets, semicolons, or quotes.",
		})
	}

	if strings.Contains(outputLower, "referenceerror") {
		issues = append(issues, ValidationIssue{
			Type:       "syntax",
			Severity:   "high",
			Message:    "Reference error - undefined variable or function",
			Suggestion: "Check that all variables and functions are properly defined and imported.",
		})
	}

	if strings.Contains(outputLower, "cannot resolve module") || strings.Contains(outputLower, "module not found") {
		issues = append(issues, ValidationIssue{
			Type:       "import",
			Severity:   "high",
			Message:    "Module import error",
			Suggestion: "Check your import statements. Use 'search' tool with query 'k6 modules' to see available modules.",
		})
	}

	if strings.Contains(outputLower, "network") || strings.Contains(outputLower, "connection") {
		issues = append(issues, ValidationIssue{
			Type:       "network",
			Severity:   "medium",
			Message:    "Network connectivity issue",
			Suggestion: "Check that the target URL is accessible and network connection is available.",
		})
	}

	return issues
}

// generateValidationSummary creates a comprehensive summary of validation results
func generateValidationSummary(result *ValidationResult) ValidationSummary {
	summary := ValidationSummary{
		IssueCount: len(result.Issues),
		ReadyToRun: result.Valid && result.ExitCode == 0,
	}

	if result.Valid && result.ExitCode == 0 {
		if summary.IssueCount == 0 {
			summary.Status = "success"
			summary.Description = "Script validation passed with no issues"
			summary.Severity = "none"
		} else {
			summary.Status = "warning"
			summary.Description = fmt.Sprintf("Script validation passed but found %d minor issues", summary.IssueCount)
			summary.Severity = "low"
		}
	} else {
		summary.Status = "failed"
		summary.Description = "Script validation failed"
		summary.ReadyToRun = false

		// Determine severity based on issues
		maxSeverity := "low"
		for _, issue := range result.Issues {
			if compareSeverity(issue.Severity, maxSeverity) > 0 {
				maxSeverity = issue.Severity
			}
		}
		summary.Severity = maxSeverity
	}

	return summary
}

// compareSeverity compares two severity levels, returns 1 if a > b, -1 if a < b, 0 if equal
func compareSeverity(a, b string) int {
	severityOrder := map[string]int{
		"none":     0,
		"low":      1,
		"medium":   2,
		"high":     3,
		"critical": 4,
	}

	return severityOrder[a] - severityOrder[b]
}

// generateNextSteps provides actionable next steps based on validation results
func generateNextSteps(result *ValidationResult) []string {
	if result.Valid && result.ExitCode == 0 {
		steps := []string{"Your script is ready to run!"}

		if len(result.Issues) > 0 {
			steps = append(steps, "Consider addressing the minor issues found for better script quality")
		}

		steps = append(steps,
			"Use the 'run' tool to execute your script with desired parameters",
			"Use the 'search' tool to find examples for advanced testing scenarios",
		)

		return steps
	}

	// Script has issues
	steps := []string{"Fix the validation errors before running the script"}

	// Add specific steps based on issue types
	hasSecurityIssues := false
	hasSyntaxIssues := false
	hasImportIssues := false

	for _, issue := range result.Issues {
		switch issue.Type {
		case "security":
			hasSecurityIssues = true
		case "syntax":
			hasSyntaxIssues = true
		case "import":
			hasImportIssues = true
		}
	}

	if hasSecurityIssues {
		steps = append(steps, "Remove dangerous patterns and use only k6 APIs")
	}

	if hasSyntaxIssues {
		steps = append(steps, "Fix JavaScript syntax errors")
	}

	if hasImportIssues {
		steps = append(steps, "Correct import statements for k6 modules")
	}

	steps = append(steps,
		"Use the 'search' tool for k6 documentation and examples",
		"Start with a simple script template if needed",
	)

	return steps
}

// removeDuplicates removes duplicate strings from a slice
func removeDuplicates(slice []string) []string {
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

// addWorkflowIntegrationSuggestions adds suggestions for transitioning between validation and run
func addWorkflowIntegrationSuggestions(result *ValidationResult) {
	if result == nil {
		return
	}

	// Add specific workflow suggestions based on validation status
	switch {
	case result.Valid && result.Summary.ReadyToRun:
		// Script is ready to run - suggest next steps
		workflowSuggestions := []string{
			"✓ Validation passed! Your script is ready for load testing",
			"Use the 'run' tool to execute your script with different configurations:",
			"  • Basic test: {\"vus\": 1, \"duration\": \"30s\"}",
			"  • Load test: {\"vus\": 10, \"duration\": \"5m\"}",
			"  • Stress test: {\"vus\": 50, \"duration\": \"10m\"}",
		}

		// Insert workflow suggestions at the beginning of next steps
		result.NextSteps = append(workflowSuggestions, result.NextSteps...)

		// Add run-specific recommendations
		runRecommendations := []string{
			"Start with a small load (1-5 VUs) to verify functionality",
			"Gradually increase load to find performance limits",
			"Monitor response times and error rates during execution",
		}
		result.Recommendations = append(result.Recommendations, runRecommendations...)
	case result.Valid && len(result.Issues) > 0:
		// Script is valid but has minor issues
		result.NextSteps = append([]string{
			"Consider addressing the validation issues before running at scale",
			"You can still run the script, but monitor for the highlighted issues",
		}, result.NextSteps...)
	default:
		// Script has critical issues
		result.NextSteps = append([]string{
			"⚠ Fix validation errors before attempting to run the script",
			"Critical issues must be resolved for successful execution",
		}, result.NextSteps...)
	}

	// Add general workflow recommendations
	generalWorkflow := []string{
		"Recommended testing workflow: validate → run (small load) → analyze → scale up",
		"Use the 'search' tool for examples of advanced k6 patterns and configurations",
	}
	result.Recommendations = append(result.Recommendations, generalWorkflow...)

	// Remove duplicates after adding workflow suggestions
	result.NextSteps = removeDuplicates(result.NextSteps)
	result.Recommendations = removeDuplicates(result.Recommendations)
}
