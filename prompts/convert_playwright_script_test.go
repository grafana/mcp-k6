//nolint:forbidigo // Tests need direct filesystem and process access to build fixtures.
package prompts

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	"github.com/grafana/mcp-k6/internal/logging"
)

// secretMarker is written to a file outside the working directory.
// It must never appear in any prompt result or error.
const secretMarker = "MCP_K6_SECRET_MARKER_7f3b9c" //nolint:gosec // Test marker, not a credential.

// fileAccessLayout is the on-disk layout used by the file access tests:
//
//	<tmp>/                   HOME / USERPROFILE
//	<tmp>/work/              working directory
//	<tmp>/outside/secret.txt file outside the working directory containing secretMarker
type fileAccessLayout struct {
	home    string
	work    string
	outside string
	secret  string
}

// newFileAccessLayout creates the layout, points the home directory at it and
// changes the working directory to <tmp>/work for the duration of the test.
func newFileAccessLayout(t *testing.T) fileAccessLayout {
	t.Helper()

	// Resolve symlinks in the temp dir itself (macOS /var -> /private/var) so
	// the paths used by the test match the working directory.
	tmp, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)

	l := fileAccessLayout{
		home:    tmp,
		work:    filepath.Join(tmp, "work"),
		outside: filepath.Join(tmp, "outside"),
	}
	l.secret = filepath.Join(l.outside, "secret.txt")

	require.NoError(t, os.MkdirAll(l.work, 0o700))
	require.NoError(t, os.MkdirAll(l.outside, 0o700))
	require.NoError(t, os.WriteFile(l.secret, []byte("const s = '"+secretMarker+"';\n"), 0o600))

	t.Setenv("HOME", l.home)
	t.Setenv("USERPROFILE", l.home)
	t.Chdir(l.work)

	return l
}

// quietContext returns a context whose logger discards output, keeping test
// output readable.
func quietContext() context.Context {
	return logging.ContextWithLogger(context.Background(), slog.New(slog.DiscardHandler))
}

func newConvertRequest(arg string) mcp.GetPromptRequest {
	return mcp.GetPromptRequest{
		Params: mcp.GetPromptParams{
			Name:      "convert_playwright_script",
			Arguments: map[string]string{"playwright_script": arg},
		},
	}
}

// callConvert invokes the prompt handler and returns everything the client
// would see: the serialized result, or the error text.
func callConvert(t *testing.T, arg string) (string, error) {
	t.Helper()

	result, err := convertPlaywrightScript(quietContext(), newConvertRequest(arg))
	if err != nil {
		return err.Error(), err
	}

	raw, mErr := json.Marshal(result)
	require.NoError(t, mErr)
	return string(raw), nil
}

type leakCase struct {
	name string
	arg  string
}

func assertNoLeak(t *testing.T, cases []leakCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response, _ := callConvert(t, tc.arg)
			if strings.Contains(response, secretMarker) {
				t.Fatalf("secret file was disclosed for argument %q", tc.arg)
			}
		})
	}
}

