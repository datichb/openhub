package views

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// Wiki proposals with their decision (modernisation tui-wiki-bricks): the
// list of the checkpoint card (sessions_checkpoint_wiki.go) and the queue
// of the project (project_wiki_view.go) share it. Each proposal on one
// line, its before/after, source, author and tag below or beside; y
// accept, n refuse (confirmed by the instruction prompt), e modify (on one
// line, or in $EDITOR), s leave pending, A accept all. No limit: the list
// scrolls. Proposal texts come from agents: always escaped.

// WikiProposalView is a wiki proposal shown with its decision.
type WikiProposalView struct {
	ID      string
	Agent   string
	Page    string
	Section string
	Tag     string
	Source  string
	Status  string // pending | accepted | refused | applied | stale
	Content string
	// Replaces is the line of the page the content replaces (p5-fixes):
	// shown before the new one.
	Replaces string

	// Project queue (optional).
	SessionID string
	Workflow  string
	Created   time.Time
	// Edited: the user changed the content of the agent.
	Edited bool
	// Error is the last failure to apply it (stale: the line is gone).
	Error string
}

// Statuses of a proposal (domain.Wiki*).
const (
	wikiStatusPending  = "pending"
	wikiStatusAccepted = "accepted"
	wikiStatusRefused  = "refused"
	wikiStatusStale    = "stale"
)

// wikiItemStale is the state of a stale proposal in the list.
const wikiItemStale ItemState = "stale"

func (p WikiProposalView) open() bool {
	return p.Status == wikiStatusPending || p.Status == wikiStatusAccepted
}

// wikiListOpts configures a list of proposals.
type wikiListOpts struct {
	Title  string
	Layout ActionListLayout
	// Queue: the queue of the project (sections by status, age, no state
	// column; reloaded after each decision). Otherwise the list of a card
	// (state column, decisions applied in place).
	Queue bool
	Shell ShellAccess
	// Load reads the proposals (off the event loop).
	Load func(ctx context.Context) ([]WikiProposalView, error)
	// Decide records a decision (off the event loop).
	Decide func(ctx context.Context, id, choice, content string) error
	// Edit edits a content in $EDITOR, the terminal suspended (event loop;
	// nil: one-line edit only).
	Edit func(name, content string) (string, error)
	// OnOpen runs on Enter (optional).
	OnOpen    func()
	OpenLabel string
	Now       func() time.Time
}

// wikiList is a list of proposals with their decision.
type wikiList struct {
	cfg   wikiListOpts
	list  *ActionList
	app   *tview.Application
	ctx   context.Context
	gen   uint64
	props []WikiProposalView
	err   error
	query string
}

func newWikiList(cfg wikiListOpts) *wikiList {
	w := &wikiList{cfg: cfg}
	open := func(it ActionItem) bool { p, ok := w.prop(it.ID); return ok && p.open() }
	pending := func(it ActionItem) bool { p, ok := w.prop(it.ID); return ok && p.Status == wikiStatusPending }
	lc := ActionListConfig{
		Title:  cfg.Title,
		Layout: cfg.Layout,
		Shell:  cfg.Shell,
		Empty:  i18n.T("tui.wikilist.empty"),
		StateLabels: map[ItemState]string{
			ItemPending:   "? " + i18n.T("tui.wikilist.state.pending"),
			ItemAccepted:  "✓ " + i18n.T("tui.wikilist.state.accepted"),
			ItemRefused:   "✗ " + i18n.T("tui.wikilist.state.refused"),
			wikiItemStale: "⚠ " + i18n.T("tui.wikilist.state.stale"),
		},
		Actions: []ItemAction{
			{Key: keys.Accept, Label: i18n.T("tui.wikilist.accept"), Applies: pending,
				Run: func(it ActionItem, _ string) { w.decide(it.ID, wikiAccept, "") }},
			{Key: keys.Refuse, Applies: open, ReasonOptional: true,
				ReasonTitle: func(it ActionItem) string { return w.refuseTitle(it.ID) },
				Run:         func(it ActionItem, _ string) { w.decide(it.ID, wikiRefuse, "") }},
			{Key: keys.Modify, Applies: open, Run: func(it ActionItem, _ string) { w.modify(it.ID) }},
			{Key: keys.Leave, Applies: open, Run: func(it ActionItem, _ string) { w.leave(it.ID) }},
		},
		Global: []GlobalAction{{Key: keys.AcceptAll, Run: w.acceptAll}},
	}
	if cfg.OnOpen != nil {
		lc.OnOpen = func(ActionItem) { cfg.OnOpen() }
		lc.OpenLabel = cfg.OpenLabel
	}
	if cfg.Queue {
		lc.Headers = []string{i18n.T("tui.wikilist.col.id"), i18n.T("tui.wikilist.col.page"), i18n.T("tui.wikilist.col.kind"),
			i18n.T("tui.wikilist.col.tag"), i18n.T("tui.wikilist.col.age")}
	}
	w.list = NewActionList(lc)
	return w
}

