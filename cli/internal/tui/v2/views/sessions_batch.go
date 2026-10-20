package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/glyph"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
)

// ─────────────────────────────────────────────────────────────────────────────
// Validation by batch (R6, CP-R6; `oh session approve --batch` in the CLI):
// B lists the checkpoints waiting (those of the fan-out of the selected
// session, else every one) in place of the list, each with its session, its
// summary and the verification by oh, filtered by checkpoint (/), each one
// validated, left waiting or refused with an instruction (y/s/n), a failed
// verification opened on its card (Enter); a confirmation (C), then the
// backend answers them one by one through the same service as the CLI and
// the result of each is shown.
// ─────────────────────────────────────────────────────────────────────────────

// Batch actions (sessionsvc.BatchAction).
const (
	BatchApprove = "approve"
	BatchKeep    = "keep"
	BatchReject  = "reject"
)

// BatchDecision is a checkpoint waiting, in a batch.
type BatchDecision struct {
	ID         string
	SessionID  string
	Checkpoint string
	Summary    string
	// Verify is the state of the verification by oh ("—" = none).
	Verify string
	// Approver lists the approvers the checkpoint names (CP-R9, "" =
	// none); ApproverMismatch: Identity, who validates, is none of them
	// (warned on the line, traced, not refused).
	Approver         string
	Identity         string
	ApproverMismatch bool
}

// BatchChoice is what to do with a decision of the batch.
type BatchChoice struct {
	ID      string
	Action  string // BatchApprove, BatchKeep, BatchReject
	Message string
}

// BatchResult is the answer of a decision of the batch.
type BatchResult struct {
	BatchDecision
	Action string
	// Error is why it could not be answered (it stays waiting).
	Error string
}

// SessionBatcher is implemented by backends able to validate by batch.
type SessionBatcher interface {
	// BatchItems lists the checkpoints waiting: in the fan-out of
	// sessionID when it is in one, else in every session ("" = every one).
	BatchItems(ctx context.Context, sessionID string) ([]BatchDecision, error)
	// ApplyBatch answers them one by one, as chosen.
	ApplyBatch(ctx context.Context, choices []BatchChoice) ([]BatchResult, error)
}

// Failed reports whether only an override passes the checkpoint (the
// verification by oh failed or cannot run): it is not validated in a
// batch, its card is opened instead.
func (d BatchDecision) Failed() bool {
	return d.Verify == verifyFail || d.Verify == verifyUnavailable
}

// verifyCell is the verification column of a batch line.
func (d BatchDecision) verifyCell() string {
	switch d.Verify {
	case verifyPass:
		return "✓ " + i18n.T("tui.batch.verify_ok")
	case verifyFail, verifyUnavailable:
		return "✗ " + i18n.T("tui.batch.verify_failed")
	case verifyRunning:
		return "… " + i18n.T("tui.batch.verify_running")
	}
	return "—"
}

// batchKeep is the state of a decision left waiting.
const batchKeepState ItemState = "keep"

// batchPane is the batch, shown in place of the list (B): one line per
// checkpoint waiting, its choice (validate, leave waiting, refuse with an
// instruction), a filter by checkpoint (/, as --checkpoint), the card of a
// line on Enter (the only way for a failed verification), C to confirm.
type batchPane struct {
	v      *SessionsView
	b      SessionBatcher
	scope  string // session of the batch ("" = every one)
	parent string // parent of the fan-out of the scope ("" = none)
	all    []BatchDecision
	filter string
	choice map[string]BatchChoice
	root   *tview.Flex
	list   *ActionList
	foot   *tview.TextView
}

// confirmBatchKey confirms the batch (a letter of the view).
var confirmBatchKey = keys.Own("batch.confirm", 'C', "tui.batch.confirm_key")

// openBatch loads the checkpoints of the batch, off the event loop, then
// shows them in place of the list; the checkpoint of the decision selected
// is the first filter.
func (v *SessionsView) openBatch(r *SessionRow, d ...*SessionDecision) {
	b, ok := v.cfg.Backend.(SessionBatcher)
	if !ok || v.shell == nil || v.content == nil {
		return
	}
	id, parent := "", ""
	if r != nil {
		id = r.ID
		if r.InFanout() {
			parent = firstText(r.FanoutParent, r.ID)
		}
	}
	decision := ""
	if len(d) > 0 && d[0] != nil {
		decision = d[0].ID
	}
	var items []BatchDecision
	v.async(func(ctx context.Context) (string, error) {
		var err error
		items, err = b.BatchItems(ctx, id)
		return "", err
	}, func(string) {
		if len(items) == 0 {
			v.toast(i18n.T("tui.batch.empty"), false)
			return
		}
		p := newBatchPane(v, b, id, parent, items)
		for _, it := range items {
			if it.ID == decision {
				p.filter = it.Checkpoint
				p.render()
			}
		}
		v.showBatch(p)
	})
}