//nolint:paralleltest // Uses t.Chdir and t.Setenv, which are incompatible with t.Parallel.
func TestConvertPlaywrightScriptDoesNotReadOutsideWorkingDirectory(t *testing.T) {
	l := newFileAccessLayout(t)

	relSecret := filepath.Join("..", "outside", "secret.txt")
	homeSecret := filepath.Join("~", "outside", "secret.txt")

	cases := []leakCase{
		{"bare absolute path", l.secret},
		{"bare relative traversal", relSecret},
		{"bare home path", homeSecret},
		{"at absolute path", "@" + l.secret},
		{"at relative traversal", "@" + relSecret},
		{"at home path", "@" + homeSecret},
		{"at quoted absolute path", "@\"" + l.secret + "\""},
	}

	if runtime.GOOS == "windows" {
		cases = append(cases,
			leakCase{"at forward slash absolute path", "@" + filepath.ToSlash(l.secret)},
			leakCase{"at extended-length path", `@\\?\` + l.secret},
			leakCase{"bare extended-length path", `\\?\` + l.secret},
		)
	}

	assertNoLeak(t, cases)
}

//nolint:paralleltest // Uses t.Chdir and t.Setenv, which are incompatible with t.Parallel.
func TestConvertPlaywrightScriptDoesNotFollowSymlinksOutsideWorkingDirectory(t *testing.T) {
	l := newFileAccessLayout(t)

	links := map[string]string{
		"abs-link.js":  l.secret,
		"rel-link.js":  filepath.Join("..", "outside", "secret.txt"),
		"outside-dir":  l.outside,
		"rel-dir-link": filepath.Join("..", "outside"),
	}
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(l.work, name)); err != nil {
			t.Skipf("cannot create symlinks on this system: %v", err)
		}
	}

	viaDir := func(dir string) string { return filepath.Join(dir, "secret.txt") }

	assertNoLeak(t, []leakCase{
		{"bare absolute file symlink", "abs-link.js"},
		{"bare relative file symlink", "rel-link.js"},
		{"bare directory symlink", viaDir("outside-dir")},
		{"at absolute file symlink", "@abs-link.js"},
		{"at relative file symlink", "@rel-link.js"},
		{"at absolute directory symlink", "@" + viaDir("outside-dir")},
		{"at relative directory symlink", "@" + viaDir("rel-dir-link")},
	})
}

//nolint:paralleltest // Uses t.Chdir and t.Setenv, which are incompatible with t.Parallel.
func TestConvertPlaywrightScriptOnlyReadsScriptFiles(t *testing.T) {
	l := newFileAccessLayout(t)

	// Simulate an MCP client that launches the server from the user's home
	// directory: credentials then live inside the working directory, so only
	// the extension filter keeps them from being read.
	t.Setenv("HOME", l.work)
	t.Setenv("USERPROFILE", l.work)

	secret := []byte("-----BEGIN KEY----- " + secretMarker + "\n")
	sshDir := filepath.Join(l.work, ".ssh")
	require.NoError(t, os.MkdirAll(sshDir, 0o700))
	for _, name := range []string{
		filepath.Join(".ssh", "id_rsa"),
		".env",
		"notes.txt",
		"config.json",
		"script.js.bak",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(l.work, name), secret, 0o600))
	}

	t.Run("rejects non-script files", func(t *testing.T) {
		cases := []leakCase{
			{"at file without extension", "@" + filepath.Join(".ssh", "id_rsa")},
			{"at home file without extension", "@" + filepath.Join("~", ".ssh", "id_rsa")},
			{"at dotfile", "@.env"},
			{"at txt file", "@notes.txt"},
			{"at json file", "@config.json"},
			{"at script extension not last", "@script.js.bak"},
			{"bare file without extension", filepath.Join(".ssh", "id_rsa")},
			{"bare home file without extension", filepath.Join("~", ".ssh", "id_rsa")},
			{"bare txt file", "notes.txt"},
		}
		if runtime.GOOS == "windows" {
			cases = append(cases,
				leakCase{"at txt file default data stream", "@notes.txt::$DATA"},
			)
		}
		assertNoLeak(t, cases)
	})

	t.Run("rejects script-named symlinks to non-script files", func(t *testing.T) {
		links := map[string]string{
			"key.js":     filepath.Join(".ssh", "id_rsa"),
			"env.ts":     ".env",
			"chain-1.js": "chain-2.js",
			"chain-2.js": filepath.Join(".ssh", "id_rsa"),
		}
		for name, target := range links {
			if err := os.Symlink(target, filepath.Join(l.work, name)); err != nil {
				t.Skipf("cannot create symlinks on this system: %v", err)
			}
		}

		assertNoLeak(t, []leakCase{
			{"at symlink to key", "@key.js"},
			{"at symlink to dotfile", "@env.ts"},
			{"at symlink chain to key", "@chain-1.js"},
			{"bare symlink to key", "key.js"},
		})
	})

	t.Run("reads script files", func(t *testing.T) {
		for _, name := range []string{
			"script.js", "script.mjs", "script.cjs",
			"script.ts", "script.mts", "script.cts",
			"UPPER.JS",
		} {
			t.Run(name, func(t *testing.T) {
				marker := "SCRIPT_CONTENT_" + strings.ReplaceAll(name, ".", "_")
				require.NoError(t, os.WriteFile(filepath.Join(l.work, name),
					[]byte("const s = '"+marker+"';\n"), 0o600))

				response, err := callConvert(t, "@"+name)
				require.NoError(t, err)
				require.Contains(t, response, marker, "script file %q was not read", name)
			})
		}
	})
}
