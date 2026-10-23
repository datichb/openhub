package theme

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

const esc = "\x1b["

// withColors forces a color terminal for the test, whatever the test stdout is.
func withColors(t *testing.T) {
	t.Helper()
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })
}

// U-09: NO_COLOR (any non-empty value) turns the colors of the CLI off.
func TestNoColorRemovesEveryEscapeSequence(t *testing.T) {
	withColors(t)
	t.Setenv("NO_COLOR", "")
	ApplyNoColor()
	for name, out := range map[string]string{
		"success": SuccessStyle.Render("ok"), "error": ErrorStyle.Render("ko"),
		"warning": WarningStyle.Render("!"), "title": Title.Render("oh"), "bold": Bold.Render("b"),
	} {
		if !strings.Contains(out, esc) {
			t.Fatalf("control: %s is not colored without NO_COLOR: %q", name, out)
		}
	}

	t.Setenv("NO_COLOR", "1")
	ApplyNoColor()
	for name, out := range map[string]string{
		"success": SuccessStyle.Render("ok"), "error": ErrorStyle.Render("ko"),
		"warning": WarningStyle.Render("!"), "title": Title.Render("oh"), "bold": Bold.Render("b"),
	} {
		if strings.Contains(out, esc) {
			t.Errorf("%s still has an escape sequence with NO_COLOR=1: %q", name, out)
		}
	}
	if got := SuccessStyle.Render("ok"); got != "ok" {
		t.Errorf("plain text expected, got %q", got)
	}
}

// U-09: any non-empty value counts, as no-color.org says.
func TestNoColorValues(t *testing.T) {
	for value, want := range map[string]bool{"": false, "1": true, "0": true, "true": true, "x": true} {
		t.Setenv("NO_COLOR", value)
		if NoColor() != want {
			t.Errorf("NO_COLOR=%q: %v, want %v", value, NoColor(), want)
		}
	}
}

// U-09: the tview color tags of the TUI are empty with NO_COLOR.
func TestNoColorEmptiesTheTUIColorTags(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if got := ColorTag("#64a0ff"); got != "[#64a0ff]" {
		t.Fatalf("control: %q", got)
	}
	t.Setenv("NO_COLOR", "1")
	if got := ColorTag("#64a0ff"); got != "" {
		t.Errorf("ColorTag with NO_COLOR = %q", got)
	}
}

// U-09: every tag builder of the theme follows NO_COLOR, and gives its
// colors back without it.
func TestNoColorEmptiesEveryTagBuilder(t *testing.T) {
	builders := map[string]func() string{
		"ColorTag":    func() string { return ColorTag(AccentHex) },
		"ColorTagOf":  func() string { return ColorTagOf(FgMuted) },
		"BgTag":       func() string { return BgTag(BgElementHex) },
		"BgTag tcell": func() string { return BgTag(BgElement) },
		"StyleTag":    func() string { return StyleTag(AttrBold) },
	}
	t.Setenv("NO_COLOR", "")
	for name, f := range builders {
		if f() == "" {
			t.Errorf("control: %s is empty without NO_COLOR", name)
		}
	}
	if got := Paint(SuccessHex, "ok"); got == "ok" {
		t.Errorf("control: Paint does not color without NO_COLOR: %q", got)
	}

	t.Setenv("NO_COLOR", "1")
	for name, f := range builders {
		if got := f(); got != "" {
			t.Errorf("%s with NO_COLOR = %q", name, got)
		}
	}
	if got := Paint(SuccessHex, "ok"); got != "ok" {
		t.Errorf("Paint with NO_COLOR = %q, want the text alone", got)
	}
}
