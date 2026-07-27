package views

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/beads"
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
}

// BoardView implements View for the kanban board.
type BoardView struct {
	cfg          BoardViewConfig
	columnFlex   *tview.Flex
	columnLists  []*tview.List
	// ticketsByID maps ticket ID → BoardTicket for O(1) lookup on Enter.
	ticketsByID  map[string]BoardTicket
	focusCol     int
	app          *tview.Application
	done         chan struct{}
	once         sync.Once
	initialized  bool
	shell        ShellAccess
}

var _ View = (*BoardView)(nil)

// NewBoardView creates a new board view with the given config.
func NewBoardView(cfg BoardViewConfig) *BoardView {
	return &BoardView{cfg: cfg}
}

// SetShell provides the shell reference for toast/modal interactions.
func (v *BoardView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *BoardView) ID() string { return "board" }

// Title returns the display title.
func (v *BoardView) Title() string { return "Board" }

// StatusHints returns keybinding hints.
func (v *BoardView) StatusHints() string {
	if !v.initialized {
		return "i initialiser beads · Ctrl+P commandes · Esc retour"
	}
	return "h/l colonnes · j/k items · Enter détail · r refresh · Esc retour"
}

// Mount builds the kanban board and inserts it into the content panel.
func (v *BoardView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.done = make(chan struct{})
	v.ticketsByID = make(map[string]BoardTicket)

	v.initialized = true
	if v.cfg.CheckInitialized != nil {
		v.initialized = v.cfg.CheckInitialized()
	}

	if !v.initialized {
		v.mountUninitializedScreen(content)
		return
	}

	columns := DefaultColumns()
	v.columnLists = make([]*tview.List, len(columns))
	v.columnFlex = tview.NewFlex()

	for i, col := range columns {
		list := tview.NewList().
			ShowSecondaryText(true).
			SetHighlightFullLine(true).
			SetMainTextColor(theme.FgPrimary).
			SetSecondaryTextColor(theme.FgMuted)
		list.SetBackgroundColor(theme.BgPanel)
		list.SetBorder(true)
		list.SetBorderColor(theme.BorderNormal)
		list.SetTitle(" " + col.Name + " ")
		list.SetTitleColor(col.Color)
		list.SetBorderPadding(0, 0, 1, 1)
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
		go v.refreshLoop(rate, columns)
	}
}

