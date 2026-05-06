package logging_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/grafana/mcp-k6/internal/logging"
)

//nolint:forbidigo // test helper for logrus<->slog bridge
func newTestLogrus(out *bytes.Buffer) *logrus.Logger {
	l := logrus.New()
	l.SetOutput(out)
	l.SetFormatter(&logrus.JSONFormatter{TimestampFormat: time.RFC3339})
	l.SetLevel(logrus.DebugLevel)
	return l
}

func TestNewLogrusLogger_returnsValidSlogLogger(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := newTestLogrus(&buf)
	sl := logging.NewLogrusLogger(l)
	if sl == nil {
		t.Fatal("expected non-nil *slog.Logger")
	}
}

func TestLogrusHandler_Enabled(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := newTestLogrus(&buf)

	tests := []struct {
		logrusLevel logrus.Level
		slogLevel   slog.Level
		want        bool
	}{
		{logrus.InfoLevel, slog.LevelDebug, false},
		{logrus.InfoLevel, slog.LevelInfo, true},
		{logrus.InfoLevel, slog.LevelWarn, true},
		{logrus.DebugLevel, slog.LevelDebug, true},
	}
	for _, tc := range tests {
		l.SetLevel(tc.logrusLevel)
		h := logging.NewLogrusHandler(l)
		got := h.Enabled(context.Background(), tc.slogLevel)
		if got != tc.want {
			t.Errorf("Enabled(logrus=%v, slog=%v) = %v, want %v", tc.logrusLevel, tc.slogLevel, got, tc.want)
		}
	}
}

func TestLogrusHandler_Handle_levels(t *testing.T) {
	t.Parallel()

	cases := []struct {
		slogLevel slog.Level
		wantLevel string
	}{
		{slog.LevelDebug, "debug"},
		{slog.LevelInfo, "info"},
		{slog.LevelWarn, "warning"},
		{slog.LevelError, "error"},
	}

	for _, tc := range cases {
		var buf bytes.Buffer
		l := newTestLogrus(&buf)
		sl := logging.NewLogrusLogger(l)
		sl.Log(context.Background(), tc.slogLevel, "hello")

		var entry map[string]any
		if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
			t.Fatalf("level=%v: failed to parse JSON: %v (output: %s)", tc.slogLevel, err, buf.String())
		}
		if got := entry["level"]; got != tc.wantLevel {
			t.Errorf("slogLevel=%v: got logrus level %q, want %q", tc.slogLevel, got, tc.wantLevel)
		}
		if got := entry["msg"]; got != "hello" {
			t.Errorf("unexpected msg: %v", got)
		}
	}
}

func TestLogrusHandler_Handle_attributes(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := newTestLogrus(&buf)
	sl := logging.NewLogrusLogger(l)

	sl.Info("test", slog.String("tool", "run"), slog.Int("vus", 10))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v (output: %s)", err, buf.String())
	}
	if got := entry["tool"]; got != "run" {
		t.Errorf("tool field: got %v, want run", got)
	}
	if got, ok := entry["vus"].(float64); !ok || int(got) != 10 {
		t.Errorf("vus field: got %v, want 10", entry["vus"])
	}
}

func TestLogrusHandler_WithAttrs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := newTestLogrus(&buf)
	sl := logging.NewLogrusLogger(l).With(slog.String("component", "mcp"))

	sl.Info("test")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v (output: %s)", err, buf.String())
	}
	if got := entry["component"]; got != "mcp" {
		t.Errorf("component field: got %v, want mcp", got)
	}
}

func TestLogrusHandler_WithGroup(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := newTestLogrus(&buf)
	sl := logging.NewLogrusLogger(l).WithGroup("db")

	sl.Info("query", slog.String("table", "users"))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v (output: %s)", err, buf.String())
	}
	if got := entry["db.table"]; got != "users" {
		t.Errorf("db.table field: got %v, want users", got)
	}
}

func TestLogrusHandler_Handle_noOutput_whenLevelFiltered(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := newTestLogrus(&buf)
	l.SetLevel(logrus.WarnLevel)
	sl := logging.NewLogrusLogger(l)

	sl.Debug("should not appear")
	sl.Info("should not appear")

	if buf.Len() > 0 {
		t.Errorf("expected no output, got: %s", buf.String())
	}
}
