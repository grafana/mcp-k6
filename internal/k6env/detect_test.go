package k6env_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/grafana/k6-mcp/internal/k6env"
)

func TestLocateReportsExecutable(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	createStub(t, dir, stubContent())

	t.Setenv("PATH", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")
	}

	info, err := k6env.Locate(context.Background())
	if err != nil {
		t.Fatalf("Locate returned error: %v", err)
	}

	if info.Path == "" {
		t.Fatalf("Locate returned empty path")
	}
}

func TestLocateReturnsErrorWhenMissing(t *testing.T) {
	t.Setenv("PATH", "")

	_, err := k6env.Locate(context.Background())
	if err == nil {
		t.Fatalf("expected error when k6 is missing")
	}
}

func TestInfoVersion(t *testing.T) {
	dir := t.TempDir()
	path := createStub(t, dir, versionStubContent())

	t.Setenv("PATH", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")
	}

	info, err := k6env.Locate(context.Background())
	if err != nil {
		t.Fatalf("Locate returned error: %v", err)
	}

	version, err := info.Version(context.Background())
	if err != nil {
		t.Fatalf("Version returned error: %v", err)
	}

	expected := "k6 v0.0.0-test"
	if version != expected {
		t.Fatalf("Version = %q, want %q (stub path: %s)", version, expected, path)
	}
}

func createStub(t *testing.T, dir, content string) string {
	t.Helper()
	var filename string
	if runtime.GOOS == "windows" {
		filename = "k6.bat"
	} else {
		filename = "k6"
	}

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatalf("failed to write stub executable: %v", err)
	}

	return path
}

func stubContent() string {
	if runtime.GOOS == "windows" {
		return "@echo off\n"
	}

	return "#!/bin/sh\ntrue\n"
}

func versionStubContent() string {
	if runtime.GOOS == "windows" {
		return "@echo off\nif \"%1\"==\"version\" (\n  echo k6 v0.0.0-test\n  exit /b 0\n)\necho unexpected args >&2\nexit /b 1\n"
	}

	return "#!/bin/sh\nif [ \"$1\" = \"version\" ]; then\n  echo \"k6 v0.0.0-test\"\n  exit 0\nfi\necho \"unexpected args\" 1>&2\nexit 1\n"
}
