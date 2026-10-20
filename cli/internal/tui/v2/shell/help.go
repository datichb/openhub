package shell

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// showHelpOverlay displays a contextual help overlay with global shortcuts,
// view-specific commands, and all omnibar commands — grouped by category.
//
// Inspired by lazygit's '?' keybinding panel and htop's F1 help screen:
// instant, non-destructive, scrollable, contextual.
//
// Activation: '?' or F1 from any view (not while omnibar or another overlay is active).
// Dismissal:  Esc, or click outside (light dismiss).
func (s *Shell) showHelpOverlay() {
	// Do not stack on top of an existing overlay — silently ignore.
	if s.pages.HasPage("inline-overlay") || s.pages.HasPage("sub-overlay") {
		return
	}

	accent := theme.ColorTag(theme.ActiveMode.PrimaryHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	var b strings.Builder

	// ── Section 1 : Global shortcuts ──────────────────────────────────────────
	fmt.Fprintf(&b, "\n  %s%s%s\n\n",
		accent, i18n.T("tui.help.section.global"), reset)
	rows := []struct{ key, desc string }{
		{"Ctrl+P · /", i18n.T("tui.help.ctrl_p")},
		{"Ctrl+T", i18n.T("tui.keys.help.ctrl_t")},
		{"Ctrl+Q", i18n.T("tui.help.quit")},
	}
	for _, r := range rows {
		fmt.Fprintf(&b, "  %s%-14s%s  %s\n", accent, r.key, reset, r.desc)
	}
	fmt.Fprintf(&b, "  %s%s%s\n", muted, i18n.T("tui.keys.help.letters"), reset)

	// ── Key convention (package keys) ─────────────────────────────────────────
	fmt.Fprintf(&b, "\n  %s%s%s\n\n", accent, i18n.T("tui.keys.help.convention"), reset)
	views.WriteKeyConvention(&b, "  ")

	// ── Launch form and wizards ───────────────────────────────────────────────
	fmt.Fprintf(&b, "\n  %s%s%s\n\n", accent, i18n.T("tui.keys.help.forms"), reset)
	for _, r := range []struct{ key, desc string }{
		{"Ctrl+S", i18n.T("tui.keys.help.ctrl_s")},
		{"Alt+← / PgUp", i18n.T("tui.keys.help.back")},
		{"Alt+→ / PgDn", i18n.T("tui.keys.help.next")},
		{"Esc", i18n.T("tui.keys.help.esc_wizard")},
	} {
		fmt.Fprintf(&b, "  %s%-14s%s  %s\n", accent, r.key, reset, r.desc)
	}

	// ── Keys of the active view (its hints bar) ───────────────────────────────
	if cur := s.router.Current(); cur != nil {
		if hints := cur.StatusHints(); hints != "" {
			fmt.Fprintf(&b, "\n  %s%s : %s%s\n\n  %s\n",
				accent, i18n.T("tui.keys.help.view_keys"), cur.Title(), reset, hints)
		}
	}

	// ── Legend of the active view's pictograms ────────────────────────────────
	if cur := s.router.Current(); cur != nil {
		if lp, ok := cur.(views.LegendProvider); ok {
			if legend := lp.Legend(); legend != "" {
				fmt.Fprintf(&b, "\n  %s%s : %s%s\n\n", accent, i18n.T("tui.help.section.legend"), cur.Title(), reset)
				for _, line := range strings.Split(legend, "\n") {
					fmt.Fprintf(&b, "  %s\n", line)
				}
			}
		}
	}

	// ── Section 2 : Contextual commands from the active view ──────────────────
	if cur := s.router.Current(); cur != nil {
		if cp, ok := cur.(views.CommandProvider); ok {
			cmds := cp.ContextCommands()
			if len(cmds) > 0 {
				fmt.Fprintf(&b, "\n  %s%s : %s%s\n\n",
					accent, i18n.T("tui.help.section.view"), cur.Title(), reset)
				for _, cmd := range cmds {
					shortcut := cmd.ID
					if len(cmd.Aliases) > 0 {
						shortcut = cmd.Aliases[0]
					}
					desc := cmd.Description
					if desc == "" {
						desc = cmd.Label
					}
					fmt.Fprintf(&b, "  %s%-14s%s  %s\n", accent, shortcut, reset, desc)
				}
			}
		}
	}

	// ── Section 3 : Omnibar commands grouped by category ──────────────────────
	fmt.Fprintf(&b, "\n  %s%s%s\n\n",
		accent, i18n.T("tui.help.section.commands"), reset)

	groups := helpGroupByCategory(s.registry.All())
	for _, g := range groups {
		if g.name != "" {
			fmt.Fprintf(&b, "  %s%s%s\n", muted, g.name, reset)
		}
		for _, cmd := range g.commands {
			desc := cmd.Description
			if desc == "" {
				desc = cmd.Category
			}
			fmt.Fprintf(&b, "    %-14s  %s\n", cmd.Label, desc)
		}
		b.WriteString("\n")
	}

	// The text keeps the focus (↑↓ scroll it); Tab reaches the button.
	text := b.String()
	textView := tview.NewTextView().SetText(text)
	styleScrollableText(textView)
	actions := []views.ModalAction{
		{Label: i18n.T("tui.keys.notice.show"), Callback: func() { views.ShowKeysNotice(s, nil) }},
	}
	s.showModal(modalConfig{
		Title:        i18n.T("tui.help.title"),
		Content:      textView,
		Actions:      actions,
		Size:         modalSizeHint{ContentLines: countLines(text), HasButtons: true, ButtonCount: len(actions), ProportionalWidth: -4},
		Hints:        i18n.T("tui.modal.hints.scrollable"),
		LightDismiss: true,
		PageName:     "inline-overlay",
		FocusReturn:  s.focusReturn(),
		FocusTarget:  textView,
	})
}

// OfferKeysNotice shows the notice of the new keys at startup, unless a
// window is already open (it will come at a later launch); seen records it
// was shown. Call it on the event loop.
func (s *Shell) OfferKeysNotice(seen func()) bool {
	if s.hasOverlay() {
		return false
	}
	views.ShowKeysNotice(s, nil)
	if seen != nil {
		seen()
	}
	return true
}

// helpGroup is a named group of commands for display in the help overlay.
type helpGroup struct {
	name     string
	commands []Command
}

// helpGroupByCategory returns commands grouped and sorted by category.
// Commands without a category are placed in a leading unnamed group.
func helpGroupByCategory(commands []Command) []helpGroup {
	order := []string{}
	seen := map[string]bool{}
	buckets := map[string][]Command{}

	for _, cmd := range commands {
		cat := cmd.Category
		if !seen[cat] {
			seen[cat] = true
			order = append(order, cat)
		}
		buckets[cat] = append(buckets[cat], cmd)
	}

	groups := make([]helpGroup, 0, len(order))
	for _, cat := range order {
		groups = append(groups, helpGroup{name: cat, commands: buckets[cat]})
	}
	return groups
}
