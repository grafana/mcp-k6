package security

import (
	"context"
	"errors"
	"testing"
)

func TestValidateScriptContentAllowsStaticFunctionDefinitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		script string
	}{
		{
			name: "default function without space before parens",
			script: `import http from "k6/http";

export default function() {
  http.get("https://example.com");
}`,
		},
		{
			name: "async default function",
			script: `export default async function() {
}`,
		},
		{
			name: "callback function",
			script: `import { group } from "k6";

export default function() {
  group("browse", function() {
  });
}`,
		},
		{
			name: "shared array initializer function",
			script: `import { SharedArray } from "k6/data";

const users = new SharedArray("users", function() {
  return [];
});

export default function() {
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := ValidateScriptContent(context.Background(), tt.script); err != nil {
				t.Fatalf("ValidateScriptContent() returned unexpected error: %v", err)
			}
		})
	}
}

func TestValidateScriptContentRejectsDangerousPatterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		script string
	}{
		{
			name: "function constructor",
			script: `export default function() {
  Function("return 1")();
}`,
		},
		{
			name: "new function constructor",
			script: `export default function() {
  new Function("return 1")();
}`,
		},
		{
			name: "eval",
			script: `export default function() {
  eval("1 + 1");
}`,
		},
		{
			name: "dynamic import",
			script: `export default function() {
  import("k6/http");
}`,
		},
		{
			name: "child process require",
			script: `export default function() {
  require("child_process");
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateScriptContent(context.Background(), tt.script)
			if err == nil {
				t.Fatal("ValidateScriptContent() returned nil error")
			}

			var securityErr *Error
			if !errors.As(err, &securityErr) {
				t.Fatalf("ValidateScriptContent() error type = %T, want *Error", err)
			}
			if securityErr.Type != "DANGEROUS_PATTERN" {
				t.Fatalf("ValidateScriptContent() error type = %q, want DANGEROUS_PATTERN", securityErr.Type)
			}
		})
	}
}
