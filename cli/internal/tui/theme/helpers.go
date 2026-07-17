package theme

import "fmt"

// ColorTag returns a tview dynamic color tag for the given hex color.
// Example: ColorTag(AccentHex) returns "[#64a0ff]".
func ColorTag(hex string) string {
	return fmt.Sprintf("[%s]", hex)
}

// TagReset resets all tview color/style attributes.
const TagReset = "[-:-:-]"

// TagColor resets only the foreground color.
const TagColor = "[-]"
