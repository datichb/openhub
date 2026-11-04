package views

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/textfmt"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
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
	// ProjectPath returns the active project path for bd show calls.
	// If nil, ticket detail is disabled.
	ProjectPath func() string
	// CheckInitialized is called on each Mount to determine if beads is set up.
	// Returning false shows the "not configured" invite screen instead of columns.
	// If nil, the board always assumes it is initialized (backward compat).
	CheckInitialized func() bool
	// OnInitBeads is called when the user requests to initialize beads.
	// If nil, the [i] key and the init action are disabled.
	OnInitBeads func()
	// OnLinkTracker is called when the user presses 'L' to link a ticket to a
	// tracker reference. Receives ticketID and externalRef (ADR-032).
	// If nil, the link action is disabled.
	OnLinkTracker func(ticketID, externalRef string) error
	// QuickActions provides callbacks for launching agent sessions from the board.
	// If nil, the 'a' key quick-action feature is disabled.
	QuickActions *BoardQuickActions
}

// BoardView implements View for the kanban board.
type BoardView struct {
	cfg         BoardViewConfig
	columnFlex  *tview.Flex
	columnCards []*widgets.CardColumn
	// ticketsByID maps ticket ID → BoardTicket for O(1) lookup on Enter.
	ticketsByID map[string]BoardTicket
	// ticketIDLookup maps [colIndex][itemIndex] → ticket ID for selected-item resolution.
	ticketIDLookup [][]string
	focusCol       int
	app            *tview.Application
	done           chan struct{}
	once           sync.Once
	initialized    bool
	shell          ShellAccess
	content        *tview.Flex     // parent container — stored to swap empty↔board
	emptyTV        *tview.TextView // empty-state placeholder, nil once board is shown

	checkReads  loadSlot // the check that Beads is set up: only the latest is applied
	loads       loadSlot // the reads of the tickets: only the latest is applied
	detailReads loadSlot // the read of the ticket being opened
	descReads   loadSlot // the read of the description for the quick actions
}

var _ View = (*BoardView)(nil)
var _ CommandProvider = (*BoardView)(nil)

// NewBoardView creates a new board view with the given config.
func NewBoardView(cfg BoardViewConfig) *BoardView {
	return &BoardView{cfg: cfg}
}

// SetShell provides the shell reference for toast/modal interactions.
func (v *BoardView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *BoardView) ID() string { return "board" }

// Title returns the display title.
func (v *BoardView) Title() string { return i18n.T("tui.board.title") }

// StatusHints returns keybinding hints.
func (v *BoardView) StatusHints() string {
	if !v.initialized {
		return keys.Hints(boardInitKey.Hint(), "Ctrl+P "+i18n.T("tui.hints.commands"), keys.Back.Hint())
	}
	return keys.Hints(columnsHint(), "j/k "+i18n.T("tui.hints.items"), keys.Open.HintAs(i18n.T("tui.hints.detail")),
		boardActionsKey.Hint(), boardLinkKey.Hint(), keys.Refresh.Hint(), keys.Back.Hint())
}

// Mount builds the kanban board and inserts it into the content panel. Whether
// Beads is set up is checked off the event loop (it runs bd): the board shows
// « loading » until then, with the tickets the view was given.
func (v *BoardView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.content = content
	v.done = make(chan struct{})
	v.ticketsByID = make(map[string]BoardTicket)

	v.initialized = true
	check := v.cfg.CheckInitialized
	if check == nil {
		v.mountBoard(content, app)
		return
	}
	content.AddItem(stateView(stateLoading, ""), 0, 1, true)
	load(app, shellContext(v.shell), &v.checkReads, func(context.Context) (bool, error) {
		return check(), nil
	}, func(ok bool, _ error) {
		if v.content == nil {
			return // unmounted meanwhile
		}
		v.content.Clear()
		v.initialized = ok
		if app != nil {
			app.SetFocus(v.content)
		}
		if !ok {
			v.mountUninitializedScreen(v.content)
		} else {
			v.mountBoard(v.content, app)
		}
		refreshHints(v.shell)
	})
}

