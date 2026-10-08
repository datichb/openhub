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
	actions := []ModalAction{
		{Label: i18n.T("tui.checkpoint.decide"), Callback: func() { v.checkpointForm(r, &dec, checkpointApprove) }},
	}
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
	parts := []string{"⏸ " + firstText(c.Checkpoint, d.Summary)}
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

// checkpointForm asks for the decision and the message to the agent.
func (v *SessionsView) checkpointForm(r *SessionRow, d *SessionDecision, choice string) {
	if v.shell == nil {
		return
	}
	dec := *d
	title := i18n.Tf("tui.checkpoint.form_title", firstText(d.Summary, d.ID))
	if r != nil {
		title += " · " + sessionLabel(*r)
	}
	v.shell.ShowInlineForm(InlineFormConfig{
		Title: tview.Escape(title),
		Fields: []FormField{
			{Key: "decision", Label: i18n.T("tui.inbox.decision"), Type: FieldSelect, Default: choice,
				Options: []SelectOption{
					{Label: i18n.T("tui.checkpoint.choice.approve"), Value: checkpointApprove},
					{Label: i18n.T("tui.checkpoint.choice.fix"), Value: checkpointFix},
					{Label: i18n.T("tui.checkpoint.choice.other"), Value: checkpointOther},
				}},
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
