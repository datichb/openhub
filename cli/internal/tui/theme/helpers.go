package theme

import "fmt"

// ColorTag returns a tview dynamic color tag for the given hex color.
// Example: ColorTag(AccentHex) returns "[#64a0ff]". With NO_COLOR it returns
// "" (no color tag, the text keeps the terminal colors).
func ColorTag(hex string) string {
	if NoColor() {
		return ""
	}
	return fmt.Sprintf("[%s]", hex)
}

// TagReset resets all tview color/style attributes.
const TagReset = "[-:-:-]"

// TagColor resets only the foreground color.
const TagColor = "[-]"
