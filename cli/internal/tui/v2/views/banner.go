package views

import (
	"fmt"
	"strings"

	"github.com/common-nighthawk/go-figure"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// bannerFont is the figlet font used for big title rendering.
// "small" is 4-5 lines tall, compact, and fits most names within 72 cols.
const bannerFont = "small"

// renderBanner generates ASCII art text using a figlet font, colored in
// Action/Peach. If the rendered text exceeds maxWidth columns, it falls
// back to a simple bold-colored line.
//
// Returns a tview-compatible dynamic color tagged string (multi-line).
func renderBanner(name string, maxWidth int) string {
	fig := figure.NewFigure(name, bannerFont, false)
	lines := fig.Slicify()

	action := theme.ColorTag(theme.ActionHex)
	reset := theme.TagColor

	// Filter out fully empty leading/trailing lines
	trimmed := trimEmptyLines(lines)

	if len(trimmed) == 0 {
		return fmt.Sprintf("  %s[::b]%s[::-]%s", action, name, reset)
	}

	// Check width — find the longest line
	maxLine := 0
	for _, l := range trimmed {
		if len(l) > maxLine {
			maxLine = len(l)
		}
	}

	// Fallback: if too wide, return simple bold text
	if maxLine > maxWidth {
		return fmt.Sprintf("  %s[::b]%s[::-]%s", action, name, reset)
	}

	// Color each line with left margin
	var b strings.Builder
	for _, l := range trimmed {
		fmt.Fprintf(&b, "  %s%s%s\n", action, l, reset)
	}
	return b.String()
}

// bannerHeight returns the number of visible lines the banner will occupy.
func bannerHeight(name string, maxWidth int) int {
	fig := figure.NewFigure(name, bannerFont, false)
	lines := fig.Slicify()

	trimmed := trimEmptyLines(lines)
	if len(trimmed) == 0 {
		return 1
	}

	maxLine := 0
	for _, l := range trimmed {
		if len(l) > maxLine {
			maxLine = len(l)
		}
	}

	// Fallback = 1 line
	if maxLine > maxWidth {
		return 1
	}

	return len(trimmed)
}

// trimEmptyLines removes fully blank lines from the start and end of a slice.
func trimEmptyLines(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}
