package views

import (
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// TeamBoardView — View interface adapter for the team kanban board
// ─────────────────────────────────────────────────────────────────────────────

// TeamBoardViewConfig holds the data for the team board inside the shell.
type TeamBoardViewConfig struct {
	Tickets     []TeamTicket
	Members     []string
	RefreshFunc func() []TeamTicket
	RefreshRate time.Duration
	// SyncFunc performs a git pull on the team-state repo and returns any error.
	// Called asynchronously on Mount and on 'r' keypress before RefreshFunc.
	// If nil, no pull is attempted by the board (behaviour unchanged from before).
	SyncFunc func() error
	// Actions wires the ticket action callbacks (claim, release, transfer, status).
	// If nil, action keys (c/x/t/s) are no-ops.
	Actions *BoardActions
}

// BoardActions provides callbacks for ticket actions on the board.
type BoardActions struct {
	OnClaim    func(ticketID string) error
	OnRelease  func(ticketID string) error
	OnTransfer func(ticketID, toMember string) error
	OnStatus   func(ticketID, newStatus string) error
	Members    func() []SelectOption // returns team members for transfer
}

// TeamBoardView implements View for the team kanban board.
type TeamBoardView struct {
	cfg         TeamBoardViewConfig
	columnFlex  *tview.Flex
	columnLists []*tview.List
	focusCol    int
	app         *tview.Application
	done        chan struct{}
	once        sync.Once
	shell       ShellAccess
	actions     *BoardActions
}

var _ View = (*TeamBoardView)(nil)

// NewTeamBoardView creates a new team board view.
// If cfg.Actions is non-nil, it is used as the initial board action set.
func NewTeamBoardView(cfg TeamBoardViewConfig) *TeamBoardView {
	v := &TeamBoardView{cfg: cfg}
	if cfg.Actions != nil {
		v.actions = cfg.Actions
	}
	return v
}

// SetShell provides the shell reference for modal interactions.
func (v *TeamBoardView) SetShell(s ShellAccess) { v.shell = s }

// SetActions configures the callbacks for ticket actions.
func (v *TeamBoardView) SetActions(a *BoardActions) { v.actions = a }

// ID returns the view identifier.
func (v *TeamBoardView) ID() string { return "team.board" }

// Title returns the display title.
func (v *TeamBoardView) Title() string { return "Team Board" }

// StatusHints returns keybinding hints.
func (v *TeamBoardView) StatusHints() string {
	return "h/l colonnes · j/k items · c claim · x release · t transfer · s status · r refresh"
}

// Mount builds the team board and inserts it into the content panel.
func (v *TeamBoardView) Mount(content *tview.Flex, app *tview.Application) {
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

	if v.cfg.RefreshFunc != nil {
		rate := v.cfg.RefreshRate
		if rate == 0 {
			rate = 5 * time.Second
		}
		// Async pull on entry so the board starts with fresh data.
		syncFuncAsync(v.app, v.cfg.SyncFunc, v.shell, func(_ error) {
			v.refreshOnEventLoop(columns)
		})
		go v.refreshLoop(rate, columns)
	}
}

// Unmount stops the refresh goroutine.
func (v *TeamBoardView) Unmount() {
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

// HandleKey processes team board key events.
func (v *TeamBoardView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Rune() {
	case 'l':
		v.moveFocus(1)
		return nil
	case 'h':
		v.moveFocus(-1)
		return nil
	case 'r':
		if v.cfg.RefreshFunc != nil {
			// Async pull first, then refresh on completion.
			syncFuncAsync(v.app, v.cfg.SyncFunc, v.shell, func(_ error) {
				v.refreshOnEventLoop(DefaultColumns())
			})
		}
		return nil
	case 'c':
		v.claimTicket()
		return nil
	case 'x':
		v.releaseTicket()
		return nil
	case 't':
		v.transferTicket()
		return nil
	case 's':
		v.changeStatus()
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

func (v *TeamBoardView) populateColumns(tickets []TeamTicket, columns []BoardColumnDef) {
	for _, list := range v.columnLists {
		list.Clear()
	}
	for _, t := range tickets {
		for i, col := range columns {
			if t.Status == col.Status {
				assignee := ""
				if t.Assignee != "" {
					assignee = " " + widgets.ColorTag(theme.Accent) + "@" + t.Assignee + "[-]"
				}
				labelStr := formatTicketLabels(t.Labels)
				v.columnLists[i].AddItem(t.Title+assignee+labelStr, t.ID, 0, nil)
				break
			}
		}
	}
}

// formatTicketLabels renders a compact label string for board display.
// Well-known labels get special icons; others are displayed as-is.
func formatTicketLabels(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	result := " "
	for _, l := range labels {
		switch l {
		case "agent-reviewed":
			result += "[green][AI][-]"
		case "needs-human-review":
			result += "[yellow][!][-]"
		default:
			result += "[gray][" + l + "][-]"
		}
	}
	return result
}

func (v *TeamBoardView) moveFocus(delta int) {
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

func (v *TeamBoardView) updateColumnFocus() {
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

func (v *TeamBoardView) refreshLoop(rate time.Duration, columns []BoardColumnDef) {
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

// refresh fetches tickets from the remote and schedules a board repopulation via
// QueueUpdateDraw. Safe to call from ANY goroutine (ticker, background workers).
// Do NOT call from inside a QueueUpdateDraw callback — use refreshOnEventLoop instead.
func (v *TeamBoardView) refresh(columns []BoardColumnDef) {
	if v.cfg.RefreshFunc == nil || v.app == nil {
		return
	}
	tickets := v.cfg.RefreshFunc()
	v.app.QueueUpdateDraw(func() {
		v.populateColumns(tickets, columns)
	})
}

// refreshOnEventLoop fetches tickets and repopulates the board immediately.
// MUST be called from inside the tview event loop (QueueUpdateDraw callback or
// tview handler). Does NOT call QueueUpdateDraw — avoids the nested deadlock that
// would occur if QueueUpdateDraw were called from inside a running QueueUpdateDraw
// callback (the unbuffered done-channel would block forever).
func (v *TeamBoardView) refreshOnEventLoop(columns []BoardColumnDef) {
	if v.cfg.RefreshFunc == nil {
		return
	}
	tickets := v.cfg.RefreshFunc()
	v.populateColumns(tickets, columns)
}

// ─── Ticket Actions ──────────────────────────────────────────────────────────

func (v *TeamBoardView) selectedTicketID() string {
	if v.focusCol < 0 || v.focusCol >= len(v.columnLists) {
		return ""
	}
	list := v.columnLists[v.focusCol]
	if list.GetItemCount() == 0 {
		return ""
	}
	idx := list.GetCurrentItem()
	if idx < 0 {
		return ""
	}
	_, secondary := list.GetItemText(idx)
	return secondary
}

func (v *TeamBoardView) claimTicket() {
	if v.actions == nil || v.actions.OnClaim == nil || v.shell == nil {
		return
	}
	ticketID := v.selectedTicketID()
	if ticketID == "" {
		return
	}
	go func() {
		err := v.actions.OnClaim(ticketID)
		if v.app == nil {
			return
		}
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.shell.ShowToastMsg("Claim échoué: "+err.Error(), false)
			} else {
				v.shell.ShowToastMsg("Ticket claim: "+ticketID, true)
				if v.cfg.RefreshFunc != nil {
					v.refreshOnEventLoop(DefaultColumns())
				}
			}
		})
	}()
}

func (v *TeamBoardView) releaseTicket() {
	if v.actions == nil || v.actions.OnRelease == nil || v.shell == nil {
		return
	}
	ticketID := v.selectedTicketID()
	if ticketID == "" {
		return
	}
	go func() {
		err := v.actions.OnRelease(ticketID)
		if v.app == nil {
			return
		}
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.shell.ShowToastMsg("Release échoué: "+err.Error(), false)
			} else {
				v.shell.ShowToastMsg("Ticket libéré: "+ticketID, true)
				if v.cfg.RefreshFunc != nil {
					v.refreshOnEventLoop(DefaultColumns())
				}
			}
		})
	}()
}

func (v *TeamBoardView) transferTicket() {
	if v.actions == nil || v.actions.OnTransfer == nil || v.actions.Members == nil || v.shell == nil {
		return
	}
	ticketID := v.selectedTicketID()
	if ticketID == "" {
		return
	}
	members := v.actions.Members()
	if len(members) == 0 {
		v.shell.ShowToastMsg("Aucun membre dans l'équipe", false)
		return
	}
	v.shell.ShowSelectModal("Transférer "+ticketID+" à", members, "", func(toMember string) {
		go func() {
			err := v.actions.OnTransfer(ticketID, toMember)
			if v.app == nil {
				return
			}
			v.app.QueueUpdateDraw(func() {
				if err != nil {
					v.shell.ShowToastMsg("Transfert échoué: "+err.Error(), false)
				} else {
					v.shell.ShowToastMsg("Transféré à "+toMember, true)
					if v.cfg.RefreshFunc != nil {
						v.refreshOnEventLoop(DefaultColumns())
					}
				}
			})
		}()
	})
}

func (v *TeamBoardView) changeStatus() {
	if v.actions == nil || v.actions.OnStatus == nil || v.shell == nil {
		return
	}
	ticketID := v.selectedTicketID()
	if ticketID == "" {
		return
	}

	statusOptions := []SelectOption{
		{Label: "Planned (TODO)", Value: "planned"},
		{Label: "In Progress", Value: "in_progress"},
		{Label: "Review", Value: "review"},
		{Label: "Blocked", Value: "blocked"},
		{Label: "Done", Value: "done"},
	}

	v.shell.ShowSelectModal("Status de "+ticketID, statusOptions, "", func(newStatus string) {
		go func() {
			err := v.actions.OnStatus(ticketID, newStatus)
			if v.app == nil {
				return
			}
			v.app.QueueUpdateDraw(func() {
				if err != nil {
					v.shell.ShowToastMsg("Changement échoué: "+err.Error(), false)
				} else {
					v.shell.ShowToastMsg("Status mis à jour", true)
					if v.cfg.RefreshFunc != nil {
						v.refreshOnEventLoop(DefaultColumns())
					}
				}
			})
		}()
	})
}