// mount binds the list to the application (async loads and decisions).
func (w *wikiList) mount(app *tview.Application, ctx context.Context) {
	w.app, w.ctx = app, ctx
	w.gen++
}

// unmount drops the pending loads and decisions.
func (w *wikiList) unmount() {
	w.gen++
	w.app = nil
}

// hints is the hints line of the list.
func (w *wikiList) hints() string { return w.list.Hints() }

func (w *wikiList) prop(id string) (WikiProposalView, bool) {
	for _, p := range w.props {
		if p.ID == id {
			return p, true
		}
	}
	return WikiProposalView{}, false
}

// set replaces the proposals.
func (w *wikiList) set(props []WikiProposalView, err error) {
	w.props, w.err = props, err
	w.render()
}

// setQuery filters the proposals (page, section, content, agent, id).
func (w *wikiList) setQuery(q string) {
	w.query = strings.TrimSpace(q)
	w.render()
}

func (w *wikiList) matches(p WikiProposalView) bool {
	if w.query == "" {
		return true
	}
	hay := strings.ToLower(strings.Join([]string{p.ID, p.Page, p.Section, p.Content, p.Replaces, p.Agent, p.Workflow, p.Tag}, " "))
	for _, word := range strings.Fields(strings.ToLower(w.query)) {
		if !strings.Contains(hay, word) {
			return false
		}
	}
	return true
}

func (w *wikiList) now() time.Time {
	if w.cfg.Now != nil {
		return w.cfg.Now()
	}
	return time.Now()
}

func (w *wikiList) render() {
	title := w.cfg.Title
	if w.query != "" {
		title += " · /" + w.query
	}
	w.list.SetTitle(title)
	if w.err != nil {
		w.list.SetEmpty(i18n.Tf("tui.wikilist.error", w.err.Error()))
		w.list.SetItems(nil)
		return
	}
	w.list.SetEmpty(i18n.T("tui.wikilist.empty"))
	var shown []WikiProposalView
	for _, p := range w.props {
		if w.matches(p) {
			shown = append(shown, p)
		}
	}
	if !w.cfg.Queue {
		items := make([]ActionItem, 0, len(shown))
		for _, p := range shown {
			items = append(items, w.item(p, ""))
		}
		w.list.SetItems(items)
		return
	}
	var items []ActionItem
	for _, status := range []string{wikiStatusPending, wikiStatusAccepted, wikiStatusStale} {
		var group []WikiProposalView
		for _, p := range shown {
			if p.Status == status {
				group = append(group, p)
			}
		}
		if len(group) == 0 {
			continue
		}
		section := wikiQueueSection(status, group)
		for _, p := range group {
			items = append(items, w.item(p, section))
		}
	}
	w.list.SetItems(items)
}

