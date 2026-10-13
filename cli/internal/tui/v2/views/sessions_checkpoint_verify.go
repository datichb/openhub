package views

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// Verification part of the checkpoint card (R3, CP-R5): the result of the
// verification run by oh, the evidence of the agent checked against what oh
// observed, and a ticket whose fix rounds reached the cap. A failed
// verification is passed only with the « override » choice (traced).
// ─────────────────────────────────────────────────────────────────────────────

// checkpointOverride validates despite a failed verification.
const checkpointOverride = "override"

// Verification statuses of CheckpointVerifyView.
const (
	verifyRunning = "running"
	verifyPass    = "pass"
	verifyFail    = "fail"
	verifyNone    = "none"
	// verifyUnavailable: the verification cannot run where the work is
	// (R3-remote): passed only overridden, like a failure.
	verifyUnavailable = "unavailable"
)

// checkpointExcerptLines is the number of output lines shown on the card.
const checkpointExcerptLines = 12

// CheckpointVerifyView is the verification part of a checkpoint card.
type CheckpointVerifyView struct {
	// Status: "" (no verification) | running | pass | fail | none | unavailable.
	Status  string
	Summary string
	Excerpt string
	// Evidence lines: command and whether oh observed it.
	Evidence []CheckpointEvidenceView
	// Unresolved is the ticket whose fix rounds reached the cap.
	Unresolved string
	// Remote is the result of the verification run in the remote job of
	// the session (R3-remote); RemoteOK tells whether it let it pass.
	Remote   string
	RemoteOK bool
}

// CheckpointEvidenceView is an evidence of the agent.
type CheckpointEvidenceView struct {
	Cmd          string
	Exit         int
	Observed     bool
	Contradicted bool
}

// Failed reports whether only an override passes the checkpoint: the
// verification failed or cannot run.
func (v CheckpointVerifyView) Failed() bool {
	return v.Status == verifyFail || v.Status == verifyUnavailable
}

func (v CheckpointVerifyView) empty() bool {
	return v.Status == "" && len(v.Evidence) == 0 && v.Unresolved == "" && v.Remote == ""
}

// writeCheckpointVerify renders the verification part of the card.
func writeCheckpointVerify(b *strings.Builder, v CheckpointVerifyView, section func(string)) {
	if v.empty() {
		return
	}
	if v.Unresolved != "" {
		fmt.Fprintf(b, "%s%s %s%s\n\n", theme.ColorTag(theme.WarningHex), theme.IconWarning,
			tview.Escape(i18n.Tf("tui.checkpoint.unresolved", v.Unresolved)), theme.TagColor)
	}
	if v.Status != "" {
		section(i18n.T("tui.checkpoint.verify.title"))
		color, icon := theme.SuccessHex, theme.IconSuccess
		switch v.Status {
		case verifyFail:
			color, icon = theme.ErrorHex, theme.IconError
		case verifyUnavailable:
			color, icon = theme.WarningHex, theme.IconWarning
		case verifyRunning, verifyNone:
			color, icon = theme.TextMutedHex, "…"
		}
		fmt.Fprintf(b, "  %s%s %s%s\n", theme.ColorTag(color), icon, tview.Escape(v.Summary), theme.TagColor)
		if v.Excerpt != "" && v.Status == verifyFail {
			lines := strings.Split(v.Excerpt, "\n")
			if len(lines) > checkpointExcerptLines {
				lines = lines[len(lines)-checkpointExcerptLines:]
			}
			for _, l := range lines {
				b.WriteString("    " + muted(tview.Escape(clipText(l, 200))) + "\n")
			}
		}
		if v.Failed() {
			b.WriteString("  " + muted(i18n.T("tui.checkpoint.verify.override_hint")) + "\n")
		}
	}
	if v.Remote != "" {
		if v.Status == "" {
			section(i18n.T("tui.checkpoint.verify.title"))
		}
		color, icon := theme.SuccessHex, theme.IconSuccess
		if !v.RemoteOK {
			color, icon = theme.WarningHex, theme.IconWarning
		}
		fmt.Fprintf(b, "  %s%s %s%s\n", theme.ColorTag(color), icon,
			tview.Escape(i18n.Tf("tui.checkpoint.verify.remote", v.Remote)), theme.TagColor)
	}
	if v.Status != "" || v.Remote != "" {
		b.WriteString("\n")
	}
	if len(v.Evidence) > 0 {
		section(i18n.T("tui.checkpoint.evidence.title"))
		for _, e := range v.Evidence {
			switch {
			case e.Contradicted:
				fmt.Fprintf(b, "  %s%s %s%s\n", theme.ColorTag(theme.ErrorHex), theme.IconWarning,
					tview.Escape(i18n.Tf("tui.checkpoint.evidence.contradicted", e.Cmd, e.Exit)), theme.TagColor)
			case e.Observed:
				fmt.Fprintf(b, "  %s%s %s%s\n", theme.ColorTag(theme.SuccessHex), theme.IconSuccess,
					tview.Escape(i18n.Tf("tui.checkpoint.evidence.observed", e.Cmd, e.Exit)), theme.TagColor)
			default:
				fmt.Fprintf(b, "  %s%s %s%s\n", theme.ColorTag(theme.WarningHex), theme.IconWarning,
					tview.Escape(i18n.Tf("tui.checkpoint.evidence.unobserved", e.Cmd, e.Exit)), theme.TagColor)
			}
		}
		b.WriteString("\n")
	}
}

// checkpointExtraChoices are the choices added to the decision form: the
// override of a failed verification.
func checkpointExtraChoices(v CheckpointVerifyView) []SelectOption {
	if !v.Failed() {
		return nil
	}
	return []SelectOption{{Label: i18n.T("tui.checkpoint.choice.override"), Value: checkpointOverride}}
}

// checkpointDefaultChoice is the choice selected first: fix when the
// verification failed.
func checkpointDefaultChoice(v CheckpointVerifyView) string {
	if v.Failed() {
		return checkpointFix
	}
	return checkpointApprove
}
