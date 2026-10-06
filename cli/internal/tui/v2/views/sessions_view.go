package views

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// SessionsView (P3-T16, P3-T18; replaces the empty "parallel" view, B11/T6):
// « À traiter » (decisions of every session), running, sleeping and finished
// sessions, a detail pane and the live feed (t).
// ─────────────────────────────────────────────────────────────────────────────

const (
	sessionsFinishedFor = 7 * 24 * time.Hour
	sessionsReloadDelay = 300 * time.Millisecond
	sessionsFeedMax     = 300
)

// SessionsViewConfig configures the Sessions view.
type SessionsViewConfig struct {
	Backend SessionsBackend
}

// attachMethods are the ways to open a session client (I2).
var attachMethods = []string{"auto", "iterm", "terminal", "tmux", "browser", "suspend"}

// sessionRef / decisionRef are the list item references.
type sessionRef struct{ id string }
type decisionRef struct{ id, session string }

// SessionsView is the control tower of the v5 sessions.
type SessionsView struct {
	cfg   SessionsViewConfig
	shell ShellAccess
	app   *tview.Application

	root   *tview.Flex
	top    *tview.Flex
	list   *widgets.SectionedList
	detail *tview.TextView
	feed   *tview.TextView

	ctx    context.Context
	cancel context.CancelFunc
	gen    uint64

	allProjects bool   // f: every project (default when no project is active)
	focusID     string // session to select after the next load
	rows        []SessionRow

	feedMu     sync.Mutex
	feedOn     bool
	feedID     string
	feedCancel context.CancelFunc
	feedLines  []string
	feedCost   float64
}

var _ View = (*SessionsView)(nil)

// NewSessionsView creates the Sessions view.
func NewSessionsView(cfg SessionsViewConfig) *SessionsView { return &SessionsView{cfg: cfg} }

// SetShell injects the shell.
func (v *SessionsView) SetShell(s ShellAccess) { v.shell = s }

// ID returns the view identifier.
func (v *SessionsView) ID() string { return "sessions" }

// Title returns the display title.
func (v *SessionsView) Title() string { return i18n.T("tui.sessions.title") }

// Focus selects a session the next time the view is mounted or reloaded.
func (v *SessionsView) Focus(sessionID string) { v.focusID = sessionID }

// StatusHints returns keybinding hints.
func (v *SessionsView) StatusHints() string { return i18n.T("tui.sessions.hints") }

// Mount builds the view and starts loading.
func (v *SessionsView) Mount(content *tview.Flex, app *tview.Application) {
	v.app = app
	v.gen++
	v.ctx, v.cancel = context.WithCancel(context.Background())
	if v.shell != nil && v.shell.ActiveProject() == nil {
		v.allProjects = true
	}

	v.list = widgets.NewSectionedList().SetApp(app)
	v.list.SetBorderPadding(0, 0, 1, 1)
	v.list.SetItemChangedFunc(func(int, widgets.SectionItem) { v.onSelectionChanged() })

	v.detail = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	v.detail.SetBackgroundColor(theme.BgPanel)
	v.detail.SetBorder(true).SetBorderColor(theme.BorderNormal).SetTitleAlign(tview.AlignLeft)
	v.detail.SetBorderPadding(0, 0, 1, 1)

	v.feed = tview.NewTextView().SetDynamicColors(true).SetWrap(true).SetScrollable(true)
	v.feed.SetBackgroundColor(theme.BgPanel)
	v.feed.SetBorder(true).SetBorderColor(theme.BorderNormal).SetTitleAlign(tview.AlignLeft)
	v.feed.SetBorderPadding(0, 0, 1, 1)

	v.top = tview.NewFlex()
	v.top.AddItem(v.list, 0, 1, true)
	v.root = tview.NewFlex().SetDirection(tview.FlexRow)
	v.root.AddItem(v.top, 0, 1, true)
	v.root.AddItem(v.detail, 7, 0, false)
	content.AddItem(v.root, 0, 1, true)
	app.SetFocus(v.list)

	v.list.SetItems([]widgets.SectionItem{{MainText: i18n.T("tui.sessions.loading"), IsHeader: true}})
	v.reload()
	if v.cfg.Backend != nil {
		go v.watchChanges(v.ctx, v.gen, app)
	}
	v.feedMu.Lock()
	on := v.feedOn
	v.feedMu.Unlock()
	if on {
		v.showFeedPanel(true)
	}
}

