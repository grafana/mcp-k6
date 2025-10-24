package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/grafana/k6-mcp/internal/runner"
	"github.com/mark3labs/mcp-go/mcp"
)

// RunHandler handles k6 test execution tool requests.
type RunHandler struct{}

// NewRunHandler creates a new RunHandler instance.
func NewRunHandler() *RunHandler {
	return &RunHandler{}
}

// Handle processes k6 test run requests.
func (r RunHandler) Handle(
	ctx context.Context,
	request mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	args := request.GetArguments()

	// Extract script content from arguments
	scriptValue, exists := args["script"]
	if !exists {
		return mcp.NewToolResultError(
			"Missing required parameter 'script'. " +
				"Please provide your k6 script content as a string. " +
				"Tip: Use the 'validate' tool first to check your script before running.",
		), nil
	}

	script, ok := scriptValue.(string)
	if !ok {
		return mcp.NewToolResultError(
			"Parameter 'script' must be a string containing your k6 script code. " +
				"Received: " + fmt.Sprintf("%T", scriptValue),
		), nil
	}

	// Parse run options from arguments
	options, err := parseRunOptions(args)
	if err != nil {
		// Include parameter suggestions in error message
		suggestions := suggestParameterImprovements(args)
		n := 2
		if len(suggestions) < n {
			n = len(suggestions)
		}
		suggestionText := ""
		if n > 0 {
			suggestionText = " Suggestions: " + strings.Join(suggestions[:n], "; ")
		}
		return mcp.NewToolResultError(
			fmt.Sprintf(
				"Invalid parameters: %v. Check parameter types and ranges.%s "+
					"Use the 'search' tool with query 'run options' for more examples.",
				err, suggestionText,
			),
		), nil
	}

	// Run the k6 test
	result, _ := runner.RunK6Test(ctx, script, options)
	// Note: We intentionally ignore runErr here because the result contains detailed error information

	// Convert result to JSON for structured response
	resultJSON, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to serialize run result"), err
	}

	// Include key outcome fields in logs (using slog directly to avoid dependency on logging helpers here)
	slog.InfoContext(ctx, "run completed",
		slog.Bool("success", result != nil && result.Success),
	)

	// Return structured result
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// parseRunOptions parses run options from the tool arguments.
func parseRunOptions(args map[string]interface{}) (*runner.RunOptions, error) {
	options := &runner.RunOptions{}

	// Apply smart defaults based on context
	applySmartDefaults(options, args)

	// Parse VUs
	if vusValue, exists := args["vus"]; exists {
		if vus, ok := vusValue.(float64); ok {
			options.VUs = int(vus)
		} else {
			return nil, fmt.Errorf("vus must be a number (received %T). Example: 10", vusValue)
		}
	}

	// Parse duration
	if durationValue, exists := args["duration"]; exists {
		if duration, ok := durationValue.(string); ok {
			options.Duration = duration
		} else {
			return nil, fmt.Errorf("duration must be a string (received %T). Examples: '30s', '2m', '5m'", durationValue)
		}
	}

	// Parse iterations
	if iterationsValue, exists := args["iterations"]; exists {
		if iterations, ok := iterationsValue.(float64); ok {
			options.Iterations = int(iterations)
		} else {
			return nil, fmt.Errorf("iterations must be a number (received %T). Example: 100", iterationsValue)
		}
	}

	// Parse stages
	if stagesValue, exists := args["stages"]; exists {
		stagesData, err := json.Marshal(stagesValue)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to marshal stages: %w. Expected array of {duration, target} objects",
				err,
			)
		}

		var stages []runner.Stage
		if err := json.Unmarshal(stagesData, &stages); err != nil {
			return nil, fmt.Errorf(
				"invalid stages format: %w. "+
					"Example: [{\"duration\": \"30s\", \"target\": 10}, "+
					"{\"duration\": \"1m\", \"target\": 20}]",
				err,
			)
		}
		options.Stages = stages
	}

	// Parse additional options
	if optionsValue, exists := args["options"]; exists {
		if opts, ok := optionsValue.(map[string]interface{}); ok {
			options.Options = opts
		} else {
			return nil, fmt.Errorf(
				"options must be an object (received %T). "+
					"Example: {\"thresholds\": {\"http_req_duration\": [\"p(95)<500\"]}}",
				optionsValue,
			)
		}
	}

	return options, nil
}

