package views

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Board — full-screen Kanban board with columns
// ─────────────────────────────────────────────────────────────────────────────

// BoardTicket represents a kanban ticket.
type BoardTicket struct {
	ID          string
	Title       string
	Status      string // "planned", "in_progress", "review", "done", "blocked"
	Priority    string
	Type        string
	ExternalRef string // external tracker reference (e.g. "gitlab-693") — ADR-032
}

// BoardConfig configures the board view.
type BoardConfig struct {
	Layout      layout.Config
	Tickets     []BoardTicket
	RefreshFunc func() []BoardTicket
	RefreshRate time.Duration
}

// BoardColumnDef defines a kanban column.
type BoardColumnDef struct {
	Name   string
	Status string
	Color  tcell.Color
}

// DefaultColumns returns the standard kanban columns.
func DefaultColumns() []BoardColumnDef {
	return []BoardColumnDef{
		{Name: "TODO", Status: "todo", Color: theme.Warning},
		{Name: "IN PROGRESS", Status: "in_progress", Color: theme.Accent},
		{Name: "REVIEW", Status: "review", Color: theme.FgSecondary},
		{Name: "VALIDATION", Status: "validation", Color: theme.Info},
		{Name: "DONE", Status: "done", Color: theme.Success},
		{Name: "BLOCKED", Status: "blocked", Color: theme.Error},
	}
}

// ColumnsFromConfig builds board column definitions from a BoardConfig.
// Falls back to DefaultColumns() if no custom columns are configured.
// Colors are assigned based on column roles using the DS palette, with
// optional per-column color overrides.
func ColumnsFromConfig(cfg teamstate.BoardConfig) []BoardColumnDef {
	if !cfg.HasCustomColumns() {
		return DefaultColumns()
	}
	cols := make([]BoardColumnDef, len(cfg.Columns))
	activeIdx := 0
	for i, c := range cfg.Columns {
		cols[i] = BoardColumnDef{
			Name:   c.Name,
			Status: c.ID,
			Color:  resolveColumnColor(c, activeIdx),
		}
		if c.Role == teamstate.ColumnRoleActive || c.Role == "" {
			activeIdx++
		}
	}
	return cols
}

// resolveColumnColor determines the tcell.Color for a column.
// Priority: explicit color override > role-based DS palette > active cycling.
func resolveColumnColor(c teamstate.BoardColumnConfig, activeIdx int) tcell.Color {
	// Explicit color override.
	if c.Color != "" {
		if color, ok := colorOverrides[c.Color]; ok {
			return color
		}
	}
	// Role-based DS palette.
	switch c.Role {
	case teamstate.ColumnRoleInitial:
		return theme.Warning
	case teamstate.ColumnRoleTerminal:
		return theme.Success
	case teamstate.ColumnRoleBlocked:
		return theme.Error
	default: // active or unset
		return activeColorCycle[activeIdx%len(activeColorCycle)]
	}
}

// activeColorCycle is the DS palette for "active" columns, cycling through
// Accent (Azure), FgSecondary (Overlay), Info (Sapphire).
var activeColorCycle = []tcell.Color{
	theme.Accent,
	theme.FgSecondary,
	theme.Info,
}

// colorOverrides maps user-facing color names to tcell.Color values.
var colorOverrides = map[string]tcell.Color{
	"orange": theme.Warning,
	"blue":   theme.Accent,
	"gray":   theme.FgSecondary,
	"cyan":   theme.Info,
	"green":  theme.Success,
	"red":    theme.Error,
	"purple": tcell.GetColor("#cba6f7"),
	"yellow": tcell.GetColor("#f9e2af"),
}

// newColumnSeparator returns a 1-char-wide vertical line drawn between kanban
// columns. The line uses BorderCard (#45475a) — subtle enough to delimit
// columns without competing with card borders.
func newColumnSeparator() *tview.Box {
	sep := tview.NewBox()
	sep.SetBackgroundColor(theme.BgPanel)
	sep.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		style := tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.BorderCard)
		for dy := 0; dy < height; dy++ {
			screen.SetContent(x, y+dy, '│', nil, style)
		}
		return x, y, width, height
	})
	return sep
}

