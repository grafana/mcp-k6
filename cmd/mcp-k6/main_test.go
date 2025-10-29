//go:build fts5

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

	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
)

func TestRunFailsWhenK6Missing(t *testing.T) {
	t.Setenv("PATH", "")

	logger := newTestLogger()
	var stderr bytes.Buffer

	code := run(context.Background(), logger, &stderr)
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

	originalServe := serveStdio
	serveStdio = func(*server.MCPServer, ...server.StdioOption) error {
		return nil
	}
	t.Cleanup(func() {
		serveStdio = originalServe
	})

	logger := newTestLogger()
	var stderr bytes.Buffer

	code := run(context.Background(), logger, &stderr)
	assert.Equal(t, 0, code, "run should succeed when k6 is available")
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
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
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("failed to write k6 stub: %v", err)
	}

}
