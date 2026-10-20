package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// ─────────────────────────────────────────────────────────────────────────────
// Fan-out tab (R6, modernisation tui-sessions): the state of the fan-out of
// the session (its parent for a child), the children (ticket, session,
// state, branch, cost, end), the fix rounds, the last refusal of the merge
// checkpoint, the integration branch and the budget; Enter opens a child,
// p starts the driver again (`oh fanout drive`), d cleans up (`oh fanout
// clean`, preview, --discard to tick, confirmed), B the batch.
// ─────────────────────────────────────────────────────────────────────────────

// FanoutView is the fan-out of a parent, display-ready.
type FanoutView struct {
	Parent       string
	Experimental bool
	Status       string // waiting | running | ready | fixing | merged | failed | stopped
	Parallel     int
	// Driver: the driver of the fan-out runs (its lock is held).
	Driver    bool
	BudgetUSD float64
	SpentUSD  float64
	Children  []FanoutChildView
	// FixRounds are the fix rounds run, FixMax those the workflow allows.
	FixRounds int
	FixMax    int
	// LastRefusal is the instruction of the last refusal of the merge
	// checkpoint ("" = none).
	LastRefusal string
	Integration string
	Commits     int // commits of the integration branch since its base (-1 = unknown)
	// Cleanable: the merge checkpoint passed or the parent stopped for good.
	Cleanable bool
	Cleaned   bool
}

// FanoutChildView is a child of a fan-out.
type FanoutChildView struct {
	Ticket  string
	Session string
	State   string // pending | running | merged | conflict | failed | budget
	Branch  string
	CostUSD float64
	End     string // why it ended ("" = not yet)
	Detail  string // error or conflicting files
}

// FanoutCleanView is what a clean-up removes and keeps.
type FanoutCleanView struct {
	Worktrees []string
	Branches  []string
	Discarded []string // "branch commit": deleted with commits found nowhere else
	Kept      []string // "branch: reason"
}

// Empty reports a clean-up with nothing to remove.
func (c FanoutCleanView) Empty() bool {
	return len(c.Worktrees) == 0 && len(c.Branches) == 0 && len(c.Discarded) == 0
}

// SessionFanouter is implemented by backends able to show and act on a
// fan-out (`oh fanout status|drive|clean`).
type SessionFanouter interface {
	// Fanout is the fan-out of a session (its parent's for a child).
	Fanout(ctx context.Context, sessionID string) (FanoutView, error)
	// RelaunchFanout starts the driver of a parent again; it returns the
	// message of the toast.
	RelaunchFanout(ctx context.Context, parentID string) (string, error)
	// CleanFanout cleans up the fan-out of a parent (dry: only plans).
	CleanFanout(ctx context.Context, parentID string, discard, dry bool) (FanoutCleanView, error)
}

// Own keys of the view: p starts the driver again, B opens the batch, a
// attaches, m sends an instruction, i interrupts the turn.
var (
	relaunchKey  = keys.Own("sessions.relaunch", 'p', "tui.sessions.fanout.relaunch")
	batchKey     = keys.Own("sessions.batch", 'B', "tui.sessions.batch_label")
	attachKey    = keys.Own("sessions.attach", 'a', "tui.sessions.key.attach")
	sendKey      = keys.Own("sessions.send", 'm', "tui.sessions.key.send")
	interruptKey = keys.Own("sessions.interrupt", 'i', "tui.sessions.key.interrupt")
)

type fanoutPane struct {
	v      *SessionsView
	root   *tview.Flex
	head   *tview.TextView
	list   *ActionList
	foot   *tview.TextView
	view   *FanoutView
	parent string
}

