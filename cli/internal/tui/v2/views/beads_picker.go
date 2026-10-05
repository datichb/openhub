package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// ─────────────────────────────────────────────────────────────────────────────
// Beads picker (v5 T2, 10-tui §4.2): tview ticket selector with search, label
// and epic filters, grouping by epic, preview and multi-selection. It replaces
// the huh selector used during the TUI; the launch sheet (beads-id inputs)
// embeds it as a primitive.
// ─────────────────────────────────────────────────────────────────────────────

// BeadsSource provides the tickets of a project.
type BeadsSource interface {
	// Tickets returns every ticket, epics included (the picker filters).
	Tickets(ctx context.Context) ([]beads.Ticket, error)
	// Detail returns a ticket with its description and acceptance criteria.
	Detail(ctx context.Context, id string) (*beads.TicketDetail, error)
}

// NewBeadsSource returns the source reading a project with the bd CLI.
func NewBeadsSource(projectPath string) BeadsSource { return bdSource{path: projectPath} }

type bdSource struct{ path string }

func (s bdSource) Tickets(context.Context) ([]beads.Ticket, error) { return beads.ListAll(s.path) }
func (s bdSource) Detail(_ context.Context, id string) (*beads.TicketDetail, error) {
	return beads.Show(s.path, id)
}

// BeadsPickerConfig configures a Beads picker.
type BeadsPickerConfig struct {
	Source BeadsSource
	// Title overrides the default title.
	Title string
	// Filter is the initial label filter (e.g. "ai-delegated"; "" = all).
	Filter string
	// Epic restricts the initial list to an epic.
	Epic string
	// Multi enables multi-selection (one session per ticket).
	Multi bool
	// Preselected tickets (multi mode).
	Preselected []string
	// ClaimedBy returns the member holding a claim on a ticket ("" = none).
	ClaimedBy func(id string) string
	// OnDone receives the chosen tickets; OnCancel is called on Esc.
	OnDone   func(ids []string)
	OnCancel func()
}

// ApplyInput sets the filter, epic and multi-selection of a workflow input
// (`inputs.<id>.picker`, type beads-id / beads-ids).
func (c *BeadsPickerConfig) ApplyInput(in workflow.Input) {
	if in.Type == workflow.InputBeadsIDs {
		c.Multi = true
	}
	if p := in.Picker; p != nil {
		c.Filter, c.Epic = p.Filter, p.Epic
		c.Multi = c.Multi || p.Multi
	}
}

// BeadsPicker is the picker primitive. It keeps the keyboard focus itself and
// routes keys to the search field while searching.
type BeadsPicker struct {
	*tview.Flex
	cfg BeadsPickerConfig
	app *tview.Application
	ctx context.Context

	search  *tview.InputField
	filters *tview.TextView
	list    *beadsList
	preview *tview.TextView
	footer  *tview.TextView

	m         *beadsPickerModel
	loading   bool
	loadErr   error
	searching bool
	details   map[string]*beads.TicketDetail
	detailErr map[string]error
	pending   map[string]bool
}

// NewBeadsPicker builds the picker. app may be nil (tests): loading is then
// synchronous. Call Load to read the tickets from the source.
func NewBeadsPicker(ctx context.Context, app *tview.Application, cfg BeadsPickerConfig) *BeadsPicker {
	if ctx == nil {
		ctx = context.Background()
	}
	p := &BeadsPicker{
		Flex:      tview.NewFlex().SetDirection(tview.FlexRow),
		cfg:       cfg,
		app:       app,
		ctx:       ctx,
		m:         newBeadsPickerModel(nil, cfg.Filter, cfg.Epic, nil),
		details:   map[string]*beads.TicketDetail{},
		detailErr: map[string]error{},
		pending:   map[string]bool{},
	}
	title := cfg.Title
	if title == "" {
		title = i18n.T("tui.picker.beads.title")
		if cfg.Multi {
			title = i18n.T("tui.picker.beads.title_multi")
		}
	}
	p.SetBorder(true).SetTitle(" " + title + " ").SetTitleAlign(tview.AlignLeft)
	p.SetBorderColor(theme.ActiveMode.Primary)
	p.SetBackgroundColor(theme.BgPanel)

	p.search = tview.NewInputField().
		SetLabel("/ ").
		SetLabelColor(theme.ActiveMode.Secondary).
		SetFieldBackgroundColor(theme.BgPanel).
		SetFieldTextColor(theme.FgPrimary).
		SetPlaceholder(i18n.T("tui.picker.beads.search_placeholder")).
		SetPlaceholderTextColor(theme.FgMuted)
	p.search.SetBackgroundColor(theme.BgPanel)
	p.search.SetChangedFunc(func(text string) {
		p.m.setQuery(text)
		p.refresh()
	})

	p.filters = tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignRight)
	p.filters.SetBackgroundColor(theme.BgPanel)

	p.list = &beadsList{Box: tview.NewBox().SetBackgroundColor(theme.BgPanel), p: p}

	p.preview = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	p.preview.SetBorder(true).SetTitle(" " + i18n.T("tui.picker.beads.preview") + " ").SetTitleAlign(tview.AlignLeft)
	p.preview.SetBorderColor(theme.FgMuted)
	p.preview.SetBackgroundColor(theme.BgPanel)

	p.footer = tview.NewTextView().SetDynamicColors(true)
	p.footer.SetBackgroundColor(theme.BgPanel)

	top := tview.NewFlex().
		AddItem(p.search, 0, 1, false).
		AddItem(p.filters, 0, 1, false)
	p.AddItem(top, 1, 0, false).
		AddItem(p.list, 0, 1, false).
		AddItem(p.preview, 6, 0, false).
		AddItem(p.footer, 1, 0, false)
	p.refresh()
	return p
}

