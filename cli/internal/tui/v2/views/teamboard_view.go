package views

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/i18n"
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

	// Filtering state
	allTickets     []TeamTicket // all tickets from RefreshFunc (unfiltered)
	filterText     string       // text search filter (matches ID, title, assignee)
	filterAssignee string       // filter by specific assignee
	filterLabel    string       // filter by specific label

	// Debouncing: prevents double-press on non-modal actions (claim, release, refresh).
	actionInProgress bool
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
func (v *TeamBoardView) Title() string { return i18n.T("tui.team.board") }

// StatusHints returns keybinding hints.
func (v *TeamBoardView) StatusHints() string {
	hints := "h/l colonnes · j/k items · / search · f filter · c claim · x release · t transfer · s status · r refresh"
	if v.filterText != "" || v.filterAssignee != "" || v.filterLabel != "" {
		hints += " · [yellow]FILTRÉ[-]"
	}
	return hints
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

	if len(v.cfg.Tickets) == 0 {
		muted := theme.ColorTag(theme.TextMutedHex)
		emptyTV := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter)
		emptyTV.SetBackgroundColor(theme.BgPanel)
		emptyTV.SetText(fmt.Sprintf("\n\n  %sAucun ticket dans l'équipe. Les tickets apparaîtront après un sync (r).%s", muted, theme.TagColor))
		content.AddItem(emptyTV, 0, 1, true)
	} else {
		content.AddItem(v.columnFlex, 0, 1, true)
	}

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
		if v.cfg.RefreshFunc != nil && !v.actionInProgress {
			v.actionInProgress = true
			// Async pull first, then refresh on completion.
			syncFuncAsync(v.app, v.cfg.SyncFunc, v.shell, func(_ error) {
				v.refreshOnEventLoop(DefaultColumns())
				v.actionInProgress = false
			})
		}
		return nil
	case '/':
		v.showSearchFilter()
		return nil
	case 'f':
		v.showFilterMenu()
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
	case tcell.KeyEscape:
		// Clear filters on Escape if any are active
		if v.filterText != "" || v.filterAssignee != "" || v.filterLabel != "" {
			v.clearFilters()
			return nil
		}
	}

	return event
}

