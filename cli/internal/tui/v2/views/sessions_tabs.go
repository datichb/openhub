package views

import (
	"context"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// ─────────────────────────────────────────────────────────────────────────────
// Detail of a session in tabs (modernisation tui-sessions): Détail (state,
// outputs recorded by oh, `next:` steps), Trace (the text of `oh session
// trace` in a reader), Rapports (the reports of the session and a reader),
// Fan-out (sessions_fanout.go). Tab / Shift+Tab change the tab; outside
// Détail the keys go to the tab (CapturesInput) and Esc comes back to
// Détail and the list.
// ─────────────────────────────────────────────────────────────────────────────

type sessionTab int

const (
	tabDetail sessionTab = iota
	tabTrace
	tabReports
	tabFanout
)

var sessionTabKeys = map[sessionTab]string{
	tabDetail: "detail", tabTrace: "trace", tabReports: "reports", tabFanout: "fanout",
}

func (t sessionTab) label() string { return i18n.T("tui.sessions.tab." + sessionTabKeys[t]) }

// SessionTracer is implemented by backends that give the trace of a
// session as `oh session trace` prints it.
type SessionTracer interface {
	SessionTrace(ctx context.Context, sessionID string) (string, error)
}

// SessionReportLister is implemented by backends that list the reports of
// a session (`oh session report`); bodies are read with
// CheckpointReportReader.
type SessionReportLister interface {
	SessionReports(ctx context.Context, sessionID string) ([]CheckpointReport, error)
}

// sessionTabs is the right column of the view.
type sessionTabs struct {
	root    *tview.Flex
	bar     *tview.TextView
	pages   *tview.Pages
	footer  *tview.TextView
	detail  *tview.TextView
	trace   *textReader
	reports *reportsPane
	fanout  *fanoutPane

	cur     sessionTab
	focused bool   // the keys go to the tab
	session string // session shown by the tab
}

func newSessionTabs(v *SessionsView) *sessionTabs {
	t := &sessionTabs{}
	t.bar = tview.NewTextView().SetDynamicColors(true)
	t.bar.SetBackgroundColor(theme.BgPanel)
	t.footer = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	t.footer.SetBackgroundColor(theme.BgPanel)
	t.detail = tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetScrollable(true)
	t.detail.SetBackgroundColor(theme.BgPanel)
	t.trace = newTextReader()
	t.trace.ask = v.askSearch
	t.reports = newReportsPane(v)
	t.fanout = newFanoutPane(v)
	t.pages = tview.NewPages().
		AddPage("detail", t.detail, true, true).
		AddPage("trace", t.trace.root, true, false).
		AddPage("reports", t.reports.root, true, false).
		AddPage("fanout", t.fanout.root, true, false)
	t.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(t.bar, 1, 0, false).
		AddItem(t.pages, 0, 1, false).
		AddItem(t.footer, 2, 0, false)
	t.root.SetBackgroundColor(theme.BgPanel)
	t.root.SetBorder(true).SetBorderColor(theme.FgMuted).SetTitleAlign(tview.AlignLeft)
	t.root.SetBorderPadding(0, 0, 1, 1)
	t.renderBar(nil)
	return t
}

// available are the tabs of a session: Fan-out only in a fan-out.
func availableTabs(r *SessionRow) []sessionTab {
	tabs := []sessionTab{tabDetail, tabTrace, tabReports}
	if r != nil && r.InFanout() {
		tabs = append(tabs, tabFanout)
	}
	return tabs
}

func (t *sessionTabs) renderBar(r *SessionRow) {
	var parts []string
	for _, tab := range availableTabs(r) {
		if tab == t.cur {
			parts = append(parts, theme.ColorTag(theme.ActiveMode.PrimaryHex)+tview.Escape("["+tab.label()+"]")+theme.TagColor)
			continue
		}
		parts = append(parts, " "+muted(tab.label())+" ")
	}
	t.bar.SetText(strings.Join(parts, " "))
	t.footer.SetText(muted(t.hints()))
}

// hints are the keys of the tab shown (footer of the column: the hints
// line of the shell is the one of the list).
func (t *sessionTabs) hints() string {
	tab := i18n.T("tui.sessions.tab.next")
	back := keys.Back.HintAs(i18n.T("tui.sessions.tab.back"))
	switch t.cur {
	case tabTrace:
		return keys.Hints(keys.Search.Hint(), readerMatchHint(), keys.Ends.Hint(), keys.Refresh.Hint(), tab, back)
	case tabReports:
		return keys.Hints(keys.Open.HintAs(i18n.T("tui.sessions.reports.read")), keys.Search.Hint(), readerMatchHint(), tab, back)
	case tabFanout:
		return t.fanout.hints(tab, back)
	}
	return keys.Hints(tab)
}

func readerMatchHint() string {
	return keys.NextMatch.Keys() + "/" + keys.PrevMatch.Keys() + " " + keys.NextMatch.Label() + "/" + keys.PrevMatch.Label()
}

// ── Switching ───────────────────────────────────────────────────────────

// switchTab moves to the next (delta 1) or previous (-1) tab of the
// selected session.
func (v *SessionsView) switchTab(delta int) {
	r, _ := v.selected()
	tabs := availableTabs(r)
	i := 0
	for j, t := range tabs {
		if t == v.tabs.cur {
			i = j
		}
	}
	v.showTab(tabs[(i+delta+len(tabs))%len(tabs)])
}

// showTab shows a tab of the selected session and gives it the keys.
func (v *SessionsView) showTab(t sessionTab) {
	if v.tabs == nil {
		return
	}
	r, _ := v.selected()
	if r == nil || (t == tabFanout && !r.InFanout()) {
		t = tabDetail
	}
	v.tabs.cur = t
	v.tabs.pages.SwitchToPage(sessionTabKeys[t])
	v.tabs.renderBar(r)
	v.tabs.focused = t != tabDetail
	defer v.refreshHints()
	if t == tabDetail {
		if v.app != nil && v.list != nil {
			v.app.SetFocus(v.list)
		}
		return
	}
	id := r.ID
	if v.tabs.session != id {
		v.tabs.session = id
		v.tabs.trace.SetText("")
		v.tabs.reports.reset()
		v.tabs.fanout.reset()
	}
	switch t {
	case tabTrace:
		v.loadTrace(id)
		v.focus(v.tabs.trace.text)
	case tabReports:
		v.loadReports(id)
		v.focus(v.tabs.reports.list.Focusable())
	case tabFanout:
		v.loadFanout(id)
		v.focus(v.tabs.fanout.list.Focusable())
	}
}

func (v *SessionsView) focus(p tview.Primitive) {
	if v.app != nil {
		v.app.SetFocus(p)
	}
}

// CapturesInput gives the keys to the tab shown (Esc included) or to the
// batch; the list keeps the keys of the shell.
func (v *SessionsView) CapturesInput() bool {
	return v.batch != nil || (v.tabs != nil && v.tabs.focused)
}

// handleTabKey routes the keys of a tab other than Détail.
func (v *SessionsView) handleTabKey(ev *tcell.EventKey) *tcell.EventKey {
	switch ev.Key() {
	case tcell.KeyTab:
		v.switchTab(1)
		return nil
	case tcell.KeyBacktab:
		v.switchTab(-1)
		return nil
	}
	switch v.tabs.cur {
	case tabTrace:
		if keys.Back.Matches(ev) {
			v.showTab(tabDetail)
			return nil
		}
		if keys.Refresh.Matches(ev) {
			v.tabs.session = ""
			v.showTab(tabTrace)
			return nil
		}
		return v.tabs.trace.HandleKey(ev)
	case tabReports:
		return v.tabs.reports.handleKey(ev)
	case tabFanout:
		return v.tabs.fanout.handleKey(ev)
	}
	return ev
}

// askSearch prompts for a search in a reader.
func (v *SessionsView) askSearch(title, current string, done func(string)) {
	if v.shell != nil {
		v.shell.ShowInputModal(title, current, done)
	}
}

// ── Trace ───────────────────────────────────────────────────────────────

func (v *SessionsView) loadTrace(id string) {
	tr, ok := v.cfg.Backend.(SessionTracer)
	if !ok {
		v.tabs.trace.SetEmpty(i18n.T("tui.sessions.trace.unavailable"))
		return
	}
	v.tabs.trace.SetEmpty(i18n.T("tui.sessions.trace.loading"))
	var text string
	v.async(func(ctx context.Context) (string, error) {
		var err error
		text, err = tr.SessionTrace(ctx, id)
		return "", err
	}, func(string) {
		if v.tabs == nil || v.tabs.session != id {
			return
		}
		v.tabs.trace.SetEmpty(i18n.T("tui.sessions.trace.empty"))
		v.tabs.trace.SetText(text)
	})
}

// ── Reports ─────────────────────────────────────────────────────────────

// reportsPane lists the reports of the session (left) and reads the
// selected one (right).
type reportsPane struct {
	v       *SessionsView
	root    *tview.Flex
	list    *ActionList
	reader  *textReader
	bodies  map[string]string
	reading bool // the keys go to the reader
}

func newReportsPane(v *SessionsView) *reportsPane {
	p := &reportsPane{v: v, bodies: map[string]string{}}
	p.reader = newTextReader()
	p.reader.ask = v.askSearch
	p.list = NewActionList(ActionListConfig{
		Title:     i18n.T("tui.sessions.reports.title"),
		Layout:    DetailNone,
		Empty:     i18n.T("tui.sessions.reports.loading"),
		OnChange:  p.show,
		OnOpen:    func(ActionItem) { p.read(true) },
		OpenLabel: i18n.T("tui.sessions.reports.read"),
	})
	p.root = tview.NewFlex().
		AddItem(p.list.Primitive(), 0, 2, true).
		AddItem(p.reader.root, 0, 3, false)
	p.root.SetBackgroundColor(theme.BgPanel)
	return p
}

func (p *reportsPane) reset() {
	p.bodies = map[string]string{}
	p.reading = false
	p.list.SetEmpty(i18n.T("tui.sessions.reports.loading"))
	p.list.SetItems(nil)
	p.reader.SetEmpty("")
	p.reader.SetText("")
}

// reportItems are the reports as list elements.
func reportItems(list []CheckpointReport) []ActionItem {
	items := make([]ActionItem, 0, len(list))
	withChild := false
	for _, r := range list {
		withChild = withChild || r.Child != ""
	}
	for i := len(list) - 1; i >= 0; i-- { // the last one first
		r := list[i]
		at := ""
		if !r.At.IsZero() {
			at = r.At.Local().Format("15:04")
		}
		cells := []string{r.ID, firstText(r.Agent, "?"), at, reportSize(r.Bytes), clipText(r.Title, 40)}
		title := r.ID + " · " + firstText(r.Agent, "?")
		if withChild { // the child that filed it, after the id
			cells = slices.Insert(cells, 1, clipText(r.Child, 14))
			if r.Child != "" {
				title = r.Child + " · " + title
			}
		}
		items = append(items, ActionItem{ID: r.key(), Cells: cells, DetailTitle: title, Ref: r})
	}
	return items
}

func reportSize(n int) string {
	if n >= 1024 {
		return i18n.Tf("tui.checkpoint.report_kib", float64(n)/1024)
	}
	return i18n.Tf("tui.checkpoint.report_bytes", n)
}

func (v *SessionsView) loadReports(id string) {
	p := v.tabs.reports
	rl, ok := v.cfg.Backend.(SessionReportLister)
	if !ok {
		p.list.SetEmpty(i18n.T("tui.sessions.reports.unavailable"))
		p.list.SetItems(nil)
		return
	}
	var list []CheckpointReport
	v.async(func(ctx context.Context) (string, error) {
		var err error
		list, err = rl.SessionReports(ctx, id)
		return "", err
	}, func(string) {
		if v.tabs == nil || v.tabs.session != id {
			return
		}
		p.list.SetEmpty(i18n.T("tui.sessions.reports.none"))
		p.list.SetItems(reportItems(list))
	})
}

// show reads the selected report in the reader (loaded once).
func (p *reportsPane) show(it ActionItem) {
	r, ok := it.Ref.(CheckpointReport)
	if !ok {
		return
	}
	if body, ok := p.bodies[r.key()]; ok {
		p.reader.SetText(body)
		return
	}
	rd, ok := p.v.cfg.Backend.(CheckpointReportReader)
	sessionID := p.v.tabs.session
	if !ok || sessionID == "" {
		return
	}
	sessionID = r.owner(sessionID)
	p.reader.SetEmpty(i18n.T("tui.sessions.reports.loading"))
	p.reader.SetText("")
	var body string
	p.v.async(func(ctx context.Context) (string, error) {
		var err error
		_, body, err = rd.CheckpointReport(ctx, sessionID, r.ID)
		return "", err
	}, func(string) {
		p.bodies[r.key()] = body
		if cur, ok := p.list.Current(); ok && cur.ID == r.key() {
			p.reader.SetText(body)
		}
	})
}

// read gives the keys to the reader (true) or back to the list.
func (p *reportsPane) read(on bool) {
	p.reading = on
	if on {
		p.v.focus(p.reader.text)
	} else {
		p.v.focus(p.list.Focusable())
	}
}

func (p *reportsPane) handleKey(ev *tcell.EventKey) *tcell.EventKey {
	if keys.Back.Matches(ev) {
		if p.reading {
			p.read(false)
		} else {
			p.v.showTab(tabDetail)
		}
		return nil
	}
	if keys.Refresh.Matches(ev) {
		p.v.tabs.session = ""
		p.v.showTab(tabReports)
		return nil
	}
	switch {
	case keys.Search.Matches(ev), keys.NextMatch.Matches(ev), keys.PrevMatch.Matches(ev):
		return p.reader.HandleKey(ev)
	}
	if p.reading {
		return p.reader.HandleKey(ev)
	}
	return ev
}