// wikiQueueSection is the title of a section of the queue.
func wikiQueueSection(status string, group []WikiProposalView) string {
	switch status {
	case wikiStatusAccepted:
		return i18n.Tf("tui.wikiqueue.section.accepted", len(group))
	case wikiStatusStale:
		return i18n.Tf("tui.wikiqueue.section.stale", len(group))
	}
	var wfs []string
	seen := map[string]bool{}
	for _, p := range group {
		if p.Workflow != "" && !seen[p.Workflow] {
			seen[p.Workflow] = true
			wfs = append(wfs, p.Workflow)
		}
	}
	if len(wfs) == 0 {
		return i18n.Tf("tui.wikiqueue.section.pending", len(group))
	}
	return i18n.Tf("tui.wikiqueue.section.pending_from", len(group), strings.Join(wikiClip(wfs, 3), ", "))
}

func wikiKind(p WikiProposalView) string {
	if p.Replaces != "" {
		return i18n.T("tui.wikilist.kind.replace")
	}
	return i18n.T("tui.wikilist.kind.add")
}

func wikiItemState(status string) ItemState {
	switch status {
	case wikiStatusAccepted:
		return ItemAccepted
	case wikiStatusRefused:
		return ItemRefused
	case wikiStatusStale:
		return wikiItemStale
	case wikiStatusPending:
		return ItemPending
	}
	return ItemState(status)
}

func (w *wikiList) item(p WikiProposalView, section string) ActionItem {
	it := ActionItem{ID: p.ID, Section: section, DetailTitle: wikiDetailTitle(p), Detail: w.detail(p)}
	if w.cfg.Queue {
		it.Cells = []string{p.ID, p.Page, wikiKind(p), p.Tag, shortAge(w.now(), p.Created)}
		return it
	}
	it.Cells = []string{p.ID, p.Page, wikiKind(p), p.Tag}
	it.State = wikiItemState(p.Status)
	if p.Edited && p.Status == wikiStatusAccepted {
		it.StateText = "✓ " + i18n.T("tui.wikilist.state.edited")
	}
	return it
}

// wikiDetailTitle is « wp-2 · technical/stack.md · section « Paquets » ».
func wikiDetailTitle(p WikiProposalView) string {
	parts := []string{p.ID, p.Page}
	if p.Section != "" {
		parts = append(parts, i18n.Tf("tui.wikilist.section", p.Section))
	}
	return strings.Join(parts, " · ")
}

// detail renders the before/after of a proposal, its origin and its state
// (tview tags, escaped texts).
func (w *wikiList) detail(p WikiProposalView) string {
	var b strings.Builder
	var head []string
	if w.cfg.Queue {
		head = append(head, firstText(p.Workflow, p.SessionID))
		if p.Workflow != "" && p.SessionID != "" {
			head = append(head, p.SessionID)
		}
		if !p.Created.IsZero() {
			head = append(head, i18n.Tf("tui.wikilist.ago", shortAge(w.now(), p.Created)))
		}
		head = append(head, p.Tag)
		if line := joinNonEmpty(head, " · "); line != "" {
			b.WriteString(muted(tview.Escape(line)) + "\n")
		}
	}
	if p.Replaces != "" {
		fmt.Fprintf(&b, "%s%s%s\n", theme.ColorTag(theme.ErrorHex), tview.Escape("− "+p.Replaces), theme.TagColor)
		fmt.Fprintf(&b, "%s%s%s\n", theme.ColorTag(theme.SuccessHex), tview.Escape("+ "+p.Content), theme.TagColor)
	} else {
		for _, l := range strings.Split(p.Content, "\n") {
			fmt.Fprintf(&b, "%s%s%s\n", theme.ColorTag(theme.SuccessHex), tview.Escape("+ "+l), theme.TagColor)
		}
	}
	var origin []string
	if p.Source != "" {
		origin = append(origin, i18n.Tf("tui.wikilist.source", p.Source))
	}
	origin = append(origin, i18n.Tf("tui.wikilist.author", firstText(p.Agent, "?")))
	if !w.cfg.Queue && p.Tag != "" {
		origin = append(origin, p.Tag)
	}
	b.WriteString(muted(tview.Escape(strings.Join(origin, " · "))) + "\n")
	if p.Edited {
		b.WriteString(muted(i18n.T("tui.wikilist.edited")) + "\n")
	}
	switch {
	case p.Status == wikiStatusStale:
		fmt.Fprintf(&b, "%s%s%s\n", theme.ColorTag(theme.WarningHex), tview.Escape(i18n.T("tui.wikilist.stale")), theme.TagColor)
	case p.Error != "":
		fmt.Fprintf(&b, "%s%s%s\n", theme.ColorTag(theme.ErrorHex), tview.Escape(i18n.Tf("tui.wikilist.apply_error", p.Error)), theme.TagColor)
	}
	return b.String()
}