// mountBoard builds the columns of the board and starts reading the tickets.
func (v *BoardView) mountBoard(content *tview.Flex, app *tview.Application) {
	columns := DefaultColumns()
	v.columnCards = make([]*widgets.CardColumn, len(columns))
	v.columnFlex = tview.NewFlex()

	for i, col := range columns {
		if i > 0 {
			v.columnFlex.AddItem(newColumnSeparator(), 1, 0, false)
		}
		cc := widgets.NewCardColumn(col.Name, col.Color)
		v.columnCards[i] = cc
		// All columns added with Focus=false; focus is managed by updateColumnFocus.
		v.columnFlex.AddItem(cc, 0, 1, false)
	}
	// When focus returns to the columnFlex (e.g. after closing a modal),
	// redirect it to the currently tracked column.
	v.columnFlex.SetFocusFunc(func() {
		v.updateColumnFocus()
	})

	v.populateColumns(v.cfg.Tickets, columns)

	if len(v.cfg.Tickets) == 0 {
		v.emptyTV = tview.NewTextView().
			SetDynamicColors(true).
			SetTextAlign(tview.AlignCenter)
		v.emptyTV.SetBackgroundColor(theme.BgPanel)
		v.emptyTV.SetText("\n\n  " + stateText(stateEmpty, i18n.T("tui.board.no_tickets")))
		content.AddItem(v.emptyTV, 0, 1, true)
	} else {
		content.AddItem(v.columnFlex, 0, 1, true)
	}

	v.focusCol = 0
	v.updateColumnFocus()

	if v.cfg.RefreshFunc != nil && app != nil {
		rate := v.cfg.RefreshRate
		if rate == 0 {
			rate = 5 * time.Second
		}
		v.reload()
		everyOnLoop(shellContext(v.shell), app, rate, v.done, v.reload)
	}
}

// mountUninitializedScreen renders the "beads not configured" invite screen.
func (v *BoardView) mountUninitializedScreen(content *tview.Flex) {
	accent := func(s string) string { return theme.Paint(theme.ActiveMode.PrimaryHex, s) }
	muted := func(s string) string { return theme.Paint(theme.TextMutedHex, s) }

	var b strings.Builder
	fmt.Fprintf(&b, "\n\n  %s\n\n", accent("⊞ "+i18n.T("tui.board.not_configured")))
	fmt.Fprintf(&b, "  %s\n\n", muted(i18n.T("tui.board.not_configured_desc")))

	if v.cfg.OnInitBeads != nil {
		fmt.Fprintf(&b, "  %s %s  ·  %s %s\n\n", accent(bracketed(boardInitKey.Keys())), i18n.T("tui.board.init_board"), accent("Ctrl+P"), i18n.T("tui.hints.commands"))
		fmt.Fprintf(&b, "  %s\n", muted(i18n.T("tui.board.init_safe")))
	} else {
		fmt.Fprintf(&b, "  %s\n", muted(i18n.T("tui.board.init_terminal")))
	}

	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	tv.SetBackgroundColor(theme.BgPanel)
	tv.SetBorderPadding(1, 0, 2, 2)
	tv.SetText(b.String())

	content.AddItem(tv, 0, 1, true)
}

// Unmount stops the refresh goroutine.
func (v *BoardView) Unmount() {
	v.checkReads.Invalidate()
	v.loads.Invalidate()
	v.detailReads.Invalidate()
	v.descReads.Invalidate()
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
	v.ticketsByID = nil
	v.ticketIDLookup = nil
	v.once = sync.Once{}
}