// Unmount stops background work.
func (v *SessionsView) Unmount() {
	v.gen++
	if v.cancel != nil {
		v.cancel()
	}
	v.stopFollow()
	v.app, v.root, v.top, v.list, v.detail, v.feed = nil, nil, nil, nil, nil, nil
}

// ── Loading ───────────────────────────────────────────────────────────────

func (v *SessionsView) projectFilter() string {
	if v.allProjects || v.shell == nil || v.shell.ActiveProject() == nil {
		return ""
	}
	return v.shell.ActiveProject().ID
}

// reload fetches the sessions off the event loop and redraws.
func (v *SessionsView) reload() {
	if v.cfg.Backend == nil || v.app == nil {
		return
	}
	gen, ctx, app, project := v.gen, v.ctx, v.app, v.projectFilter()
	go func() {
		rows, err := v.cfg.Backend.List(ctx, project, true)
		app.QueueUpdateDraw(func() {
			if v.gen != gen || v.list == nil {
				return
			}
			if err != nil {
				v.toast(err.Error(), false)
				return
			}
			v.rows = rows
			v.render()
		})
	}()
}

func (v *SessionsView) watchChanges(ctx context.Context, gen uint64, app *tview.Application) {
	changes := v.cfg.Backend.Changes(ctx)
	var timer *time.Timer
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-changes:
			if !ok {
				return
			}
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(sessionsReloadDelay, func() {
				app.QueueUpdateDraw(func() {
					if v.gen == gen {
						v.reload()
					}
				})
			})
		}
	}
}

// ── Rendering ─────────────────────────────────────────────────────────────

// sessionsSections groups rows: decisions first, then running, sleeping,
// and recently finished sessions.
func sessionsSections(rows []SessionRow, now time.Time) (decisions []SessionDecision, running, sleeping, finished []SessionRow) {
	for _, r := range rows {
		switch {
		case r.Finished:
			if now.Sub(r.Changed) <= sessionsFinishedFor || r.Changed.IsZero() {
				finished = append(finished, r)
			}
		case r.State == "sleeping":
			sleeping = append(sleeping, r)
			decisions = append(decisions, r.Decisions...)
		default:
			running = append(running, r)
			decisions = append(decisions, r.Decisions...)
		}
	}
	sort.SliceStable(decisions, func(i, j int) bool { return decisions[i].Created.Before(decisions[j].Created) })
	return decisions, running, sleeping, finished
}

func (v *SessionsView) row(id string) *SessionRow {
	for i := range v.rows {
		if v.rows[i].ID == id {
			return &v.rows[i]
		}
	}
	return nil
}

func (v *SessionsView) render() {
	keep := v.focusID
	if keep == "" {
		keep = v.selectedKey()
	}
	v.focusID = ""
	decisions, running, sleeping, finished := sessionsSections(v.rows, time.Now())
	var items []widgets.SectionItem
	header := func(key string, n int) {
		items = append(items, widgets.SectionItem{MainText: i18n.Tf(key, n), IsHeader: true})
	}
	if len(decisions) > 0 {
		header("tui.sessions.section.inbox", len(decisions))
		for _, d := range decisions {
			items = append(items, v.decisionItem(d))
		}
	}
	header("tui.sessions.section.running", len(running))
	if len(running) == 0 {
		items = append(items, widgets.SectionItem{MainText: "  " + muted(i18n.T("tui.sessions.none")), Reference: nil})
	}
	for _, r := range running {
		items = append(items, sessionItem(r))
	}
	if len(sleeping) > 0 {
		header("tui.sessions.section.sleeping", len(sleeping))
		for _, r := range sleeping {
			items = append(items, sessionItem(r))
		}
	}
	if len(finished) > 0 {
		header("tui.sessions.section.finished", len(finished))
		for _, r := range finished {
			items = append(items, sessionItem(r))
		}
	}
	v.list.SetItems(items)
	v.selectKey(keep)
	v.onSelectionChanged()
}

