package views

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Board — full-screen Kanban board with columns
// ─────────────────────────────────────────────────────────────────────────────

// BoardConfig configures the board view.
type BoardConfig struct {
	Layout      layout.Config
	Tickets     []BoardTicket
	RefreshFunc func() []BoardTicket
	RefreshRate time.Duration
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
				mainText := priority + tview.Escape(ticket.Title)
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
