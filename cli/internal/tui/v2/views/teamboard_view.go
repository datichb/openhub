package views

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/textfmt"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
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
	Tickets []TeamTicket
	// MemberID returns the ID of the member who works on the board (the one of
	// the active team): « Mes tickets » keeps the tickets they hold
	// (TeamTicket.AssigneeID). Nil or empty when unknown: the filter says so
	// instead of showing an empty board.
	MemberID    func() string
	RefreshFunc func() []TeamTicket
	RefreshRate time.Duration
	// SyncFunc performs a git pull on the team-state repo and returns any error.
	// Called asynchronously on Mount and on 'r' keypress before RefreshFunc.
	// If nil, no pull is attempted by the board (behaviour unchanged from before).
	SyncFunc func() error
	// TrackerSyncFunc performs a full tracker API sync (GitLab/Jira).
	// Called on manual refresh ('r' key) BEFORE SyncFunc+RefreshFunc, but NOT
	// on the auto-refresh timer. If nil, only git pull + local refresh is performed.
	TrackerSyncFunc func() error
	// BoardConfig holds the team's custom board column layout.
	// When empty (no custom columns), DefaultColumns() is used.
	BoardConfig teamstate.BoardConfig
	// Layout returns the board columns and the label → column mapping that
	// the last read of the tickets (RefreshFunc) used. The board takes it at
	// each mount and after each read: columns edited meanwhile (the editor of
	// the C key, a colleague's change pulled) appear without restarting oh.
	// It must not do any I/O (it is called on the event loop at mount).
	Layout func() BoardLayout
	// OnBoardConfig opens the editor of the columns and of the labels mapped
	// to them (C), the same as the « Colonnes du board » entry of the Team
	// mode. onSaved is called on the event loop once the editor saved its
	// changes: the board then reads its tickets again with the new columns.
	// Nil: the C key does nothing.
	OnBoardConfig func(onSaved func())
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

// BoardLayout is the layout of the board of the team: its columns and the
// labels that put a ticket in one of them.
type BoardLayout struct {
	Board              teamstate.BoardConfig
	LabelStatusMapping map[string]string
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
	content     *tview.Flex     // parent container — stored to swap empty↔board
	emptyTV     *tview.TextView // empty-state placeholder, nil once board is shown
	actions     *BoardActions

	// Windowed column scroll — only a subset of columns is visible at a time.
	colViewStart int              // index of the first visible column in columnCards
	visibleCols  int              // number of columns visible (auto-detected from terminal width)
	allColumns   []BoardColumnDef // all column definitions for reference

	// ticketIDLookup maps [colIndex][itemIndex] → ticket ID for selected-item resolution.
	// Rebuilt every time populateColumns or repopulateWithFilters is called.
	ticketIDLookup [][]string

	// Filtering state
	allTickets     []TeamTicket // all tickets from RefreshFunc (unfiltered)
	filterText     string       // text search filter (matches ID, title, assignee)
	filterAssignee string       // filter by specific assignee
	filterOwner    ownerFilter  // filter by who holds the ticket: me, or nobody (the pool)
	filterLabel    string       // filter by specific label

	// Project tabs
	projectTabs  []string // project IDs, "" = all: ["", "88280c5b", "a1b2c3d4"], by display name
	projectNames map[string]projectName
	activeTabIdx int
	tabBar       *tview.TextView
	boardLayout  *tview.Flex // vertical flex: tabBar + columnFlex

	// Debouncing: prevents double-press on non-modal actions (claim, release, refresh).
	actionInProgress atomic.Bool

	// Beads summary cache — read off the event loop (ADR-032).
	// Stores a map[string]BeadsSummary keyed by external ref (e.g. "gitlab-693").
	beadsSummary atomic.Value // holds map[string]BeadsSummary

	loaded       bool     // the tickets were read at least once since the mount
	configured   bool     // the team-state is resolved (else the empty state says so)
	loads        loadSlot // the reads of the tickets: only the latest is applied
	summaryReads loadSlot // the reads of the beads summary
	detailReads  loadSlot // the read of the full description of a ticket
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
	hints := keys.Hints(columnsHint(), "j/k "+i18n.T("tui.hints.items"),
		keys.Open.HintAs(i18n.T("tui.hints.detail")), boardActionsKey.Hint(),
		projectsHint(), keys.Filter.Hint(),
		teamClaimKey.Hint(), releaseKey.Hint(),
		teamTransferKey.Hint(), teamStatusKey.Hint(), boardColumnsKey.Hint(), keys.Refresh.Hint())
	if v.filtered() {
		hints += " · " + theme.Paint(theme.WarningHex, i18n.T("tui.hints.filtered"))
	}
	return hints
}

