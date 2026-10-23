package theme

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/glyph"
)

// The theme names the symbols of internal/glyph and adds none of its own to
// the table of meanings.
func TestSymbolsAreTheGlyphTable(t *testing.T) {
	theme := map[string]string{
		"success": SymSuccess, "failure": SymFailure, "inactive": SymInactive, "absent": SymAbsent,
		"warning": SymWarning, "active": SymLive, "current": SymCurrent, "modified": SymModified,
		"default": SymDefault, "cursor": SymCursor, "selected": SymSelected,
	}
	if len(theme) != len(glyph.Symbols) {
		t.Fatalf("%d symbols in the theme, %d in glyph", len(theme), len(glyph.Symbols))
	}
	for _, s := range glyph.Symbols {
		if theme[s.Name] != s.Glyph {
			t.Errorf("symbol %s: theme %q, glyph %q", s.Name, theme[s.Name], s.Glyph)
		}
	}
}

// The screens show the same symbols as before the table: moving the Icon*
// onto it changed nothing. An Icon* that does not follow the table (see the
// divergences in icons.go) is listed here with the value it has today; the
// day a screen group moves it onto the table, this test is updated with it.
func TestIconsKeepTheirSymbols(t *testing.T) {
	tests := []struct{ name, got, want string }{
		{"IconActive", IconActive, "◆"}, {"IconDone", IconDone, "●"}, {"IconPending", IconPending, "○"},
		{"IconSkipped", IconSkipped, "◌"}, {"IconSuccess", IconSuccess, "✓"}, {"IconError", IconError, "✗"},
		{"IconWarning", IconWarning, "!"}, {"IconInfo", IconInfo, "ℹ"}, {"IconArrow", IconArrow, "▸"},
		{"IconCollapsed", IconCollapsed, "▾"}, {"IconDot", IconDot, "·"}, {"IconConnector", IconConnector, "───"},
		{"IconGutter", IconGutter, "▎"}, {"IconStepActive", IconStepActive, "◆"}, {"IconSepBold", IconSepBold, "━"},
		{"IconSepNormal", IconSepNormal, "─"}, {"IconSepConn", IconSepConn, "┃"}, {"IconDraft", IconDraft, "✎"},
		{"IconPinned", IconPinned, "★"}, {"IconRuntimeLocal", IconRuntimeLocal, "⌂"},
		{"IconRuntimeContainer", IconRuntimeContainer, "▣"}, {"IconRuntimeRemote", IconRuntimeRemote, "☁"},
		{"IconValid", IconValid, "✔"}, {"IconInvalid", IconInvalid, "✗"}, {"IconWarn", IconWarn, "⚠"},
		{"IconQueued", IconQueued, "⏳"}, {"IconNewBrick", IconNewBrick, "+"}, {"IconExtends", IconExtends, "↳"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, was %q", tt.name, tt.got, tt.want)
		}
	}
}
