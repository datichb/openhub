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
// TeamBoard — team kanban with member assignment
// ─────────────────────────────────────────────────────────────────────────────

// TeamTicket extends BoardTicket with team assignment and label info.
type TeamTicket struct {
	ID          string
	Title       string
	Status      string
	Priority    string
	Assignee    string
	Project     string
	Description string
	// Labels are displayed as compact tags on the board item.
	Labels []string
}

// TeamBoardConfig configures the team board.
type TeamBoardConfig struct {
	Layout      layout.Config
	Tickets     []TeamTicket
	Members     []string
	RefreshFunc func() []TeamTicket
	RefreshRate time.Duration
}

// RunTeamBoard launches the full-screen team kanban board.
func RunTeamBoard(cfg TeamBoardConfig) error {
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

	// ── Populate ──
	populateColumns := func(tickets []TeamTicket) {
		for _, list := range columnLists {
			list.Clear()
		}
		for _, ticket := range tickets {
			for i, col := range columns {
				if ticket.Status == col.Status {
					assignee := ""
					if ticket.Assignee != "" {
						assignee = fmt.Sprintf(" %s@%s[-]",
							widgets.ColorTag(theme.Accent), ticket.Assignee)
					}
					columnLists[i].AddItem(
						ticket.Title+assignee,
						fmt.Sprintf("  %s", ticket.ID),
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

	// ── Refresh ──
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