// Mount builds the team board and inserts it into the content panel.
func (v *TeamBoardView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.content = content
	v.done = make(chan struct{})
	v.loaded = false
	v.actionInProgress.Store(false)

	if v.cfg.Layout != nil {
		l := v.cfg.Layout()
		v.cfg.BoardConfig, v.cfg.LabelStatusMapping = l.Board, l.LabelStatusMapping
	}
	columns := ColumnsFromConfig(v.cfg.BoardConfig)
	v.columnFlex = tview.NewFlex()
	v.focusCol, v.colViewStart = 0, 0
	v.setColumns(columns)
	v.rebuildColumnFlex()

	// When focus returns to the columnFlex (e.g. after closing a modal),
	// redirect it to the currently tracked column.
	v.columnFlex.SetFocusFunc(func() {
		v.updateColumnFocus()
	})

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

	// Empty-state: show a helpful message when no tickets are available (ADR-032).
	// The boardLayout is ALWAYS added to content so that async refresh (git pull)
	// can populate it later. The empty-state message is shown on top and swapped
	// out by populateColumns once real tickets arrive.
	hasTickets := len(v.cfg.Tickets) > 0
	isConfigured := v.cfg.IsConfigured == nil || v.cfg.IsConfigured()
	v.configured = isConfigured

	switch {
	case !hasTickets && !isConfigured:
		v.emptyTV = tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter)
		v.emptyTV.SetBackgroundColor(theme.BgPanel)
		v.emptyTV.SetText("\n\n" + tagged(theme.WarningHex, i18n.T("tui.teamboard.empty_no_team")) +
			"\n\n" + i18n.Tf("tui.teamboard.empty_no_team_hint", tagged(theme.TextPrimaryHex, "Team Init")))
		content.AddItem(v.emptyTV, 0, 1, true)
	case !hasTickets:
		v.emptyTV = tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter)
		v.emptyTV.SetBackgroundColor(theme.BgPanel)
		v.setNoTicketsText()
		content.AddItem(v.emptyTV, 0, 1, true)
	default:
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
			v.reload(nil)
		})
		if app != nil {
			everyOnLoop(shellContext(v.shell), app, rate, v.done, v.refreshIfIdle)
		}
	}

	// The beads summary: its own read, at a slower interval (ADR-032).
	if v.cfg.BeadsSummaryFunc != nil {
		v.loadSummary()
		if app != nil {
			everyOnLoop(shellContext(v.shell), app, 30*time.Second, v.done, v.loadSummary)
		}
	}
}

// setColumns replaces the columns of the board: one card column each, the
// focus and the window of visible columns kept inside them.
func (v *TeamBoardView) setColumns(columns []BoardColumnDef) {
	v.allColumns = columns
	v.columnCards = make([]*widgets.CardColumn, len(columns))
	for i, col := range columns {
		v.columnCards[i] = widgets.NewCardColumn(tview.Escape(col.Name), col.Color)
	}
	// Default to 4 visible columns.
	v.visibleCols = min(4, len(columns))
	v.focusCol = max(0, min(v.focusCol, len(columns)-1))
	if v.focusCol < v.colViewStart {
		v.colViewStart = v.focusCol
	}
	if v.focusCol >= v.colViewStart+v.visibleCols {
		v.colViewStart = v.focusCol - v.visibleCols + 1
	}
	v.colViewStart = max(0, min(v.colViewStart, len(columns)-v.visibleCols))
}

// applyLayout takes the layout the last read of the tickets used: the
// mapping of the labels, and, when the columns changed, the new columns
// (rebuilt, the focus kept as near as it can be).
func (v *TeamBoardView) applyLayout(l BoardLayout) {
	v.cfg.LabelStatusMapping = l.LabelStatusMapping
	columns := ColumnsFromConfig(l.Board)
	if slices.Equal(v.allColumns, columns) {
		v.cfg.BoardConfig = l.Board
		return
	}
	v.cfg.BoardConfig = l.Board
	v.setColumns(columns)
	v.rebuildColumnFlex()
	v.updateColumnFocus()
}

// editColumns is C: the editor of the columns and of the labels mapped to
// them; once it saved, the board reads its tickets again.
func (v *TeamBoardView) editColumns() {
	if v.cfg.OnBoardConfig == nil || v.actionInProgress.Load() {
		return
	}
	v.cfg.OnBoardConfig(func() { v.reload(nil) })
}

// setNoTicketsText says, in the empty-state placeholder, that the board has no
// ticket: « loading » while the first read is on its way.
func (v *TeamBoardView) setNoTicketsText() {
	if v.emptyTV == nil || !v.configured {
		return
	}
	if v.cfg.RefreshFunc != nil && !v.loaded {
		v.emptyTV.SetText("\n\n" + stateText(stateLoading, ""))
		return
	}
	v.emptyTV.SetText("\n\n" + tagged(theme.WarningHex, i18n.T("tui.teamboard.empty_no_tickets")) +
		"\n\n" + i18n.Tf("tui.teamboard.empty_no_tickets_hint", tagged(theme.TextPrimaryHex, i18n.T("tui.cmd.sync_tracker")),
		theme.StyleTag(theme.AttrBold)+teamRefreshHint()+theme.TagAttrs))
}

