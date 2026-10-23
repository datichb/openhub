package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/launch"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Pauses planned for the mode of a launch (R3, CP-R10): shown by oh run and
// in the recap of the TUI launch form, so that « auto » holds no surprise
// (it keeps the mandatory decisions).

// recapPauses describes the pauses planned in mode ("" without a workflow).
func recapPauses(wf *sessionspec.WorkflowRuntime, mode string) string {
	if wf == nil || len(wf.Checkpoints) == 0 {
		return ""
	}
	pauses := bundle.PlannedPauses(wf, mode)
	if len(pauses) == 0 {
		return i18n.T("cmd.run.pauses.none")
	}
	parts := make([]string, len(pauses))
	for i, p := range pauses {
		parts[i] = p.ID
		switch {
		case p.Mandatory:
			parts[i] += " (" + i18n.T("cmd.run.recap.cp.mandatory") + ")"
		case p.Conditional:
			parts[i] += " (" + i18n.T("cmd.run.pauses.conditional") + ")"
		}
	}
	text := i18n.Tf("cmd.run.pauses.count", len(pauses), strings.Join(parts, " · "))
	if mode == workflow.ModeAuto {
		text += " — " + i18n.T("cmd.run.pauses.auto")
	}
	return text
}

// printRunPauses prints the pauses planned for the launch (oh run).
func printRunPauses(w io.Writer, p *launch.Prepared) {
	if p.Bundle == nil || p.Bundle.Spec.Workflow == nil {
		return
	}
	if text := recapPauses(p.Bundle.Spec.Workflow, p.Resolution.Mode); text != "" {
		fmt.Fprintf(w, "  %s %s\n", theme.Subtitle.Render(i18n.T("cmd.run.recap.pauses")), text)
	}
}