func muted(s string) string {
	return theme.ColorTag(theme.TextMutedHex) + s + theme.TagColor
}

func (v *SessionsView) decisionItem(d SessionDecision) widgets.SectionItem {
	r := v.row(d.SessionID)
	who := ""
	if r != nil {
		who = sessionLabel(*r)
	}
	color := theme.WarningHex
	if d.Kind == DecisionKindError || d.Kind == DecisionKindBudget {
		color = theme.ErrorHex
	}
	return widgets.SectionItem{
		MainText:      fmt.Sprintf("%s%s%s %s  %s", theme.ColorTag(color), d.Icon, theme.TagColor, who, d.Summary),
		SecondaryText: "   " + ago(d.Created),
		Reference:     decisionRef{id: d.ID, session: d.SessionID},
	}
}

func sessionLabel(r SessionRow) string {
	label := r.Workflow
	if label == "" {
		label = r.Agent
	}
	if r.Project != "" {
		label += " · " + r.Project
	}
	return label
}

func sessionItem(r SessionRow) widgets.SectionItem {
	color := theme.ActiveMode.PrimaryHex
	switch r.State {
	case "waiting":
		color = theme.WarningHex
	case "failed":
		color = theme.ErrorHex
	case "sleeping", "stopped", "completed":
		color = theme.TextMutedHex
	}
	main := fmt.Sprintf("%s%s%s %s", theme.ColorTag(color), r.StateIcon, theme.TagColor, sessionLabel(r))
	if r.Title != "" && !strings.Contains(r.Title, r.Agent) {
		main += "  " + muted(r.Title)
	}
	second := fmt.Sprintf("   %s · %s · $%.2f", r.StateLabel, r.Agent, r.Cost)
	if n := len(r.Decisions); n > 0 && !r.Finished {
		second += " · " + i18n.Tf("tui.sessions.waiting_decisions", n)
	}
	return widgets.SectionItem{MainText: main, SecondaryText: second, Reference: sessionRef{id: r.ID}}
}

func ago(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return i18n.T("tui.pm.age.now")
	case d < time.Hour:
		return i18n.Tf("tui.pm.age.minutes", int(d.Minutes()))
	case d < 24*time.Hour:
		return i18n.Tf("tui.pm.age.hours", int(d.Hours()))
	}
	return i18n.Tf("tui.pm.age.days", int(d.Hours()/24))
}

func refKey(ref any) string {
	switch r := ref.(type) {
	case sessionRef:
		return "s:" + r.id
	case decisionRef:
		return "d:" + r.id
	}
	return ""
}

func (v *SessionsView) selectedKey() string {
	if v.list == nil {
		return ""
	}
	if _, it, ok := v.list.CurrentItem(); ok {
		return refKey(it.Reference)
	}
	return ""
}

// selectKey moves the cursor to the item with key (a session ID alone
// selects that session).
func (v *SessionsView) selectKey(key string) {
	if key == "" {
		return
	}
	if !strings.HasPrefix(key, "s:") && !strings.HasPrefix(key, "d:") {
		key = "s:" + key
	}
	for i, it := range v.list.GetItems() {
		if refKey(it.Reference) == key {
			v.list.SelectIndex(i)
			return
		}
	}
}

// selected returns the session (and decision) under the cursor.
func (v *SessionsView) selected() (*SessionRow, *SessionDecision) {
	if v.list == nil {
		return nil, nil
	}
	_, it, ok := v.list.CurrentItem()
	if !ok {
		return nil, nil
	}
	switch ref := it.Reference.(type) {
	case sessionRef:
		return v.row(ref.id), nil
	case decisionRef:
		r := v.row(ref.session)
		if r == nil {
			return nil, nil
		}
		for i := range r.Decisions {
			if r.Decisions[i].ID == ref.id {
				return r, &r.Decisions[i]
			}
		}
		return r, nil
	}
	return nil, nil
}

func (v *SessionsView) onSelectionChanged() {
	r, _ := v.selected()
	v.renderDetail(r)
	v.feedMu.Lock()
	on, cur := v.feedOn, v.feedID
	v.feedMu.Unlock()
	if on && r != nil && r.ID != cur {
		v.startFollow(r)
	}
}

