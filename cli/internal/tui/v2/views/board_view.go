package views

import (
	"fmt"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// BoardView — View interface adapter for the kanban board
// ─────────────────────────────────────────────────────────────────────────────

// BoardViewConfig holds the data needed to render the board inside the shell.
type BoardViewConfig struct {
	Tickets     []BoardTicket
	RefreshFunc func() []BoardTicket
	RefreshRate time.Duration
}

// BoardView implements View for the kanban board.
type BoardView struct {
	cfg         BoardViewConfig
	columnFlex  *tview.Flex
	columnLists []*tview.List
	focusCol    int
	app         *tview.Application
	done        chan struct{}
	once        sync.Once
}

var _ View = (*BoardView)(nil)

// NewBoardView creates a new board view with the given config.
func NewBoardView(cfg BoardViewConfig) *BoardView {
	return &BoardView{cfg: cfg}
}

// ID returns the view identifier.
func (v *BoardView) ID() string { return "board" }

// Title returns the display title.
func (v *BoardView) Title() string { return "Board" }

// StatusHints returns keybinding hints.
func (v *BoardView) StatusHints() string {
	return "h/l colonnes · j/k items · r refresh · Esc retour"
}

// Mount builds the kanban board and inserts it into the content panel.
func (v *BoardView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.done = make(chan struct{})

	columns := DefaultColumns()
	v.columnLists = make([]*tview.List, len(columns))
	v.columnFlex = tview.NewFlex()

	for i, col := range columns {
		list := tview.NewList().
			ShowSecondaryText(false).
			SetHighlightFullLine(true).
			SetMainTextColor(theme.FgPrimary)
		list.SetBackgroundColor(theme.BgPanel)
		list.SetBorder(true)
		list.SetBorderColor(theme.BorderNormal)
		list.SetTitle(" " + col.Name + " ")
		list.SetTitleColor(col.Color)
		v.columnLists[i] = list
		v.columnFlex.AddItem(list, 0, 1, i == 0)
	}

	v.populateColumns(v.cfg.Tickets, columns)
	content.AddItem(v.columnFlex, 0, 1, true)

	v.focusCol = 0
	v.updateColumnFocus()

	// Start refresh goroutine
	if v.cfg.RefreshFunc != nil {
		rate := v.cfg.RefreshRate
		if rate == 0 {
			rate = 5 * time.Second
		}
		go v.refreshLoop(rate, columns)
	}
}

// Unmount stops the refresh goroutine.
func (v *BoardView) Unmount() {
	v.once.Do(func() {
		if v.done != nil {
			close(v.done)
		}
	})
	v.app = nil
	v.columnFlex = nil
	v.columnLists = nil
	v.once = sync.Once{}
}

// HandleKey processes board-specific key events.
func (v *BoardView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'l':
		v.moveFocus(1)
		return nil
	case 'h':
		v.moveFocus(-1)
		return nil
	case 'r':
		if v.cfg.RefreshFunc != nil {
			v.refresh(DefaultColumns())
		}
		return nil
	}

	switch event.Key() {
	case tcell.KeyRight:
		v.moveFocus(1)
		return nil
	case tcell.KeyLeft:
		v.moveFocus(-1)
		return nil
	}

	return event
}

func (v *BoardView) populateColumns(tickets []BoardTicket, columns []BoardColumnDef) {
	for _, list := range v.columnLists {
		list.Clear()
	}
	for _, t := range tickets {
		for i, col := range columns {
			if t.Status == col.Status {
				color := priorityColor(t.Priority)
				text := fmt.Sprintf("%s%s%s", widgets.ColorTag(color), t.Title, "[-]")
				v.columnLists[i].AddItem(text, t.ID, 0, nil)
				break
			}
		}
	}
}

func (v *BoardView) moveFocus(delta int) {
	if len(v.columnLists) == 0 {
		return
	}
	v.focusCol += delta
	if v.focusCol < 0 {
		v.focusCol = 0
	}
	if v.focusCol >= len(v.columnLists) {
		v.focusCol = len(v.columnLists) - 1
	}
	v.updateColumnFocus()
}

func (v *BoardView) updateColumnFocus() {
	for i, list := range v.columnLists {
		if i == v.focusCol {
			list.SetBorderColor(theme.BorderFocus)
			if v.app != nil {
				v.app.SetFocus(list)
			}
		} else {
			list.SetBorderColor(theme.BorderNormal)
		}
	}
}

func (v *BoardView) refreshLoop(rate time.Duration, columns []BoardColumnDef) {
	ticker := time.NewTicker(rate)
	defer ticker.Stop()
	for {
		select {
		case <-v.done:
			return
		case <-ticker.C:
			v.refresh(columns)
		}
	}
}

func (v *BoardView) refresh(columns []BoardColumnDef) {
	if v.cfg.RefreshFunc == nil || v.app == nil {
		return
	}
	tickets := v.cfg.RefreshFunc()
	v.app.QueueUpdateDraw(func() {
		v.populateColumns(tickets, columns)
	})
}
