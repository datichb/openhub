package views

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// BeadsSummary holds the task completion count for a tracker ticket's linked beads.
type BeadsSummary struct {
	Done  int
	Total int
}

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
	// IsConfigured returns whether the team-state repo is resolved and cloned.
	// Used to display an appropriate empty-state message (ADR-032).
	IsConfigured func() bool
	// BeadsSummaryFunc returns bead task counts keyed by external ref (e.g. "gitlab-693").
	// Called on a separate goroutine with a longer interval (ADR-032).
	// If nil, no badge is displayed.
	BeadsSummaryFunc func() map[string]BeadsSummary
	// LabelStatusMapping holds the label → status mapping from team config.
	// Used to filter workflow labels from the board display (line 2) since
	// they are already reflected by the column position.
	LabelStatusMapping map[string]string
	// QuickActions provides callbacks for launching agent sessions from the board.
	// If nil, the 'a' key quick-action feature is disabled.
	QuickActions *BoardQuickActions
}

// BoardActions provides callbacks for ticket actions on the board.
type BoardActions struct {
	OnClaim    func(ticketID string) error
	OnRelease  func(ticketID string) error
	OnTransfer func(ticketID, toMember string) error
	OnStatus   func(ticketID, newStatus string) error
	Members    func() []SelectOption // returns team members for transfer
	// FetchDetail fetches the full ticket detail from the external tracker.
	// Returns title and full description (not truncated). If nil, the detail
	// modal only shows the cached data from the team-state TOML.
	FetchDetail func(project, ticketID string) (title, description string, err error)
}

// TeamBoardView implements View for the team kanban board.
type TeamBoardView struct {
	cfg         TeamBoardViewConfig
	columnFlex  *tview.Flex
	columnCards []*widgets.CardColumn
	focusCol    int
	app         *tview.Application
	done        chan struct{}
	once        sync.Once
	shell       ShellAccess
	actions     *BoardActions

	// Windowed column scroll — only a subset of columns is visible at a time.
	colViewStart int // index of the first visible column in columnCards
	visibleCols  int // number of columns visible (auto-detected from terminal width)
	allColumns   []BoardColumnDef // all column definitions for reference

	// ticketIDLookup maps [colIndex][itemIndex] → ticket ID for selected-item resolution.
	// Rebuilt every time populateColumns or repopulateWithFilters is called.
	ticketIDLookup [][]string

	// Filtering state
	allTickets     []TeamTicket // all tickets from RefreshFunc (unfiltered)
	filterText     string       // text search filter (matches ID, title, assignee)
	filterAssignee string       // filter by specific assignee
	filterLabel    string       // filter by specific label

	// Project tabs
	projectTabs  []string // ["", "proj-a", "proj-b"] — "" = all
	activeTabIdx int
	tabBar       *tview.TextView
	boardLayout  *tview.Flex // vertical flex: tabBar + columnFlex

	// Debouncing: prevents double-press on non-modal actions (claim, release, refresh).
	actionInProgress bool

	// Beads summary cache — populated by a background goroutine (ADR-032).
	// Stores a map[string]BeadsSummary keyed by external ref (e.g. "gitlab-693").
	beadsSummary atomic.Value // holds map[string]BeadsSummary
}

var _ View = (*TeamBoardView)(nil)
var _ CommandProvider = (*TeamBoardView)(nil)

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
	hints := fmt.Sprintf("h/l %s · j/k %s · Enter %s · a %s · [[] /] %s · / %s · f %s · c %s · x %s · t %s · s %s · r %s",
		i18n.T("tui.hints.columns"),
		i18n.T("tui.hints.items"),
		i18n.T("tui.hints.detail"),
		i18n.T("tui.hints.actions"),
		i18n.T("tui.hints.projects"),
		i18n.T("tui.hints.search"),
		i18n.T("tui.hints.filter"),
		i18n.T("tui.hints.claim"),
		i18n.T("tui.hints.release"),
		i18n.T("tui.hints.transfer"),
		i18n.T("tui.hints.status"),
		i18n.T("tui.hints.refresh"),
	)
	if v.filterText != "" || v.filterAssignee != "" || v.filterLabel != "" {
		hints += " · [yellow]" + i18n.T("tui.hints.filtered") + "[-]"
	}
	return hints
}

