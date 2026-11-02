package views

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// The layouts of the inline wizard: classic (full width, info panel below) and
// grouped (centered content, sidebar), and the step bar states they show.

// mountClassicLayout builds the original full-width layout (no groups).
func (w *InlineWizardView) mountClassicLayout(content *tview.Flex) {
	w.stepBar = widgets.NewStepBar(w.stepStates)

	w.mainFlex = tview.NewFlex().SetDirection(tview.FlexRow)
	w.mainFlex.SetBackgroundColor(theme.BgPanel)
	w.mainFlex.
		AddItem(w.stepBar.TextView, 1, 0, false).
		AddItem(w.stepHeader, 2, 0, false).
		AddItem(w.stepContent, 0, 1, true).
		AddItem(w.infoPanel, 0, 0, false).
		AddItem(w.hintsBar.TextView, 1, 0, false)
	w.hintsHost = w.mainFlex

	content.AddItem(w.mainFlex, 0, 1, true)
}

// fillBackground paints BgPanel over a rect.
func fillBackground(screen tcell.Screen, x, y, width, height int) {
	bg := tcell.StyleDefault.Background(theme.BgPanel)
	for row := y; row < y+height; row++ {
		for col := x; col < x+width; col++ {
			screen.SetContent(col, row, ' ', nil, bg)
		}
	}
}

// mountGroupedLayout builds the full-screen layout with a right-side info
// sidebar. The step bar and step header are omitted (the sidebar provides
// the progression view).
func (w *InlineWizardView) mountGroupedLayout(content *tview.Flex) {
	// Initialize group states (used by renderGroupedSidebar)
	w.groupStates = w.buildGroupStates()
	// No stepBar in grouped mode — sidebar replaces it.

	// Info sidebar: padding
	w.infoPanel.SetBorderPadding(1, 1, 1, 1)

	w.centerStepContent()
	leftCol := w.newLeftColumn()

	// bodyRow: leftCol (left, 3/4) + separator (1 col) + sidebar (right, 1/4)
	w.bodyRow = tview.NewFlex().SetDirection(tview.FlexColumn)
	w.bodyRow.SetBackgroundColor(theme.BgPanel)
	w.bodyRow.
		AddItem(leftCol, 0, 3, true).
		AddItem(newSidebarSeparator(), 1, 0, false).
		AddItem(w.infoPanel, 0, 1, false)

	// Full-screen layout
	w.mainFlex = tview.NewFlex().SetDirection(tview.FlexRow)
	w.mainFlex.SetBackgroundColor(theme.BgPanel)
	w.mainFlex.AddItem(w.bodyRow, 0, 1, true)

	content.AddItem(w.mainFlex, 0, 1, true)
}

// centerStepContent puts the content of the steps in a centered wrapper
// (flexbox-style): all wizard pages get the same width via proportional
// spacers, like CSS `max-width` + `margin: auto`. Pages fill stepContentInner
// without knowing their width — the wrapper decides.
func (w *InlineWizardView) centerStepContent() {
	bg := theme.BgPanel
	w.stepContentInner = tview.NewFlex().SetDirection(tview.FlexRow)
	w.stepContentInner.SetBackgroundColor(bg)
	centeredWrapper := tview.NewFlex().SetDirection(tview.FlexColumn)
	centeredWrapper.SetBackgroundColor(bg)
	centeredWrapper.
		AddItem(tview.NewBox().SetBackgroundColor(bg), 0, 1, false). // left spacer
		AddItem(w.stepContentInner, 0, 5, true).                     // content (5/7 of width)
		AddItem(tview.NewBox().SetBackgroundColor(bg), 0, 1, false)  // right spacer
	w.stepContent.AddItem(centeredWrapper, 0, 1, true)
}

