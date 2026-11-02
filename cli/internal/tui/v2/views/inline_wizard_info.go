package views

import (
	"fmt"
	"strings"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// Info panel of the inline wizard: the sidebar of the grouped layout and the
// recap below the step of the classic one.

func (w *InlineWizardView) renderInfoPanel() {
	if w.infoPanel == nil {
		return
	}

	if len(w.cfg.Groups) > 0 {
		w.renderGroupedSidebar()
	} else {
		w.renderClassicInfoPanel()
	}
}

// renderGroupedSidebar renders the right-side info sidebar (grouped mode)
// with a detailed tree-style recap including individual sub-steps.
func (w *InlineWizardView) renderGroupedSidebar() {
	var b strings.Builder

	accent := theme.ColorTagOf(theme.ActiveMode.Primary)
	success := theme.ColorTagOf(theme.Success)
	muted := theme.ColorTagOf(theme.FgMuted)
	secondary := theme.ColorTagOf(theme.FgSecondary)
	primary := theme.ColorTagOf(theme.FgPrimary)
	reset := "[-]"

	fmt.Fprintf(&b, " %s%s %s%s\n", accent, theme.IconActive, i18n.T("wizard.sidebar.title"), reset)
	fmt.Fprintf(&b, " %s─────────────────%s\n\n", muted, reset)

	// Build a map of step index → accumulated info (using stored stepIdx)
	infoByStep := make(map[int][]InfoField)
	for _, si := range w.infoAccum {
		infoByStep[si.stepIdx] = si.fields
	}

	for gi, g := range w.cfg.Groups {
		if g.Hidden != nil && g.Hidden() {
			continue
		}
		endIdx := len(w.cfg.Steps)
		if gi+1 < len(w.cfg.Groups) {
			endIdx = w.cfg.Groups[gi+1].StartIdx
		}

		// Determine group status
		groupStatus := widgets.StepPending
		allDone := true
		for si := g.StartIdx; si < endIdx; si++ {
			switch w.stepStates[si].Status {
			case widgets.StepActive:
				groupStatus = widgets.StepActive
				allDone = false
			case widgets.StepPending:
				allDone = false
			}
		}
		if allDone && groupStatus != widgets.StepActive {
			groupStatus = widgets.StepDone
		}

		// ── Group header ──
		switch groupStatus {
		case widgets.StepDone:
			fmt.Fprintf(&b, " %s%s%s %s%s%s\n", success, theme.IconDone, reset, primary, g.Label, reset)
		case widgets.StepActive:
			fmt.Fprintf(&b, " %s%s%s %s%s%s\n", accent, theme.IconActive, reset, primary, g.Label, reset)
		default:
			fmt.Fprintf(&b, " %s%s %s%s\n", muted, theme.IconPending, g.Label, reset)
		}

		// ── Sub-steps within this group ──
		// Collect visible sub-steps (skip SidebarHidden and dynamically skipped)
		type subStep struct {
			idx    int
			label  string
			status widgets.StepStatus
		}
		var subs []subStep
		for si := g.StartIdx; si < endIdx; si++ {
			step := w.cfg.Steps[si]
			if step.SidebarHidden {
				continue
			}
			if step.Skip {
				continue
			}
			// Don't show dynamically-skipped future steps (e.g. Deploy without project)
			if w.stepStates[si].Status == widgets.StepPending && step.SkipIf != nil && step.SkipIf() {
				continue
			}
			subs = append(subs, subStep{idx: si, label: step.Label, status: w.stepStates[si].Status})
		}

		for _, ss := range subs {
			switch ss.status {
			case widgets.StepDone:
				// Show InfoFields with tree connectors
				fields := infoByStep[ss.idx]
				if len(fields) == 0 {
					fmt.Fprintf(&b, "   %s%s%s %s%s%s\n", success, theme.IconDone, reset, secondary, ss.label, reset)
					continue
				}
				for fi, f := range fields {
					if f.Value == "" {
						continue
					}
					connector := "├"
					if fi == len(fields)-1 {
						connector = "└"
					}
					fmt.Fprintf(&b, "   %s%s %s:%s %s\n", muted, connector, f.Label, reset, f.Value)
				}

			case widgets.StepActive:
				// ◆ Label ........ en cours
				dots := " "
				labelLen := len([]rune(ss.label))
				// Fill dots to ~24 chars total (label + dots + " en cours")
				if remaining := 20 - labelLen; remaining > 2 {
					dots = " " + strings.Repeat(".", remaining) + " "
				}
				fmt.Fprintf(&b, "   %s%s %s%s%s%s%s\n",
					accent, theme.IconActive, ss.label, muted, dots, i18n.T("wizard.sidebar.in_progress"), reset)

			case widgets.StepSkipped:
				fmt.Fprintf(&b, "   %s%s %s%s\n", muted, theme.IconSkipped, ss.label, reset)

			default: // Pending
				fmt.Fprintf(&b, "   %s%s %s%s\n", muted, theme.IconPending, ss.label, reset)
			}
		}

		b.WriteString("\n")
	}

	w.infoPanel.SetText(b.String())
}

// renderClassicInfoPanel renders the bottom info panel (classic mode).
func (w *InlineWizardView) renderClassicInfoPanel() {
	var b strings.Builder

	// Completed steps with their info fields
	for _, si := range w.infoAccum {
		fmt.Fprintf(&b, "  %s%s[-] %s%s[-]\n",
			theme.ColorTagOf(theme.Success), theme.IconDone,
			theme.ColorTagOf(theme.FgPrimary), si.label)
		for _, f := range si.fields {
			if f.Value != "" {
				fmt.Fprintf(&b, "    %s%s:[-] %s\n",
					theme.ColorTagOf(theme.FgSecondary), f.Label, f.Value)
			}
		}
		b.WriteString("\n")
	}

	// Current and remaining steps
	for i := w.currentStep; i < len(w.cfg.Steps); i++ {
		if w.cfg.Steps[i].Skip || w.stepStates[i].Status == widgets.StepDone {
			continue
		}
		if w.stepStates[i].Status == widgets.StepSkipped {
			fmt.Fprintf(&b, "  %s%s %s (skipped)[-]\n",
				theme.ColorTagOf(theme.FgMuted), theme.IconSkipped, w.cfg.Steps[i].Label)
			continue
		}
		icon := theme.IconPending
		color := theme.ColorTagOf(theme.FgMuted)
		if i == w.currentStep {
			icon = theme.IconActive
			color = theme.ColorTagOf(theme.ActiveMode.Primary)
		}
		fmt.Fprintf(&b, "  %s%s %s[-]\n", color, icon, w.cfg.Steps[i].Label)
	}

	text := b.String()
	w.infoPanel.SetText(text)

	// Resize info panel: count lines, show only if non-empty
	lines := strings.Count(text, "\n")
	if lines > 0 {
		// Cap at 12 lines to leave room for the step content
		if lines > 12 {
			lines = 12
		}
		w.updateInfoPanelSize(lines)
	} else {
		w.updateInfoPanelSize(0)
	}
}
