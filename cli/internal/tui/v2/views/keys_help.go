package views

import (
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// Key convention in the help (help view, ? overlay) and the notice of the
// new keys (shown once at the first launch, again from the help).

// KeysNoticePreference is the preference (global scope) that records the
// revision of the table of key changes (keys.ChangesRevision) whose notice
// was shown. A 5.0 recorded a boolean there: it counts as not seen.
const KeysNoticePreference = "tui.keys_notice"

// WriteKeyConvention writes the table of the key convention, one block per
// context, each line prefixed by indent. Keys are in the accent colour.
func WriteKeyConvention(b *strings.Builder, indent string) {
	accent := theme.ColorTag(theme.ActiveMode.PrimaryHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor
	for i, c := range keys.Contexts() {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(b, "%s%s%s%s\n", indent, muted, c.Title(), reset)
		for _, r := range keys.HelpRows(c) {
			fmt.Fprintf(b, "%s  %s%-8s%s  %s\n", indent, accent, r.Keys, reset, r.Label)
		}
	}
}

// WriteSymbolLegend writes the legend of the symbols, one line per row of the
// table of meanings of the theme (theme.Meanings), each prefixed by indent:
// the symbol in the color that goes with it, then what it says.
func WriteSymbolLegend(b *strings.Builder, indent string) {
	for _, m := range theme.Meanings {
		symbol := m.Symbol
		if m.Hex != "" {
			symbol = theme.Paint(m.Hex, symbol)
		}
		fmt.Fprintf(b, "%s  %s  %s\n", indent, symbol, i18n.T("a12.legend."+m.Name))
	}
}

// KeysNoticeText is the text of the notice of the new keys (plain text).
func KeysNoticeText() string {
	var b strings.Builder
	b.WriteString(i18n.T("tui.keys.notice.intro") + "\n\n")
	line := func(c keys.Context, actions ...keys.Action) {
		fmt.Fprintf(&b, "  %-10s %s\n", c.Title(), keys.HintsOf(actions...))
	}
	line(keys.List, keys.Create, keys.Edit, keys.Delete, keys.Archive, keys.Resume, keys.Compare)
	line(keys.Decision, keys.Accept, keys.Refuse, keys.Modify, keys.Leave, keys.AcceptAll)
	line(keys.Everywhere, keys.Ends, keys.Filter, keys.Refresh, keys.Help)
	b.WriteString("\n" + i18n.T("tui.keys.notice.changed") + "\n")
	for _, g := range keys.ChangesByView() {
		parts := make([]string, len(g.Changes))
		for i, c := range g.Changes {
			parts[i] = c.Text()
		}
		fmt.Fprintf(&b, "  %-26s %s\n", g.View, keys.Hints(parts...))
	}
	b.WriteString("\n" + i18n.T("tui.keys.notice.again"))
	return b.String()
}

// ShowKeysNotice shows the notice of the new keys; onClose (optional) runs
// when it is acknowledged.
func ShowKeysNotice(sh ShellAccess, onClose func()) {
	if sh == nil {
		return
	}
	sh.ShowScrollableModal(i18n.T("tui.keys.notice.title"), KeysNoticeText(), []ModalAction{
		{Label: i18n.T("tui.keys.notice.ok"), Callback: onClose},
	})
}
