package tools

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateInput(t *testing.T) {
	t.Parallel()

	t.Run("empty script is rejected", func(t *testing.T) {
		t.Parallel()

		err := validateInput("")
		require.Error(t, err)
		var valErr *ValidationError
		require.ErrorAs(t, err, &valErr)
		require.Equal(t, errTypeInputValidation, valErr.Type)
	})

	t.Run("oversized script is rejected", func(t *testing.T) {
		t.Parallel()

		err := validateInput(strings.Repeat("a", MaxScriptSize+1))
		require.Error(t, err)
		var valErr *ValidationError
		require.ErrorAs(t, err, &valErr)
		require.Equal(t, errTypeInputValidation, valErr.Type)
		require.Contains(t, valErr.Message, "size")
	})

	t.Run("valid script passes", func(t *testing.T) {
		t.Parallel()

		require.NoError(t, validateInput("export default function() {}"))
	})
}

func TestIsThresholdFailure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		stderr, stdout string
		want           bool
	}{
		{name: "threshold failure with no syntax error", stderr: "some thresholds have failed", want: true},
		{name: "threshold wording variant", stdout: "threshold violation detected", want: true},
		{name: "threshold failure alongside syntax error is not a threshold failure", stderr: "thresholds have failed: SyntaxError: bad", want: false},
		{name: "unrelated failure", stderr: "connection refused", want: false},
		{name: "clean output", stderr: "", stdout: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, isThresholdFailure(tt.stderr, tt.stdout))
		})
	}
}

func TestHasSyntaxErrors(t *testing.T) {
	t.Parallel()

	// hasSyntaxErrors matches lowercase patterns against already-lowercased output.
	for _, pattern := range []string{
		"syntaxerror", "referenceerror", "typeerror", "cannot resolve module",
		"module not found", "unexpected token", "unexpected end of input",
		"invalid or unexpected token", "parsing error", "compilation error",
	} {
		require.Truef(t, hasSyntaxErrors("k6 failed: "+pattern), "expected %q to be detected", pattern)
	}

	require.False(t, hasSyntaxErrors("some thresholds have failed"))
}

func TestMapErrorTypeToIssueType(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		errTypeInputValidation: "syntax",
		"FILE_CREATION":        "system",
		"FILE_WRITE":           "system",
		errTypeK6NotFound:      "environment",
		errTypeExecutionError:  "environment",
		"TIMEOUT":              "performance",
		"SOMETHING_ELSE":       "unknown",
	}

	for input, want := range tests {
		require.Equalf(t, want, mapErrorTypeToIssueType(input), "input %q", input)
	}
}

func TestMapErrorTypeToSeverity(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		errTypeK6NotFound:      "critical",
		errTypeExecutionError:  "critical",
		errTypeInputValidation: "high",
		"FILE_CREATION":        "high",
		"TIMEOUT":              "medium",
		"SOMETHING_ELSE":       "medium",
	}

	for input, want := range tests {
		require.Equalf(t, want, mapErrorTypeToSeverity(input), "input %q", input)
	}
}

func TestGetSuggestionForErrorType(t *testing.T) {
	t.Parallel()

	require.Contains(t, getSuggestionForErrorType(errTypeInputValidation, "script content cannot be empty"), "import")
	require.Contains(t, getSuggestionForErrorType(errTypeInputValidation, "script size exceeds"), "size")
	require.Contains(t, getSuggestionForErrorType(errTypeInputValidation, "other"), "syntax")
	require.Contains(t, getSuggestionForErrorType(errTypeK6NotFound, ""), "Install k6")
	require.Contains(t, getSuggestionForErrorType("TIMEOUT", ""), "infinite loops")
	require.NotEmpty(t, getSuggestionForErrorType("SOMETHING_ELSE", ""))
}

func TestCreateValidationIssueFromError(t *testing.T) {
	t.Parallel()

	t.Run("validation error is mapped", func(t *testing.T) {
		t.Parallel()

		issue := createValidationIssueFromError(&ValidationError{
			Type:    errTypeInputValidation,
			Message: "script content cannot be empty",
		})
		require.Equal(t, "syntax", issue.Type)
		require.Equal(t, "high", issue.Severity)
		require.Contains(t, issue.Suggestion, "import")
	})

	t.Run("plain error falls back to unknown", func(t *testing.T) {
		t.Parallel()

		issue := createValidationIssueFromError(errors.New("boom"))
		require.Equal(t, "unknown", issue.Type)
		require.Equal(t, "medium", issue.Severity)
		require.Equal(t, "boom", issue.Message)
	})
}