// Load reads the tickets from the source (in the background when an
// application is set).
func (p *BeadsPicker) Load() {
	if p.cfg.Source == nil {
		p.setTickets(nil, fmt.Errorf("no Beads source"))
		return
	}
	p.loading = true
	p.refresh()
	if p.app == nil {
		ts, err := p.cfg.Source.Tickets(p.ctx)
		p.setTickets(ts, err)
		return
	}
	go func() {
		ts, err := p.cfg.Source.Tickets(p.ctx)
		if p.ctx.Err() != nil {
			return
		}
		p.app.QueueUpdateDraw(func() { p.setTickets(ts, err) })
	}()
}

// SetTickets replaces the tickets (already loaded by the caller).
func (p *BeadsPicker) SetTickets(ts []beads.Ticket) { p.setTickets(ts, nil) }

func (p *BeadsPicker) setTickets(ts []beads.Ticket, err error) {
	p.loading = false
	p.loadErr = err
	m := newBeadsPickerModel(ts, p.cfg.Filter, p.cfg.Epic, p.cfg.Preselected)
	m.query = p.m.query
	m.selected = append(m.selected, p.m.selected...)
	m.rebuild()
	p.m = m
	p.refresh()
}

// Selected returns the selected tickets (multi mode), in selection order.
func (p *BeadsPicker) Selected() []string { return append([]string(nil), p.m.selected...) }

// CapturesInput reports whether the search field receives the keys
// (implements InputCapturing: the shell must not steal printable keys).
func (p *BeadsPicker) CapturesInput() bool { return p.searching }

// StatusHints returns the key hints of the picker.
func (p *BeadsPicker) StatusHints() string {
	if p.searching {
		return i18n.T("tui.picker.beads.hints_search")
	}
	if p.cfg.Multi {
		return i18n.T("tui.picker.beads.hints_multi")
	}
	return i18n.T("tui.picker.beads.hints")
}

// Focus keeps the focus on the picker (keys are routed by InputHandler).
func (p *BeadsPicker) Focus(func(tview.Primitive)) {
	p.Box.Focus(nil)
	if p.searching {
		p.search.Focus(nil)
	}
}

// Blur removes the focus from the picker and its search field.
func (p *BeadsPicker) Blur() {
	p.search.Blur()
	p.Box.Blur()
}

// HasFocus reports whether the picker has the focus.
func (p *BeadsPicker) HasFocus() bool { return p.Box.HasFocus() }

// MouseHandler keeps the focus on the picker (a click on the search field
// would otherwise move the application focus to it).
func (p *BeadsPicker) MouseHandler() func(tview.MouseAction, *tcell.EventMouse, func(tview.Primitive)) (bool, tview.Primitive) {
	return p.WrapMouseHandler(func(action tview.MouseAction, ev *tcell.EventMouse, setFocus func(tview.Primitive)) (bool, tview.Primitive) {
		if !p.InRect(ev.Position()) {
			return false, nil
		}
		if action == tview.MouseLeftClick {
			setFocus(p)
		}
		return true, nil
	})
}

// InputHandler routes the keys (see StatusHints).
func (p *BeadsPicker) InputHandler() func(*tcell.EventKey, func(tview.Primitive)) {
	return p.WrapInputHandler(func(ev *tcell.EventKey, setFocus func(tview.Primitive)) {
		if p.searching {
			p.handleSearchKey(ev, setFocus)
			return
		}
		p.handleListKey(ev)
	})
}

func (p *BeadsPicker) handleSearchKey(ev *tcell.EventKey, setFocus func(tview.Primitive)) {
	switch ev.Key() {
	case tcell.KeyEscape:
		p.search.SetText("") // triggers setQuery("")
		p.leaveSearch()
	case tcell.KeyEnter, tcell.KeyTab, tcell.KeyDown:
		p.leaveSearch()
	case tcell.KeyUp:
		p.leaveSearch()
		p.moveCursor(-1)
	default:
		if h := p.search.InputHandler(); h != nil {
			h(ev, setFocus)
		}
	}
}

