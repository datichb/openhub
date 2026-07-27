package views

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// HomeView is the splash/landing view for the TUI shell.
type HomeView struct {
	app     *tview.Application
	content *tview.Flex
}

var _ View = (*HomeView)(nil)

func NewHomeView() *HomeView { return &HomeView{} }

func (v *HomeView) ID() string    { return "home" }
func (v *HomeView) Title() string { return "Home" }

func (v *HomeView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.content = content

	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetScrollable(false)
	tv.SetBackgroundColor(theme.BgPanel)
	tv.SetText(buildHomeText())

	// Horizontal centering: left spacer + fixed-width content + right spacer
	hCenter := tview.NewFlex()
	hCenter.SetBackgroundColor(theme.BgPanel)
	hCenter.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)
	hCenter.AddItem(tv, 68, 0, false)
	hCenter.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)

	// Vertical centering: top spacer + content + bottom spacer
	content.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)
	content.AddItem(hCenter, 34, 0, false)
	content.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)
}

func (v *HomeView) Unmount() {
	v.app = nil
	v.content = nil
}

func (v *HomeView) StatusHints() string {
	return "Ctrl+P commandes · Ctrl+T mode projet · ? aide · q quitter"
}

func (v *HomeView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	return event
}

// visibleWidth returns the terminal display width of s (in cells), ignoring
// tview colour tags of the form [#xxxxxx], [-], [::b], etc.
// It uses go-runewidth to correctly account for wide Unicode characters
// (e.g. box-drawing symbols, emoji, CJK) that occupy 2 terminal cells.
func visibleWidth(s string) int {
	inTag := false
	width := 0
	for _, r := range s {
		switch {
		case r == '[' && !inTag:
			inTag = true
		case r == ']' && inTag:
			inTag = false
		case !inTag:
			width += runewidth.RuneWidth(r)
		}
	}
	return width
}

// buildHomeText constructs the full home screen as a tview-colored string.
func buildHomeText() string {
	action := theme.ColorTag(theme.ActionHex)
	accent := theme.ColorTag(theme.AccentHex)
	primary := theme.ColorTag(theme.TextPrimaryHex)
	secondary := theme.ColorTag(theme.TextSecondaryHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	var b strings.Builder

	// ── Logo ANSI Shadow ────────────────────────────────────────────────
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "%s ██████╗ ██████╗ ███████╗███╗   ██╗██╗  ██╗██╗   ██╗██████╗%s\n", action, reset)
	fmt.Fprintf(&b, "%s██╔═══██╗██╔══██╗██╔════╝████╗  ██║██║  ██║██║   ██║██╔══██╗%s\n", action, reset)
	fmt.Fprintf(&b, "%s██║   ██║██████╔╝█████╗  ██╔██╗ ██║███████║██║   ██║██████╔╝%s\n", action, reset)
	fmt.Fprintf(&b, "%s██║   ██║██╔═══╝ ██╔══╝  ██║╚██╗██║██╔══██║██║   ██║██╔══██╗%s\n", action, reset)
	fmt.Fprintf(&b, "%s╚██████╔╝██║     ███████╗██║ ╚████║██║  ██║╚██████╔╝██████╔╝%s\n", action, reset)
	fmt.Fprintf(&b, "%s ╚═════╝ ╚═╝     ╚══════╝╚═╝  ╚═══╝╚═╝  ╚═╝ ╚═════╝ ╚═════╝%s\n", action, reset)
	fmt.Fprintf(&b, "\n")

	// ── Section builder ─────────────────────────────────────────────────
	// boxWidth = total visible chars on each line including the two │ borders.
	// contentWidth = visible chars available for content between │  and  │.
	const boxWidth = 64
	const contentWidth = boxWidth - 4 // subtract "│ " left + " │" right

	section := func(title string, rows []string) {
		// Top border: ╭─ Title ──...──╮
		titleVisible := len([]rune(title))
		dashes := boxWidth - 2 - 2 - 1 - titleVisible - 1 // ╭─ [space]title[space] ...dashes... ╮
		if dashes < 0 {
			dashes = 0
		}
		fmt.Fprintf(&b, "%s╭─ %s%s%s %s%s╮%s\n",
			muted,
			accent, title, reset,
			muted, strings.Repeat("─", dashes),
			reset,
		)

		// Empty padding row
		fmt.Fprintf(&b, "%s│%s%s%s│%s\n", muted, reset, strings.Repeat(" ", boxWidth-2), muted, reset)

		// Content rows with right border
		for _, row := range rows {
			pad := contentWidth - visibleWidth(row)
			if pad < 0 {
				pad = 0
			}
			fmt.Fprintf(&b, "%s│%s  %s%s  %s│%s\n",
				muted, reset,
				row,
				strings.Repeat(" ", pad),
				muted, reset,
			)
		}

		// Empty padding row
		fmt.Fprintf(&b, "%s│%s%s%s│%s\n", muted, reset, strings.Repeat(" ", boxWidth-2), muted, reset)

		// Bottom border: ╰──...──╯
		fmt.Fprintf(&b, "%s╰%s╯%s\n", muted, strings.Repeat("─", boxWidth-2), reset)
		fmt.Fprintf(&b, "\n")
	}

	item := func(icon, label, desc string) string {
		labelField := fmt.Sprintf("%s%-13s%s", primary, label, reset)
		descField := fmt.Sprintf("%s%s%s", secondary, desc, reset)
		return fmt.Sprintf("%s%s%s  %s  %s",
			accent, icon, reset,
			labelField,
			descField,
		)
	}

	// col1Width = visible width of column 1 (key + space + description + gap)
	const col1Width = 36 // "Ctrl+P " (7) + description (25) + "  " (2) + gap for col2

	shortcutPair := func(k1, d1, k2, d2 string) string {
		// Build col1 as plain text, measure it, then wrap in colour.
		col1Plain := fmt.Sprintf("%-7s %-25s", k1, d1)
		col1Coloured := fmt.Sprintf("%s%-7s%s %s%-25s%s",
			accent, k1, reset,
			secondary, d1, reset,
		)
		// Pad col1 to fixed visible width so col2 always starts at the same position.
		pad := col1Width - len([]rune(col1Plain))
		if pad < 0 {
			pad = 0
		}
		col2Coloured := fmt.Sprintf("%s%-4s%s %s%s%s",
			accent, k2, reset,
			secondary, d2, reset,
		)
		return col1Coloured + strings.Repeat(" ", pad) + col2Coloured
	}

	// ── Navigation ──────────────────────────────────────────────────────
	section("Navigation", []string{
		item("⊞", "Board", "Kanban du projet actif"),
		item("◈", "Projets", "Gérer les projets"),
		item("⊛", "Worktrees", "Git worktrees"),
		item("◎", "Métriques", "Statistiques d'usage"),
		item("⊟", "Config", "Configuration du hub"),
	})

	// ── Actions rapides ─────────────────────────────────────────────────
	section("Actions rapides", []string{
		item("▶", "Start", "Lancer une session"),
		item("◉", "Audit", "Audit multi-domaine"),
		item("◈", "Review", "Code review"),
		item("◆", "Debug", "Session de debug"),
	})

	// ── Raccourcis ──────────────────────────────────────────────────────
	section("Raccourcis", []string{
		shortcutPair("Ctrl+P", "rechercher une commande", "?", "aide"),
		shortcutPair("Ctrl+T", "mode projet / hub", "q", "quitter"),
	})

	return b.String()
}