// applySmartDefaults applies context-aware defaults to run options
func applySmartDefaults(options *runner.RunOptions, args map[string]interface{}) {
	// Check if user specified any custom parameters
	hasCustomVUs := args["vus"] != nil
	hasCustomDuration := args["duration"] != nil
	hasCustomIterations := args["iterations"] != nil
	hasCustomStages := args["stages"] != nil

	// If no parameters specified, apply intelligent defaults
	if !hasCustomVUs && !hasCustomDuration && !hasCustomIterations && !hasCustomStages {
		// Default to a basic load test
		options.VUs = 1
		options.Duration = "30s"
		return
	}

	// Smart defaults based on what user specified
	if hasCustomStages {
		// If stages are specified, don't set VUs or duration as they'll be controlled by stages
		return
	}

	if hasCustomIterations && !hasCustomVUs {
		// If iterations specified but no VUs, suggest appropriate VU count
		if iterations, ok := args["iterations"].(float64); ok {
			iterationCount := int(iterations)
			switch {
			case iterationCount <= 10:
				options.VUs = 1
			case iterationCount <= 100:
				options.VUs = 5
			case iterationCount <= 1000:
				options.VUs = 10
			default:
				options.VUs = 20
			}
		}
	}

	if hasCustomVUs && !hasCustomDuration && !hasCustomIterations {
		// If VUs specified but no duration/iterations, suggest appropriate duration
		if vus, ok := args["vus"].(float64); ok {
			vusCount := int(vus)
			switch {
			case vusCount <= 5:
				options.Duration = "1m"
			case vusCount <= 20:
				options.Duration = "2m"
			default:
				options.Duration = "5m"
			}
		}
	}
}

// suggestParameterImprovements suggests better parameter combinations
func suggestParameterImprovements(args map[string]interface{}) []string {
	var suggestions []string

	// Check for common parameter issues
	vusValue, hasVUs := args["vus"].(float64)
	if !hasVUs {
		return suggestions
	}

	vusCount := int(vusValue)

	if duration, hasDuration := args["duration"].(string); hasDuration {
		// Suggest improvements for VU/duration combinations
		if vusCount == 1 && duration == "30s" {
			suggestions = append(suggestions,
				"Consider increasing VUs to 5-10 for more realistic load testing",
				"Try extending duration to 2-5 minutes for better insights",
			)
		}

		if vusCount > 20 && duration == "30s" {
			suggestions = append(suggestions,
				"With high VU count, consider longer duration (5-10 minutes) for stability",
			)
		}
	}

	if iterationsValue, hasIterations := args["iterations"].(float64); hasIterations {
		iterationCount := int(iterationsValue)

		// Check for inefficient VU/iteration combinations
		if iterationCount < vusCount {
			suggestions = append(suggestions,
				fmt.Sprintf(
					"You have more VUs (%d) than iterations (%d). "+
						"Consider increasing iterations or reducing VUs",
					vusCount, iterationCount,
				),
			)
		}

		if iterationCount > vusCount*100 {
			suggestions = append(suggestions,
				"Very high iteration count per VU. Consider using duration-based testing instead",
			)
		}
	}

	// Check for missing important parameters
	if _, hasVUs := args["vus"]; !hasVUs {
		if _, hasStages := args["stages"]; !hasStages {
			suggestions = append(suggestions, "Consider specifying VUs for better load simulation")
		}
	}

	// Suggest preset configurations
	suggestions = append(suggestions,
		"Try preset configurations: 'smoke_test', 'load_test', 'stress_test' for common scenarios",
	)

	return suggestions
}