func newBatchPane(v *SessionsView, b SessionBatcher, scope, parent string, items []BatchDecision) *batchPane {
	p := &batchPane{v: v, b: b, scope: scope, parent: parent, choice: map[string]BatchChoice{}}
	passable := func(it ActionItem) bool { d, _ := it.Ref.(BatchDecision); return !d.Failed() }
	p.list = NewActionList(ActionListConfig{
		Headers: []string{i18n.T("tui.batch.col.session"), i18n.T("tui.batch.col.checkpoint"), i18n.T("tui.batch.col.summary"),
			i18n.T("tui.batch.col.verify")},
		StateHeader: i18n.T("tui.batch.col.choice"),
		StateLabels: map[ItemState]string{
			ItemAccepted:   "✓ " + i18n.T("tui.batch.choice.approve"),
			ItemRefused:    "✗ " + i18n.T("tui.batch.choice.reject"),
			batchKeepState: "· " + i18n.T("tui.batch.choice.keep"),
		},
		Empty: i18n.T("tui.batch.filter_empty"),
		Shell: v.shell,
		Actions: []ItemAction{
			{Key: keys.Accept, Label: i18n.T("tui.batch.choice.approve"), Applies: passable, Run: func(it ActionItem, _ string) { p.set(it.ID, BatchApprove, "") }},
			{Key: keys.Refuse, Run: func(it ActionItem, reason string) { p.set(it.ID, BatchReject, reason) },
				ReasonTitle: func(it ActionItem) string { return i18n.Tf("tui.batch.reason_title", it.ID) }},
			{Key: keys.Leave, Run: func(it ActionItem, _ string) { p.set(it.ID, BatchKeep, "") }},
		},
		Global:    []GlobalAction{{Key: keys.AcceptAll, Label: i18n.T("tui.batch.approve_all"), Run: func([]ActionItem) { p.approveAll() }}},
		OnOpen:    p.openCard,
		OpenLabel: i18n.T("tui.batch.open_card"),
	})
	p.foot = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	p.foot.SetBackgroundColor(theme.BgPanel)
	p.root = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(p.list.Primitive(), 0, 1, true).
		AddItem(p.foot, 3, 0, false)
	p.root.SetBackgroundColor(theme.BgPanel)
	p.setItems(items)
	return p
}

// setItems takes the decisions waiting (the choices made are kept).
func (p *batchPane) setItems(items []BatchDecision) {
	p.all = items
	for _, d := range items {
		if _, ok := p.choice[d.ID]; !ok {
			action := BatchApprove
			if d.Failed() {
				action = BatchKeep
			}
			p.choice[d.ID] = BatchChoice{ID: d.ID, Action: action}
		}
	}
	p.render()
}

// visible are the decisions of the filter.
func (p *batchPane) visible() []BatchDecision {
	var out []BatchDecision
	for _, d := range p.all {
		if p.filter == "" || d.Checkpoint == p.filter {
			out = append(out, d)
		}
	}
	return out
}

func (p *batchPane) title() string {
	scope := i18n.T("tui.batch.scope_all")
	if p.parent != "" {
		scope = i18n.Tf("tui.batch.scope_fanout", shortSessionID(p.parent))
	}
	filter := i18n.T("tui.batch.filter_all")
	if p.filter != "" {
		filter = p.filter
	}
	return i18n.Tf("tui.batch.pane_title", scope, filter, len(p.visible()))
}

func (p *batchPane) render() {
	p.list.SetTitle(tview.Escape(p.title()))
	vis := p.visible()
	items := make([]ActionItem, 0, len(vis))
	for _, d := range vis {
		items = append(items, p.item(d))
	}
	p.list.SetItems(items)
	n := map[string]int{}
	for _, d := range vis {
		n[p.choice[d.ID].Action]++
	}
	p.foot.SetText(i18n.Tf("tui.batch.count", n[BatchApprove], n[BatchReject], n[BatchKeep]) + "\n" + muted(p.hints()))
}

