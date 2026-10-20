package views

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/glyph"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// Checkpoint decision card (P3-T17, 10 §7.2): what the agent reports, the
// changes so far, its last messages and the timeline, then the decision
// (validate / fix first / other instruction) with a message to the agent.
// ─────────────────────────────────────────────────────────────────────────────

// Checkpoint choices (understood by the CheckpointService resolver).
const (
	checkpointApprove = "approve"
	checkpointFix     = "fix"
	checkpointOther   = "other"
)

// checkpointCardFiles is the number of changed files listed on the card.
const checkpointCardFiles = 8

// CheckpointCardView is the display-ready card of a checkpoint decision.
type CheckpointCardView struct {
	Checkpoint string
	Label      string
	Summary    string
	Files      []CheckpointFile
	Additions  int
	Deletions  int
	Patch      string
	Messages   []string // "agent › text", oldest first
	Timeline   string
	Verify     CheckpointVerifyView     // R3 (sessions_checkpoint_verify.go)
	Reports    []CheckpointReport       // sessions_checkpoint_reports.go
	Wiki       []CheckpointWikiProposal // sessions_checkpoint_wiki.go
	Approver   CheckpointApproverView   // CP-R9
}

// CheckpointApproverView is the approver of a checkpoint (CP-R9): the
// people it names, the person who would validate now, and whether that
// person is none of them (warned; the validation is traced as out of
// approver, not refused).
type CheckpointApproverView struct {
	Expected string // "Grace Hopper, lead@example.com" ("" = none)
	Identity string // "Ada Lovelace <ada@example.com>"
	Mismatch bool
}

// CheckpointFile is a changed file of the session.
type CheckpointFile struct {
	File      string
	Status    string
	Additions int
	Deletions int
}

// CheckpointBackend is implemented by session backends that can build the
// checkpoint card (optional: without it, the card shows the decision only).
type CheckpointBackend interface {
	CheckpointCard(ctx context.Context, decisionID string) (CheckpointCardView, error)
}

// openCheckpoint loads the card off the event loop, then shows it.
func (v *SessionsView) openCheckpoint(r *SessionRow, d *SessionDecision) {
	dec, row := *d, r
	cb, ok := v.cfg.Backend.(CheckpointBackend)
	if !ok {
		v.showCheckpoint(row, &dec, CheckpointCardView{Label: dec.Summary, Summary: dec.Message})
		return
	}
	var card CheckpointCardView // written off the loop, read on it after
	v.async(func(ctx context.Context) (string, error) {
		var err error
		card, err = cb.CheckpointCard(ctx, dec.ID)
		return "", err
	}, func(string) {
		v.showCheckpoint(row, &dec, card)
	})
}

func (v *SessionsView) showCheckpoint(r *SessionRow, d *SessionDecision, c CheckpointCardView) {
	if v.shell == nil {
		return
	}
	dec := *d
	id := ""
	if r != nil {
		id = r.ID
	}
	actions := v.verifyFailedActions(r, &dec, c)
	actions = append(actions, ModalAction{Label: i18n.T("tui.checkpoint.decide"), Separator: len(actions) > 0, Callback: func() {
		v.checkpointForm(r, &dec, c.Approver, checkpointDefaultChoice(c.Verify), checkpointExtraChoices(c.Verify)...)
	}})
	actions = append(actions, v.reportActions(dec.SessionID, dec.ID, c.Reports)...)
	actions = append(actions, v.wikiActions(r, &dec, c.Wiki)...)
	if c.Patch != "" {
		patch := c.Patch
		actions = append(actions, ModalAction{Label: i18n.T("tui.checkpoint.diff_full"), Callback: func() {
			v.shell.ShowScrollableModal(i18n.T("tui.checkpoint.diff_title"), tview.Escape(patch), nil)
			v.trackDecision(dec.ID)
		}})
	}
	actions = append(actions, ModalAction{Label: i18n.T("tui.sessions.attach"), Callback: func() {
		if id != "" {
			v.cfg.Backend.Attach(id, "")
		}
	}})
	v.shell.ShowScrollableModal(checkpointTitle(r, d, c), checkpointBody(c), actions)
	v.trackDecision(dec.ID)
}

func checkpointTitle(r *SessionRow, d *SessionDecision, c CheckpointCardView) string {
	parts := []string{glyph.Checkpoint + " " + firstText(c.Checkpoint, d.Summary)}
	if c.Label != "" && c.Label != c.Checkpoint {
		parts = append(parts, c.Label)
	}
	if r != nil {
		parts = append(parts, sessionLabel(*r))
	}
	return tview.Escape(strings.Join(parts, " · "))
}