func (p *BeadsPicker) leaveSearch() {
	p.searching = false
	p.search.Blur()
	p.refresh()
}

func (p *BeadsPicker) handleListKey(ev *tcell.EventKey) {
	switch ev.Key() {
	case tcell.KeyUp:
		p.moveCursor(-1)
	case tcell.KeyDown:
		p.moveCursor(1)
	case tcell.KeyPgUp:
		p.moveCursor(-10)
	case tcell.KeyPgDn:
		p.moveCursor(10)
	case tcell.KeyHome:
		p.m.moveToEdge(false)
		p.refresh()
	case tcell.KeyEnd:
		p.m.moveToEdge(true)
		p.refresh()
	case tcell.KeyEnter:
		if ids := p.m.result(p.cfg.Multi); len(ids) > 0 && p.cfg.OnDone != nil {
			p.cfg.OnDone(ids)
		}
	case tcell.KeyEscape:
		if p.cfg.OnCancel != nil {
			p.cfg.OnCancel()
		}
	case tcell.KeyRune:
		switch ev.Rune() {
		case '/':
			p.searching = true
			p.search.Focus(nil)
			p.refresh()
		case ' ':
			if p.cfg.Multi {
				p.m.toggle()
				p.moveCursor(1)
			}
		case 'j':
			p.moveCursor(1)
		case 'k':
			p.moveCursor(-1)
		case 'f':
			p.m.cycleLabel()
			p.refresh()
		case 'e':
			p.m.cycleEpic()
			p.refresh()
		}
	}
}

func (p *BeadsPicker) moveCursor(delta int) {
	p.m.move(delta)
	p.refresh()
}

// refresh updates the filters line, the preview and the footer (the list
// draws itself from the model).
func (p *BeadsPicker) refresh() {
	label := p.m.labelFilter()
	if label == "" {
		label = i18n.T("tui.picker.beads.filter_all")
	}
	epic := p.epicLabel(p.m.epicFilter())
	p.filters.SetText(fmt.Sprintf("[%s]%s[-] %s ▾  [%s]%s[-] %s ▾ ",
		theme.TextMutedHex, i18n.T("tui.picker.beads.filter"), tview.Escape(label),
		theme.TextMutedHex, i18n.T("tui.picker.beads.epic"), tview.Escape(epic)))

	hints := p.StatusHints()
	if p.cfg.Multi {
		hints = i18n.Tf("tui.picker.beads.selected_count", len(p.m.selected)) + " · " + hints
	}
	p.footer.SetText(fmt.Sprintf("[%s]%s[-]", theme.TextMutedHex, hints))
	p.refreshPreview()
}

func (p *BeadsPicker) epicLabel(id string) string {
	switch id {
	case beadsEpicAll:
		return i18n.T("tui.picker.beads.epic_all")
	case beadsEpicNone:
		return i18n.T("tui.picker.beads.epic_none")
	}
	if e, ok := p.m.epics[id]; ok && e.Title != "" {
		return id + " · " + e.Title
	}
	return id
}

func (p *BeadsPicker) refreshPreview() {
	t := p.m.current()
	if t == nil {
		p.preview.SetText("")
		return
	}
	head := fmt.Sprintf("[::b]%s[::-] · %s", tview.Escape(t.ID), tview.Escape(t.Title))
	d, ok := p.details[t.ID]
	switch {
	case p.detailErr[t.ID] != nil:
		p.preview.SetText(head + "\n" + fmt.Sprintf("[%s]%s[-]", theme.ErrorHex, tview.Escape(i18n.Tf("tui.picker.beads.preview_error", p.detailErr[t.ID]))))
	case !ok:
		p.preview.SetText(head + "\n" + fmt.Sprintf("[%s]%s[-]", theme.TextMutedHex, i18n.T("tui.picker.beads.preview_loading")))
		p.requestDetail(t.ID)
	default:
		text := head
		if n := acceptanceCount(d.Acceptance); n > 0 {
			text += " — " + i18n.Tf("tui.picker.beads.preview_criteria", n)
		}
		if desc := strings.TrimSpace(d.Description); desc != "" {
			text += "\n" + tview.Escape(desc)
		}
		p.preview.SetText(text)
	}
	p.preview.ScrollToBeginning()
}

func (p *BeadsPicker) requestDetail(id string) {
	if p.pending[id] || p.cfg.Source == nil {
		return
	}
	p.pending[id] = true
	store := func(d *beads.TicketDetail, err error) {
		delete(p.pending, id)
		if err != nil {
			p.detailErr[id] = err
		} else {
			p.details[id] = d
		}
		if t := p.m.current(); t != nil && t.ID == id {
			p.refreshPreview()
		}
	}
	if p.app == nil {
		d, err := p.cfg.Source.Detail(p.ctx, id)
		store(d, err)
		return
	}
	go func() {
		d, err := p.cfg.Source.Detail(p.ctx, id)
		if p.ctx.Err() != nil {
			return
		}
		p.app.QueueUpdateDraw(func() { store(d, err) })
	}()
}