// HandleKey processes board-specific key events.
func (v *BoardView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if !v.initialized {
		if boardInitKey.Matches(event) && v.cfg.OnInitBeads != nil {
			v.cfg.OnInitBeads()
			return nil
		}
		return event
	}

	switch {
	case keys.Open.Matches(event):
		v.showTicketDetail()
	case event.Key() == tcell.KeyRight, colNextKey.Matches(event):
		v.moveFocus(1)
	case event.Key() == tcell.KeyLeft, colPrevKey.Matches(event):
		v.moveFocus(-1)
	case keys.Refresh.Matches(event):
		v.reload()
	case boardLinkKey.Matches(event):
		v.linkTicketToTracker()
	case boardActionsKey.Matches(event):
		v.showBoardQuickActions()
	default:
		return event
	}
	return nil
}

// reload reads the tickets off the event loop (bd list) and redraws the board.
func (v *BoardView) reload() {
	refresh := v.cfg.RefreshFunc
	if refresh == nil || v.columnCards == nil {
		return
	}
	load(v.app, shellContext(v.shell), &v.loads, func(context.Context) ([]BoardTicket, error) {
		return refresh(), nil
	}, func(tickets []BoardTicket, err error) {
		if v.columnCards == nil || err != nil {
			return // unmounted meanwhile, or the read failed: the board stays
		}
		v.populateColumns(tickets, DefaultColumns())
	})
}

// showTicketDetail shows the full detail of the selected ticket, read with
// bd show off the event loop (the basic card when it cannot be read).
func (v *BoardView) showTicketDetail() {
	if v.shell == nil || len(v.columnCards) == 0 {
		return
	}

	ticketID := v.selectedTicketID()
	if ticketID == "" {
		return
	}

	// Look up cached ticket for the fallback display.
	ticket, ok := v.ticketsByID[ticketID]
	if !ok {
		return
	}

	sh := v.shell
	closeAction := []ModalAction{{Label: i18n.T("tui.board.close"), Callback: nil}}
	path := ""
	if v.cfg.ProjectPath != nil {
		path = v.cfg.ProjectPath()
	}
	load(v.app, shellContext(sh), &v.detailReads, func(context.Context) (*beads.TicketDetail, error) {
		if path == "" {
			return nil, nil
		}
		return beads.Show(path, ticketID)
	}, func(detail *beads.TicketDetail, err error) {
		if err != nil {
			slog.Warn("board: failed to fetch ticket detail", "id", ticketID, "error", err)
		}
		if err == nil && detail != nil {
			sh.ShowScrollableModal(ticketID, formatTicketDetail(detail), closeAction)
			return
		}
		// Fallback: basic info from the BoardTicket.
		sh.ShowScrollableModal(ticketID, formatBoardTicket(ticket), closeAction)
	})
}

// selectedTicketID returns the ticket ID of the currently selected card.
func (v *BoardView) selectedTicketID() string {
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
	if v.focusCol < len(v.ticketIDLookup) && idx < len(v.ticketIDLookup[v.focusCol]) {
		return v.ticketIDLookup[v.focusCol][idx]
	}
	return ""
}

// selectedTicket returns the full BoardTicket of the currently selected card.
func (v *BoardView) selectedTicket() (BoardTicket, bool) {
	id := v.selectedTicketID()
	if id == "" {
		return BoardTicket{}, false
	}
	t, ok := v.ticketsByID[id]
	return t, ok
}

// showBoardQuickActions opens the quick action modal for the selected ticket;
// its description is read off the event loop (best effort).
func (v *BoardView) showBoardQuickActions() {
	if v.shell == nil || v.cfg.QuickActions == nil || len(v.columnCards) == 0 {
		return
	}

	ticket, ok := v.selectedTicket()
	if !ok {
		return
	}

	tc := TicketContext{
		ID:    ticket.ID,
		Title: ticket.Title,
	}
	sh, qa := v.shell, v.cfg.QuickActions
	fetch := qa.FetchDescription
	path := ""
	if v.cfg.ProjectPath != nil {
		path = v.cfg.ProjectPath()
	}
	load(v.app, shellContext(sh), &v.descReads, func(context.Context) (string, error) {
		if fetch == nil || path == "" {
			return "", nil
		}
		return fetch(path, ticket.ID), nil
	}, func(desc string, _ error) {
		tc.Description = desc
		showQuickActionModal(sh, tc, qa)
	})
}