func joinNonEmpty(parts []string, sep string) string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, sep)
}

// shortAge is « 3 min », « 3 h », « 2 j » ("" for an unknown date).
func shortAge(now, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := now.Sub(t)
	switch {
	case d < time.Hour:
		return i18n.Tf("tui.wikilist.age.min", max(int(d.Minutes()), 0))
	case d < 24*time.Hour:
		return i18n.Tf("tui.wikilist.age.hours", int(d.Hours()))
	}
	return i18n.Tf("tui.wikilist.age.days", int(d.Hours()/24))
}

func (w *wikiList) refuseTitle(id string) string {
	p, _ := w.prop(id)
	return i18n.Tf("tui.wikilist.refuse_title", id, path.Base(p.Page))
}

// ── Loading and decisions ───────────────────────────────────────────────────

// reload reads the proposals off the event loop.
func (w *wikiList) reload() {
	if w.cfg.Load == nil || w.app == nil {
		return
	}
	w.gen++
	gen, app, ctx := w.gen, w.app, w.context()
	load := w.cfg.Load
	go func() {
		props, err := load(ctx)
		app.QueueUpdateDraw(func() {
			if w.gen != gen {
				return
			}
			w.set(props, err)
		})
	}()
}

func (w *wikiList) context() context.Context {
	if w.ctx != nil {
		return w.ctx
	}
	return context.Background()
}

// decision is a decision to record.
type wikiDecision struct {
	id, choice, content string
}

func (w *wikiList) decide(id, choice, content string) {
	w.record([]wikiDecision{{id, choice, content}})
}

// record records decisions off the event loop, then updates the list (in
// place for a card, reloaded for the queue).
func (w *wikiList) record(list []wikiDecision) {
	if w.cfg.Decide == nil || w.app == nil || len(list) == 0 {
		return
	}
	gen, app, ctx, decide := w.gen, w.app, w.context(), w.cfg.Decide
	go func() {
		var done []wikiDecision
		var err error
		for _, d := range list {
			if err = decide(ctx, d.id, d.choice, d.content); err != nil {
				break
			}
			done = append(done, d)
		}
		app.QueueUpdateDraw(func() {
			if w.gen != gen {
				return
			}
			w.applied(done)
			if err != nil {
				w.toast(i18n.Tf("tui.wikilist.failed", err.Error()), false)
			} else {
				w.toast(wikiRecordedText(done), true)
			}
			if w.cfg.Queue {
				w.reload()
			}
		})
	}()
}

func wikiRecordedText(done []wikiDecision) string {
	if len(done) == 1 {
		return i18n.Tf("tui.wikilist.recorded."+done[0].choice, done[0].id)
	}
	return i18n.Tf("tui.wikilist.recorded_many", len(done))
}

// applied updates the proposals decided (the card keeps the refused ones,
// shown as such).
func (w *wikiList) applied(done []wikiDecision) {
	if len(done) == 0 {
		return
	}
	for _, d := range done {
		for i := range w.props {
			p := &w.props[i]
			if p.ID != d.id {
				continue
			}
			switch d.choice {
			case wikiAccept:
				p.Status = wikiStatusAccepted
			case wikiEdit:
				p.Status, p.Content, p.Edited = wikiStatusAccepted, d.content, true
			case wikiRefuse:
				p.Status = wikiStatusRefused
			case wikiPend:
				p.Status = wikiStatusPending
			}
		}
	}
	w.render()
	if !w.cfg.Queue && len(done) == 1 {
		w.selectNext(done[0].id)
	}
}