// mountUninitializedScreen renders the "beads not configured" invite screen.
func (v *BoardView) mountUninitializedScreen(content *tview.Flex) {
	accent := theme.ColorTag(theme.AccentHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor

	var b strings.Builder
	fmt.Fprintf(&b, "\n\n  %s⊞ Board non configuré%s\n\n", accent, reset)
	fmt.Fprintf(&b, "  %sCe projet n'utilise pas encore le suivi de tickets (beads).%s\n\n", muted, reset)

	if v.cfg.OnInitBeads != nil {
		fmt.Fprintf(&b, "  %s[i]%s Initialiser le board  ·  %sCtrl+P%s commandes\n\n", accent, reset, accent, reset)
		fmt.Fprintf(&b, "  %sLe board sera prêt dès l'initialisation — aucune donnée existante ne sera modifiée.%s\n", muted, reset)
	} else {
		fmt.Fprintf(&b, "  %sUtilisez %soh beads init%s depuis le terminal pour initialiser.%s\n", muted, accent, muted, reset)
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
	v.once.Do(func() {
		if v.done != nil {
			close(v.done)
		}
	})
	v.app = nil
	v.columnFlex = nil
	v.columnLists = nil
	v.ticketsByID = nil
	v.once = sync.Once{}
}

// HandleKey processes board-specific key events.
func (v *BoardView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if !v.initialized {
		if event.Rune() == 'i' && v.cfg.OnInitBeads != nil {
			v.cfg.OnInitBeads()
			return nil
		}
		return event
	}

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
	case 'r':
		if v.cfg.RefreshFunc != nil {
			v.refresh(DefaultColumns())
		}
		return nil
	}

	return event
}

// showTicketDetail fetches and displays the full detail of the selected ticket.
func (v *BoardView) showTicketDetail() {
	if v.shell == nil || len(v.columnLists) == 0 {
		return
	}

	list := v.columnLists[v.focusCol]
	if list == nil {
		return
	}

	// Guard against empty columns — tview returns 0 (not -1) for empty lists.
	if list.GetItemCount() == 0 {
		return
	}

	idx := list.GetCurrentItem()
	if idx < 0 || idx >= list.GetItemCount() {
		return
	}

	// The secondary text format is "  <ID> · <type>".
	// Extract the ID as the first token before " · ".
	_, secondary := list.GetItemText(idx)
	secondary = strings.TrimSpace(secondary)
	ticketID := secondary
	if sep := strings.Index(secondary, " · "); sep >= 0 {
		ticketID = secondary[:sep]
	}
	ticketID = strings.TrimSpace(ticketID)
	if ticketID == "" {
		return
	}

	// Look up cached ticket for immediate display while bd show runs.
	ticket, ok := v.ticketsByID[ticketID]
	if !ok {
		return
	}

	// Fetch full detail from bd show if projectPath is available.
	if v.cfg.ProjectPath != nil {
		path := v.cfg.ProjectPath()
		if path != "" {
			detail, err := beads.Show(path, ticketID)
			if err != nil {
				slog.Warn("board: failed to fetch ticket detail", "id", ticketID, "error", err)
				// Fall through to show basic info from cached ticket.
			} else {
				v.shell.ShowScrollableModal(
					ticketID,
					formatTicketDetail(detail),
					[]ModalAction{{Label: "Fermer", Callback: nil}},
				)
				return
			}
		}
	}

	// Fallback: show basic info from BoardTicket.
	v.shell.ShowScrollableModal(
		ticketID,
		formatBoardTicket(ticket),
		[]ModalAction{{Label: "Fermer", Callback: nil}},
	)
}

// formatTicketDetail formats a TicketDetail for the scrollable modal.
func formatTicketDetail(d *beads.TicketDetail) string {
	accent := theme.ColorTag(theme.AccentHex)
	muted := theme.ColorTag(theme.TextMutedHex)
	reset := theme.TagColor
	sep := fmt.Sprintf("%s%s%s", muted, strings.Repeat("─", 40), reset)

	var b strings.Builder

	// ── Header fields ──────────────────────────────────────────────────────
	fmt.Fprintf(&b, "\n")
	field := func(label, value string) {
		if value == "" {
			return
		}
		fmt.Fprintf(&b, "  %s%-14s%s %s\n", accent, label, reset, value)
	}
	field("Titre", d.Title)
	field("Statut", d.Status)
	field("Priorité", d.PriorityString())
	field("Type", d.Type)
	if d.Assignee != "" {
		field("Assigné", d.Assignee)
	}
	if d.Parent != "" {
		field("Parent", d.Parent)
	}
	if len(d.Labels) > 0 {
		field("Labels", strings.Join(d.Labels, ", "))
	}
	if d.ExternalRef != "" {
		field("Réf. externe", d.ExternalRef)
	}
	if d.Estimate > 0 {
		h := d.Estimate / 60
		m := d.Estimate % 60
		est := fmt.Sprintf("%dh%02d", h, m)
		if h == 0 {
			est = fmt.Sprintf("%dmin", m)
		}
		field("Estimation", est)
	}

	section := func(title, content string) {
		if strings.TrimSpace(content) == "" {
			return
		}
		fmt.Fprintf(&b, "\n%s\n", sep)
		fmt.Fprintf(&b, "  %s%s%s\n\n", accent, title, reset)
		for _, line := range strings.Split(strings.TrimSpace(content), "\n") {
			fmt.Fprintf(&b, "  %s\n", line)
		}
	}

	section("Description", d.Description)
	section("Critères d'acceptation", d.Acceptance)
	section("Notes", d.Notes)
	section("Design", d.Design)
	if d.CloseReason != "" {
		section("Raison de clôture", d.CloseReason)
	}

	fmt.Fprintf(&b, "\n")
	return b.String()
}

// formatBoardTicket formats a BoardTicket (fallback when bd show is unavailable).
func formatBoardTicket(t BoardTicket) string {
	accent := theme.ColorTag(theme.AccentHex)
	reset := theme.TagColor

	var b strings.Builder
	fmt.Fprintf(&b, "\n")
	field := func(label, value string) {
		if value == "" {
			return
		}
		fmt.Fprintf(&b, "  %s%-14s%s %s\n", accent, label, reset, value)
	}
	field("Titre", t.Title)
	field("Statut", t.Status)
	field("Priorité", t.Priority)
	field("Type", t.Type)
	fmt.Fprintf(&b, "\n")
	return b.String()
}

func (v *BoardView) populateColumns(tickets []BoardTicket, columns []BoardColumnDef) {
	// Guard against concurrent Unmount — columnLists may be nil if the view was
	// navigated away from between the refresh goroutine scheduling and execution.
	if v.columnLists == nil {
		return
	}
	for _, list := range v.columnLists {
		list.Clear()
	}
	// Rebuild lookup map.
	newMap := make(map[string]BoardTicket, len(tickets))
	for _, t := range tickets {
		newMap[t.ID] = t
		for i, col := range columns {
			if t.Status == col.Status {
				color := priorityColor(t.Priority)
				prefix := priorityPrefix(t.Priority)
				// Main text: "P1 · titre tronqué…" coloured by priority
				mainText := fmt.Sprintf("%s%s%s%s",
					widgets.ColorTag(color),
					prefix,
					truncateTitle(t.Title, 28),
					"[-]",
				)
				// Secondary text: "  ID · type" dimmed — ID is first token for lookup
				secondary := fmt.Sprintf("  %s · %s", t.ID, t.Type)
				v.columnLists[i].AddItem(mainText, secondary, 0, nil)
				break
			}
		}
	}
	v.ticketsByID = newMap
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
	if v.columnLists == nil {
		return
	}
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
	// Capture app locally before the blocking RefreshFunc call.
	// Unmount() may set v.app = nil while RefreshFunc is running (bd list is
	// an external process call that can take hundreds of milliseconds).
	// Using a local copy guarantees we never call nil.QueueUpdateDraw.
	app := v.app
	if v.cfg.RefreshFunc == nil || app == nil || v.columnLists == nil {
		return
	}
	tickets := v.cfg.RefreshFunc()
	app.QueueUpdateDraw(func() {
		// Re-check inside the queued func — Unmount may have run between
		// RefreshFunc() completing and the draw cycle executing this closure.
		if v.columnLists == nil {
			return
		}
		v.populateColumns(tickets, columns)
	})
}
