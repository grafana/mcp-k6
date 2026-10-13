package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

// snapshotTools returns every tool the server registers (see
// mcpserver/server.go), so the snapshot guard covers the full MCP tool contract.
// Keep this list in sync with the registrations in mcpserver.
func snapshotTools() []mcp.Tool {
	return []mcp.Tool{
		InfoTool,
		ValidateTool,
		RunTool,
		SearchTerraformTool,
		ListSectionsTool,
		GetDocumentationTool,
	}
}

// TestToolSchemaSnapshots locks each tool's serialized JSON schema against a
// committed golden file. Any change to a tool's name, description, input schema,
// or annotations changes what clients and LLMs see, so it must be an intentional,
// reviewable diff. Regenerate the goldens after an intended change with:
//
//	UPDATE_TOOLSNAPS=true go test ./tools/ -run TestToolSchemaSnapshots
func TestToolSchemaSnapshots(t *testing.T) {
	t.Parallel()

	update := os.Getenv("UPDATE_TOOLSNAPS") != ""

	for _, tool := range snapshotTools() {
		t.Run(tool.Name, func(t *testing.T) {
			t.Parallel()

			got := canonicalToolSchema(t, tool)
			golden := filepath.Join("testdata", "toolsnaps", tool.Name+".json")

			if update {
				writeGolden(t, golden, got)
				return
			}

			//nolint:forbidigo // Test reads a committed golden file under testdata.
			want, err := os.ReadFile(golden)
			require.NoErrorf(t, err,
				"missing snapshot for tool %q; regenerate with UPDATE_TOOLSNAPS=true go test ./tools/ -run TestToolSchemaSnapshots",
				tool.Name)

			require.Equalf(t, string(want), string(got),
				"tool %q schema changed; if intentional, regenerate with UPDATE_TOOLSNAPS=true go test ./tools/ -run TestToolSchemaSnapshots",
				tool.Name)
		})
	}
}

// canonicalToolSchema marshals a tool to a stable, indented JSON form so goldens
// are deterministic and produce readable diffs. Round-tripping through a generic
// value sorts map keys (encoding/json orders object keys), which keeps the
// output independent of map iteration order.
func canonicalToolSchema(t *testing.T, tool mcp.Tool) string {
	t.Helper()

	raw, err := json.Marshal(tool)
	require.NoErrorf(t, err, "marshal tool %q", tool.Name)

	var generic any
	require.NoErrorf(t, json.Unmarshal(raw, &generic), "unmarshal tool %q", tool.Name)

	canonical, err := json.MarshalIndent(generic, "", "  ")
	require.NoErrorf(t, err, "re-marshal tool %q", tool.Name)

	return string(canonical) + "\n"
}

func writeGolden(t *testing.T, path, content string) {
	t.Helper()

	//nolint:forbidigo // Test helper creates the testdata snapshot directory.
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	//nolint:forbidigo // Test helper writes a committed golden file under testdata.
	require.NoErrorf(t, os.WriteFile(path, []byte(content), 0o600), "write snapshot %q", path)
}
