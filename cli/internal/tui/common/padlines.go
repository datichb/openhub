package common

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PadLines normalizes all lines in a rendered string to the same width,
// filling short lines with spaces. When combined with lipgloss.Background(),
// this eliminates the "uneven fill" / "banding" problem where lines of
// different widths create visible gaps in the background color.
//
// Usage:
//
//	content := form.View()
//	padded := common.PadLines(content, 60)
//	styled := lipgloss.NewStyle().Background(common.Surface).Render(padded)
func PadLines(s string, width int) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		// Calculate visible width (strip ANSI sequences for measurement)
		visWidth := lipgloss.Width(line)
		if visWidth < width {
			lines[i] = line + strings.Repeat(" ", width-visWidth)
		}
	}
	return strings.Join(lines, "\n")
}

// PadLinesAuto is like PadLines but automatically determines the width
// from the longest line in the string.
func PadLinesAuto(s string) string {
	lines := strings.Split(s, "\n")
	maxWidth := 0
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w > maxWidth {
			maxWidth = w
		}
	}
	if maxWidth == 0 {
		return s
	}
	for i, line := range lines {
		visWidth := lipgloss.Width(line)
		if visWidth < maxWidth {
			lines[i] = line + strings.Repeat(" ", maxWidth-visWidth)
		}
	}
	return strings.Join(lines, "\n")
}
