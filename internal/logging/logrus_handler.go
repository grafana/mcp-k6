// Package logging provides structured logging functionality for the k6 MCP server.
package logging

import (
	"context"
	"log/slog"
	"strings"

	"github.com/sirupsen/logrus"
)

// LogrusHandler is a slog.Handler that forwards records to a *logrus.Logger.
// It allows slog-native code to emit through a logrus pipeline (formatter,
// hooks, output writer), which is useful when embedding mcp-k6 inside a host
// application that owns logging configuration via logrus (e.g. k6).
type LogrusHandler struct {
	//nolint:forbidigo // bridging logrus <-> slog: using *logrus.Logger is intentional
	logger *logrus.Logger
	attrs  logrus.Fields
	groups []string
}

// NewLogrusHandler returns a slog.Handler that writes records to l.
//
//nolint:forbidigo // bridging logrus <-> slog: using *logrus.Logger is intentional
func NewLogrusHandler(l *logrus.Logger) *LogrusHandler {
	return &LogrusHandler{logger: l, attrs: logrus.Fields{}}
}

// NewLogrusLogger returns an *slog.Logger backed by l.
//
//nolint:forbidigo // bridging logrus <-> slog: using *logrus.Logger is intentional
func NewLogrusLogger(l *logrus.Logger) *slog.Logger {
	return slog.New(NewLogrusHandler(l))
}

// Enabled reports whether the underlying logrus logger has the corresponding
// level enabled.
func (h *LogrusHandler) Enabled(_ context.Context, level slog.Level) bool {
	return h.logger.IsLevelEnabled(toLogrusLevel(level))
}

// Handle translates the slog.Record into a logrus log entry.
func (h *LogrusHandler) Handle(ctx context.Context, r slog.Record) error {
	fields := make(logrus.Fields, len(h.attrs)+r.NumAttrs())
	for k, v := range h.attrs {
		fields[k] = v
	}
	prefix := groupPrefix(h.groups)
	r.Attrs(func(a slog.Attr) bool {
		addAttr(fields, prefix, a)
		return true
	})

	entry := h.logger.WithContext(ctx).WithFields(fields)
	if !r.Time.IsZero() {
		entry = entry.WithTime(r.Time)
	}
	entry.Log(toLogrusLevel(r.Level), r.Message)
	return nil
}

// WithAttrs returns a new handler with the given attributes appended.
func (h *LogrusHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	cloned := h.clone()
	prefix := groupPrefix(h.groups)
	for _, a := range attrs {
		addAttr(cloned.attrs, prefix, a)
	}
	return cloned
}

// WithGroup returns a new handler that nests subsequent attributes under name.
func (h *LogrusHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	cloned := h.clone()
	cloned.groups = append(cloned.groups, name)
	return cloned
}

func (h *LogrusHandler) clone() *LogrusHandler {
	attrs := make(logrus.Fields, len(h.attrs))
	for k, v := range h.attrs {
		attrs[k] = v
	}
	groups := make([]string, len(h.groups))
	copy(groups, h.groups)
	return &LogrusHandler{
		logger: h.logger,
		attrs:  attrs,
		groups: groups,
	}
}

func groupPrefix(groups []string) string {
	if len(groups) == 0 {
		return ""
	}
	return strings.Join(groups, ".") + "."
}

func addAttr(fields logrus.Fields, prefix string, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		nested := prefix
		if a.Key != "" {
			nested = prefix + a.Key + "."
		}
		for _, ga := range a.Value.Group() {
			addAttr(fields, nested, ga)
		}
		return
	}
	fields[prefix+a.Key] = a.Value.Any()
}

func toLogrusLevel(level slog.Level) logrus.Level {
	switch {
	case level <= slog.LevelDebug:
		return logrus.DebugLevel
	case level <= slog.LevelInfo:
		return logrus.InfoLevel
	case level <= slog.LevelWarn:
		return logrus.WarnLevel
	default:
		return logrus.ErrorLevel
	}
}