// RunBoard launches the full-screen kanban board.
func RunBoard(cfg BoardConfig) error {
	columns := DefaultColumns()

	shell := layout.Build(cfg.Layout)

	// ── Card columns ──
	columnCards := make([]*widgets.CardColumn, len(columns))
	columnFlex := tview.NewFlex()
	columnFlex.SetBackgroundColor(theme.BgPanel)

	for i, col := range columns {
		if i > 0 {
			columnFlex.AddItem(newColumnSeparator(), 1, 0, false)
		}
		cc := widgets.NewCardColumn(col.Name, col.Color)
		columnCards[i] = cc
		columnFlex.AddItem(cc, 0, 1, i == 0)
	}

	// ── Populate columns ──
	populateColumns := func(tickets []BoardTicket) {
		for _, cc := range columnCards {
			cc.Clear()
		}
		for _, ticket := range tickets {
			for i, col := range columns {
				if ticket.Status != col.Status {
					continue
				}
				// Line 1: priority + title
				priority := ""
				if ticket.Priority != "" {
					priority = fmt.Sprintf("%s%s[-] ",
						widgets.ColorTag(priorityColor(ticket.Priority)), ticket.Priority)
				}
				mainText := priority + ticket.Title
				// Line 2: ID
				secondary := ticket.ID
				// Line 3: external ref (if any)
				meta := ""
				if ticket.ExternalRef != "" {
					meta = fmt.Sprintf("← %s", ticket.ExternalRef)
				}
				columnCards[i].AddCard(widgets.Card{
					MainText:      mainText,
					SecondaryText: secondary,
					MetaText:      meta,
				})
				break
			}
		}
	}
	populateColumns(cfg.Tickets)

	// ── Insert into shell content ──
	shell.Content.AddItem(columnFlex, 0, 1, true)

	// ── Focus management ──
	focusCol := 0
	updateFocus := func() {
		for i, cc := range columnCards {
			if i == focusCol {
				cc.SetFocused(true).SetNavArrows(i > 0, i < len(columnCards)-1)
				shell.App.SetFocus(cc)
			} else {
				cc.SetFocused(false).SetNavArrows(false, false)
			}
		}
	}
	updateFocus()

	// ── Refresh timer ──
	done := make(chan struct{})
	if cfg.RefreshFunc != nil {
		rate := cfg.RefreshRate
		if rate == 0 {
			rate = 5 * time.Second
		}
		go func() {
			ticker := time.NewTicker(rate)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ticker.C:
					tickets := cfg.RefreshFunc()
					shell.App.QueueUpdateDraw(func() {
						populateColumns(tickets)
					})
				}
			}
		}()
	}

	// ── Input ──
	shell.Content.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEscape || event.Rune() == 'q':
			close(done)
			shell.App.Stop()
			return nil
		case event.Key() == tcell.KeyLeft || event.Rune() == 'h':
			if focusCol > 0 {
				focusCol--
				updateFocus()
			}
			return nil
		case event.Key() == tcell.KeyRight || event.Rune() == 'l':
			if focusCol < len(columns)-1 {
				focusCol++
				updateFocus()
			}
			return nil
		case event.Rune() == 'r':
			if cfg.RefreshFunc != nil {
				tickets := cfg.RefreshFunc()
				populateColumns(tickets)
			}
			return nil
		}
		return event
	})

	return shell.App.SetRoot(shell.Root, true).EnableMouse(true).Run()
}

func priorityColor(priority string) tcell.Color {
	switch priority {
	case "P0", "critical":
		return theme.Error
	case "P1", "high":
		return theme.Warning
	case "P2", "medium":
		return theme.Accent
	case "P3", "low":
		return theme.FgSecondary
	default:
		return theme.FgMuted
	}
}

// truncateTitle shortens title to maxLen runes, appending "…" if truncated.
func truncateTitle(title string, maxLen int) string {
	runes := []rune(title)
	if len(runes) <= maxLen {
		return title
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}

// priorityPrefix returns a short coloured prefix for the ticket priority,
// e.g. "[P0] " in red, "[P1] " in yellow, etc.
func priorityPrefix(priority string) string {
	if priority == "" {
		return ""
	}
	return priority + " · "
}