// Mount builds the team board and inserts it into the content panel.
func (v *TeamBoardView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.done = make(chan struct{})

	columns := DefaultColumns()
	v.allColumns = columns
	v.columnCards = make([]*widgets.CardColumn, len(columns))
	v.columnFlex = tview.NewFlex()

	for i, col := range columns {
		cc := widgets.NewCardColumn(col.Name, col.Color)
		v.columnCards[i] = cc
	}

	// Auto-detect visible column count (min 25 chars per column).
	// Default to 4 visible columns; will be adjusted on first draw.
	v.visibleCols = 4
	if len(columns) < v.visibleCols {
		v.visibleCols = len(columns)
	}
	v.colViewStart = 0
	v.rebuildColumnFlex()

	// Tab bar for project filtering
	v.tabBar = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignLeft)
	v.tabBar.SetBackgroundColor(theme.BgPanel)
	v.buildProjectTabs(v.cfg.Tickets)
	v.renderTabBar()

	// Board layout: tabBar (1 line) + columns
	v.boardLayout = tview.NewFlex().SetDirection(tview.FlexRow)
	v.boardLayout.AddItem(v.tabBar, 1, 0, false)
	v.boardLayout.AddItem(v.columnFlex, 0, 1, true)

	v.populateColumns(v.cfg.Tickets, columns)

	// Empty-state: show a helpful message when no tickets are available (ADR-032)
	hasTickets := len(v.cfg.Tickets) > 0
	isConfigured := v.cfg.IsConfigured == nil || v.cfg.IsConfigured()

	if !hasTickets && !isConfigured {
		emptyTV := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter)
		emptyTV.SetBackgroundColor(theme.BgPanel)
		emptyTV.SetText("\n\n[yellow]Aucune équipe configurée.[-]\n\nUtilisez [white]Team Init[-] pour commencer.")
		content.AddItem(emptyTV, 0, 1, true)
	} else if !hasTickets {
		emptyTV := tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter)
		emptyTV.SetBackgroundColor(theme.BgPanel)
		emptyTV.SetText("\n\n[yellow]Aucun ticket.[-]\n\nLancez [white]Sync Tracker[-] ([::b]r[::-]) pour synchroniser.")
		content.AddItem(emptyTV, 0, 1, true)
	} else {
		content.AddItem(v.boardLayout, 0, 1, true)
	}

	// Always add boardLayout — async sync will populate it with tickets.
	// Empty columns are visually clear enough without a special empty-state message.

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

	// Start beads summary goroutine — separate, slower interval (ADR-032).
	if v.cfg.BeadsSummaryFunc != nil {
		// Initial fetch
		go func() {
			summary := v.cfg.BeadsSummaryFunc()
			if summary != nil {
				v.beadsSummary.Store(summary)
				// Trigger a repaint with the badges
				v.refresh(columns)
			}
		}()
		// Periodic refresh (30s)
		go v.beadsSummaryLoop(30*time.Second, columns)
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
	v.columnCards = nil
	v.once = sync.Once{}
}

// HandleKey processes team board key events.
func (v *TeamBoardView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter:
		v.showTicketDetail()
		return nil
	case tcell.KeyRight:
		v.moveFocus(1)
		return nil
	case tcell.KeyLeft:
		v.moveFocus(-1)
		return nil
	}

	switch event.Rune() {
	case 'l':
		v.moveFocus(1)
		return nil
	case 'h':
		v.moveFocus(-1)
		return nil
	case '[':
		v.prevProjectTab()
		return nil
	case ']':
		v.nextProjectTab()
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
	case 'a':
		v.showTeamBoardQuickActions()
		return nil
	}

	return event
}