func (v *SessionsView) renderDetail(r *SessionRow) {
	if v.detail == nil {
		return
	}
	if r == nil {
		v.detail.SetTitle("")
		v.detail.SetText(muted(i18n.T("tui.sessions.detail_empty")))
		return
	}
	v.detail.SetTitle(" " + i18n.Tf("tui.sessions.detail_title", sessionLabel(*r)) + " ")
	var b strings.Builder
	parts := []string{}
	for _, p := range []string{r.Workflow, r.Mode, runtimeIcon(r.Runtime), r.Location, shortHash(r.Bundle)} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	b.WriteString(strings.Join(parts, " · ") + "\n")
	fmt.Fprintf(&b, "%s %s · %s · $%.3f · %s\n", r.StateIcon, r.StateLabel, r.Agent, r.Cost, i18n.Tf("tui.sessions.started", ago(r.Started)))
	for _, d := range r.Decisions {
		fmt.Fprintf(&b, "%s%s%s %s\n", theme.ColorTag(theme.WarningHex), d.Icon, theme.TagColor, d.Summary)
	}
	b.WriteString(muted(r.ID))
	v.detail.SetText(b.String())
}

func runtimeIcon(rt string) string {
	switch rt {
	case "local", "":
		return "⌂"
	case "container":
		return "▣"
	case "remote":
		return "☁"
	}
	return rt
}

func shortHash(h string) string {
	if len(h) > 8 {
		return h[:8] + "…"
	}
	return h
}

// ── Keys ──────────────────────────────────────────────────────────────────

// HandleKey processes the view keys (10 §7.1).
func (v *SessionsView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() == tcell.KeyEnter {
		v.onEnter()
		return nil
	}
	if event.Key() != tcell.KeyRune {
		return event
	}
	r, d := v.selected()
	switch event.Rune() {
	case 'r':
		v.reload()
	case 'f':
		v.allProjects = !v.allProjects
		v.toast(i18n.T(map[bool]string{true: "tui.sessions.filter_all", false: "tui.sessions.filter_project"}[v.allProjects]), true)
		v.reload()
	case 't':
		v.toggleFeed(r)
	case 'y':
		if d != nil && d.Kind == DecisionKindPermission {
			v.decide(d, "once", "", nil)
		}
	case 'n':
		if d != nil && d.Kind == DecisionKindPermission {
			v.decide(d, "reject", "", nil)
		}
	case 'x':
		if d != nil && (d.Kind == DecisionKindError || d.Kind == DecisionKindBudget) {
			v.decide(d, "dismiss", "", nil)
		}
	case 'a':
		if r != nil {
			v.cfg.Backend.Attach(r.ID, "")
		}
	case 'A':
		if r != nil && v.shell != nil {
			id := r.ID
			opts := make([]SelectOption, 0, len(attachMethods))
			for _, m := range attachMethods {
				opts = append(opts, SelectOption{Label: i18n.T("tui.sessions.attach_how." + m), Value: m})
			}
			v.shell.ShowSelectModal(i18n.T("tui.sessions.attach_how_title"), opts, "auto", func(how string) {
				v.cfg.Backend.Attach(id, how)
			})
		}
	case 'm':
		if r != nil && v.shell != nil {
			id := r.ID
			v.shell.ShowInputModal(i18n.T("tui.sessions.send_title"), "", func(text string) {
				if strings.TrimSpace(text) != "" {
					v.run(func(ctx context.Context) error { return v.cfg.Backend.Send(ctx, id, text) }, i18n.T("tui.sessions.sent"))
				}
			})
		}
	case 'i':
		if r != nil {
			id := r.ID
			v.run(func(ctx context.Context) error { return v.cfg.Backend.Interrupt(ctx, id) }, i18n.T("tui.sessions.interrupted"))
		}
	case 'M':
		if r != nil && v.shell != nil {
			id := r.ID
			v.shell.ShowInputModal(i18n.T("tui.sessions.model_title"), "", func(model string) {
				if strings.TrimSpace(model) != "" {
					v.run(func(ctx context.Context) error { return v.cfg.Backend.SwitchModel(ctx, id, model) }, i18n.T("tui.sessions.model_switched"))
				}
			})
		}
	case 's':
		if r != nil && !r.Finished && v.shell != nil {
			id := r.ID
			v.shell.ShowSelectModal(i18n.Tf("tui.sessions.stop_confirm", sessionLabel(*r)), []SelectOption{
				{Label: i18n.T("tui.sessions.stop_yes"), Value: "yes"},
				{Label: i18n.T("tui.sessions.cancel"), Value: "no"},
			}, "no", func(val string) {
				if val == "yes" {
					v.run(func(ctx context.Context) error { return v.cfg.Backend.Stop(ctx, id) }, i18n.T("tui.sessions.stopped"))
				}
			})
		}
	case 'c':
		if r != nil && r.State == "sleeping" {
			id := r.ID
			v.run(func(ctx context.Context) error { return v.cfg.Backend.Resume(ctx, id) }, i18n.T("tui.sessions.resumed"))
		}
	case 'o':
		if r != nil {
			v.showMR(r.ID)
		}
	case 'e':
		if ch, ok := v.cfg.Backend.(SessionChainer); ok && r != nil && v.shell != nil {
			id := r.ID
			var opts []SelectOption
			v.async(func(ctx context.Context) (string, error) {
				var err error
				opts, err = ch.ChainOptions(ctx, id)
				return "", err
			}, func(string) {
				if len(opts) == 0 {
					v.toast(i18n.T("tui.launch.chain_none"), false)
					return
				}
				v.shell.ShowSelectModal(i18n.T("tui.launch.chain_title"), opts, "", func(wf string) { ch.Chain(id, wf) })
			})
		}
	case 'w':
		if r != nil && !r.Finished {
			id := r.ID
			v.async(func(ctx context.Context) (string, error) { return v.cfg.Backend.OpenBrowser(ctx, id) }, func(url string) {
				v.toast(i18n.Tf("tui.sessions.browser_opened", url), true)
			})
		}
	default:
		return event
	}
	return nil
}