// selectNext selects the open proposal after id (decisions one after
// another).
func (w *wikiList) selectNext(id string) {
	items := w.list.Items()
	for i, it := range items {
		if it.ID != id {
			continue
		}
		for _, next := range items[i+1:] {
			if p, ok := w.prop(next.ID); ok && p.Status == wikiStatusPending {
				w.list.Select(next.ID)
				return
			}
		}
		return
	}
}

// acceptAll accepts the pending proposals shown (A).
func (w *wikiList) acceptAll(items []ActionItem) {
	var list []wikiDecision
	for _, it := range items {
		if p, ok := w.prop(it.ID); ok && p.Status == wikiStatusPending {
			list = append(list, wikiDecision{id: p.ID, choice: wikiAccept})
		}
	}
	if len(list) == 0 {
		w.toast(i18n.T("tui.wikilist.none_pending"), false)
		return
	}
	w.record(list)
}

// leave puts an accepted proposal back in the queue; on a pending one it
// moves to the next (s: decide later).
func (w *wikiList) leave(id string) {
	p, _ := w.prop(id)
	if p.Status == wikiStatusAccepted {
		w.decide(id, wikiPend, "")
		return
	}
	items := w.list.Items()
	for i, it := range items {
		if it.ID == id && i+1 < len(items) {
			w.list.Select(items[i+1].ID)
			return
		}
	}
}

// modify edits the content of a proposal: on one line (⏎ for a line
// break), or in $EDITOR when the view can.
func (w *wikiList) modify(id string) {
	p, ok := w.prop(id)
	sh := w.cfg.Shell
	if !ok || sh == nil {
		return
	}
	content := strings.ReplaceAll(p.Content, "\n", wikiNewline)
	fields := []FormField{}
	if w.cfg.Edit != nil {
		fields = append(fields, FormField{Key: "where", Label: i18n.T("tui.wikilist.edit_where"), Type: FieldSelect, Default: wikiEdit,
			Options: []SelectOption{
				{Label: i18n.T("tui.wikilist.edit_here"), Value: wikiEdit},
				{Label: i18n.T("tui.checkpoint.wiki_choice.editor"), Value: wikiEditor},
			}})
	}
	hint := i18n.T("tui.wikilist.edit_hint")
	if p.Replaces != "" {
		hint = i18n.T("tui.wikilist.edit_hint_replace")
	}
	fields = append(fields, FormField{Key: "content", Label: i18n.T("tui.checkpoint.wiki_content"), Type: FieldText, Default: content, Hint: hint})
	sh.ShowInlineForm(InlineFormConfig{
		Title:  tview.Escape(i18n.Tf("tui.wikilist.edit_title", p.ID, p.Page)),
		Fields: fields,
		OnSubmit: func(values map[string]string, _ map[string][]string) {
			text := fromWikiField(values["content"])
			if values["where"] == wikiEditor && w.cfg.Edit != nil {
				w.editInEditor(p, text)
				return
			}
			w.edited(p, text)
		},
	})
}

// editInEditor edits the content in $EDITOR (event loop: the terminal is
// suspended), starting from the field.
func (w *wikiList) editInEditor(p WikiProposalView, text string) {
	out, err := w.cfg.Edit(p.ID+".md", text+"\n")
	if err != nil {
		w.toast(err.Error(), false)
		return
	}
	w.edited(p, out)
}

// edited records an edit; an emptied or untouched content records nothing.
func (w *wikiList) edited(p WikiProposalView, text string) {
	text = strings.TrimSpace(text)
	if text == "" || text == strings.TrimSpace(p.Content) {
		w.toast(i18n.T("tui.checkpoint.wiki_unchanged"), true)
		return
	}
	w.decide(p.ID, wikiEdit, text)
}

func (w *wikiList) toast(msg string, ok bool) {
	if w.cfg.Shell != nil {
		w.cfg.Shell.ShowToastMsg(msg, ok)
	}
}
