package views

import (
	"context"
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// Validation by batch (R6, CP-R6; `oh session approve --batch` in the CLI):
// B lists the checkpoints waiting (those of the fan-out of the selected
// session, else every one), each with its session, its summary and the
// verification by oh, each one validated, left waiting or refused with an
// instruction; a confirmation, then the backend answers them one by one
// through the same service as the CLI and the result of each is shown.
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

// openBatch loads the checkpoints of the batch, off the event loop.
func (v *SessionsView) openBatch(r *SessionRow) {
	b, ok := v.cfg.Backend.(SessionBatcher)
	if !ok || v.shell == nil {
		return
	}
	id := ""
	if r != nil {
		id = r.ID
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
		v.batchForm(b, items)
	})
}

// batchLabel is the line of a decision in the batch form.
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

// batchForm asks, for each decision, validate / leave waiting / refuse,
// and the instruction of a refusal.
func (v *SessionsView) batchForm(b SessionBatcher, items []BatchDecision) {
	fields := make([]FormField, 0, 2*len(items))
	for _, d := range items {
		fields = append(fields,
			FormField{Key: "a:" + d.ID, Label: tview.Escape(v.batchLabel(d)), Type: FieldSelect, Default: BatchApprove, Options: []SelectOption{
				{Label: i18n.T("tui.batch.approve"), Value: BatchApprove},
				{Label: i18n.T("tui.batch.keep"), Value: BatchKeep},
				{Label: i18n.T("tui.batch.reject"), Value: BatchReject},
			}},
			FormField{Key: "m:" + d.ID, Label: "  " + i18n.T("tui.batch.message"), Type: FieldText},
		)
	}
	v.shell.ShowInlineForm(InlineFormConfig{
		Title:  i18n.Tf("tui.batch.title", len(items)),
		Fields: fields,
		OnSubmit: func(values map[string]string, _ map[string][]string) {
			choices, err := batchChoices(items, values)
			if err != "" {
				v.toast(err, false)
				return
			}
			v.confirmBatch(b, items, choices)
		},
	})
}

// batchChoices reads the form: a refusal needs its instruction.
func batchChoices(items []BatchDecision, values map[string]string) (choices []BatchChoice, refused string) {
	out := make([]BatchChoice, 0, len(items))
	for _, d := range items {
		c := BatchChoice{ID: d.ID, Action: firstText(values["a:"+d.ID], BatchApprove), Message: strings.TrimSpace(values["m:"+d.ID])}
		switch c.Action {
		case BatchApprove, BatchKeep, BatchReject:
		default:
			c.Action = BatchKeep
		}
		if c.Action == BatchReject && c.Message == "" {
			return nil, i18n.Tf("tui.batch.message_required", firstText(d.Checkpoint, d.ID))
		}
		out = append(out, c)
	}
	return out, ""
}

// confirmBatch shows what the batch will do, then answers it.
func (v *SessionsView) confirmBatch(b SessionBatcher, items []BatchDecision, choices []BatchChoice) {
	n := map[string]int{}
	var detail strings.Builder
	for i, c := range choices {
		n[c.Action]++
		line := i18n.T("tui.batch.action." + c.Action)
		if c.Message != "" {
			line += " — « " + clipText(c.Message, 60) + " »"
		}
		detail.WriteString(i18n.Tf("tui.batch.line", v.batchLabel(items[i]), line) + "\n")
	}
	if n[BatchApprove]+n[BatchReject] == 0 {
		v.toast(i18n.T("tui.batch.nothing"), false)
		return
	}
	title := i18n.Tf("tui.batch.confirm", n[BatchApprove], n[BatchReject], n[BatchKeep])
	v.shell.Confirm(title, tview.Escape(detail.String()), false, func() {
		var res []BatchResult
		v.async(func(ctx context.Context) (string, error) {
			var err error
			res, err = b.ApplyBatch(ctx, choices)
			return "", err
		}, func(string) {
			v.showBatchResult(res)
			v.reload()
		})
	})
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