func (v *SessionsView) onEnter() {
	r, d := v.selected()
	switch {
	case d != nil:
		v.openDecision(r, d)
	case r != nil:
		v.toggleFeed(r)
	}
}

// ── Actions ───────────────────────────────────────────────────────────────

func (v *SessionsView) toast(msg string, ok bool) {
	if v.shell != nil {
		v.shell.ShowToastMsg(msg, ok)
	}
}

// run executes a backend call off the event loop, then toasts and reloads.
func (v *SessionsView) run(fn func(ctx context.Context) error, okMsg string) {
	v.async(func(ctx context.Context) (string, error) { return okMsg, fn(ctx) }, func(msg string) {
		v.toast(msg, true)
		v.reload()
	})
}

func (v *SessionsView) async(fn func(ctx context.Context) (string, error), done func(string)) {
	if v.cfg.Backend == nil || v.app == nil {
		return
	}
	ctx, app, gen := v.ctx, v.app, v.gen
	go func() {
		res, err := fn(ctx)
		app.QueueUpdateDraw(func() {
			if v.gen != gen {
				return
			}
			if err != nil {
				v.toast(err.Error(), false)
				v.reload()
				return
			}
			done(res)
		})
	}()
}

func (v *SessionsView) decide(d *SessionDecision, choice, message string, answers map[string]string) {
	id := d.ID
	v.run(func(ctx context.Context) error {
		return v.cfg.Backend.Decide(ctx, id, choice, message, answers)
	}, i18n.T("tui.inbox.answered"))
}

func (v *SessionsView) showMR(id string) {
	v.async(func(ctx context.Context) (string, error) { return v.cfg.Backend.MRDescription(ctx, id) }, func(md string) {
		if v.shell != nil {
			v.shell.ShowScrollableModal(i18n.T("tui.sessions.mr_title"), tview.Escape(md), nil)
		}
	})
}

// ── Decision cards (simple): permission, question, alerts ─────────────────
// The checkpoint card (diff, last messages) belongs to the checkpoint track.

