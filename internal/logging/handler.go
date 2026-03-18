package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
)

// color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
)

// levelLabels maps slog levels to short colored labels.
var levelLabels = map[slog.Level]string{
	slog.LevelDebug: colorGray + "DBG" + colorReset,
	slog.LevelInfo:  colorCyan + "INF" + colorReset,
	slog.LevelWarn:  colorYellow + "WRN" + colorReset,
	slog.LevelError: colorRed + "ERR" + colorReset,
}

// plainLabels maps slog levels to uncolored labels.
var plainLabels = map[slog.Level]string{
	slog.LevelDebug: "DBG",
	slog.LevelInfo:  "INF",
	slog.LevelWarn:  "WRN",
	slog.LevelError: "ERR",
}

// TermHandler is an slog.Handler that produces compact, colored terminal
// output. Format: "LVL message key=value key=value\n"
type TermHandler struct {
	w     io.Writer
	mu    sync.Mutex
	level slog.Level
	color bool
	attrs []slog.Attr
}

// NewTermHandler creates a handler that writes human-friendly log lines.
// Color is enabled when w is a terminal (os.File with IsTerminal).
func NewTermHandler(w io.Writer, level slog.Level) *TermHandler {
	return &TermHandler{
		w:     w,
		level: level,
		color: isTerminal(w),
	}
}

func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func (h *TermHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

func (h *TermHandler) Handle(_ context.Context, r slog.Record) error {
	var label string
	if h.color {
		label = levelLabels[r.Level]
	} else {
		label = plainLabels[r.Level]
	}
	if label == "" {
		label = r.Level.String()
	}

	buf := make([]byte, 0, 128)
	buf = append(buf, label...)
	buf = append(buf, ' ')
	buf = append(buf, r.Message...)

	// pre-set attrs from WithAttrs
	for _, a := range h.attrs {
		buf = append(buf, ' ')
		buf = appendAttr(buf, a)
	}

	// per-record attrs
	r.Attrs(func(a slog.Attr) bool {
		buf = append(buf, ' ')
		buf = appendAttr(buf, a)
		return true
	})

	buf = append(buf, '\n')

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.w.Write(buf)
	return err
}

func (h *TermHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &TermHandler{
		w:     h.w,
		level: h.level,
		color: h.color,
		attrs: newAttrs,
	}
}

func (h *TermHandler) WithGroup(name string) slog.Handler {
	// Groups not used in this project; return as-is.
	return h
}

func appendAttr(buf []byte, a slog.Attr) []byte {
	buf = append(buf, a.Key...)
	buf = append(buf, '=')
	buf = append(buf, fmt.Sprintf("%v", a.Value.Any())...)
	return buf
}
