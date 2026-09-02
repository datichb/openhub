package shell

import (
	"fmt"
	"strings"

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

	accent := theme.ColorTag(theme.AccentHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	var b strings.Builder

	// ── Section 1 : Global shortcuts ──────────────────────────────────────────
	b.WriteString(fmt.Sprintf("\n  %s%s%s\n\n",
		accent, i18n.T("tui.help.section.global"), reset))
	rows := []struct{ key, desc string }{
		{"Ctrl+P", i18n.T("tui.help.ctrl_p")},
		{"?  / F1", i18n.T("tui.help.help")},
		{"Enter", i18n.T("tui.help.enter")},
		{"Esc", i18n.T("tui.help.esc")},
		{"j/k / ↑↓", i18n.T("tui.help.arrows")},
		{"Ctrl+Q", i18n.T("tui.help.quit")},
		{"[lettre]", i18n.T("tui.help.rune_search")},
	}
	for _, r := range rows {
		b.WriteString(fmt.Sprintf("  %s%-14s%s  %s\n", accent, r.key, reset, r.desc))
	}

	// ── Section 2 : Contextual commands from the active view ──────────────────
	if cur := s.router.Current(); cur != nil {
		if cp, ok := cur.(views.CommandProvider); ok {
			cmds := cp.ContextCommands()
			if len(cmds) > 0 {
				b.WriteString(fmt.Sprintf("\n  %s%s : %s%s\n\n",
					accent, i18n.T("tui.help.section.view"), cur.Title(), reset))
				for _, cmd := range cmds {
					shortcut := cmd.ID
					if len(cmd.Aliases) > 0 {
						shortcut = cmd.Aliases[0]
					}
					desc := cmd.Description
					if desc == "" {
						desc = cmd.Label
					}
					b.WriteString(fmt.Sprintf("  %s%-14s%s  %s\n", accent, shortcut, reset, desc))
				}
			}
		}
	}

	// ── Section 3 : Omnibar commands grouped by category ──────────────────────
	b.WriteString(fmt.Sprintf("\n  %s%s%s\n\n",
		accent, i18n.T("tui.help.section.commands"), reset))

	groups := helpGroupByCategory(s.registry.All())
	for _, g := range groups {
		if g.name != "" {
			b.WriteString(fmt.Sprintf("  %s%s%s\n", muted, g.name, reset))
		}
		for _, cmd := range g.commands {
			desc := cmd.Description
			if desc == "" {
				desc = cmd.Category
			}
			b.WriteString(fmt.Sprintf("    %-14s  %s\n", cmd.Label, desc))
		}
		b.WriteString("\n")
	}

	s.showInlineScrollable(i18n.T("tui.help.title"), b.String(), nil)
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