func TestGetRecommendationsForIssue(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, getRecommendationsForIssue(ValidationIssue{Type: "syntax"}))
	require.NotEmpty(t, getRecommendationsForIssue(ValidationIssue{Type: "environment"}))
	require.NotEmpty(t, getRecommendationsForIssue(ValidationIssue{Type: "anything-else"}))
}

func TestCompareSeverity(t *testing.T) {
	t.Parallel()

	require.Positive(t, compareSeverity("critical", "high"))
	require.Negative(t, compareSeverity("low", "medium"))
	require.Zero(t, compareSeverity("high", "high"))
	require.Positive(t, compareSeverity("high", "none"))
}

func TestRemoveDuplicates(t *testing.T) {
	t.Parallel()

	require.Equal(t, []string{"a", "b", "c"}, removeDuplicates([]string{"a", "b", "a", "c", "b"}))
	require.Nil(t, removeDuplicates(nil))
	require.Equal(t, []string{"x"}, removeDuplicates([]string{"x", "x", "x"}))
}

func TestAnalyzeScriptContent(t *testing.T) {
	t.Parallel()

	hasIssue := func(issues []ValidationIssue, msgSubstr string) bool {
		for _, i := range issues {
			if strings.Contains(i.Message, msgSubstr) {
				return true
			}
		}
		return false
	}

	t.Run("missing import and default function are flagged", func(t *testing.T) {
		t.Parallel()

		issues := analyzeScriptContent("const x = 1;")
		require.True(t, hasIssue(issues, "Missing k6 module imports"))
		require.True(t, hasIssue(issues, "Missing default export function"))
	})

	t.Run("console.log is flagged as low severity", func(t *testing.T) {
		t.Parallel()

		issues := analyzeScriptContent("import http from 'k6/http';\nexport default function() {\n  console.log('hi');\n  http.get('https://example.com');\n}")
		var found *ValidationIssue
		for i := range issues {
			if strings.Contains(issues[i].Message, "console.log") {
				found = &issues[i]
			}
		}
		require.NotNil(t, found)
		require.Equal(t, "low", found.Severity)
		require.Positive(t, found.LineNumber)
	})

	t.Run("well-formed script with http call has no missing-component issues", func(t *testing.T) {
		t.Parallel()

		issues := analyzeScriptContent("import http from 'k6/http';\nexport default function() {\n  http.get('https://example.com');\n}")
		require.False(t, hasIssue(issues, "Missing k6 module imports"))
		require.False(t, hasIssue(issues, "Missing default export function"))
		require.False(t, hasIssue(issues, "doesn't appear to make any HTTP requests"))
	})

	t.Run("script with no requests or checks is flagged", func(t *testing.T) {
		t.Parallel()

		issues := analyzeScriptContent("import { sleep } from 'k6';\nexport default function() {\n  sleep(1);\n}")
		require.True(t, hasIssue(issues, "doesn't appear to make any HTTP requests"))
	})
}

func TestAnalyzeK6Output(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		stderr   string
		wantType string
	}{
		{name: "syntax error", stderr: "ERRO SyntaxError: unexpected token", wantType: "syntax"},
		{name: "reference error", stderr: "ReferenceError: foo is not defined", wantType: "syntax"},
		{name: "module error", stderr: "cannot resolve module 'k6/xyz'", wantType: "import"},
		{name: "network error", stderr: "dial tcp: connection refused", wantType: "network"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			issues := analyzeK6Output(tt.stderr, "")
			require.NotEmpty(t, issues)
			require.Equal(t, tt.wantType, issues[0].Type)
		})
	}

	require.Empty(t, analyzeK6Output("", ""))
}