// teamRefreshHint is the key that refreshes the board, as the hint of the empty
// state names it.
func teamRefreshHint() string { return keys.Refresh.Keys() }

// Unmount stops the refresh goroutine.
func (v *TeamBoardView) Unmount() {
	v.loads.Invalidate()
	v.summaryReads.Invalidate()
	v.detailReads.Invalidate()
	v.actionInProgress.Store(false)
	v.once.Do(func() {
		if v.done != nil {
			close(v.done)
		}
	})
	v.app = nil
	v.content = nil
	v.emptyTV = nil
	v.columnFlex = nil
	v.columnCards = nil
	v.once = sync.Once{}
}

// HandleKey processes team board key events.
func (v *TeamBoardView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case keys.Open.Matches(event):
		v.showTicketDetail()
	case event.Key() == tcell.KeyRight, colNextKey.Matches(event):
		v.moveFocus(1)
	case event.Key() == tcell.KeyLeft, colPrevKey.Matches(event):
		v.moveFocus(-1)
	case projectPrevKey.Matches(event):
		v.prevProjectTab()
	case projectNextKey.Matches(event):
		v.nextProjectTab()
	case keys.Refresh.Matches(event):
		v.manualRefresh()
	case keys.Filter.Matches(event):
		v.showFilterMenu()
	case teamClaimKey.Matches(event):
		v.claimTicket()
	case releaseKey.Matches(event):
		v.releaseTicket()
	case teamTransferKey.Matches(event):
		v.transferTicket()
	case teamStatusKey.Matches(event):
		v.changeStatus()
	case boardColumnsKey.Matches(event):
		v.editColumns()
	case boardActionsKey.Matches(event):
		v.showTeamBoardQuickActions()
	default:
		return event
	}
	return nil
}

// manualRefresh is r: tracker sync (API), git pull, then a new read of the
// tickets. TrackerSyncFunc may be nil (no tracker configured).
func (v *TeamBoardView) manualRefresh() {
	if v.cfg.RefreshFunc == nil || v.actionInProgress.Load() {
		return
	}
	v.actionInProgress.Store(true)
	trackerSync := v.cfg.TrackerSyncFunc
	if trackerSync == nil {
		trackerSync = func() error { return nil }
	}
	syncFuncAsync(v.app, trackerSync, v.shell, func(_ error) {
		// Git pull to fetch our tracker-sync commits + colleagues' changes.
		syncFuncAsync(v.app, v.cfg.SyncFunc, v.shell, func(_ error) {
			v.reload(func() { v.actionInProgress.Store(false) })
		})
	})
}

// refreshIfIdle is the timer's refresh: it leaves the board alone while an
// action is on its way, so that the data it brings is not overwritten by older
// data.
func (v *TeamBoardView) refreshIfIdle() {
	if v.actionInProgress.Load() {
		return
	}
	v.reload(nil)
}

// reload reads the tickets off the event loop and redraws the board, then
// calls done (on the loop). Only the latest read is applied: done is not
// called for one that a newer read or an unmount dropped.
func (v *TeamBoardView) reload(done func()) {
	refresh := v.cfg.RefreshFunc
	if refresh == nil || v.columnCards == nil {
		if done != nil {
			done()
		}
		return
	}
	layout := v.cfg.Layout
	load(v.app, shellContext(v.shell), &v.loads, func(context.Context) (boardRead, error) {
		r := boardRead{tickets: refresh()}
		if layout != nil {
			l := layout()
			r.layout = &l
		}
		return r, nil
	}, func(r boardRead, err error) {
		if v.columnCards == nil {
			return // unmounted meanwhile
		}
		if err == nil {
			v.loaded = true
			if r.layout != nil {
				v.applyLayout(*r.layout)
			}
			v.populateColumns(r.tickets, v.allColumns)
			v.setNoTicketsText()
		}
		if done != nil {
			done()
		}
	})
}

// boardRead is what a read of the board brings: the tickets, and the layout
// they were mapped to the columns with (nil when the board has no Layout).
type boardRead struct {
	tickets []TeamTicket
	layout  *BoardLayout
}

// loadSummary reads the beads summary (bd runs) off the event loop, then
// redraws the cards with their badges.
func (v *TeamBoardView) loadSummary() {
	fetch := v.cfg.BeadsSummaryFunc
	if fetch == nil || v.columnCards == nil {
		return
	}
	load(v.app, shellContext(v.shell), &v.summaryReads, func(context.Context) (map[string]BeadsSummary, error) {
		return fetch(), nil
	}, func(summary map[string]BeadsSummary, err error) {
		if v.columnCards == nil || err != nil || summary == nil {
			return
		}
		v.beadsSummary.Store(summary)
		v.repopulateWithFilters()
	})
}