func newFanoutPane(v *SessionsView) *fanoutPane {
	p := &fanoutPane{v: v}
	p.head = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	p.head.SetBackgroundColor(theme.BgPanel)
	p.foot = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	p.foot.SetBackgroundColor(theme.BgPanel)
	p.list = NewActionList(ActionListConfig{
		Title: i18n.T("tui.sessions.fanout.children"),
		Headers: []string{i18n.T("tui.sessions.fanout.col.ticket"), i18n.T("tui.sessions.fanout.col.session"),
			i18n.T("tui.sessions.fanout.col.branch"), i18n.T("tui.sessions.fanout.col.cost"), i18n.T("tui.sessions.fanout.col.end")},
		StateHeader: i18n.T("tui.sessions.fanout.col.state"),
		Layout:      DetailNone,
		Empty:       i18n.T("tui.sessions.fanout.loading"),
		OnOpen:      p.openChild,
		OpenLabel:   i18n.T("tui.sessions.fanout.open"),
	})
	p.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(p.head, 2, 0, false).
		AddItem(p.list.Primitive(), 0, 1, true).
		AddItem(p.foot, 2, 0, false)
	p.root.SetBackgroundColor(theme.BgPanel)
	return p
}

func (p *fanoutPane) reset() {
	p.view, p.parent = nil, ""
	p.head.SetText("")
	p.foot.SetText("")
	p.list.SetEmpty(i18n.T("tui.sessions.fanout.loading"))
	p.list.SetItems(nil)
}

func (p *fanoutPane) hints(tab, back string) string {
	return keys.Hints(keys.Open.HintAs(i18n.T("tui.sessions.fanout.open")),
		relaunchKey.Hint(),
		keys.Delete.HintAs(i18n.T("tui.sessions.fanout.clean")),
		batchKey.Hint(),
		keys.Refresh.Hint(), tab, back)
}

func (v *SessionsView) loadFanout(id string) {
	p := v.tabs.fanout
	fb, ok := v.cfg.Backend.(SessionFanouter)
	if !ok {
		p.list.SetEmpty(i18n.T("tui.sessions.fanout.unavailable"))
		p.list.SetItems(nil)
		return
	}
	var fv FanoutView
	v.async(func(ctx context.Context) (string, error) {
		var err error
		fv, err = fb.Fanout(ctx, id)
		return "", err
	}, func(string) {
		if v.tabs == nil || v.tabs.session != id {
			return
		}
		p.set(fv)
	})
}

// set shows a fan-out.
func (p *fanoutPane) set(fv FanoutView) {
	p.view, p.parent = &fv, fv.Parent
	p.head.SetText(fanoutHead(fv))
	p.foot.SetText(fanoutFoot(fv))
	p.list.SetEmpty(i18n.T("tui.sessions.fanout.no_child"))
	p.list.SetItems(fanoutItems(fv))
}

func fanoutStatus(s string) string {
	if s == "" {
		return "—"
	}
	return i18n.T("tui.sessions.fanout.status." + s)
}

// fanoutHead is the first lines: experimental, parallel, driver, budget.
func fanoutHead(fv FanoutView) string {
	parts := []string{}
	if fv.Experimental {
		parts = append(parts, theme.ColorTag(theme.WarningHex)+i18n.T("tui.sessions.fanout.experimental")+theme.TagColor)
	}
	parts = append(parts, tview.Escape(fanoutStatus(fv.Status)))
	if fv.Parallel > 0 {
		parts = append(parts, i18n.Tf("tui.sessions.fanout.parallel", fv.Parallel))
	}
	if fv.Driver {
		parts = append(parts, theme.ColorTag(theme.SuccessHex)+i18n.T("tui.sessions.fanout.driver_on")+theme.TagColor)
	} else {
		parts = append(parts, muted(i18n.T("tui.sessions.fanout.driver_off")))
	}
	if fv.BudgetUSD > 0 {
		parts = append(parts, i18n.Tf("tui.sessions.fanout.budget", fv.SpentUSD, fv.BudgetUSD)+" "+budgetBar(fv.SpentUSD, fv.BudgetUSD, 8))
	} else if fv.SpentUSD > 0 {
		parts = append(parts, fmt.Sprintf("$%.2f", fv.SpentUSD))
	}
	return strings.Join(parts, muted(" · ")) + "\n" + muted(tview.Escape(i18n.Tf("tui.sessions.fanout.parent", fv.Parent)))
}