func TestGenerateValidationSummary(t *testing.T) {
	t.Parallel()

	t.Run("success with no issues", func(t *testing.T) {
		t.Parallel()

		s := generateValidationSummary(&ValidationResponse{Valid: true, ExitCode: 0})
		require.Equal(t, "success", s.Status)
		require.Equal(t, "none", s.Severity)
		require.True(t, s.ReadyToRun)
	})

	t.Run("warning when valid but issues present", func(t *testing.T) {
		t.Parallel()

		s := generateValidationSummary(&ValidationResponse{
			Valid: true, ExitCode: 0,
			Issues: []ValidationIssue{{Severity: "low"}},
		})
		require.Equal(t, "warning", s.Status)
		require.True(t, s.ReadyToRun)
		require.Equal(t, 1, s.IssueCount)
	})

	t.Run("failed uses max severity and is not ready", func(t *testing.T) {
		t.Parallel()

		s := generateValidationSummary(&ValidationResponse{
			Valid: false, ExitCode: 1,
			Issues: []ValidationIssue{{Severity: "low"}, {Severity: "critical"}, {Severity: "medium"}},
		})
		require.Equal(t, "failed", s.Status)
		require.False(t, s.ReadyToRun)
		require.Equal(t, "critical", s.Severity)
	})
}

func TestGenerateValidationNextSteps(t *testing.T) {
	t.Parallel()

	ready := generateValidationNextSteps(&ValidationResponse{Valid: true, ExitCode: 0})
	require.Contains(t, ready[0], "ready to run")

	failed := generateValidationNextSteps(&ValidationResponse{
		Valid: false, ExitCode: 1,
		Issues: []ValidationIssue{{Type: "syntax"}, {Type: "import"}},
	})
	joined := strings.Join(failed, "\n")
	require.Contains(t, joined, "Fix JavaScript syntax errors")
	require.Contains(t, joined, "Correct import statements")
}

func TestEnhanceValidationResult(t *testing.T) {
	t.Parallel()

	t.Run("nil result is a no-op", func(t *testing.T) {
		t.Parallel()

		require.NotPanics(t, func() { enhanceValidationResult(nil, "script") })
	})

	t.Run("populates summary, recommendations and next steps", func(t *testing.T) {
		t.Parallel()

		result := &ValidationResponse{Valid: true, ExitCode: 0}
		enhanceValidationResult(result, "const x = 1;")

		require.NotEmpty(t, result.Summary.Status)
		require.NotEmpty(t, result.Issues) // missing import + default function
		require.NotEmpty(t, result.NextSteps)
		require.Equal(t, result.Recommendations, removeDuplicates(result.Recommendations), "recommendations must be de-duplicated")
		require.Equal(t, result.NextSteps, removeDuplicates(result.NextSteps), "next steps must be de-duplicated")
	})
}

func TestAddWorkflowIntegrationSuggestions(t *testing.T) {
	t.Parallel()

	t.Run("nil result is a no-op", func(t *testing.T) {
		t.Parallel()

		require.NotPanics(t, func() { addWorkflowIntegrationSuggestions(nil) })
	})

	t.Run("ready-to-run script gets run guidance first", func(t *testing.T) {
		t.Parallel()

		result := &ValidationResponse{Valid: true, Summary: ValidationSummary{ReadyToRun: true}}
		addWorkflowIntegrationSuggestions(result)
		require.Contains(t, result.NextSteps[0], "Validation passed")
	})

	t.Run("invalid script gets fix-first guidance", func(t *testing.T) {
		t.Parallel()

		result := &ValidationResponse{Valid: false}
		addWorkflowIntegrationSuggestions(result)
		require.Contains(t, result.NextSteps[0], "Fix validation errors")
	})
}

// TestValidateK6ScriptWithStub drives the full validate path against a stubbed k6
// binary that exits 0, exercising validateInput -> executeK6Validation ->
// enhanceValidationResult without a real k6 install.
func TestValidateK6ScriptWithStub(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("recording shell stub is Unix-only")
	}

	dir := t.TempDir()
	createRecordingK6Stub(t, dir, filepath.Join(dir, "args.txt"))
	t.Setenv("PATH", dir)

	result, err := validateK6Script(context.Background(), "import http from 'k6/http';\nexport default function() { http.get('https://example.com'); }")
	require.NoError(t, err)
	require.True(t, result.Valid)
	require.Equal(t, 0, result.ExitCode)
	require.True(t, result.Summary.ReadyToRun)
	require.NotEmpty(t, result.NextSteps)
}
