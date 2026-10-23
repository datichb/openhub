package theme

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
)

// The tview dynamic-color tags of the TUI come from this file and from
// nowhere else: ColorTag and ColorTagOf (foreground), BgTag (background),
// StyleTag (attributes) and Paint (a colored text, closed). They all follow
// NO_COLOR (see NoColor): with it set they return no tag, and Paint returns
// the text alone, so a screen needs no check of its own.

// Color is what a tag is built from: a hex string ("#64a0ff", the *Hex
// constants of the theme) or a tcell.Color (BgPanel, FgMuted, ActiveMode.Primary…).
type Color interface {
	string | tcell.Color
}

// ColorTag returns a tview dynamic color tag for the given hex color.
// Example: ColorTag(AccentHex) returns "[#64a0ff]". With NO_COLOR it returns
// "" (no color tag, the text keeps the terminal colors).
func ColorTag(hex string) string {
	if NoColor() {
		return ""
	}
	return fmt.Sprintf("[%s]", hex)
}

// ColorTagOf is ColorTag for a tcell.Color: ColorTagOf(FgMuted) returns
// "[#7f849c]". With NO_COLOR it returns "".
func ColorTagOf(c tcell.Color) string {
	if NoColor() {
		return ""
	}
	r, g, b := c.RGB()
	return fmt.Sprintf("[#%02x%02x%02x]", r, g, b)
}

// fg is the foreground tag of a hex string or of a tcell.Color.
func fg[C Color](c C) string {
	switch v := any(c).(type) {
	case tcell.Color:
		return ColorTagOf(v)
	case string:
		return ColorTag(v)
	}
	return ""
}

// Paint colors text with c and gives the foreground back after it:
// Paint(SuccessHex, "ok") returns "[#a6e3a1]ok[-]". With NO_COLOR it returns
// text unchanged. It does not escape text: pass tview.Escape(s) for a text
// that comes from outside.
func Paint[C Color](c C, text string) string {
	tag := fg(c)
	if tag == "" {
		return text
	}
	return tag + text + TagColor
}

// BgTag returns the tview tag that sets the background to c (BgTag(BgElementHex)
// returns "[:#313244]"); TagBg gives it back. With NO_COLOR it returns "".
func BgTag[C Color](c C) string {
	if NoColor() {
		return ""
	}
	switch v := any(c).(type) {
	case tcell.Color:
		r, g, b := v.RGB()
		return fmt.Sprintf("[:#%02x%02x%02x]", r, g, b)
	case string:
		return fmt.Sprintf("[:%s]", v)
	}
	return ""
}

// Attributes of StyleTag (the letters of the tview tags).
const (
	AttrBold    = "b"
	AttrItalic  = "i"
	AttrDim     = "d"
	AttrBlink   = "l"
	AttrStrike  = "s"
	AttrReverse = "r"
)

// StyleTag returns the tview tag that turns attributes on: StyleTag(AttrBold)
// returns "[::b]", StyleTag(AttrBold+AttrDim) "[::bd]"; TagAttrs turns them
// off. With NO_COLOR it returns "", as the lipgloss styles of the theme drop
// their bold too. Without attributes it returns "".
func StyleTag(attrs string) string {
	if NoColor() || attrs == "" {
		return ""
	}
	return "[::" + attrs + "]"
}

// TagReset resets all tview color/style attributes.
const TagReset = "[-:-:-]"

// TagColor resets only the foreground color.
const TagColor = "[-]"

// TagBg resets only the background color.
const TagBg = "[:-]"

// TagAttrs resets only the attributes (bold, dim…).
const TagAttrs = "[::-]"