func (p *batchPane) hints() string {
	return keys.Hints(p.list.Hints(), keys.Filter.HintAs(i18n.T("tui.batch.filter")),
		confirmBatchKey.Hint(), keys.Back.HintAs(i18n.T("tui.batch.cancel")))
}

func (p *batchPane) item(d BatchDecision) ActionItem {
	c := p.choice[d.ID]
	state := batchKeepState
	switch c.Action {
	case BatchApprove:
		state = ItemAccepted
	case BatchReject:
		state = ItemRefused
	}
	who := shortSessionID(d.SessionID)
	if r := p.v.row(d.SessionID); r != nil {
		who += " " + sessionLabel(*r)
	}
	verify := d.verifyCell()
	if d.ApproverMismatch {
		verify += "  ⚠ " + i18n.T("tui.batch.out_of_approver")
	}
	var detail strings.Builder
	detail.WriteString(tview.Escape(firstText(d.Summary, "—")) + "\n")
	fmt.Fprintf(&detail, "%s\n", tview.Escape(i18n.Tf("tui.batch.verify", firstText(d.Verify, "—"))))
	if d.Failed() {
		fmt.Fprintf(&detail, "%s%s%s\n", theme.ColorTag(theme.ErrorHex), i18n.T("tui.batch.failed_hint"), theme.TagColor)
	}
	if d.Approver != "" {
		detail.WriteString(tview.Escape(i18n.Tf("tui.checkpoint.approver", d.Approver)) + "\n")
	}
	if d.ApproverMismatch {
		fmt.Fprintf(&detail, "%s%s%s\n", theme.ColorTag(theme.WarningHex), tview.Escape(i18n.Tf("tui.batch.approver_mismatch", d.Approver)), theme.TagColor)
	}
	if c.Message != "" {
		detail.WriteString(tview.Escape(i18n.Tf("tui.batch.refusal", c.Message)) + "\n")
	}
	return ActionItem{ID: d.ID, Cells: []string{clipText(who, 28), firstText(d.Checkpoint, "—"), clipText(firstText(d.Summary, "—"), 36), verify},
		State: state, DetailTitle: firstText(d.Checkpoint, d.ID) + " · " + shortSessionID(d.SessionID), Detail: detail.String(), Ref: d}
}

func (p *batchPane) set(id, action, message string) {
	p.choice[id] = BatchChoice{ID: id, Action: action, Message: strings.TrimSpace(message)}
	p.render()
}

func (p *batchPane) approveAll() {
	for _, d := range p.visible() {
		if !d.Failed() {
			p.choice[d.ID] = BatchChoice{ID: d.ID, Action: BatchApprove}
		}
	}
	p.render()
}

// chooseFilter asks the checkpoint of the batch (/, as --checkpoint).
func (p *batchPane) chooseFilter() {
	seen := map[string]bool{}
	opts := []SelectOption{{Label: i18n.T("tui.batch.filter_all"), Value: ""}}
	for _, d := range p.all {
		if d.Checkpoint != "" && !seen[d.Checkpoint] {
			seen[d.Checkpoint] = true
			opts = append(opts, SelectOption{Label: d.Checkpoint, Value: d.Checkpoint})
		}
	}
	p.v.shell.ShowSelectModal(i18n.T("tui.batch.filter_title"), opts, p.filter, func(cp string) {
		p.filter = cp
		p.render()
	})
}

// openCard opens the card of a checkpoint of the batch (its decision).
func (p *batchPane) openCard(it ActionItem) {
	d, ok := it.Ref.(BatchDecision)
	if !ok {
		return
	}
	r := p.v.row(d.SessionID)
	dec := SessionDecision{ID: d.ID, SessionID: d.SessionID, Kind: DecisionKindCheckpoint, Icon: glyph.Checkpoint, Summary: firstText(d.Checkpoint, d.Summary), Approver: d.Approver}
	if r != nil {
		for _, x := range r.Decisions {
			if x.ID == d.ID {
				dec = x
			}
		}
	}
	p.v.openCheckpoint(r, &dec)
}

// refresh reloads the decisions waiting (one answered elsewhere, or from
// its card, leaves the batch).
func (p *batchPane) refresh() {
	var items []BatchDecision
	b, scope := p.b, p.scope
	p.v.async(func(ctx context.Context) (string, error) {
		var err error
		items, err = b.BatchItems(ctx, scope)
		return "", err
	}, func(string) {
		if p.v.batch != p {
			return
		}
		p.setItems(items)
	})
}