// buildTeamCard builds a single card for a team ticket. Centralizes the rendering
// logic so that populateColumns and repopulateWithFilters produce identical output.
func (v *TeamBoardView) buildTeamCard(t TeamTicket) widgets.Card {
	// ── Line 1: [Project] Title... [N/M] ──
	projectTag := ""
	projectDisplay := t.ProjectName
	if projectDisplay == "" {
		projectDisplay = t.Project
	}
	if projectDisplay != "" {
		projectTag = projectChip(projectDisplay) + " "
	}
	// Truncate title to keep line 1 readable
	title := textfmt.Clip(t.Title, 40)
	// Beads badge: [done/total] when linked beads exist (ADR-032)
	badgeStr := ""
	if summary := v.getBeadsSummary(); summary != nil {
		if bs, ok := summary[t.ID]; ok && bs.Total > 0 {
			color := theme.TextMutedHex
			if bs.Done == bs.Total {
				color = theme.SuccessHex
			}
			badgeStr = " " + theme.Paint(color, bracketed(fmt.Sprintf("%d/%d", bs.Done, bs.Total)))
		}
	}
	mainText := projectTag + tview.Escape(title) + badgeStr

	// ── Line 2: ID · priority ──
	secondary := t.ID
	if t.Priority != "" {
		secondary += " · " + t.Priority
	}

	// ── Line 3: @assignee · label1 · label2 (workflow labels filtered) ──
	parts := []string{formatAssignee(t.Assignee, theme.ColorTagOf(theme.ActiveMode.Primary))}
	parts = append(parts, ticketLabelParts(t.Labels, v.cfg.LabelStatusMapping)...)
	meta := strings.Join(parts, " · ")

	return widgets.Card{
		MainText:      mainText,
		SecondaryText: secondary,
		MetaText:      meta,
	}
}

func (v *TeamBoardView) populateColumns(tickets []TeamTicket, columns []BoardColumnDef) {
	// Defense-in-depth: if Unmount ran between the caller's check and this call,
	// bail out to avoid index-out-of-range on v.columnCards.
	if v.columnCards == nil {
		return
	}
	v.allTickets = tickets

	// Transition empty → populated: if we were showing the empty-state placeholder
	// and real tickets arrived, swap the placeholder for the board layout.
	if v.emptyTV != nil && len(tickets) > 0 && v.content != nil {
		v.content.Clear()
		v.content.AddItem(v.boardLayout, 0, 1, true)
		v.emptyTV = nil
	}

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
			if t.Status != col.Status {
				continue
			}
			v.columnCards[i].AddCard(v.buildTeamCard(t))
			v.ticketIDLookup[i] = append(v.ticketIDLookup[i], t.ID)
			break
		}
	}
}

// ownerFilter keeps the tickets by who holds them.
type ownerFilter int

const (
	ownerAny  ownerFilter = iota
	ownerMine             // the tickets of the member who works on the board
	ownerPool             // the tickets nobody holds, to take with the take key
)

// memberID is the member who works on the board ("" when unknown).
func (v *TeamBoardView) memberID() string {
	if v.cfg.MemberID == nil {
		return ""
	}
	return v.cfg.MemberID()
}

// filtered reports whether a filter is active on the board.
func (v *TeamBoardView) filtered() bool {
	return v.filterText != "" || v.filterAssignee != "" || v.filterLabel != "" || v.filterOwner != ownerAny
}