// newLeftColumn wraps stepContent + hintsBar so that hints are centered
// relative to the content area, not the full width including the sidebar.
func (w *InlineWizardView) newLeftColumn() *tview.Flex {
	leftCol := tview.NewFlex().SetDirection(tview.FlexRow)
	leftCol.SetBackgroundColor(theme.BgPanel)
	leftCol.SetBorderPadding(0, 0, 2, 1)
	// Force-clear the full rect including padding columns. Flex has
	// dontClear=true by default, so the 2-col left + 1-col right padding
	// would never be repainted — stale pixels bleed through as a visible
	// vertical strip.
	leftCol.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		fillBackground(screen, x, y, width, height)
		// Return inner rect matching SetBorderPadding(0, 0, 2, 1).
		innerW := width - 2 - 1
		if innerW < 0 {
			innerW = 0
		}
		return x + 2, y, innerW, height
	})
	leftCol.AddItem(w.stepContent, 0, 1, true)
	leftCol.AddItem(w.hintsBar.TextView, 1, 0, false)
	w.hintsHost = leftCol
	return leftCol
}

// newSidebarSeparator is the vertical separator (│) between the left column
// and the sidebar.
func newSidebarSeparator() *tview.Box {
	sep := tview.NewBox()
	sep.SetBackgroundColor(theme.BgPanel)
	sep.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		style := tcell.StyleDefault.Foreground(theme.BorderCard).Background(theme.BgPanel)
		for row := y; row < y+height; row++ {
			screen.SetContent(x, row, '│', nil, style)
		}
		return x + 1, y, width - 1, height
	})
	return sep
}

// buildGroupStates computes the step bar states from the per-step states,
// aggregated by group boundaries.
func (w *InlineWizardView) buildGroupStates() []widgets.Step {
	groups := w.cfg.Groups
	states := make([]widgets.Step, len(groups))
	for gi, g := range groups {
		states[gi] = widgets.Step{Label: g.Label, Status: widgets.StepPending}

		// Determine end index (next group's start, or len(steps))
		endIdx := len(w.cfg.Steps)
		if gi+1 < len(groups) {
			endIdx = groups[gi+1].StartIdx
		}

		allDone := true
		hasActive := false
		for si := g.StartIdx; si < endIdx; si++ {
			switch w.stepStates[si].Status {
			case widgets.StepActive:
				hasActive = true
				allDone = false
			case widgets.StepPending:
				allDone = false
			}
		}

		if hasActive {
			states[gi].Status = widgets.StepActive
		} else if allDone {
			states[gi].Status = widgets.StepDone
		}
	}
	return states
}

// refreshGroupStates recomputes group states and updates the step bar (if present).
func (w *InlineWizardView) refreshGroupStates() {
	if len(w.cfg.Groups) == 0 {
		return
	}
	w.groupStates = w.buildGroupStates()
	if w.stepBar != nil {
		w.stepBar.SetSteps(w.groupStates)
	}
}

// syncStepBar updates the step bar after a step state change.
// In grouped mode, it recomputes all group states. In classic mode,
// it updates the individual step directly.
func (w *InlineWizardView) syncStepBar(idx int, status widgets.StepStatus) {
	w.stepStates[idx] = widgets.Step{Label: w.stepStates[idx].Label, Status: status}
	if len(w.cfg.Groups) > 0 {
		w.refreshGroupStates()
	} else if w.stepBar != nil {
		w.stepBar.UpdateStatus(idx, status)
	}
}

func (w *InlineWizardView) updateInfoPanelSize(lines int) {
	if w.mainFlex == nil || w.infoPanel == nil || w.stepBar == nil {
		return
	}
	// Remove and re-add the info panel with the new height.
	// tview.Flex doesn't support dynamic resizing of items, so we rebuild.
	w.mainFlex.Clear()
	w.mainFlex.
		AddItem(w.stepBar.TextView, 1, 0, false).
		AddItem(w.stepHeader, 2, 0, false).
		AddItem(w.stepContent, 0, 1, true).
		AddItem(w.infoPanel, lines, 0, false).
		AddItem(w.hintsBar.TextView, 1, 0, false)
}