// ContextCommands implements CommandProvider — injects ticket-specific commands
// into the omnibar when the board view is active and a ticket is selected.
func (v *BoardView) ContextCommands() []ContextCommand {
	if v.cfg.QuickActions == nil || !v.initialized {
		return nil
	}

	ticket, ok := v.selectedTicket()
	if !ok {
		return nil
	}

	tc := TicketContext{ID: ticket.ID, Title: ticket.Title}
	if v.cfg.ProjectPath != nil {
		tc.ProjectPath = v.cfg.ProjectPath()
	}
	return ticketContextCommands("board.run.", tc, v.cfg.QuickActions)
}

// linkTicketToTracker prompts for an external ref and links the selected ticket (ADR-032).
func (v *BoardView) linkTicketToTracker() {
	if v.shell == nil || v.cfg.OnLinkTracker == nil || len(v.columnCards) == 0 {
		return
	}

	ticketID := v.selectedTicketID()
	if ticketID == "" {
		return
	}

	// Pre-fill with existing external ref if available.
	current := ""
	if t, ok := v.ticketsByID[ticketID]; ok {
		current = t.ExternalRef
	}

	sh, link, app := v.shell, v.cfg.OnLinkTracker, v.app
	sh.ShowInputModal(i18n.T("tui.board.link_prompt"), current, func(ref string) {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			return
		}
		safego.Go(func() { // an action, not a read: its result is shown even if the board was refreshed
			err := link(ticketID, ref)
			done := func() {
				if err != nil {
					sh.ShowToastMsg(i18n.T("tui.board.link_failed")+err.Error(), false)
					return
				}
				sh.ShowToastMsg(i18n.Tf("tui.board.link_success", ticketID, ref), true)
				v.reload() // to show the updated label
			}
			if app == nil {
				done()
				return
			}
			app.QueueUpdateDraw(done)
		})
	})
}

// formatTicketDetail formats a TicketDetail for the scrollable modal.
func formatTicketDetail(d *beads.TicketDetail) string {
	accent := func(s string) string { return theme.Paint(theme.ActiveMode.PrimaryHex, s) }
	sep := theme.Paint(theme.TextMutedHex, strings.Repeat("─", 40))

	var b strings.Builder

	// ── Header fields ──────────────────────────────────────────────────────
	fmt.Fprintf(&b, "\n")
	field := func(label, value string) {
		if value == "" {
			return
		}
		fmt.Fprintf(&b, "  %s %s\n", accent(fmt.Sprintf("%-14s", label)), tview.Escape(value))
	}
	field(i18n.T("tui.board.field_title"), d.Title)
	field(i18n.T("tui.board.field_status"), d.Status)
	field(i18n.T("tui.board.field_priority"), d.PriorityString())
	field(i18n.T("tui.board.field_type"), d.Type)
	if d.Assignee != "" {
		field(i18n.T("tui.board.field_assignee"), d.Assignee)
	}
	if d.Parent != "" {
		field(i18n.T("tui.board.field_parent"), d.Parent)
	}
	if len(d.Labels) > 0 {
		field(i18n.T("tui.board.field_labels"), strings.Join(d.Labels, ", "))
	}
	if d.ExternalRef != "" {
		field(i18n.T("tui.board.field_external_ref"), d.ExternalRef)
	}
	if d.Estimate > 0 {
		h := d.Estimate / 60
		m := d.Estimate % 60
		est := fmt.Sprintf("%dh%02d", h, m)
		if h == 0 {
			est = fmt.Sprintf("%dmin", m)
		}
		field(i18n.T("tui.board.field_estimate"), est)
	}

	section := func(title, content string) {
		if strings.TrimSpace(content) == "" {
			return
		}
		fmt.Fprintf(&b, "\n%s\n", sep)
		fmt.Fprintf(&b, "  %s\n\n", accent(title))
		for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
			fmt.Fprintf(&b, "  %s\n", tview.Escape(line))
		}
	}

	section(i18n.T("tui.board.section_description"), d.Description)
	section(i18n.T("tui.board.section_acceptance"), d.Acceptance)
	section(i18n.T("tui.board.section_notes"), d.Notes)
	section(i18n.T("tui.board.section_design"), d.Design)
	if d.CloseReason != "" {
		section(i18n.T("tui.board.section_close_reason"), d.CloseReason)
	}

	fmt.Fprintf(&b, "\n")
	return b.String()
}

