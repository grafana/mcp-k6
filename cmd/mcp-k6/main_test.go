package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	k6docslib "github.com/grafana/k6-docs-lib"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"go.k6.io/k6/lib/fsext"

	"github.com/grafana/mcp-k6/internal/docs"
	"github.com/grafana/mcp-k6/mcpserver"
)

func TestRunFailsWhenK6Missing(t *testing.T) {
	t.Setenv("PATH", "")

	logger := newTestLogger()
	var stderr bytes.Buffer

	code := mcpserver.Run(context.Background(), logger, &stderr, mcpserver.DefaultConfig())
	assert.NotEqual(t, 0, code, "run should return non-zero exit code when k6 is missing")
	assert.Contains(t, stderr.String(), "mcp-k6 requires the `k6` executable")
}

func TestRunSucceedsWithStubbedK6(t *testing.T) {
	dir := t.TempDir()
	createK6Stub(t, dir)

	t.Setenv("PATH", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")
	}

	stubServe := func(*server.MCPServer, ...server.StdioOption) error {
		return nil
	}

	logger := newTestLogger()
	var stderr bytes.Buffer

	provider := newTestDocsProvider(t)

	code := mcpserver.Run(context.Background(), logger, &stderr, mcpserver.DefaultConfig(),
		mcpserver.WithServeStdio(stubServe),
		mcpserver.WithDocsProvider(provider),
	)
	assert.Equal(t, 0, code, "run should succeed when k6 is available")
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func newTestDocsProvider(t *testing.T) *docs.Provider {
	t.Helper()

	idx := &k6docslib.Index{
		Version:  "vtest",
		Sections: []k6docslib.Section{},
	}

	mi := k6docslib.NewMultiIndex()
	mi.Add("vtest", idx)
	mi.SetLatest("vtest")

	return docs.NewFromMultiIndex(mi, t.TempDir(), fsext.NewMemMapFs())
}

func createK6Stub(t *testing.T, dir string) {
	t.Helper()

	var filename, content string
	if runtime.GOOS == "windows" {
		filename = "k6.bat"
		content = "@echo off\nexit /b 0\n"
	} else {
		filename = "k6"
		content = "#!/bin/sh\nexit 0\n"
	}

	path := filepath.Join(dir, filename)
	//nolint:gosec,forbidigo // Test helper needs executable permissions for the stub.
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("failed to write k6 stub: %v", err)
	}
}
