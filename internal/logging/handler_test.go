package logging

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTermHandlerWritesMessage(t *testing.T) {
	var buf bytes.Buffer
	h := NewTermHandler(&buf, slog.LevelInfo)
	logger := slog.New(h)

	logger.Info("hello world")

	out := buf.String()
	assert.Contains(t, out, "INF")
	assert.Contains(t, out, "hello world")
	assert.True(t, out[len(out)-1] == '\n')
}

func TestTermHandlerIncludesAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := NewTermHandler(&buf, slog.LevelInfo)
	logger := slog.New(h)

	logger.Info("cache status", "stale", true, "entries", 42)

	out := buf.String()
	assert.Contains(t, out, "stale=true")
	assert.Contains(t, out, "entries=42")
}

func TestTermHandlerRespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	h := NewTermHandler(&buf, slog.LevelWarn)
	logger := slog.New(h)

	logger.Info("should be suppressed")
	logger.Warn("should appear")

	out := buf.String()
	assert.NotContains(t, out, "suppressed")
	assert.Contains(t, out, "should appear")
	assert.Contains(t, out, "WRN")
}

func TestTermHandlerLevels(t *testing.T) {
	tests := []struct {
		name  string
		level slog.Level
		label string
		log   func(*slog.Logger, string)
	}{
		{"debug", slog.LevelDebug, "DBG", func(l *slog.Logger, msg string) { l.Debug(msg) }},
		{"info", slog.LevelInfo, "INF", func(l *slog.Logger, msg string) { l.Info(msg) }},
		{"warn", slog.LevelWarn, "WRN", func(l *slog.Logger, msg string) { l.Warn(msg) }},
		{"error", slog.LevelError, "ERR", func(l *slog.Logger, msg string) { l.Error(msg) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			h := NewTermHandler(&buf, slog.LevelDebug)
			logger := slog.New(h)

			tt.log(logger, "test message")

			out := buf.String()
			assert.Contains(t, out, tt.label)
			assert.Contains(t, out, "test message")
		})
	}
}

func TestTermHandlerNoColorForNonTerminal(t *testing.T) {
	var buf bytes.Buffer
	h := NewTermHandler(&buf, slog.LevelInfo)
	logger := slog.New(h)

	// bytes.Buffer is not a terminal, so no color codes
	assert.False(t, h.color)

	logger.Warn("no color")
	out := buf.String()
	assert.NotContains(t, out, "\033[")
}

func TestTermHandlerWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := NewTermHandler(&buf, slog.LevelInfo)
	logger := slog.New(h).With("component", "cache")

	logger.Info("refreshed")

	out := buf.String()
	assert.Contains(t, out, "component=cache")
	assert.Contains(t, out, "refreshed")
}

func TestTermHandlerEnabled(t *testing.T) {
	var buf bytes.Buffer
	h := NewTermHandler(&buf, slog.LevelWarn)

	require.False(t, h.Enabled(context.Background(), slog.LevelDebug))
	require.False(t, h.Enabled(context.Background(), slog.LevelInfo))
	require.True(t, h.Enabled(context.Background(), slog.LevelWarn))
	require.True(t, h.Enabled(context.Background(), slog.LevelError))
}