// formatBoardTicket formats a BoardTicket (fallback when bd show is unavailable).
func formatBoardTicket(t BoardTicket) string {
	var b strings.Builder
	fmt.Fprintf(&b, "\n")
	field := func(label, value string) {
		if value == "" {
			return
		}
		fmt.Fprintf(&b, "  %s %s\n", theme.Paint(theme.ActiveMode.PrimaryHex, fmt.Sprintf("%-14s", label)), tview.Escape(value))
	}
	field(i18n.T("tui.board.field_title"), t.Title)
	field(i18n.T("tui.board.field_status"), t.Status)
	field(i18n.T("tui.board.field_priority"), t.Priority)
	field(i18n.T("tui.board.field_type"), t.Type)
	fmt.Fprintf(&b, "\n")
	return b.String()
}

func (v *BoardView) populateColumns(tickets []BoardTicket, columns []BoardColumnDef) {
	// Guard against concurrent Unmount — columnCards may be nil if the view was
	// navigated away from between the refresh goroutine scheduling and execution.
	if v.columnCards == nil {
		return
	}

	// Transition empty → populated: if we were showing the empty-state placeholder
	// and real tickets arrived, swap the placeholder for the board layout.
	if v.emptyTV != nil && len(tickets) > 0 && v.content != nil {
		v.content.Clear()
		v.content.AddItem(v.columnFlex, 0, 1, true)
		v.emptyTV = nil
	}

	// Reset ticket ID lookup.
	v.ticketIDLookup = make([][]string, len(v.columnCards))
	cards := make([][]widgets.Card, len(v.columnCards))

	// Rebuild lookup map.
	newMap := make(map[string]BoardTicket, len(tickets))
	for _, t := range tickets {
		newMap[t.ID] = t
		for i, col := range columns {
			if t.Status != col.Status {
				continue
			}
			color := priorityColor(t.Priority)
			prefix := priorityPrefix(t.Priority)
			// Line 1 (main): "P1 · title" coloured by priority
			mainText := theme.Paint(color, prefix+tview.Escape(textfmt.Clip(t.Title, 36)))
			// Line 2 (secondary): "ID · type"
			secondary := tview.Escape(fmt.Sprintf("%s · %s", t.ID, t.Type))
			// Line 3 (meta): external ref if present
			meta := ""
			if t.ExternalRef != "" {
				meta = tview.Escape(fmt.Sprintf("← %s", t.ExternalRef))
			}
			cards[i] = append(cards[i], widgets.Card{
				MainText:      mainText,
				SecondaryText: secondary,
				MetaText:      meta,
				Reference:     t.ID,
			})
			v.ticketIDLookup[i] = append(v.ticketIDLookup[i], t.ID)
			break
		}
	}
	// The board is read again every few seconds: the user stays on their
	// card, and the column stays scrolled as it was.
	for i, cc := range v.columnCards {
		cc.ReplaceCards(cards[i])
	}
	v.ticketsByID = newMap
}

func (v *BoardView) moveFocus(delta int) {
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
	v.updateColumnFocus()
}

func (v *BoardView) updateColumnFocus() {
	if v.columnCards == nil {
		return
	}
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