// checkpointBody renders the card content (tview tags, escaped text).
func checkpointBody(c CheckpointCardView) string {
	var b strings.Builder
	section := func(title string) {
		fmt.Fprintf(&b, "%s─ %s%s\n", theme.ColorTag(theme.TextMutedHex), title, theme.TagColor)
	}
	if c.Summary != "" {
		b.WriteString(tview.Escape(c.Summary) + "\n\n")
	}
	writeCheckpointApprover(&b, c.Approver)
	writeCheckpointVerify(&b, c.Verify, section)
	writeCheckpointReports(&b, c.Reports)
	writeCheckpointWiki(&b, c.Wiki)
	section(i18n.T("tui.checkpoint.changes"))
	if len(c.Files) == 0 {
		b.WriteString("  " + muted(i18n.T("tui.checkpoint.no_changes")) + "\n")
	} else {
		fmt.Fprintf(&b, "  %s\n", i18n.Tf("tui.checkpoint.changes_stat", c.Additions, c.Deletions, len(c.Files)))
		for i, f := range c.Files {
			if i == checkpointCardFiles {
				fmt.Fprintf(&b, "  %s\n", muted(i18n.Tf("tui.checkpoint.more_files", len(c.Files)-i)))
				break
			}
			fmt.Fprintf(&b, "  %s %s  %s+%d%s %s−%d%s\n", firstText(f.Status, "M")[:1], tview.Escape(f.File),
				theme.ColorTag(theme.SuccessHex), f.Additions, theme.TagColor, theme.ColorTag(theme.ErrorHex), f.Deletions, theme.TagColor)
		}
	}
	b.WriteString("\n")
	section(i18n.T("tui.checkpoint.messages"))
	if len(c.Messages) == 0 {
		b.WriteString("  " + muted(i18n.T("tui.checkpoint.no_messages")) + "\n")
	}
	for _, m := range c.Messages {
		b.WriteString("  " + tview.Escape(clipText(m, 300)) + "\n")
	}
	if c.Timeline != "" {
		b.WriteString("\n")
		section(i18n.T("tui.checkpoint.timeline"))
		b.WriteString("  " + tview.Escape(c.Timeline) + "\n")
	}
	return b.String()
}

// writeCheckpointApprover writes the approver the checkpoint names and,
// when the person who validates is none of them, the warning (CP-R9).
func writeCheckpointApprover(b *strings.Builder, a CheckpointApproverView) {
	if a.Expected == "" {
		return
	}
	fmt.Fprintf(b, "%s\n", tview.Escape(i18n.Tf("tui.checkpoint.approver", a.Expected)))
	switch {
	case a.Mismatch:
		fmt.Fprintf(b, "%s%s%s\n", theme.ColorTag(theme.WarningHex),
			tview.Escape(i18n.Tf("tui.checkpoint.approver_mismatch", firstText(a.Identity, "?"))), theme.TagColor)
	case a.Identity != "":
		b.WriteString(muted(tview.Escape(i18n.Tf("tui.checkpoint.approver_you", a.Identity))) + "\n")
	}
	b.WriteString("\n")
}

// approveCheckpoint validates a checkpoint at once (y). When its
// verification by oh failed, the card opens on « Corriger » / « Passer
// outre… » instead (the validation is refused by oh). When it names
// approvers and the person who validates is none of them, oh asks for a
// confirmation first, with the warning (CP-R9: never refused).
func (v *SessionsView) approveCheckpoint(r *SessionRow, d *SessionDecision) {
	dec, row := *d, r
	cb, ok := v.cfg.Backend.(CheckpointBackend)
	if dec.Approver == "" || !ok || v.shell == nil {
		v.approveOrOpen(row, &dec)
		return
	}
	var card CheckpointCardView
	v.async(func(ctx context.Context) (string, error) {
		var err error
		card, err = cb.CheckpointCard(ctx, dec.ID)
		return "", err
	}, func(string) {
		switch {
		case card.Verify.Failed():
			v.showCheckpoint(row, &dec, card)
		case !card.Approver.Mismatch:
			v.decide(&dec, checkpointApprove, "", nil)
		default:
			v.shell.Confirm(i18n.Tf("tui.checkpoint.approver_confirm", firstText(card.Checkpoint, dec.Summary)),
				tview.Escape(i18n.Tf("tui.checkpoint.approver", card.Approver.Expected)+"\n"+i18n.Tf("tui.checkpoint.approver_mismatch", firstText(card.Approver.Identity, "?"))),
				false, func() { v.decide(&dec, checkpointApprove, "", nil) })
		}
	})
}

