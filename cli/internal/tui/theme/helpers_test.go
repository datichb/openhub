package theme

import (
	"fmt"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// legacyColorTag and legacyHexTag are the two functions that ColorTagOf and
// ColorTag replaced, as they were written (widgets.colorTag, theme.ColorTag):
// the new output must be the same, byte for byte.
func legacyColorTag(c tcell.Color) string {
	if NoColor() {
		return ""
	}
	r, g, b := c.RGB()
	return fmt.Sprintf("[#%02x%02x%02x]", r, g, b)
}

func legacyHexTag(hex string) string {
	if NoColor() {
		return ""
	}
	return fmt.Sprintf("[%s]", hex)
}

func sampleColors() map[string]tcell.Color {
	return map[string]tcell.Color{
		"BgApp": BgApp, "BgPanel": BgPanel, "BgElement": BgElement, "FgPrimary": FgPrimary,
		"FgSecondary": FgSecondary, "FgMuted": FgMuted, "Success": Success, "Primary": ActiveMode.Primary,
		"Accent": ActiveMode.Accent, "dim overlay": BgDimOverlay, "red": tcell.ColorRed, "black": tcell.ColorBlack,
		"white": tcell.ColorWhite, "rgb": tcell.NewRGBColor(1, 2, 3), "hex": tcell.GetColor("#64a0ff"),
		"default": tcell.ColorDefault,
	}
}

func TestColorTagOfMatchesTheOldTags(t *testing.T) {
	for _, nc := range []string{"", "1"} {
		t.Setenv("NO_COLOR", nc)
		for name, c := range sampleColors() {
			if got, want := ColorTagOf(c), legacyColorTag(c); got != want {
				t.Errorf("NO_COLOR=%q %s: ColorTagOf = %q, old = %q", nc, name, got, want)
			}
			if got, want := fg(c), legacyColorTag(c); got != want {
				t.Errorf("NO_COLOR=%q %s: fg = %q, old = %q", nc, name, got, want)
			}
		}
		for _, hex := range []string{AccentHex, SuccessHex, WarningHex, ErrorHex, TextMutedHex, "#000000", "red"} {
			if got, want := ColorTag(hex), legacyHexTag(hex); got != want {
				t.Errorf("NO_COLOR=%q %s: ColorTag = %q, old = %q", nc, hex, got, want)
			}
			if got, want := fg(hex), legacyHexTag(hex); got != want {
				t.Errorf("NO_COLOR=%q %s: fg = %q, old = %q", nc, hex, got, want)
			}
		}
	}
}

func TestPaint(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	for name, tt := range map[string]struct{ got, want string }{
		"hex":             {Paint(SuccessHex, "ok"), "[#a6e3a1]ok[-]"},
		"tcell":           {Paint(FgMuted, "ok"), ColorTagOf(FgMuted) + "ok[-]"},
		"empty text":      {Paint(AccentHex, ""), "[" + AccentHex + "][-]"},
		"same as by hand": {Paint(ErrorHex, "x"), ColorTag(ErrorHex) + "x" + TagColor},
	} {
		if tt.got != tt.want {
			t.Errorf("%s: %q, want %q", name, tt.got, tt.want)
		}
	}

	t.Setenv("NO_COLOR", "1")
	if got := Paint(SuccessHex, "ok"); got != "ok" {
		t.Errorf("Paint hex with NO_COLOR = %q", got)
	}
	if got := Paint(Success, "ok"); got != "ok" {
		t.Errorf("Paint tcell with NO_COLOR = %q", got)
	}
}

func TestBgTag(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if got := BgTag(BgElementHex); got != "[:#313244]" {
		t.Errorf("BgTag hex = %q", got)
	}
	if got := BgTag(BgElement); got != "[:#313244]" {
		t.Errorf("BgTag tcell = %q", got)
	}
	t.Setenv("NO_COLOR", "1")
	if BgTag(BgElementHex) != "" || BgTag(BgElement) != "" {
		t.Error("BgTag with NO_COLOR is not empty")
	}
}

func TestStyleTag(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	for attrs, want := range map[string]string{AttrBold: "[::b]", AttrBold + AttrDim: "[::bd]", "": ""} {
		if got := StyleTag(attrs); got != want {
			t.Errorf("StyleTag(%q) = %q, want %q", attrs, got, want)
		}
	}
	t.Setenv("NO_COLOR", "1")
	if got := StyleTag(AttrBold); got != "" {
		t.Errorf("StyleTag with NO_COLOR = %q", got)
	}
}

// What the helpers build is read by tview as intended: the tags are removed
// from the text, and nothing else is.
func TestTagsAreValidTview(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	for name, s := range map[string]string{
		"paint hex":   Paint(SuccessHex, "ok"),
		"paint tcell": Paint(FgMuted, "ok"),
		"bg":          BgTag(BgElementHex) + "ok" + TagBg,
		"bg tcell":    BgTag(BgElement) + "ok" + TagBg,
		"style":       StyleTag(AttrBold+AttrDim) + "ok" + TagAttrs,
		"all":         StyleTag(AttrBold) + BgTag(BgElementHex) + Paint(AccentHex, "ok") + TagReset,
	} {
		if plain := stripTags(s); plain != "ok" {
			t.Errorf("%s: %q reads %q in tview, want \"ok\"", name, s, plain)
		}
	}
}

// stripTags is the text tview displays for a string with dynamic colors.
func stripTags(s string) string {
	tv := tview.NewTextView().SetDynamicColors(true)
	tv.SetText(s)
	return tv.GetText(true)
}
