package views

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/common-nighthawk/go-figure"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// hubBannerName is the name drawn in the banner of the hub: the product. The
// project and the team draw their own name, as it was typed (never in capitals).
const hubBannerName = "OPENHUB"

// bannerFont is the figlet font used for big title rendering.
// "small" is 4-5 lines tall, compact, and fits most names within 72 cols.
const bannerFont = "small"

// asciiLetters are the letters that do not decompose into a letter and an
// accent.
var asciiLetters = strings.NewReplacer("ß", "ss", "æ", "ae", "Æ", "AE", "œ", "oe", "Œ", "OE",
	"ø", "o", "Ø", "O", "ł", "l", "Ł", "L", "đ", "d", "Đ", "D", "ð", "d", "Ð", "D", "þ", "th", "Þ", "TH")

// asciiName writes name with the letters of ASCII: accents are removed
// (« Équipe » → « Equipe »). It reports false when a character stays outside
// ASCII (Chinese, emoji…).
func asciiName(name string) (string, bool) {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, asciiLetters.Replace(name))
	if err != nil {
		return name, false
	}
	for _, r := range out {
		if r > unicode.MaxASCII {
			return name, false
		}
	}
	return out, true
}

// renderBanner generates ASCII art text using a figlet font, colored with the
// provided hex color. If the rendered text exceeds maxWidth columns, it falls
// back to a simple bold-colored line.
//
// Returns a tview-compatible dynamic color tagged string (multi-line)
// and the number of visible lines the banner occupies.
func renderBanner(name string, maxWidth int, colorHex string) (banner string, lineCount int) {
	color := theme.ColorTag(colorHex)
	reset := theme.TagColor
	plain := func() (string, int) {
		return fmt.Sprintf("%s%s%s%s%s", color, theme.StyleTag(theme.AttrBold), name, theme.TagAttrs, reset), 1
	}

	// The font draws ASCII only (« É » would come out as « ? »): the accents
	// are dropped, and a name that stays non-ASCII is shown as a plain line.
	art, ok := asciiName(name)
	if !ok {
		return plain()
	}
	fig := figure.NewFigure(art, bannerFont, false)
	lines := fig.Slicify()

	// Filter out fully empty leading/trailing lines
	trimmed := trimEmptyLines(lines)

	if len(trimmed) == 0 {
		return plain()
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
		return plain()
	}

	// Color each line with left margin and bold
	var b strings.Builder
	for _, l := range trimmed {
		fmt.Fprintf(&b, "%s%s%s%s%s\n", color, theme.StyleTag(theme.AttrBold), l, theme.TagAttrs, reset)
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

// compactBanner is the one-line banner of the compact landings: the
// name in bold, then an optional muted suffix.
func compactBanner(name, colorHex, suffix string) string {
	line := fmt.Sprintf("%s%s%s%s%s", theme.ColorTag(colorHex), theme.StyleTag(theme.AttrBold), name, theme.TagAttrs, theme.TagColor)
	if suffix != "" {
		line += "  " + theme.ColorTag(theme.TextMutedHex) + suffix + theme.TagColor
	}
	return line
}
