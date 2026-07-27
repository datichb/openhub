package views

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Board — full-screen Kanban board with columns
// ─────────────────────────────────────────────────────────────────────────────

// BoardTicket represents a kanban ticket.
type BoardTicket struct {
	ID       string
	Title    string
	Status   string // "todo", "in_progress", "done", "blocked"
	Priority string
	Type     string
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
		{Name: "DONE", Status: "done", Color: theme.Success},
		{Name: "BLOCKED", Status: "blocked", Color: theme.Error},
	}
}

// RunBoard launches the full-screen kanban board.
func RunBoard(cfg BoardConfig) error {
	columns := DefaultColumns()

	shell := layout.Build(cfg.Layout)

	// ── Column lists ──
	columnLists := make([]*tview.List, len(columns))
	columnFlex := tview.NewFlex()
	columnFlex.SetBackgroundColor(theme.BgPanel)

	for i, col := range columns {
		list := tview.NewList().
			ShowSecondaryText(true).
			SetHighlightFullLine(true).
			SetSelectedBackgroundColor(theme.BgElement).
			SetSelectedTextColor(theme.FgPrimary).
			SetSecondaryTextColor(theme.FgSecondary)
		list.SetBackgroundColor(theme.BgPanel).
			SetBorder(true).
			SetBorderColor(theme.BorderNormal).
			SetTitle(fmt.Sprintf(" %s ", col.Name)).
			SetTitleColor(col.Color)

		columnLists[i] = list
		columnFlex.AddItem(list, 0, 1, i == 0)
	}

	// ── Populate columns ──
	populateColumns := func(tickets []BoardTicket) {
		for _, list := range columnLists {
			list.Clear()
		}
		for _, ticket := range tickets {
			for i, col := range columns {
				if ticket.Status == col.Status {
					priority := ""
					if ticket.Priority != "" {
						priority = fmt.Sprintf("%s%s[-] ",
							widgets.ColorTag(priorityColor(ticket.Priority)), ticket.Priority)
					}
					columnLists[i].AddItem(
						priority+ticket.Title,
						"  "+ticket.ID,
						0, nil,
					)
					break
				}
			}
		}
	}
	populateColumns(cfg.Tickets)

	// ── Insert into shell content ──
	shell.Content.AddItem(columnFlex, 0, 1, true)

	// ── Focus management ──
	focusCol := 0
	updateFocus := func() {
		for i, list := range columnLists {
			if i == focusCol {
				list.SetBorderColor(theme.BorderFocus)
				shell.App.SetFocus(list)
			} else {
				list.SetBorderColor(theme.BorderNormal)
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