// ErrOverrideRequired is matched by the error of a backend refusing the
// validation of a checkpoint whose verification failed (only « Passer
// outre » passes it).
var ErrOverrideRequired = errors.New("checkpoint verification must be overridden")

// approveOrOpen validates a checkpoint; refused by its verification, it
// opens its card instead of a toast.
func (v *SessionsView) approveOrOpen(r *SessionRow, d *SessionDecision) {
	dec, row := *d, r
	v.untrackDecision()
	failed := false
	v.async(func(ctx context.Context) (string, error) {
		err := v.cfg.Backend.Decide(ctx, dec.ID, checkpointApprove, "", nil)
		if errors.Is(err, ErrOverrideRequired) {
			failed = true
			return "", nil
		}
		return i18n.T("tui.inbox.answered"), err
	}, func(msg string) {
		if failed {
			v.openCheckpoint(row, &dec)
			return
		}
		v.toast(msg, true)
		v.reload()
	})
}

// refuseCheckpoint asks the instruction of a refusal (n), with the
// approver the checkpoint names and the warning when the person who
// answers is none of them (CP-R9).
func (v *SessionsView) refuseCheckpoint(r *SessionRow, d *SessionDecision) {
	dec, row := *d, r
	cb, ok := v.cfg.Backend.(CheckpointBackend)
	if !ok || dec.Approver == "" {
		v.checkpointForm(row, &dec, CheckpointApproverView{Expected: dec.Approver}, checkpointFix)
		return
	}
	var card CheckpointCardView
	v.async(func(ctx context.Context) (string, error) {
		var err error
		card, err = cb.CheckpointCard(ctx, dec.ID)
		return "", err
	}, func(string) {
		v.checkpointForm(row, &dec, card.Approver, checkpointFix)
	})
}

// checkpointForm asks for the decision and the message to the agent.
func (v *SessionsView) checkpointForm(r *SessionRow, d *SessionDecision, approver CheckpointApproverView, choice string, extra ...SelectOption) {
	if v.shell == nil {
		return
	}
	dec := *d
	title := i18n.Tf("tui.checkpoint.form_title", firstText(d.Summary, d.ID))
	if r != nil {
		title += " · " + sessionLabel(*r)
	}
	if approver.Mismatch {
		title += " · " + i18n.Tf("tui.checkpoint.approver_form", approver.Expected)
	}
	v.shell.ShowInlineForm(InlineFormConfig{
		Title: tview.Escape(title),
		Fields: []FormField{
			{Key: "decision", Label: i18n.T("tui.inbox.decision"), Type: FieldSelect, Default: choice, Hint: approverHint(approver),
				Options: append([]SelectOption{
					{Label: i18n.T("tui.checkpoint.choice.approve"), Value: checkpointApprove},
					{Label: i18n.T("tui.checkpoint.choice.fix"), Value: checkpointFix},
					{Label: i18n.T("tui.checkpoint.choice.other"), Value: checkpointOther},
				}, extra...)},
			{Key: "message", Label: i18n.T("tui.checkpoint.message"), Type: FieldText},
		},
		OnSubmit: func(values map[string]string, _ map[string][]string) {
			ch, msg := values["decision"], strings.TrimSpace(values["message"])
			if ch != checkpointApprove && msg == "" {
				v.toast(i18n.T("tui.checkpoint.message_required"), false)
				return
			}
			v.decide(&dec, ch, msg, nil)
		},
	})
	v.trackDecision(dec.ID)
}

// approverHint is the approver line of the decision form: who the
// checkpoint names and, when the person who answers is none of them, the
// warning (CP-R9).
func approverHint(a CheckpointApproverView) string {
	if a.Expected == "" {
		return ""
	}
	line := tview.Escape(i18n.Tf("tui.checkpoint.approver", a.Expected))
	switch {
	case a.Mismatch:
		line += "\n  " + theme.ColorTag(theme.WarningHex) + tview.Escape(i18n.Tf("tui.checkpoint.approver_mismatch", firstText(a.Identity, "?"))) + theme.TagColor
	case a.Identity != "":
		line += "\n  " + muted(tview.Escape(i18n.Tf("tui.checkpoint.approver_you", a.Identity)))
	}
	return line
}