func (v *SessionsView) openDecision(r *SessionRow, d *SessionDecision) {
	if v.shell == nil {
		return
	}
	who := d.Kind
	if r != nil {
		who = sessionLabel(*r)
	}
	dec := *d
	switch d.Kind {
	case DecisionKindPermission:
		// Hints are not counted by the form sizing: the request goes in the title.
		v.shell.ShowInlineForm(InlineFormConfig{
			Title: i18n.Tf("tui.inbox.permission_title", who) + " — " + tview.Escape(clipText(d.Summary, 60)),
			Fields: []FormField{
				{Key: "decision", Label: i18n.T("tui.inbox.decision"), Type: FieldSelect, Default: "once",
					Options: []SelectOption{
						{Label: i18n.T("tui.inbox.once"), Value: "once"},
						{Label: i18n.T("tui.inbox.always"), Value: "always"},
						{Label: i18n.T("tui.inbox.reject"), Value: "reject"},
					}},
				{Key: "message", Label: i18n.T("tui.inbox.message"), Type: FieldText},
			},
			OnSubmit: func(values map[string]string, _ map[string][]string) {
				v.decide(&dec, values["decision"], values["message"], nil)
			},
		})
	case DecisionKindQuestion:
		fields, toAnswers := questionForm(d.Fields)
		v.shell.ShowInlineForm(InlineFormConfig{
			Title:  i18n.Tf("tui.inbox.question_title", who),
			Fields: fields,
			OnSubmit: func(values map[string]string, multi map[string][]string) {
				v.decide(&dec, "", "", toAnswers(values, multi))
			},
		})
	case DecisionKindError, DecisionKindBudget:
		text := d.Summary
		if d.Message != "" && d.Message != d.Summary {
			text += "\n\n" + d.Message
		}
		id := ""
		if r != nil {
			id = r.ID
		}
		v.shell.ShowScrollableModal(d.Icon+" "+who, tview.Escape(text), []ModalAction{
			{Label: i18n.T("tui.inbox.dismiss"), Callback: func() { v.decide(&dec, "dismiss", "", nil) }},
			{Label: i18n.T("tui.sessions.attach"), Callback: func() {
				if id != "" {
					v.cfg.Backend.Attach(id, "")
				}
			}},
		})
	default:
		id := ""
		if r != nil {
			id = r.ID
		}
		v.shell.ShowScrollableModal(d.Icon+" "+who, tview.Escape(d.Summary), []ModalAction{
			{Label: i18n.T("tui.inbox.approve"), Callback: func() { v.decide(&dec, "approve", "", nil) }},
			{Label: i18n.T("tui.sessions.attach"), Callback: func() {
				if id != "" {
					v.cfg.Backend.Attach(id, "")
				}
			}},
		})
	}
}

// customSuffix is the form key of the free answer of a choice field.
const customSuffix = "\x00custom"

// questionForm builds the form of an agent question and the function that
// turns the submitted values into textual answers (typed by the service).
func questionForm(fields []SessionDecisionField) (out []FormField, answers func(map[string]string, map[string][]string) map[string]string) {
	for _, f := range fields {
		// The description is the question itself: it is the label (form hints
		// are not counted by the form sizing).
		label := clipText(firstText(f.Description, f.Title, f.Key), 60)
		switch {
		case f.Type == "external":
			out = append(out, FormField{Key: f.Key, Label: label + " — " + i18n.T("tui.inbox.external"), Type: FieldText})
		case f.Type == "boolean":
			out = append(out, FormField{Key: f.Key, Label: label, Type: FieldBool, Default: "false"})
		case f.Type == "multiselect":
			out = append(out, FormField{Key: f.Key, Label: label, Type: FieldMultiSelect, Options: f.Options, Required: f.Required})
			if f.Custom {
				out = append(out, FormField{Key: f.Key + customSuffix, Label: i18n.T("tui.inbox.custom"), Type: FieldText})
			}
		case len(f.Options) > 0:
			def := f.Options[0].Value
			out = append(out, FormField{Key: f.Key, Label: label, Type: FieldSelect, Options: f.Options, Default: def})
			if f.Custom {
				out = append(out, FormField{Key: f.Key + customSuffix, Label: i18n.T("tui.inbox.custom"), Type: FieldText})
			}
		default:
			out = append(out, FormField{Key: f.Key, Label: label, Type: FieldText, Required: f.Required})
		}
	}
	return out, func(values map[string]string, multi map[string][]string) map[string]string {
		ans := map[string]string{}
		for _, f := range fields {
			if f.Type == "external" {
				continue
			}
			custom := strings.TrimSpace(values[f.Key+customSuffix])
			switch {
			case f.Type == "multiselect":
				list := append([]string(nil), multi[f.Key]...)
				if custom != "" {
					list = append(list, custom)
				}
				if len(list) > 0 {
					ans[f.Key] = strings.Join(list, ",")
				}
			case custom != "":
				ans[f.Key] = custom
			default:
				if val, ok := values[f.Key]; ok && (val != "" || f.Required) {
					ans[f.Key] = val
				}
			}
		}
		return ans
	}
}

