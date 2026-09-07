// Package prettylog provides a human-friendly slog.Handler for terminal output.
//
// Logs are grouped by namespace (first two segments of the message key),
// displayed with colored icons per level, and formatted with short timestamps.
//
// Example output:
//
//	─── tracker.sync ──────────────────────────────────
//	  → 15:50:05 start     projects=1 autoplan=true
//	  → 15:50:05 members   count=1
//	  ⚠ 15:50:05 token.miss type=gitlab error=not found
//
// When output is not a TTY (pipe, CI), colors are stripped and Unicode icons
// are replaced with ASCII equivalents.
package prettylog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// ANSI escape codes for terminal colors.
const (
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31m"
	ansiYellow = "\033[33m"
	ansiBlue   = "\033[34m"
	ansiGray   = "\033[90m"
)

// Options configures the PrettyHandler.
type Options struct {
	// Level is the minimum log level to display. Nil defaults to slog.LevelInfo.
	Level slog.Leveler
}

// PrettyHandler is a slog.Handler that formats log records for human consumption.
// It groups consecutive logs by namespace and uses colored icons for levels.
type PrettyHandler struct {
	w             io.Writer
	level         slog.Leveler
	lastNamespace string
	mu            sync.Mutex
	attrs         []slog.Attr
	groups        []string
	noColor       bool
}

var _ slog.Handler = (*PrettyHandler)(nil)

// NewPrettyHandler creates a handler that writes pretty-formatted logs to w.
// If w is not a terminal, colors and Unicode icons are disabled.
func NewPrettyHandler(w io.Writer, opts *Options) *PrettyHandler {
	var level slog.Leveler = slog.LevelInfo
	if opts != nil && opts.Level != nil {
		level = opts.Level
	}
	return &PrettyHandler{
		w:       w,
		level:   level,
		noColor: !isTerminal(w),
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *PrettyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

// Handle formats and writes a log record.
func (h *PrettyHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Parse the message into namespace + action.
	namespace, action := splitMessage(r.Message)

	// Write namespace header if it changed.
	if namespace != h.lastNamespace {
		h.writeNamespaceHeader(namespace)
		h.lastNamespace = namespace
	}

	// Level icon + color.
	icon, color := h.levelStyle(r.Level)

	// Short timestamp.
	ts := r.Time.Format("15:04:05")

	// Format attributes.
	attrs := h.formatAttrs(r)

	// Build the line.
	if h.noColor {
		if attrs != "" {
			fmt.Fprintf(h.w, "  %s %s %s  %s\n", icon, ts, action, attrs)
		} else {
			fmt.Fprintf(h.w, "  %s %s %s\n", icon, ts, action)
		}
	} else {
		if attrs != "" {
			fmt.Fprintf(h.w, "  %s%s%s %s%s%s %s  %s\n",
				color, icon, ansiReset,
				ansiGray, ts, ansiReset,
				action, attrs)
		} else {
			fmt.Fprintf(h.w, "  %s%s%s %s%s%s %s\n",
				color, icon, ansiReset,
				ansiGray, ts, ansiReset,
				action)
		}
	}

	return nil
}

// WithAttrs returns a new handler with the given attributes pre-set.
func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs), len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	newAttrs = append(newAttrs, attrs...)
	return &PrettyHandler{
		w:       h.w,
		level:   h.level,
		noColor: h.noColor,
		attrs:   newAttrs,
		groups:  h.groups,
	}
}

// WithGroup returns a new handler with the given group name.
func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	newGroups := make([]string, len(h.groups), len(h.groups)+1)
	copy(newGroups, h.groups)
	newGroups = append(newGroups, name)
	return &PrettyHandler{
		w:       h.w,
		level:   h.level,
		noColor: h.noColor,
		attrs:   h.attrs,
		groups:  newGroups,
	}
}

// ── Internal helpers ─────────────────────────────────────────────────────────

// splitMessage separates "tracker.sync.members" into namespace="tracker.sync"
// and action="members". With fewer than 3 segments, the first segment becomes
// the namespace and the remainder (if any) becomes the action.
func splitMessage(msg string) (namespace, action string) {
	parts := strings.SplitN(msg, ".", 3)
	switch len(parts) {
	case 1:
		return parts[0], ""
	case 2:
		return parts[0], parts[1]
	default:
		return parts[0] + "." + parts[1], parts[2]
	}
}

// writeNamespaceHeader writes a section divider when the namespace changes.
func (h *PrettyHandler) writeNamespaceHeader(namespace string) {
	if namespace == "" {
		return
	}
	const totalWidth = 54
	prefix := "─── "
	suffix := " "
	padding := totalWidth - len(prefix) - len(namespace) - len(suffix)
	if padding < 3 {
		padding = 3
	}

	if h.noColor {
		fmt.Fprintf(h.w, "\n--- %s %s\n", namespace, strings.Repeat("-", padding))
	} else {
		fmt.Fprintf(h.w, "\n%s%s%s %s%s\n",
			ansiBlue, prefix, namespace, strings.Repeat("─", padding), ansiReset)
	}
}

// levelStyle returns the icon and ANSI color for a log level.
func (h *PrettyHandler) levelStyle(level slog.Level) (icon string, color string) {
	if h.noColor {
		switch {
		case level >= slog.LevelError:
			return "x", ""
		case level >= slog.LevelWarn:
			return "!", ""
		case level >= slog.LevelInfo:
			return "*", ""
		default:
			return ">", ""
		}
	}
	switch {
	case level >= slog.LevelError:
		return "✗", ansiRed
	case level >= slog.LevelWarn:
		return "⚠", ansiYellow
	case level >= slog.LevelInfo:
		return "ℹ", ansiBlue
	default:
		return "→", ansiGray
	}
}

// formatAttrs renders all attributes (pre-set + record) as a single line.
func (h *PrettyHandler) formatAttrs(r slog.Record) string {
	var parts []string

	// Pre-set attrs from WithAttrs.
	for _, a := range h.attrs {
		if s := h.formatAttr(a); s != "" {
			parts = append(parts, s)
		}
	}

	// Record attrs.
	r.Attrs(func(a slog.Attr) bool {
		if s := h.formatAttr(a); s != "" {
			parts = append(parts, s)
		}
		return true
	})

	return strings.Join(parts, " ")
}

// formatAttr renders a single attribute as "key=value" with optional coloring.
func (h *PrettyHandler) formatAttr(a slog.Attr) string {
	if a.Equal(slog.Attr{}) {
		return ""
	}
	val := a.Value.Resolve()
	if h.noColor {
		return fmt.Sprintf("%s=%s", a.Key, val.String())
	}
	return fmt.Sprintf("%s%s%s=%s", ansiGray, a.Key, ansiReset, val.String())
}

// isTerminal reports whether w is a terminal (character device).
// Uses os.File.Stat() — no external dependencies.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