func (p *batchPane) handleKey(ev *tcell.EventKey) *tcell.EventKey {
	switch {
	case keys.Back.Matches(ev):
		p.v.closeBatch()
		return nil
	case keys.Filter.Matches(ev):
		p.chooseFilter()
		return nil
	case keys.Refresh.Matches(ev):
		p.refresh()
		return nil
	case confirmBatchKey.Matches(ev):
		p.confirm()
		return nil
	}
	return ev
}

// choices are the choices of the decisions of the filter.
func (p *batchPane) choices() ([]BatchDecision, []BatchChoice) {
	vis := p.visible()
	out := make([]BatchChoice, 0, len(vis))
	for _, d := range vis {
		out = append(out, p.choice[d.ID])
	}
	return vis, out
}

// confirm shows what the batch will do, then answers it.
func (p *batchPane) confirm() {
	items, choices := p.choices()
	n := map[string]int{}
	var detail strings.Builder
	for i, c := range choices {
		n[c.Action]++
		line := i18n.T("tui.batch.action." + c.Action)
		if c.Message != "" {
			line += " — « " + clipText(c.Message, 60) + " »"
		}
		detail.WriteString(i18n.Tf("tui.batch.line", p.v.batchLabel(items[i]), line) + "\n")
	}
	if n[BatchApprove]+n[BatchReject] == 0 {
		p.v.toast(i18n.T("tui.batch.nothing"), false)
		return
	}
	b := p.b
	title := i18n.Tf("tui.batch.confirm", n[BatchApprove], n[BatchReject], n[BatchKeep])
	p.v.shell.Confirm(title, tview.Escape(detail.String()), false, func() {
		var res []BatchResult
		p.v.async(func(ctx context.Context) (string, error) {
			var err error
			res, err = b.ApplyBatch(ctx, choices)
			return "", err
		}, func(string) {
			p.v.closeBatch()
			p.v.showBatchResult(res)
			p.v.reload()
		})
	})
}

// showBatch shows the batch in place of the list.
func (v *SessionsView) showBatch(p *batchPane) {
	if v.content == nil || v.root == nil {
		return
	}
	if v.tabs != nil && v.tabs.focused {
		v.showTab(tabDetail) // the list comes back after the batch
	}
	if v.batch != nil {
		v.content.RemoveItem(v.batch.root)
	} else {
		v.content.RemoveItem(v.root)
	}
	v.batch = p
	v.content.AddItem(p.root, 0, 1, true)
	v.focus(p.list.Focusable())
	v.refreshHints()
}

// closeBatch puts the list back.
func (v *SessionsView) closeBatch() {
	if v.batch == nil || v.content == nil {
		v.batch = nil
		return
	}
	v.content.RemoveItem(v.batch.root)
	v.batch = nil
	v.content.AddItem(v.root, 0, 1, true)
	v.focus(v.list)
	v.refreshHints()
}

// batchLabel is the line of a decision in the confirmation and the result.
func (v *SessionsView) batchLabel(d BatchDecision) string {
	who := shortSessionID(d.SessionID)
	if r := v.row(d.SessionID); r != nil {
		who = sessionLabel(*r) + " " + who
	}
	verify := firstText(d.Verify, "—")
	line := clipText(fmt.Sprintf("%s · %s · %s · %s", firstText(d.Checkpoint, d.ID), who, firstText(d.Summary, "—"),
		i18n.Tf("tui.batch.verify", verify)), 90)
	if d.ApproverMismatch {
		// Never clipped away: the warning of the item (CP-R9).
		line += " · " + i18n.Tf("tui.batch.approver_mismatch", d.Approver)
	}
	return line
}

// showBatchResult lists the answer of each decision.
func (v *SessionsView) showBatchResult(res []BatchResult) {
	var body strings.Builder
	failed := 0
	for _, r := range res {
		state := i18n.T("tui.batch.action." + r.Action)
		color := theme.SuccessHex
		switch {
		case r.Error != "":
			state, color = i18n.Tf("tui.batch.failed", r.Error), theme.ErrorHex
			failed++
		case r.Action == BatchKeep:
			color = theme.TextMutedHex
		case r.Action == BatchReject:
			color = theme.WarningHex
		}
		fmt.Fprintf(&body, "%s %s%s%s\n", tview.Escape(v.batchLabel(r.BatchDecision)), theme.ColorTag(color), tview.Escape(state), theme.TagColor)
	}
	title := i18n.Tf("tui.batch.done", len(res)-failed, len(res))
	v.shell.ShowScrollableModal(title, body.String(), nil)
}