// ── Live feed (t) ─────────────────────────────────────────────────────────

func (v *SessionsView) toggleFeed(r *SessionRow) {
	v.feedMu.Lock()
	on := !v.feedOn
	v.feedOn = on
	v.feedMu.Unlock()
	v.showFeedPanel(on)
	if on && r != nil {
		v.startFollow(r)
	} else {
		v.stopFollow()
	}
}

func (v *SessionsView) showFeedPanel(on bool) {
	if v.top == nil {
		return
	}
	v.top.Clear()
	v.top.AddItem(v.list, 0, 1, true)
	if on {
		v.top.AddItem(v.feed, 0, 1, false)
	}
}

func (v *SessionsView) stopFollow() {
	v.feedMu.Lock()
	if v.feedCancel != nil {
		v.feedCancel()
	}
	v.feedCancel, v.feedID = nil, ""
	v.feedMu.Unlock()
}

func (v *SessionsView) startFollow(r *SessionRow) {
	v.stopFollow()
	if v.cfg.Backend == nil || v.feed == nil || v.app == nil {
		return
	}
	ctx, cancel := context.WithCancel(v.ctx)
	v.feedMu.Lock()
	v.feedCancel, v.feedID, v.feedLines, v.feedCost = cancel, r.ID, nil, r.Cost
	v.feedMu.Unlock()
	title := sessionLabel(*r)
	v.renderFeed(title)
	app, gen, id := v.app, v.gen, r.ID
	go func() {
		lines, err := v.cfg.Backend.Follow(ctx, id)
		if err != nil {
			app.QueueUpdateDraw(func() {
				if v.gen == gen && v.feed != nil {
					v.feed.SetText(muted(err.Error()))
				}
			})
			return
		}
		for l := range lines {
			line := l
			v.feedMu.Lock()
			if v.feedID != id {
				v.feedMu.Unlock()
				return
			}
			if line.Cost > 0 {
				v.feedCost = line.Cost
			}
			if line.Text != "" {
				v.feedLines = append(v.feedLines, fmt.Sprintf("%s %s", muted(line.Time.Local().Format("15:04")), tview.Escape(line.Text)))
				if len(v.feedLines) > sessionsFeedMax {
					v.feedLines = v.feedLines[len(v.feedLines)-sessionsFeedMax:]
				}
			}
			v.feedMu.Unlock()
			app.QueueUpdateDraw(func() {
				if v.gen == gen {
					v.renderFeed(title)
				}
			})
		}
	}()
}

func (v *SessionsView) renderFeed(title string) {
	if v.feed == nil {
		return
	}
	v.feedMu.Lock()
	lines := strings.Join(v.feedLines, "\n")
	cost := v.feedCost
	v.feedMu.Unlock()
	v.feed.SetTitle(fmt.Sprintf(" %s · $%.2f ", i18n.Tf("tui.sessions.feed_title", title), cost))
	if lines == "" {
		lines = muted(i18n.T("tui.sessions.feed_waiting"))
	}
	v.feed.SetText(lines)
	v.feed.ScrollToEnd()
}

func firstText(v ...string) string {
	for _, s := range v {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func clipText(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