// budgetBar draws the share of the budget spent.
func budgetBar(spent, budget float64, width int) string {
	n := 0
	if budget > 0 {
		n = int(spent / budget * float64(width))
	}
	n = min(max(n, 0), width)
	color := theme.SuccessHex
	if spent >= budget*0.8 {
		color = theme.WarningHex
	}
	return theme.ColorTag(color) + strings.Repeat("▓", n) + theme.TagColor + muted(strings.Repeat("░", width-n))
}

// fanoutFoot is the last lines: fix rounds, last refusal, integration.
func fanoutFoot(fv FanoutView) string {
	var lines []string
	fix := i18n.Tf("tui.sessions.fanout.fix_rounds", fv.FixRounds, fv.FixMax)
	if fv.LastRefusal != "" {
		fix += muted(" · ") + tview.Escape(i18n.Tf("tui.sessions.fanout.last_refusal", clipText(fv.LastRefusal, 60)))
	}
	lines = append(lines, fix)
	switch {
	case fv.Integration == "":
		lines = append(lines, muted(i18n.T("tui.sessions.fanout.no_integration")))
	case fv.Cleaned:
		lines = append(lines, tview.Escape(i18n.Tf("tui.sessions.fanout.integration", fv.Integration))+" "+muted(i18n.T("tui.sessions.fanout.cleaned")))
	case fv.Commits >= 0:
		lines = append(lines, tview.Escape(i18n.Tf("tui.sessions.fanout.integration_commits", fv.Integration, fv.Commits)))
	default:
		lines = append(lines, tview.Escape(i18n.Tf("tui.sessions.fanout.integration", fv.Integration)))
	}
	return strings.Join(lines, "\n")
}

// childState is the icon and label of the state of a child.
func childState(s string) (string, ItemState) {
	label := i18n.T("tui.sessions.fanout.child." + s)
	switch s {
	case "merged":
		return "✓ " + label, ItemAccepted
	case "running":
		return "● " + label, ItemNone
	case "pending":
		return "… " + label, ItemNone
	case "conflict", "failed", "budget":
		return "✗ " + label, ItemRefused
	}
	return s, ItemNone
}

func fanoutItems(fv FanoutView) []ActionItem {
	items := make([]ActionItem, 0, len(fv.Children))
	for _, c := range fv.Children {
		state, st := childState(c.State)
		cost := "—"
		if c.CostUSD > 0 {
			cost = fmt.Sprintf("$%.2f", c.CostUSD)
		}
		session := "—"
		if c.Session != "" {
			session = c.Session
		}
		items = append(items, ActionItem{
			ID:        firstText(c.Ticket, c.Session),
			Cells:     []string{c.Ticket, session, firstText(c.Branch, "—"), cost, firstText(c.End, "—")},
			State:     st,
			StateText: state,
			Ref:       c,
		})
	}
	return items
}

// openChild selects the session of a child in the list.
func (p *fanoutPane) openChild(it ActionItem) {
	c, ok := it.Ref.(FanoutChildView)
	if !ok || c.Session == "" {
		p.v.toast(i18n.T("tui.sessions.fanout.not_started"), false)
		return
	}
	p.v.showTab(tabDetail)
	if p.v.row(c.Session) == nil {
		p.v.toast(i18n.Tf("tui.sessions.fanout.child_hidden", c.Session), false)
		return
	}
	p.v.selectKey("s:" + c.Session)
	p.v.onSelectionChanged()
}

func (p *fanoutPane) handleKey(ev *tcell.EventKey) *tcell.EventKey {
	switch {
	case keys.Back.Matches(ev):
		p.v.showTab(tabDetail)
		return nil
	case keys.Refresh.Matches(ev):
		p.v.tabs.session = ""
		p.v.showTab(tabFanout)
		return nil
	case keys.Delete.Matches(ev):
		p.clean()
		return nil
	case relaunchKey.Matches(ev):
		p.relaunch()
		return nil
	case batchKey.Matches(ev):
		r := p.v.row(p.parent)
		p.v.openBatch(r)
		return nil
	}
	return ev
}

// relaunch starts the driver of the fan-out again.
func (p *fanoutPane) relaunch() {
	fb, ok := p.v.cfg.Backend.(SessionFanouter)
	if !ok || p.view == nil {
		return
	}
	if p.view.Driver {
		p.v.toast(i18n.T("tui.sessions.fanout.driver_running"), false)
		return
	}
	parent := p.parent
	p.v.async(func(ctx context.Context) (string, error) { return fb.RelaunchFanout(ctx, parent) }, func(msg string) {
		p.v.toast(msg, true)
		p.refresh()
	})
}