func (v *TeamBoardView) populateColumns(tickets []TeamTicket, columns []BoardColumnDef) {
	v.allTickets = tickets
	filtered := v.applyFilters(tickets)
	for _, list := range v.columnLists {
		list.Clear()
	}
	for _, t := range filtered {
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

// applyFilters returns the subset of tickets matching all active filters.
func (v *TeamBoardView) applyFilters(tickets []TeamTicket) []TeamTicket {
	if v.filterText == "" && v.filterAssignee == "" && v.filterLabel == "" {
		return tickets
	}

	var result []TeamTicket
	textLower := strings.ToLower(v.filterText)

	for _, t := range tickets {
		// Assignee filter
		if v.filterAssignee != "" && t.Assignee != v.filterAssignee {
			continue
		}
		// Label filter
		if v.filterLabel != "" {
			hasLabel := false
			for _, l := range t.Labels {
				if l == v.filterLabel {
					hasLabel = true
					break
				}
			}
			if !hasLabel {
				continue
			}
		}
		// Text search filter (matches ID, title, or assignee)
		if textLower != "" {
			match := strings.Contains(strings.ToLower(t.ID), textLower) ||
				strings.Contains(strings.ToLower(t.Title), textLower) ||
				strings.Contains(strings.ToLower(t.Assignee), textLower)
			if !match {
				continue
			}
		}
		result = append(result, t)
	}
	return result
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
// Skips the refresh if an action is in progress to avoid overwriting fresh post-action data.
func (v *TeamBoardView) refresh(columns []BoardColumnDef) {
	if v.cfg.RefreshFunc == nil || v.app == nil || v.actionInProgress {
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
	if v.actionInProgress {
		return
	}
	ticketID := v.selectedTicketID()
	if ticketID == "" {
		return
	}
	v.actionInProgress = true
	go func() {
		err := v.actions.OnClaim(ticketID)
		if v.app == nil {
			return
		}
		v.app.QueueUpdateDraw(func() {
			v.actionInProgress = false
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
	if v.actionInProgress {
		return
	}
	ticketID := v.selectedTicketID()
	if ticketID == "" {
		return
	}
	v.actionInProgress = true
	go func() {
		err := v.actions.OnRelease(ticketID)
		if v.app == nil {
			return
		}
		v.app.QueueUpdateDraw(func() {
			v.actionInProgress = false
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
		{Label: "Planifié (TODO)", Value: "planned"},
		{Label: "En cours", Value: "in_progress"},
		{Label: "Revue", Value: "review"},
		{Label: "Bloqué", Value: "blocked"},
		{Label: "Terminé", Value: "done"},
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

// ─── Filtering ───────────────────────────────────────────────────────────────

// showSearchFilter opens a text input for live searching tickets.
func (v *TeamBoardView) showSearchFilter() {
	if v.shell == nil {
		return
	}
	v.shell.ShowInputModal("Rechercher (titre/ID/assignee)", v.filterText, func(text string) {
		v.filterText = text
		v.repopulateWithFilters()
		if text != "" {
			v.shell.ShowToastMsg("Filtre: \""+text+"\" (Esc pour effacer)", true)
		}
	})
}

// showFilterMenu opens a predefined filter selection modal.
func (v *TeamBoardView) showFilterMenu() {
	if v.shell == nil {
		return
	}

	options := []SelectOption{
		{Label: "Mes tickets", Value: "_mine"},
		{Label: "Par assignee...", Value: "_assignee"},
		{Label: "Par label...", Value: "_label"},
		{Label: "Effacer les filtres", Value: "_clear"},
	}

	v.shell.ShowSelectModal("Filtrer le board", options, "", func(choice string) {
		switch choice {
		case "_mine":
			// Use the MemberID from the config if available via Members
			if len(v.cfg.Members) > 0 {
				v.filterAssignee = v.cfg.Members[0] // first member is self by convention
			}
			v.repopulateWithFilters()
			v.shell.ShowToastMsg("Filtre: mes tickets", true)
		case "_assignee":
			v.showAssigneeFilter()
		case "_label":
			v.showLabelFilter()
		case "_clear":
			v.clearFilters()
		}
	})
}

// showAssigneeFilter shows a modal to pick an assignee to filter by.
func (v *TeamBoardView) showAssigneeFilter() {
	if v.shell == nil {
		return
	}

	// Collect unique assignees from all tickets
	assigneeSet := make(map[string]bool)
	for _, t := range v.allTickets {
		if t.Assignee != "" {
			assigneeSet[t.Assignee] = true
		}
	}

	var options []SelectOption
	for a := range assigneeSet {
		options = append(options, SelectOption{Label: "@" + a, Value: a})
	}
	if len(options) == 0 {
		v.shell.ShowToastMsg("Aucun assignee trouvé", false)
		return
	}

	v.shell.ShowSelectModal("Filtrer par assignee", options, "", func(assignee string) {
		v.filterAssignee = assignee
		v.repopulateWithFilters()
		v.shell.ShowToastMsg("Filtre: @"+assignee, true)
	})
}

// showLabelFilter shows an input modal to type a label to filter by.
func (v *TeamBoardView) showLabelFilter() {
	if v.shell == nil {
		return
	}
	v.shell.ShowInputModal("Filtrer par label", v.filterLabel, func(label string) {
		v.filterLabel = label
		v.repopulateWithFilters()
		if label != "" {
			v.shell.ShowToastMsg("Filtre: label="+label, true)
		}
	})
}

// clearFilters removes all active filters and repopulates the board.
func (v *TeamBoardView) clearFilters() {
	v.filterText = ""
	v.filterAssignee = ""
	v.filterLabel = ""
	v.repopulateWithFilters()
	if v.shell != nil {
		v.shell.ShowToastMsg("Filtres effacés", true)
	}
}

// repopulateWithFilters re-renders the board with current filters applied.
func (v *TeamBoardView) repopulateWithFilters() {
	if len(v.allTickets) == 0 {
		return
	}
	columns := DefaultColumns()
	filtered := v.applyFilters(v.allTickets)
	for _, list := range v.columnLists {
		list.Clear()
	}
	for _, t := range filtered {
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
