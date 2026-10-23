package widgets

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// legacyColorTag is widgets.ColorTag as it was written before it delegated to
// the theme: the output must stay the same, byte for byte, with and without
// NO_COLOR.
func legacyColorTag(c tcell.Color) string {
	if theme.NoColor() {
		return ""
	}
	r, g, b := c.RGB()
	return fmt.Sprintf("[#%02x%02x%02x]", r, g, b)
}

func TestColorTagDelegatesToTheTheme(t *testing.T) {
	colors := []tcell.Color{
		theme.BgPanel, theme.FgPrimary, theme.FgMuted, theme.Success, theme.Error,
		theme.ActiveMode.Primary, theme.ActiveMode.Accent, tcell.ColorRed, tcell.ColorWhite,
		tcell.NewRGBColor(1, 2, 3), tcell.ColorDefault,
	}
	for _, nc := range []string{"", "1"} {
		t.Setenv("NO_COLOR", nc)
		for _, c := range colors {
			if got, want := ColorTag(c), legacyColorTag(c); got != want {
				t.Errorf("NO_COLOR=%q %v: ColorTag = %q, old = %q", nc, c, got, want)
			}
			if got, want := ColorTag(c), theme.ColorTagOf(c); got != want {
				t.Errorf("NO_COLOR=%q %v: ColorTag = %q, theme = %q", nc, c, got, want)
			}
		}
	}
}
