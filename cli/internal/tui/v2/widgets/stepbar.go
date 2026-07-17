// Package widgets provides reusable TUI components for the oh CLI.
package widgets

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// Step Bar — horizontal progress indicator for multi-step flows
// ─────────────────────────────────────────────────────────────────────────────

// StepStatus represents the state of a wizard step.
type StepStatus int

const (
	StepPending StepStatus = iota
	StepActive
	StepDone
	StepSkipped
)

// Step defines a step in the step bar.
type Step struct {
	Label  string
	Status StepStatus
}

// StepBar is a tview.TextView that renders a horizontal step progress bar.
// Example: ● Setup ─── ◆ Provider ─── ○ Confirm
type StepBar struct {
	*tview.TextView
	steps []Step
}

// NewStepBar creates a step bar widget.
func NewStepBar(steps []Step) *StepBar {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	tv.SetBackgroundColor(theme.BgPanel)

	sb := &StepBar{TextView: tv, steps: steps}
	sb.render()
	return sb
}

// SetSteps updates the steps and re-renders.
func (sb *StepBar) SetSteps(steps []Step) {
	sb.steps = steps
	sb.render()
}

// UpdateStatus changes the status of step at index i and re-renders.
func (sb *StepBar) UpdateStatus(i int, status StepStatus) {
	if i >= 0 && i < len(sb.steps) {
		sb.steps[i] = Step{Label: sb.steps[i].Label, Status: status}
		sb.render()
	}
}

func (sb *StepBar) render() {
	var parts []string
	for _, s := range sb.steps {
		var icon, color string
		switch s.Status {
		case StepDone:
			icon = theme.IconDone
			color = colorTag(theme.Success)
		case StepActive:
			icon = theme.IconActive
			color = colorTag(theme.Accent)
		case StepSkipped:
			icon = theme.IconSkipped
			color = colorTag(theme.FgMuted)
		default: // StepPending
			icon = theme.IconPending
			color = colorTag(theme.FgMuted)
		}
		parts = append(parts, fmt.Sprintf("%s%s %s[-]", color, icon, s.Label))
	}
	connector := fmt.Sprintf(" %s%s[-] ", colorTag(theme.FgMuted), theme.IconConnector)
	sb.SetText(strings.Join(parts, connector))
}

// ─────────────────────────────────────────────────────────────────────────────
// Status Bar — footer with keybind hints
// ─────────────────────────────────────────────────────────────────────────────

// StatusBar is a footer bar showing contextual keybind hints.
type StatusBar struct {
	*tview.TextView
}

// NewStatusBar creates a status bar with the given hint text.
func NewStatusBar(hints string) *StatusBar {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	tv.SetBackgroundColor(theme.BgPanel)
	tv.SetText(fmt.Sprintf("%s%s[-]", colorTag(theme.FgSecondary), hints))

	return &StatusBar{TextView: tv}
}

// SetHints replaces the hint text.
func (sb *StatusBar) SetHints(hints string) {
	sb.SetText(fmt.Sprintf("%s%s[-]", colorTag(theme.FgSecondary), hints))
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// colorTag converts a tcell.Color to a tview color tag string like "[#rrggbb]".
func colorTag(c tcell.Color) string {
	r, g, b := c.RGB()
	return fmt.Sprintf("[#%02x%02x%02x]", r, g, b)
}

// ColorTag is the exported version of colorTag for use in views.
func ColorTag(c tcell.Color) string {
	return colorTag(c)
}
