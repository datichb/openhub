// Package summary provides a reusable post-wizard summary card component.
// Design System: Aurum v2 "Floating Panels" — see docs/design/aurum.md
//
// After a BubbleTea alt-screen wizard exits, the terminal history is blank.
// This component prints a styled recap card so the user retains context.
package summary

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Field represents a single key-value pair in the summary card.
type Field struct {
	Label string
	Value string
}

// Config defines the content and appearance of the summary card.
type Config struct {
	// Title is the main heading (e.g., "Team Setup Complete").
	Title string
	// Icon is the status icon displayed before the title (e.g., theme.IconSuccess).
	Icon string
	// IconColor is the lipgloss color for the icon (e.g., theme.LipSuccess).
	IconColor lipgloss.TerminalColor
	// Fields are the key-value pairs displayed in the card body.
	Fields []Field
	// Footer is an optional instruction line at the bottom (e.g., "Run `oh team status`").
	Footer string
	// Width overrides the card width. 0 = auto-size to content.
	Width int
}

// Render produces a lipgloss-styled summary card suitable for printing to stdout.
func Render(cfg Config) string {
	if cfg.Width == 0 {
		cfg.Width = computeWidth(cfg)
	}

	// ── Icon + Title ──
	icon := lipgloss.NewStyle().Foreground(cfg.IconColor).Render(cfg.Icon)
	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(theme.TextLight).
		Render(cfg.Title)
	header := fmt.Sprintf("%s %s", icon, title)

	// ── Fields ──
	var fieldLines []string
	if len(cfg.Fields) > 0 {
		// Compute label alignment width.
		maxLabel := 0
		for _, f := range cfg.Fields {
			if len(f.Label) > maxLabel {
				maxLabel = len(f.Label)
			}
		}

		labelStyle := lipgloss.NewStyle().Foreground(theme.Subtle)
		valueStyle := lipgloss.NewStyle().Foreground(theme.TextLight)

		for _, f := range cfg.Fields {
			label := labelStyle.Render(fmt.Sprintf("%-*s", maxLabel, f.Label))
			value := valueStyle.Render(f.Value)
			fieldLines = append(fieldLines, fmt.Sprintf("%s  %s", label, value))
		}
	}

	// ── Footer ──
	footer := ""
	if cfg.Footer != "" {
		footer = lipgloss.NewStyle().
			Foreground(theme.Muted).
			Italic(true).
			Render(cfg.Footer)
	}

	// ── Compose content ──
	parts := []string{header}
	if len(fieldLines) > 0 {
		parts = append(parts, "") // blank line
		parts = append(parts, fieldLines...)
	}
	if footer != "" {
		parts = append(parts, "") // blank line
		parts = append(parts, footer)
	}
	content := strings.Join(parts, "\n")

	// ── Card box ──
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(theme.LipBorderElem).
		Foreground(theme.TextLight).
		Padding(1, 2).
		Width(cfg.Width)

	return "\n" + cardStyle.Render(content) + "\n"
}

// computeWidth calculates a reasonable card width from content.
func computeWidth(cfg Config) int {
	// Start with title length + icon + padding
	w := len(cfg.Title) + 6
	for _, f := range cfg.Fields {
		fw := len(f.Label) + len(f.Value) + 4
		if fw > w {
			w = fw
		}
	}
	if len(cfg.Footer)+4 > w {
		w = len(cfg.Footer) + 4
	}
	// Add padding for the box (border + padding = 6 chars)
	w += 6
	// Clamp to reasonable bounds
	if w < 40 {
		w = 40
	}
	if w > 72 {
		w = 72
	}
	return w
}