func (v *TeamBoardView) populateColumns(tickets []TeamTicket, columns []BoardColumnDef) {
	v.allTickets = tickets

	// Rebuild project tabs from latest ticket data
	v.buildProjectTabs(tickets)
	v.renderTabBar()

	// Apply project tab filter, then other filters
	filtered := v.applyProjectFilter(tickets)
	filtered = v.applyFilters(filtered)

	// Reset ticket ID lookup
	v.ticketIDLookup = make([][]string, len(v.columnCards))
	for i := range v.ticketIDLookup {
		v.ticketIDLookup[i] = nil
	}

	for _, cc := range v.columnCards {
		cc.Clear()
	}
	for _, t := range filtered {
		for i, col := range columns {
			if t.Status == col.Status {
				// ── Line 1: [Project] Title... [N/M] ──
				projectTag := ""
				projectDisplay := t.ProjectName
				if projectDisplay == "" {
					projectDisplay = t.Project
				}
				if projectDisplay != "" {
					projectTag = fmt.Sprintf("[black:%s] %s [-:-] ",
						theme.AccentHex, tview.Escape(projectDisplay))
				}
				// Truncate title to keep line 1 readable
				title := t.Title
				titleRunes := []rune(title)
				if len(titleRunes) > 40 {
					title = string(titleRunes[:37]) + "..."
				}
				// Beads badge: [done/total] when linked beads exist (ADR-032)
				badgeStr := ""
				if summary := v.getBeadsSummary(); summary != nil {
					if bs, ok := summary[t.ID]; ok && bs.Total > 0 {
						color := theme.TextMutedHex
						if bs.Done == bs.Total {
							color = theme.SuccessHex
						}
						badgeStr = fmt.Sprintf(" [%s][%d/%d][-]", color, bs.Done, bs.Total)
					}
				}
				mainText := projectTag + title + badgeStr

				// ── Line 2: ID · priority ──
				secondary := t.ID
				if t.Priority != "" {
					secondary += " · " + t.Priority
				}

				// ── Line 3: @assignee · label1 · label2 (filtered) ──
				var parts []string
				if t.Assignee != "" {
					parts = append(parts, widgets.ColorTag(theme.Accent)+"@"+t.Assignee+"[-]")
				} else {
					parts = append(parts, fmt.Sprintf("[%s]%s[-]", theme.WarningHex, i18n.T("board.claimable")))
				}
				// Filter out workflow labels (already reflected by column)
				for _, l := range t.Labels {
					if v.cfg.LabelStatusMapping != nil {
						if _, isWorkflow := v.cfg.LabelStatusMapping[l]; isWorkflow {
							continue
						}
					}
					parts = append(parts, "[gray]"+tview.Escape(l)+"[-]")
				}
				meta := ""
				if len(parts) > 0 {
					meta = strings.Join(parts, " · ")
				}

				v.columnCards[i].AddCard(widgets.Card{
					MainText:      mainText,
					SecondaryText: secondary,
					MetaText:      meta,
				})
				v.ticketIDLookup[i] = append(v.ticketIDLookup[i], t.ID)
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

// ─── Project Tabs ────────────────────────────────────────────────────────────

// buildProjectTabs extracts unique project names from tickets and builds tab list.
func (v *TeamBoardView) buildProjectTabs(tickets []TeamTicket) {
	seen := make(map[string]bool)
	var projects []string
	for _, t := range tickets {
		if t.Project != "" && !seen[t.Project] {
			seen[t.Project] = true
			projects = append(projects, t.Project)
		}
	}
	// Only show tabs if there are 2+ projects (or 1+ for the all tab to be useful)
	v.projectTabs = append([]string{""}, projects...) // "" = all
	if v.activeTabIdx >= len(v.projectTabs) {
		v.activeTabIdx = 0
	}
}

// renderTabBar renders the project tabs in the tab bar widget.
func (v *TeamBoardView) renderTabBar() {
	if v.tabBar == nil || len(v.projectTabs) <= 2 {
		// Hide tab bar when only 1 project (or none) — no need to filter
		if v.tabBar != nil {
			v.tabBar.SetText("")
		}
		return
	}

	var sb strings.Builder
	sb.WriteString(" ")
	for i, tab := range v.projectTabs {
		label := tab
		if label == "" {
			label = i18n.T("tui.board.tab_all")
		}
		if i == v.activeTabIdx {
			// Active tab: accent background
			sb.WriteString(fmt.Sprintf("[black:%s] %s [-:-]", theme.AccentHex, tview.Escape(label)))
		} else {
			// Inactive tab: muted
			sb.WriteString(fmt.Sprintf("[%s] %s [-]", theme.TextMutedHex, tview.Escape(label)))
		}
		sb.WriteString("  ")
	}
	v.tabBar.SetText(sb.String())
}

// applyProjectFilter filters tickets by the active project tab.
func (v *TeamBoardView) applyProjectFilter(tickets []TeamTicket) []TeamTicket {
	if v.activeTabIdx == 0 || v.activeTabIdx >= len(v.projectTabs) {
		return tickets // "All" tab — no filter
	}
	project := v.projectTabs[v.activeTabIdx]
	var result []TeamTicket
	for _, t := range tickets {
		if t.Project == project {
			result = append(result, t)
		}
	}
	return result
}

// prevProjectTab switches to the previous project tab.
func (v *TeamBoardView) prevProjectTab() {
	if len(v.projectTabs) <= 2 {
		return
	}
	v.activeTabIdx--
	if v.activeTabIdx < 0 {
		v.activeTabIdx = len(v.projectTabs) - 1
	}
	v.repopulateWithFilters()
}

// nextProjectTab switches to the next project tab.
func (v *TeamBoardView) nextProjectTab() {
	if len(v.projectTabs) <= 2 {
		return
	}
	v.activeTabIdx++
	if v.activeTabIdx >= len(v.projectTabs) {
		v.activeTabIdx = 0
	}
	v.repopulateWithFilters()
}

func (v *TeamBoardView) moveFocus(delta int) {
	if len(v.columnCards) == 0 {
		return
	}
	v.focusCol += delta
	if v.focusCol < 0 {
		v.focusCol = 0
	}
	if v.focusCol >= len(v.columnCards) {
		v.focusCol = len(v.columnCards) - 1
	}
	// Scroll the column window if focus moves outside visible range
	if v.focusCol < v.colViewStart {
		v.colViewStart = v.focusCol
		v.rebuildColumnFlex()
	} else if v.focusCol >= v.colViewStart+v.visibleCols {
		v.colViewStart = v.focusCol - v.visibleCols + 1
		v.rebuildColumnFlex()
	}
	v.updateColumnFocus()
}

// rebuildColumnFlex replaces the Flex contents with the current visible column window.
// Columns are separated by subtle vertical lines. Navigation arrows in the
// focused column header replace the former ◄/► scroll indicators.
func (v *TeamBoardView) rebuildColumnFlex() {
	v.columnFlex.Clear()
	end := v.colViewStart + v.visibleCols
	if end > len(v.columnCards) {
		end = len(v.columnCards)
	}
	for i := v.colViewStart; i < end; i++ {
		if i > v.colViewStart {
			v.columnFlex.AddItem(newColumnSeparator(), 1, 0, false)
		}
		v.columnFlex.AddItem(v.columnCards[i], 0, 1, i == v.focusCol)
	}
}

func (v *TeamBoardView) updateColumnFocus() {
	for i, cc := range v.columnCards {
		if i == v.focusCol {
			cc.SetFocused(true).SetNavArrows(i > 0, i < len(v.columnCards)-1)
			if v.app != nil {
				v.app.SetFocus(cc)
			}
		} else {
			cc.SetFocused(false).SetNavArrows(false, false)
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

// beadsSummaryLoop periodically refreshes the beads summary cache (ADR-032).
// Uses a separate, slower interval than the main refresh loop to avoid
// spawning too many bd subprocess invocations.
func (v *TeamBoardView) beadsSummaryLoop(rate time.Duration, columns []BoardColumnDef) {
	ticker := time.NewTicker(rate)
	defer ticker.Stop()
	for {
		select {
		case <-v.done:
			return
		case <-ticker.C:
			if v.cfg.BeadsSummaryFunc == nil {
				return
			}
			summary := v.cfg.BeadsSummaryFunc()
			if summary != nil {
				v.beadsSummary.Store(summary)
				// Trigger repaint to show updated badges
				v.refresh(columns)
			}
		}
	}
}

// getBeadsSummary returns the cached beads summary map, or nil if not loaded yet.
func (v *TeamBoardView) getBeadsSummary() map[string]BeadsSummary {
	val := v.beadsSummary.Load()
	if val == nil {
		return nil
	}
	return val.(map[string]BeadsSummary)
}

// ─── Ticket Detail ───────────────────────────────────────────────────────────

// showTicketDetail displays a scrollable modal with the selected ticket's info.
func (v *TeamBoardView) showTicketDetail() {
	if v.shell == nil {
		return
	}
	ticketID := v.selectedTicketID()
	if ticketID == "" {
		return
	}

	// Find the full ticket data
	var ticket *TeamTicket
	for i := range v.allTickets {
		if v.allTickets[i].ID == ticketID {
			ticket = &v.allTickets[i]
			break
		}
	}
	if ticket == nil {
		return
	}

	// Build detail content
	detail := v.formatTicketDetail(ticket)

	// Build actions: always "Fermer"; add "Actualiser" if FetchDetail is available.
	actions := []ModalAction{{Label: "Fermer", Callback: nil}}
	if v.actions != nil && v.actions.FetchDetail != nil {
		actions = []ModalAction{
			{Label: "Description complète", Callback: func() {
				// Fetch full description on-demand from tracker.
				go func() {
					title, desc, err := v.actions.FetchDetail(ticket.Project, ticket.ID)
					if v.app == nil {
						return
					}
					v.app.QueueUpdateDraw(func() {
						if err != nil {
							v.shell.ShowToastMsg("Erreur: "+err.Error(), false)
							return
						}
						// Update cached ticket data with fresh info.
						if title != "" {
							ticket.Title = title
						}
						if desc != "" {
							ticket.Description = desc
						}
						// Re-display with full description.
						fullDetail := v.formatTicketDetail(ticket)
						v.shell.ShowScrollableModal("Ticket: "+ticket.ID, fullDetail, []ModalAction{{Label: "Fermer", Callback: nil}})
					})
				}()
			}},
			{Label: "Fermer", Callback: nil},
		}
	}

	v.shell.ShowScrollableModal("Ticket: "+ticket.ID, detail, actions)
}

// formatTicketDetail builds the detail content string for a ticket.
func (v *TeamBoardView) formatTicketDetail(ticket *TeamTicket) string {
	var detail strings.Builder
	detail.WriteString(fmt.Sprintf("\n  [::b]%s%s\n\n", ticket.Title, theme.TagReset))
	detail.WriteString(fmt.Sprintf("  %sID:%s         %s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, ticket.ID))
	if ticket.Project != "" {
		detail.WriteString(fmt.Sprintf("  %sProjet:%s     %s\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, ticket.Project))
	}
	detail.WriteString(fmt.Sprintf("  %sStatut:%s     %s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, ticket.Status))

	assignee := "-"
	if ticket.Assignee != "" {
		assignee = "@" + ticket.Assignee
	}
	detail.WriteString(fmt.Sprintf("  %sAssignee:%s   %s\n",
		theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, assignee))

	if ticket.Priority != "" {
		detail.WriteString(fmt.Sprintf("  %sPriorité:%s   %s\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, ticket.Priority))
	}

	if len(ticket.Labels) > 0 {
		detail.WriteString(fmt.Sprintf("  %sLabels:%s     %s\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, strings.Join(ticket.Labels, ", ")))
	}

	if ticket.Description != "" {
		detail.WriteString(fmt.Sprintf("\n  %sDescription:%s\n  %s\n",
			theme.ColorTag(theme.TextSecondaryHex), theme.TagColor, tview.Escape(ticket.Description)))
	}

	return detail.String()
}

// ─── Ticket Actions ──────────────────────────────────────────────────────────

func (v *TeamBoardView) selectedTicketID() string {
	if v.focusCol < 0 || v.focusCol >= len(v.columnCards) {
		return ""
	}
	cc := v.columnCards[v.focusCol]
	if cc.GetItemCount() == 0 {
		return ""
	}
	idx := cc.GetCurrentItem()
	if idx < 0 {
		return ""
	}
	// Use the ticketIDLookup instead of secondary text (which now holds display info)
	if v.focusCol < len(v.ticketIDLookup) && idx < len(v.ticketIDLookup[v.focusCol]) {
		return v.ticketIDLookup[v.focusCol][idx]
	}
	return ""
}

// selectedTeamTicket returns the full TeamTicket of the currently selected card.
func (v *TeamBoardView) selectedTeamTicket() (*TeamTicket, bool) {
	id := v.selectedTicketID()
	if id == "" {
		return nil, false
	}
	for i := range v.allTickets {
		if v.allTickets[i].ID == id {
			return &v.allTickets[i], true
		}
	}
	return nil, false
}

// showTeamBoardQuickActions opens the quick action modal for the selected ticket.
func (v *TeamBoardView) showTeamBoardQuickActions() {
	if v.shell == nil || v.cfg.QuickActions == nil || len(v.columnCards) == 0 {
		return
	}

	ticket, ok := v.selectedTeamTicket()
	if !ok {
		return
	}

	tc := TicketContext{
		ID:          ticket.ID,
		Title:       ticket.Title,
		Description: ticket.Description,
		Project:     ticket.Project,
	}

	// Resolve the project path from the ticket's team-state project directory ID.
	if v.cfg.QuickActions.ResolveProjectByDirID != nil && ticket.Project != "" {
		if projectID, projectPath, ok := v.cfg.QuickActions.ResolveProjectByDirID(ticket.Project); ok {
			tc.ProjectPath = projectPath
			tc.ProjectID = projectID
		}
	}

	showQuickActionModal(v.shell, tc, v.cfg.QuickActions)
}

// ContextCommands implements CommandProvider — injects ticket-specific commands
// into the omnibar when the team board view is active and a ticket is selected.
func (v *TeamBoardView) ContextCommands() []ContextCommand {
	if v.cfg.QuickActions == nil {
		return nil
	}

	ticket, ok := v.selectedTeamTicket()
	if !ok {
		return nil
	}

	makeAction := func(action QuickActionType) func() {
		return func() {
			tc := TicketContext{
				ID:          ticket.ID,
				Title:       ticket.Title,
				Description: ticket.Description,
				Project:     ticket.Project,
			}
			if v.cfg.QuickActions.ResolveProjectByDirID != nil && ticket.Project != "" {
				if pid, ppath, ok := v.cfg.QuickActions.ResolveProjectByDirID(ticket.Project); ok {
					tc.ProjectPath = ppath
					tc.ProjectID = pid
				}
			}
			if action == QuickActionAudit {
				showAuditSubMenu(v.shell, tc, v.cfg.QuickActions)
			} else {
				showLaunchEnvModal(v.shell, action, "", tc, v.cfg.QuickActions)
			}
		}
	}

	id := ticket.ID
	return []ContextCommand{
		{ID: "team.board.review." + id, Label: "Review " + id, Aliases: []string{"review", "code review"}, Description: "Code review du ticket", Category: "Actions", Action: makeAction(QuickActionReview), RunsDirect: true},
		{ID: "team.board.dev." + id, Label: "Dev " + id, Aliases: []string{"dev", "develop"}, Description: "Session dev sur le ticket", Category: "Actions", Action: makeAction(QuickActionDev), RunsDirect: true},
		{ID: "team.board.audit." + id, Label: "Audit " + id, Aliases: []string{"audit"}, Description: "Audit du ticket", Category: "Actions", Action: makeAction(QuickActionAudit), RunsDirect: true},
		{ID: "team.board.debug." + id, Label: "Debug " + id, Aliases: []string{"debug"}, Description: "Debug du ticket", Category: "Actions", Action: makeAction(QuickActionDebug), RunsDirect: true},
	}
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
		if text == "" {
			// Empty search clears all filters (replaces Esc clear-filter behavior)
			v.clearFilters()
		} else {
			v.repopulateWithFilters()
			v.shell.ShowToastMsg("Filtre: \""+text+"\" (/ vide pour effacer)", true)
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

	// Reset ticket ID lookup
	v.ticketIDLookup = make([][]string, len(v.columnCards))
	for i := range v.ticketIDLookup {
		v.ticketIDLookup[i] = nil
	}

	for _, cc := range v.columnCards {
		cc.Clear()
	}
	for _, t := range filtered {
		for i, col := range columns {
			if t.Status == col.Status {
				// ── Line 1: [Project] Title... ──
				projectTag := ""
				projectDisplay := t.ProjectName
				if projectDisplay == "" {
					projectDisplay = t.Project
				}
				if projectDisplay != "" {
					projectTag = fmt.Sprintf("[black:%s] %s [-:-] ",
						theme.AccentHex, tview.Escape(projectDisplay))
				}
				title := t.Title
				titleRunes := []rune(title)
				if len(titleRunes) > 40 {
					title = string(titleRunes[:37]) + "..."
				}
				mainText := projectTag + title

				// ── Line 2: ID · priority ──
				secondary := t.ID
				if t.Priority != "" {
					secondary += " · " + t.Priority
				}

				// ── Line 3: @assignee · labels (filtered) ──
				var parts []string
				if t.Assignee != "" {
					parts = append(parts, widgets.ColorTag(theme.Accent)+"@"+t.Assignee+"[-]")
				}
				for _, l := range t.Labels {
					if v.cfg.LabelStatusMapping != nil {
						if _, isWorkflow := v.cfg.LabelStatusMapping[l]; isWorkflow {
							continue
						}
					}
					parts = append(parts, "[gray]"+tview.Escape(l)+"[-]")
				}
				meta := ""
				if len(parts) > 0 {
					meta = strings.Join(parts, " · ")
				}

				v.columnCards[i].AddCard(widgets.Card{
					MainText:      mainText,
					SecondaryText: secondary,
					MetaText:      meta,
				})
				v.ticketIDLookup[i] = append(v.ticketIDLookup[i], t.ID)
				break
			}
		}
	}
}