// applyFilters returns the subset of tickets matching all active filters.
func (v *TeamBoardView) applyFilters(tickets []TeamTicket) []TeamTicket {
	if !v.filtered() {
		return tickets
	}

	var result []TeamTicket
	textLower := strings.ToLower(v.filterText)

	for _, t := range tickets {
		// Assignee filter
		if v.filterAssignee != "" && t.Assignee != v.filterAssignee {
			continue
		}
		// Owner filter: the member's own tickets, or the pool
		switch v.filterOwner {
		case ownerMine:
			if me := v.memberID(); t.AssigneeID == "" || t.AssigneeID != me {
				continue
			}
		case ownerPool:
			if t.AssigneeID != "" {
				continue
			}
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

// formatAssignee renders "@assignee" in the given color tag, or the
// "claimable" marker when nobody holds the ticket. Both texts are escaped: a
// name like "[dev]" would otherwise be read as a color tag and vanish.
func formatAssignee(assignee, colorTag string) string {
	if assignee == "" {
		return theme.Paint(theme.WarningHex, tview.Escape(i18n.T("board.claimable")))
	}
	return colorTag + "@" + tview.Escape(assignee) + theme.TagColor
}

// ticketLabelParts renders one markup chunk per ticket label, shared by both
// team boards. Workflow labels (keys of workflow, already shown by the column)
// are skipped. Known labels get a badge, the others are shown in brackets;
// literal brackets are escaped the tview way ("[AI[]" displays "[AI]").
func ticketLabelParts(labels []string, workflow map[string]string) []string {
	var parts []string
	for _, l := range labels {
		if _, isWorkflow := workflow[l]; isWorkflow {
			continue
		}
		switch l {
		case teamstate.LabelAgentReviewed:
			parts = append(parts, theme.Paint(theme.SuccessHex, bracketed("AI")))
		case teamstate.LabelNeedsHumanReview:
			parts = append(parts, theme.Paint(theme.WarningHex, bracketed("!")))
		default:
			parts = append(parts, theme.Paint(theme.TextMutedHex, bracketed(l)))
		}
	}
	return parts
}

// ─── Project Tabs ────────────────────────────────────────────────────────────

// projectName is how a project shows on its tab.
type projectName struct {
	// Name is the name of the project, or its ID when no name is known.
	Name string
	// Unknown: no local project has this ID (the resolver gave the ID back).
	Unknown bool
}

// buildProjectTabs lists the projects of the tickets, one tab each: the tab
// shows the name of the project and filters on its ID. The tabs are sorted by
// name (then ID), so that they do not move from one read to the next.
func (v *TeamBoardView) buildProjectTabs(tickets []TeamTicket) {
	names := make(map[string]projectName)
	for _, t := range tickets {
		if t.Project == "" {
			continue
		}
		if _, ok := names[t.Project]; ok {
			continue
		}
		pn := projectName{Name: t.ProjectName}
		switch pn.Name {
		case "":
			pn.Name = t.Project
		case t.Project:
			pn.Unknown = true
		}
		names[t.Project] = pn
	}
	ids := make([]string, 0, len(names))
	for id := range names {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		a, b := strings.ToLower(names[ids[i]].Name), strings.ToLower(names[ids[j]].Name)
		if a != b {
			return a < b
		}
		return ids[i] < ids[j]
	})
	// Only show tabs if there are 2+ projects (or 1+ for the all tab to be useful)
	v.projectTabs = append([]string{""}, ids...) // "" = all
	v.projectNames = names
	if v.activeTabIdx >= len(v.projectTabs) {
		v.activeTabIdx = 0
	}
}

// tabLabel is the text of a tab: « All », the name of the project, or its ID
// marked as the ID of a project this machine does not know.
func (v *TeamBoardView) tabLabel(id string) string {
	if id == "" {
		return i18n.T("tui.board.tab_all")
	}
	pn, ok := v.projectNames[id]
	switch {
	case !ok:
		return id
	case pn.Unknown:
		return i18n.Tf("tui.teamboard.project_unknown", pn.Name)
	}
	return pn.Name
}

// renderTabBar renders the project tabs in the tab bar widget.
func (v *TeamBoardView) renderTabBar() {
	if v.tabBar == nil {
		return
	}
	if len(v.projectTabs) <= 2 {
		// Hide the tabs when only 1 project (or none) — no need to filter
		v.tabBar.SetText("")
		return
	}

	var sb strings.Builder
	sb.WriteString(" ")
	for i, tab := range v.projectTabs {
		label := v.tabLabel(tab)
		if i == v.activeTabIdx {
			// Active tab: accent background
			sb.WriteString(projectChip(label))
		} else {
			// Inactive tab: muted
			sb.WriteString(theme.Paint(theme.TextMutedHex, " "+tview.Escape(label)+" "))
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
	v.filtersChanged()
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
	v.filtersChanged()
}

func (v *TeamBoardView) moveFocus(delta int) {
	if len(v.columnCards) == 0 {
		return
	}
	// Preserve vertical selection when moving horizontally.
	prevSelected := 0
	if v.focusCol >= 0 && v.focusCol < len(v.columnCards) {
		prevSelected = v.columnCards[v.focusCol].GetCurrentItem()
		if prevSelected < 0 {
			prevSelected = 0
		}
	}
	v.focusCol += delta
	if v.focusCol < 0 {
		v.focusCol = 0
	}
	if v.focusCol >= len(v.columnCards) {
		v.focusCol = len(v.columnCards) - 1
	}
	// Apply the same vertical position (clamped) to the target column.
	if cc := v.columnCards[v.focusCol]; cc.GetItemCount() > 0 {
		cc.SetCurrentItem(prevSelected)
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
		v.columnFlex.AddItem(v.columnCards[i], 0, 1, false)
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

	// Build actions: always "Close"; add "Full description" if FetchDetail is available.
	closeLabel := i18n.T("tui.teamboard.close")
	actions := []ModalAction{{Label: closeLabel, Callback: nil}}
	if v.actions != nil && v.actions.FetchDetail != nil {
		actions = []ModalAction{
			{Label: i18n.T("tui.teamboard.full_description"), Callback: func() {
				// Fetch full description on-demand from tracker, off the event loop.
				fetch, sh := v.actions.FetchDetail, v.shell
				type fullDetail struct{ title, desc string }
				load(v.app, shellContext(sh), &v.detailReads, func(context.Context) (fullDetail, error) {
					title, desc, err := fetch(ticket.Project, ticket.ID)
					return fullDetail{title, desc}, err
				}, func(d fullDetail, err error) {
					if err != nil {
						sh.ShowToastMsg(i18n.T("tui.teamboard.error_prefix")+err.Error(), false)
						return
					}
					// Update cached ticket data with fresh info.
					if d.title != "" {
						ticket.Title = d.title
					}
					if d.desc != "" {
						ticket.Description = d.desc
					}
					// Re-display with full description.
					sh.ShowScrollableModal(i18n.T("tui.teamboard.ticket_title")+ticket.ID, v.formatTicketDetail(ticket), []ModalAction{{Label: closeLabel, Callback: nil}})
				})
			}},
			{Label: closeLabel, Callback: nil},
		}
	}

	v.shell.ShowScrollableModal(i18n.T("tui.teamboard.ticket_title")+ticket.ID, detail, actions)
}

// formatTicketDetail builds the detail content string for a ticket.
func (v *TeamBoardView) formatTicketDetail(ticket *TeamTicket) string {
	var detail strings.Builder
	label := func(s string) string { return theme.Paint(theme.TextSecondaryHex, s) }
	fmt.Fprintf(&detail, "\n  %s%s%s\n\n", theme.StyleTag(theme.AttrBold), tview.Escape(ticket.Title), theme.TagAttrs)
	fmt.Fprintf(&detail, "  %s         %s\n", label("ID:"), tview.Escape(ticket.ID))
	if ticket.Project != "" {
		fmt.Fprintf(&detail, "  %s     %s\n", label(i18n.T("tui.teamboard.detail.project")), tview.Escape(projectLabel(ticket.Project, ticket.ProjectName)))
	}
	fmt.Fprintf(&detail, "  %s     %s\n", label(i18n.T("tui.teamboard.detail.status")), tview.Escape(ticket.Status))

	assignee := "-"
	if ticket.Assignee != "" {
		assignee = "@" + ticket.Assignee
	}
	fmt.Fprintf(&detail, "  %s   %s\n", label("Assignee:"), tview.Escape(assignee))

	if ticket.Priority != "" {
		fmt.Fprintf(&detail, "  %s   %s\n", label(i18n.T("tui.teamboard.detail.priority")), tview.Escape(ticket.Priority))
	}

	if len(ticket.Labels) > 0 {
		fmt.Fprintf(&detail, "  %s     %s\n", label("Labels:"), tview.Escape(strings.Join(ticket.Labels, ", ")))
	}

	if ticket.MRURL != "" {
		fmt.Fprintf(&detail, "  %s         %s\n", label("MR:"), tview.Escape(ticket.MRURL))
	}

	if ticket.Description != "" {
		fmt.Fprintf(&detail, "\n  %s\n  %s\n", label("Description:"), tview.Escape(ticket.Description))
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

	tc := TicketContext{ID: ticket.ID, Title: ticket.Title, Description: ticket.Description, Project: ticket.Project}
	if v.cfg.QuickActions.ResolveProjectByDirID != nil && ticket.Project != "" {
		if pid, ppath, ok := v.cfg.QuickActions.ResolveProjectByDirID(ticket.Project); ok {
			tc.ProjectPath, tc.ProjectID = ppath, pid
		}
	}
	return ticketContextCommands("team.board.run.", tc, v.cfg.QuickActions)
}

func (v *TeamBoardView) claimTicket() {
	if v.actions == nil || v.actions.OnClaim == nil || v.shell == nil {
		return
	}
	if v.actionInProgress.Load() {
		return
	}
	ticketID := v.selectedTicketID()
	if ticketID == "" {
		return
	}
	v.actionInProgress.Store(true)
	onClaim, app, ctx := v.actions.OnClaim, v.app, shellContext(v.shell)
	safego.Go(func() { // an action, not a read: its result is shown even if the board was refreshed
		err := onClaim(ticketID)
		if app == nil {
			return
		}
		queueIfLive(ctx, app, func() {
			v.actionInProgress.Store(false)
			if err != nil {
				v.shell.ShowToastMsg(i18n.T("tui.teamboard.claim_failed")+err.Error(), false)
			} else {
				v.shell.ShowToastMsg(i18n.T("tui.teamboard.claim_success")+ticketID, true)
				if v.cfg.RefreshFunc != nil {
					v.reload(nil)
				}
			}
		})
	})
}

func (v *TeamBoardView) releaseTicket() {
	if v.actions == nil || v.actions.OnRelease == nil || v.shell == nil {
		return
	}
	if v.actionInProgress.Load() {
		return
	}
	ticketID := v.selectedTicketID()
	if ticketID == "" {
		return
	}
	v.shell.Confirm(i18n.T("tui.confirm.release.title"), i18n.Tf("tui.confirm.release.detail", ticketID), true, func() {
		v.doReleaseTicket(ticketID)
	})
}

func (v *TeamBoardView) doReleaseTicket(ticketID string) {
	if v.actionInProgress.Load() {
		return
	}
	v.actionInProgress.Store(true)
	onRelease, app, ctx := v.actions.OnRelease, v.app, shellContext(v.shell)
	safego.Go(func() { // an action, not a read: its result is shown even if the board was refreshed
		err := onRelease(ticketID)
		if app == nil {
			return
		}
		queueIfLive(ctx, app, func() {
			v.actionInProgress.Store(false)
			if err != nil {
				v.shell.ShowToastMsg(i18n.T("tui.teamboard.release_failed")+err.Error(), false)
			} else {
				v.shell.ShowToastMsg(i18n.T("tui.teamboard.release_success")+ticketID, true)
				if v.cfg.RefreshFunc != nil {
					v.reload(nil)
				}
			}
		})
	})
}

func (v *TeamBoardView) transferTicket() {
	if v.actions == nil || v.actions.OnTransfer == nil || v.actions.Members == nil || v.shell == nil {
		return
	}
	if v.actionInProgress.Load() {
		return
	}
	ticketID := v.selectedTicketID()
	if ticketID == "" {
		return
	}
	members := v.actions.Members()
	if len(members) == 0 {
		v.shell.ShowToastMsg(i18n.T("tui.teamboard.no_members"), false)
		return
	}
	v.actionInProgress.Store(true)
	v.shell.ShowSelectModal(i18n.Tf("tui.teamboard.transfer_title", ticketID), members, "", func(toMember string) {
		app, ctx := v.app, shellContext(v.shell)
		toName := toMember // the toast names the member as the list did
		for _, m := range members {
			if m.Value == toMember {
				toName = m.Label
			}
		}
		safego.Go(func() { // an action, not a read: its result is shown even if the board was refreshed
			err := v.actions.OnTransfer(ticketID, toMember)
			if app == nil {
				return
			}
			queueIfLive(ctx, app, func() {
				v.actionInProgress.Store(false)
				if err != nil {
					v.shell.ShowToastMsg(i18n.T("tui.teamboard.transfer_failed")+err.Error(), false)
				} else {
					v.shell.ShowToastMsg(i18n.T("tui.teamboard.transfer_success")+toName, true)
					if v.cfg.RefreshFunc != nil {
						v.reload(nil)
					}
				}
			})
		})
	})
}

func (v *TeamBoardView) changeStatus() {
	if v.actions == nil || v.actions.OnStatus == nil || v.shell == nil {
		return
	}
	if v.actionInProgress.Load() {
		return
	}
	ticketID := v.selectedTicketID()
	if ticketID == "" {
		return
	}

	statusOptions := make([]SelectOption, len(v.allColumns))
	for i, col := range v.allColumns {
		statusOptions[i] = SelectOption{Label: col.Name, Value: col.Status}
	}

	// Pre-select the ticket's current status so the user sees it highlighted.
	currentStatus := ""
	if ticket, ok := v.selectedTeamTicket(); ok {
		currentStatus = ticket.Status
	}

	v.actionInProgress.Store(true)
	v.shell.ShowSelectModal(i18n.Tf("tui.teamboard.status_title", ticketID), statusOptions, currentStatus, func(newStatus string) {
		app, ctx := v.app, shellContext(v.shell)
		safego.Go(func() { // an action, not a read: its result is shown even if the board was refreshed
			err := v.actions.OnStatus(ticketID, newStatus)
			if app == nil {
				return
			}
			queueIfLive(ctx, app, func() {
				v.actionInProgress.Store(false)
				if err != nil {
					v.shell.ShowToastMsg(i18n.T("tui.teamboard.status_failed")+err.Error(), false)
				} else {
					v.shell.ShowToastMsg(i18n.T("tui.teamboard.status_updated"), true)
					if v.cfg.RefreshFunc != nil {
						v.reload(nil)
					}
				}
			})
		})
	})
}

// ─── Filtering ───────────────────────────────────────────────────────────────

// showSearchFilter opens a text input for live searching tickets.
func (v *TeamBoardView) showSearchFilter() {
	if v.shell == nil {
		return
	}
	v.shell.ShowInputModal(i18n.T("tui.teamboard.search_prompt"), v.filterText, func(text string) {
		v.filterText = text
		if text == "" {
			// Empty search clears all filters (replaces Esc clear-filter behavior)
			v.clearFilters()
		} else {
			v.filtersChanged()
			v.shell.ShowToastMsg(i18n.Tf("tui.teamboard.filter_search_applied", text), true)
		}
	})
}

// releaseKey releases the claim of a ticket (L: libérer / release; x never
// destroys anything in the key convention).
var releaseKey = keys.Own("teamboard.release", 'L', "tui.hints.release")

// showFilterMenu is /: the filters of the board (text search first).
func (v *TeamBoardView) showFilterMenu() {
	if v.shell == nil {
		return
	}

	options := []SelectOption{
		{Label: i18n.T("tui.teamboard.filter_text"), Value: "_text"},
		{Label: i18n.T("tui.teamboard.filter_my_tickets"), Value: "_mine"},
		{Label: i18n.T("tui.teamboard.filter_unassigned"), Value: "_pool"},
		{Label: i18n.T("tui.teamboard.filter_by_assignee"), Value: "_assignee"},
		{Label: i18n.T("tui.teamboard.filter_by_label"), Value: "_label"},
		{Label: i18n.T("tui.teamboard.filter_clear"), Value: "_clear"},
	}

	v.shell.ShowSelectModal(i18n.T("tui.teamboard.filter_title"), options, "", func(choice string) {
		switch choice {
		case "_text":
			v.showSearchFilter()
		case "_mine":
			if v.memberID() == "" {
				v.shell.ShowToastMsg(i18n.T("tui.teamboard.filter_mine_unknown"), false)
				return
			}
			v.filterOwner, v.filterAssignee = ownerMine, ""
			v.filtersChanged()
			v.shell.ShowToastMsg(i18n.T("tui.teamboard.filter_mine_applied"), true)
		case "_pool":
			v.filterOwner, v.filterAssignee = ownerPool, ""
			v.filtersChanged()
			v.shell.ShowToastMsg(i18n.T("tui.teamboard.filter_unassigned_applied"), true)
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

	// By displayed name: the order of a map changes at each opening.
	var options []SelectOption
	for a := range assigneeSet {
		options = append(options, SelectOption{Label: "@" + a, Value: a})
	}
	sort.Slice(options, func(i, j int) bool {
		a, b := strings.ToLower(options[i].Value), strings.ToLower(options[j].Value)
		if a != b {
			return a < b
		}
		return options[i].Value < options[j].Value
	})
	if len(options) == 0 {
		v.shell.ShowToastMsg(i18n.T("tui.teamboard.filter_no_assignee"), false)
		return
	}

	v.shell.ShowSelectModal(i18n.T("tui.teamboard.filter_assignee_title"), options, "", func(assignee string) {
		v.filterAssignee, v.filterOwner = assignee, ownerAny
		v.filtersChanged()
		v.shell.ShowToastMsg(i18n.Tf("tui.teamboard.filter_assignee_applied", assignee), true)
	})
}

// showLabelFilter shows an input modal to type a label to filter by.
func (v *TeamBoardView) showLabelFilter() {
	if v.shell == nil {
		return
	}
	v.shell.ShowInputModal(i18n.T("tui.teamboard.filter_label_title"), v.filterLabel, func(label string) {
		v.filterLabel = label
		v.filtersChanged()
		if label != "" {
			v.shell.ShowToastMsg(i18n.Tf("tui.teamboard.filter_label_applied", label), true)
		}
	})
}

// clearFilters removes all active filters and repopulates the board.
func (v *TeamBoardView) clearFilters() {
	v.filterText = ""
	v.filterAssignee = ""
	v.filterOwner = ownerAny
	v.filterLabel = ""
	v.filtersChanged()
	if v.shell != nil {
		v.shell.ShowToastMsg(i18n.T("tui.teamboard.filters_cleared"), true)
	}
}

// filtersChanged draws the board again after a change of project tab or of
// filter: the cards, the tabs (the active one) and the hints line (the
// « filtered » mark), at once and not at the next read of the tickets.
func (v *TeamBoardView) filtersChanged() {
	v.repopulateWithFilters()
	refreshHints(v.shell)
}

// repopulateWithFilters re-renders the board with current filters applied.
func (v *TeamBoardView) repopulateWithFilters() {
	if v.columnCards == nil || len(v.allTickets) == 0 {
		return
	}
	v.renderTabBar()
	columns := v.allColumns
	filtered := v.applyProjectFilter(v.allTickets)
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
			if t.Status != col.Status {
				continue
			}
			v.columnCards[i].AddCard(v.buildTeamCard(t))
			v.ticketIDLookup[i] = append(v.ticketIDLookup[i], t.ID)
			break
		}
	}
}

// projectChip is a project name on the accent background, as the cards and
// the tabs of the board show it.
func projectChip(name string) string {
	return theme.BgTag(theme.ActiveMode.PrimaryHex) + theme.Paint(theme.BgAppHex, " "+tview.Escape(name)+" ") + theme.TagBg
}
