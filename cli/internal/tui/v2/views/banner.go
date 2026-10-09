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

// renderBanner generates ASCII art text using a figlet font, colored with the
// provided hex color. If the rendered text exceeds maxWidth columns, it falls
// back to a simple bold-colored line.
//
// Returns a tview-compatible dynamic color tagged string (multi-line)
// and the number of visible lines the banner occupies.
func renderBanner(name string, maxWidth int, colorHex string) (banner string, lineCount int) {
	fig := figure.NewFigure(name, bannerFont, false)
	lines := fig.Slicify()

	color := theme.ColorTag(colorHex)
	reset := theme.TagColor

	// Filter out fully empty leading/trailing lines
	trimmed := trimEmptyLines(lines)

	if len(trimmed) == 0 {
		return fmt.Sprintf("%s[::b]%s[::-]%s", color, name, reset), 1
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
		return fmt.Sprintf("%s[::b]%s[::-]%s", color, name, reset), 1
	}

	// Color each line with left margin and bold
	var b strings.Builder
	for _, l := range trimmed {
		fmt.Fprintf(&b, "%s[::b]%s[::-]%s\n", color, l, reset)
	}

	// Decorative underline (60% of banner width, centered, in muted mode color)
	underlineLen := maxLine * 60 / 100
	if underlineLen < 10 {
		underlineLen = maxLine
	}
	muted := theme.ColorTag(theme.ActiveMode.MutedHex)
	fmt.Fprintf(&b, "%s%s%s\n", muted, strings.Repeat("─", underlineLen), reset)

	// Bottom spacing
	fmt.Fprintf(&b, "\n")

	// lineCount = banner lines + underline + empty line
	return b.String(), len(trimmed) + 2
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

// compactBanner is the one-line banner of the compact landings (N20): the
// name in bold, then an optional muted suffix.
func compactBanner(name, colorHex, suffix string) string {
	line := fmt.Sprintf("%s[::b]%s[::-]%s", theme.ColorTag(colorHex), name, theme.TagColor)
	if suffix != "" {
		line += "  " + theme.ColorTag(theme.TextMutedHex) + suffix + theme.TagColor
	}
	return line
}