// acceptanceCount counts the acceptance criteria (list items, or 1 for a
// non-empty free text).
func acceptanceCount(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ") || (len(l) > 2 && l[0] >= '0' && l[0] <= '9' && strings.Contains(l[:3], ".")) {
			n++
		}
	}
	if n == 0 && strings.TrimSpace(s) != "" {
		return 1
	}
	return n
}

// beadsList draws the rows of the model, keeping the cursor visible.
type beadsList struct {
	*tview.Box
	p      *BeadsPicker
	offset int
}

func (l *beadsList) Draw(screen tcell.Screen) {
	l.DrawForSubclass(screen, l)
	x, y, w, h := l.GetInnerRect()
	p := l.p
	switch {
	case p.loading:
		tview.Print(screen, i18n.T("tui.picker.beads.loading"), x+1, y, w-1, tview.AlignLeft, theme.FgMuted)
		return
	case p.loadErr != nil:
		tview.Print(screen, tview.Escape(i18n.Tf("tui.picker.beads.load_error", p.loadErr)), x+1, y, w-1, tview.AlignLeft, theme.Error)
		return
	case p.m.ticketCount() == 0:
		tview.Print(screen, i18n.T("tui.picker.beads.empty"), x+1, y, w-1, tview.AlignLeft, theme.FgMuted)
		return
	}
	rows := p.m.rows
	if c := p.m.cursor; c >= 0 {
		if c < l.offset {
			l.offset = c
			if c > 0 && rows[c-1].header {
				l.offset = c - 1 // keep the epic header of the first ticket visible
			}
		} else if c >= l.offset+h {
			l.offset = c - h + 1
		}
	}
	l.offset = max(0, min(l.offset, len(rows)-h))

	idW, statusW := 0, 0
	for _, r := range rows {
		if r.ticket != nil {
			idW = max(idW, runewidth.StringWidth(r.ticket.ID))
			statusW = max(statusW, runewidth.StringWidth(r.ticket.Status))
		}
	}
	titleW := max(12, min(60, w-idW-statusW-30))
	for i := 0; i < h && l.offset+i < len(rows); i++ {
		r := rows[l.offset+i]
		line := l.format(r, idW, statusW, titleW)
		if l.offset+i == p.m.cursor {
			for cx := x; cx < x+w; cx++ {
				screen.SetContent(cx, y+i, ' ', nil, tcell.StyleDefault.Background(theme.BgElement))
			}
		}
		tview.Print(screen, line, x, y+i, w, tview.AlignLeft, theme.FgPrimary)
	}
}

func (l *beadsList) format(r beadsPickerRow, idW, statusW, titleW int) string {
	p := l.p
	if r.header {
		label := i18n.T("tui.picker.beads.group_none")
		if r.epicID != "" {
			label = i18n.Tf("tui.picker.beads.group_epic", r.epicID, p.m.epics[r.epicID].Title)
		}
		return fmt.Sprintf(" [%s]─ %s[-]", theme.TextSecondaryHex, tview.Escape(label))
	}
	t := r.ticket
	marker := "  "
	if p.m.cursor >= 0 && p.m.rows[p.m.cursor].ticket == t {
		marker = "[" + theme.AccentHex + "]" + theme.IconArrow + "[-] "
	}
	if p.cfg.Multi {
		box := "○"
		if p.m.isSelected(t.ID) {
			box = "[" + theme.SuccessHex + "]●[-]"
		}
		marker += box + " "
	}
	title := runewidth.FillRight(runewidth.Truncate(t.Title, titleW, "…"), titleW)
	claim := ""
	if who := l.claimedBy(t.ID); who != "" {
		claim = "  [" + theme.WarningHex + "]⚠ " + tview.Escape(i18n.Tf("tui.picker.beads.claimed", who)) + "[-]"
	}
	return fmt.Sprintf(" %s%s  %s  %-3s %s%s  [%s]%s[-]", marker,
		tview.Escape(runewidth.FillRight(t.ID, idW)), tview.Escape(title), tview.Escape(t.Priority),
		tview.Escape(runewidth.FillRight(t.Status, statusW)), claim, theme.TextMutedHex, tview.Escape(strings.Join(t.Labels, " ")))
}

func (l *beadsList) claimedBy(id string) string {
	if l.p.cfg.ClaimedBy == nil {
		return ""
	}
	return l.p.cfg.ClaimedBy(id)
}

var _ InputCapturing = (*BeadsPicker)(nil)