func (p *fanoutPane) refresh() {
	if p.v.tabs != nil && p.v.tabs.cur == tabFanout {
		p.v.tabs.session = ""
		p.v.showTab(tabFanout)
	}
}

// clean previews the clean-up (what goes, what stays; --discard to
// tick), then asks the confirmation.
func (p *fanoutPane) clean() {
	fb, ok := p.v.cfg.Backend.(SessionFanouter)
	if !ok || p.view == nil || p.v.shell == nil {
		return
	}
	if !p.view.Cleanable {
		p.v.toast(i18n.Tf("tui.sessions.fanout.clean_not_done", fanoutStatus(p.view.Status)), false)
		return
	}
	p.previewClean(fb, p.parent, false)
}

func (p *fanoutPane) previewClean(fb SessionFanouter, parent string, discard bool) {
	var plan FanoutCleanView
	p.v.async(func(ctx context.Context) (string, error) {
		var err error
		plan, err = fb.CleanFanout(ctx, parent, discard, true)
		return "", err
	}, func(string) {
		box := "[ ]"
		if discard {
			box = "[x]"
		}
		actions := []ModalAction{
			{Label: tview.Escape(box) + " " + i18n.T("tui.sessions.fanout.discard"), Callback: func() { p.previewClean(fb, parent, !discard) }},
			{Label: i18n.T("tui.sessions.fanout.clean_do"), Danger: true, Separator: true, Callback: func() {
				if plan.Empty() {
					p.v.toast(i18n.T("tui.sessions.fanout.clean_nothing"), false)
					return
				}
				p.v.shell.Confirm(i18n.Tf("tui.sessions.fanout.clean_confirm", shortSessionID(parent)), cleanPreview(plan, discard), true, func() {
					p.doClean(fb, parent, discard)
				})
			}},
			{Label: i18n.T("tui.sessions.cancel"), Callback: func() {}},
		}
		p.v.shell.ShowScrollableModal(i18n.Tf("tui.sessions.fanout.clean_title", shortSessionID(parent)), cleanPreview(plan, discard), actions)
	})
}

func (p *fanoutPane) doClean(fb SessionFanouter, parent string, discard bool) {
	var res FanoutCleanView
	p.v.async(func(ctx context.Context) (string, error) {
		var err error
		res, err = fb.CleanFanout(ctx, parent, discard, false)
		return "", err
	}, func(string) {
		p.v.toast(i18n.Tf("tui.sessions.fanout.cleaned_toast", len(res.Worktrees), len(res.Branches)+len(res.Discarded), len(res.Kept)), true)
		p.refresh()
		p.v.reload()
	})
}

// cleanPreview lists what a clean-up removes and keeps (escaped).
func cleanPreview(c FanoutCleanView, discard bool) string {
	var b strings.Builder
	line := func(label string, list []string, color string) {
		if len(list) == 0 {
			return
		}
		fmt.Fprintf(&b, "%s%s%s\n", theme.ColorTag(color), label, theme.TagColor)
		for _, s := range list {
			b.WriteString("  " + tview.Escape(s) + "\n")
		}
	}
	if c.Empty() {
		b.WriteString(muted(i18n.T("tui.sessions.fanout.clean_nothing")) + "\n")
	}
	line(i18n.T("tui.sessions.fanout.removed_worktrees"), c.Worktrees, theme.ErrorHex)
	line(i18n.T("tui.sessions.fanout.removed_branches"), c.Branches, theme.ErrorHex)
	line(i18n.T("tui.sessions.fanout.discarded"), c.Discarded, theme.ErrorHex)
	line(i18n.T("tui.sessions.fanout.kept"), c.Kept, theme.SuccessHex)
	if len(c.Kept) > 0 && !discard {
		b.WriteString("\n" + muted(i18n.T("tui.sessions.fanout.kept_hint")) + "\n")
	}
	return b.String()
}
