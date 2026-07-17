package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Wizard — multi-step form with info panel
// ─────────────────────────────────────────────────────────────────────────────

// InfoField represents a key/value pair displayed in the info panel.
type InfoField struct {
	Label string
	Value string
}

// WizardStep defines a single step in the wizard.
type WizardStep struct {
	// Label is displayed in the info panel progression.
	Label string

	// Form builds the tview form for this step.
	// It receives the app and an onDone callback. The step MUST call onDone()
	// when the user confirms (typically from a button callback).
	// Esc triggers skip (double-press required; blocked if Required is true).
	// Return nil to skip the form (processing-only step).
	// Mutually exclusive with CustomView.
	Form func(app *tview.Application, onDone func()) *tview.Form

	// CustomView provides a custom rendering for this step instead of a tview.Form.
	// When non-nil, Form is ignored. Use this for gate questions (Yes/No lists)
	// or other non-form UIs. The function receives the app, the container Flex
	// to insert content into, and onDone to signal step completion.
	CustomView func(app *tview.Application, container *tview.Flex, onDone func())

	// OnDone is called after the form completes (user pressed the confirm button).
	// Run side effects here (API calls, file writes).
	// The spinner is shown during execution.
	OnDone func() error

	// InfoFields returns key/value pairs to display in the info panel
	// after this step completes successfully. Called after OnDone succeeds.
	// If nil, no info is added for this step.
	InfoFields func() []InfoField

	// Processing is the message shown while OnDone executes.
	Processing string

	// Skip indicates this step is already satisfied and should be skipped entirely.
	Skip bool

	// SkipIf is evaluated dynamically just before rendering the step.
	// If non-nil and returns true, the step is skipped automatically.
	SkipIf func() bool

	// Required prevents the user from skipping this step via Escape.
	// The user must either complete the form or quit the wizard (Ctrl+C).
	Required bool
}

// WizardConfig configures the wizard.
type WizardConfig struct {
	Layout layout.Config
	Steps  []WizardStep
}

// WizardResult is returned after the wizard completes.
type WizardResult struct {
	Completed bool
	Aborted   bool
	Err       error
}

// RunWizard launches a full-screen multi-step wizard with info panel.
func RunWizard(cfg WizardConfig) WizardResult {
	if len(cfg.Steps) == 0 {
		return WizardResult{Completed: true}
	}

	var result WizardResult

	// Force InfoPanel on for wizards
	cfg.Layout.InfoPanel = true
	// Add Ctrl+S hint to status
	if cfg.Layout.StatusHints != "" {
		cfg.Layout.StatusHints += " · "
	}
	cfg.Layout.StatusHints += "ctrl+s submit"

	// Wire OnQuit to set Aborted
	cfg.Layout.OnQuit = func() {
		result.Aborted = true
	}

	// Build the shell layout
	shell := layout.Build(cfg.Layout)

	var currentStep int
	var spinner *widgets.Spinner

	// Double-Esc state: first Esc shows confirmation, second Esc confirms skip
	var escPending bool
	var escTimer *time.Timer
	// Compute the full original hints string (same logic as layout.Build)
	originalHints := cfg.Layout.StatusHints
	if originalHints != "" {
		originalHints += " · "
	}
	originalHints += "ctrl+n menu · ctrl+c quit"

	// Accumulated info fields from completed steps
	type stepInfoEntry struct {
		label  string
		fields []InfoField
	}
	infoAccumulator := make([]stepInfoEntry, 0)

	// Build step bar data (for info panel display)
	steps := make([]widgets.Step, len(cfg.Steps))
	for i, s := range cfg.Steps {
		if s.Skip {
			steps[i] = widgets.Step{Label: s.Label, Status: widgets.StepDone}
		} else {
			steps[i] = widgets.Step{Label: s.Label, Status: widgets.StepPending}
		}
	}

	// Find first non-skipped step
	currentStep = -1
	for i, s := range cfg.Steps {
		if !s.Skip {
			currentStep = i
			steps[i].Status = widgets.StepActive
			break
		}
	}
	if currentStep == -1 {
		return WizardResult{Completed: true}
	}

	// ── Widgets ──
	formContainer := tview.NewFlex().SetDirection(tview.FlexRow)
	formContainer.SetBackgroundColor(theme.BgPanel)

	spinner = widgets.NewSpinner("Processing...")
	spinner.SetBackgroundColor(theme.BgPanel)

	// ── Insert only the form container into the content panel ──
	// (No step bar — the info panel shows progression)
	shell.Content.AddItem(formContainer, 0, 1, true)

	// ── Info panel rendering ──
	renderInfoPanel := func() {
		if shell.InfoPanel == nil {
			return
		}
		var b strings.Builder

		// Completed steps with their info fields
		for _, si := range infoAccumulator {
			b.WriteString(fmt.Sprintf("  %s%s[-] %s%s[-]\n",
				widgets.ColorTag(theme.Success), theme.IconDone,
				widgets.ColorTag(theme.FgPrimary), si.label))
			for _, f := range si.fields {
				if f.Value != "" {
					b.WriteString(fmt.Sprintf("    %s%s:[-] %s\n",
						widgets.ColorTag(theme.FgSecondary), f.Label, f.Value))
				}
			}
			b.WriteString("\n")
		}

		// Current and remaining steps
		for i := currentStep; i < len(cfg.Steps); i++ {
			if cfg.Steps[i].Skip || steps[i].Status == widgets.StepDone {
				continue
			}
			if steps[i].Status == widgets.StepSkipped {
				b.WriteString(fmt.Sprintf("  %s%s %s (skipped)[-]\n",
					widgets.ColorTag(theme.FgMuted), theme.IconSkipped, cfg.Steps[i].Label))
				continue
			}
			icon := theme.IconPending
			color := widgets.ColorTag(theme.FgMuted)
			if i == currentStep {
				icon = theme.IconActive
				color = widgets.ColorTag(theme.Accent)
			}
			b.WriteString(fmt.Sprintf("  %s%s %s[-]\n", color, icon, cfg.Steps[i].Label))
		}

		shell.InfoPanel.SetText(b.String())
	}
	renderInfoPanel()

	// ── Helper: should step be skipped ──
	shouldSkip := func(idx int) bool {
		s := cfg.Steps[idx]
		if s.SkipIf != nil {
			return s.SkipIf()
		}
		return s.Skip
	}

	// ── Helper: find next pending step ──
	findNext := func(from int) int {
		for i := from + 1; i < len(cfg.Steps); i++ {
			if !shouldSkip(i) && steps[i].Status == widgets.StepPending {
				return i
			}
		}
		return -1
	}

	// ── Helper: skip current step (Esc) ──
	skipCurrent := func() {
		steps[currentStep] = widgets.Step{Label: steps[currentStep].Label, Status: widgets.StepSkipped}
		next := findNext(currentStep)
		if next == -1 {
			result = WizardResult{Completed: true}
			shell.App.Stop()
			return
		}
		currentStep = next
		steps[next] = widgets.Step{Label: steps[next].Label, Status: widgets.StepActive}
		renderInfoPanel()
	}

	// ── Helper: advance after OnDone ──
	advanceAfterDone := func(step WizardStep) {
		// Collect info fields
		if step.InfoFields != nil {
			fields := step.InfoFields()
			infoAccumulator = append(infoAccumulator, stepInfoEntry{label: step.Label, fields: fields})
		} else {
			infoAccumulator = append(infoAccumulator, stepInfoEntry{label: step.Label, fields: nil})
		}

		steps[currentStep] = widgets.Step{Label: steps[currentStep].Label, Status: widgets.StepDone}
		next := findNext(currentStep)
		if next == -1 {
			result = WizardResult{Completed: true}
			shell.App.Stop()
			return
		}
		currentStep = next
		steps[next] = widgets.Step{Label: steps[next].Label, Status: widgets.StepActive}
		renderInfoPanel()
	}

	// ── Helper: run OnDone with spinner ──
	runWithSpinner := func(step WizardStep, afterDone func()) {
		msg := "Processing..."
		if step.Processing != "" {
			msg = step.Processing
		}
		spinner.SetMessage(msg)
		formContainer.Clear()
		formContainer.AddItem(spinner.TextView, 3, 0, false)
		spinner.Start(shell.App)

		go func() {
			var err error
			if step.OnDone != nil {
				err = step.OnDone()
			}
			shell.App.QueueUpdateDraw(func() {
				spinner.Stop()
				if err != nil {
					result = WizardResult{Err: err}
					shell.App.Stop()
					return
				}
				afterDone()
			})
		}()
	}

	// ── Step rendering ──
	var doRenderStep func(int)

	renderStep := func(idx int) {
		step := cfg.Steps[idx]

		// Dynamic skip check
		if shouldSkip(idx) {
			steps[idx] = widgets.Step{Label: steps[idx].Label, Status: widgets.StepDone}
			next := findNext(idx)
			if next == -1 {
				result = WizardResult{Completed: true}
				shell.App.Stop()
				return
			}
			currentStep = next
			steps[next] = widgets.Step{Label: steps[next].Label, Status: widgets.StepActive}
			renderInfoPanel()
			doRenderStep(next)
			return
		}

		// Clear form container
		formContainer.Clear()

		// Reset double-Esc state when switching steps
		escPending = false
		if escTimer != nil {
			escTimer.Stop()
			escTimer = nil
		}
		shell.StatusBar.SetHints(originalHints)

		// ── CustomView path ──
		if step.CustomView != nil {
			onDone := func() {
				runWithSpinner(step, func() {
					advanceAfterDone(step)
					if !result.Completed {
						doRenderStep(currentStep)
					}
				})
			}
			step.CustomView(shell.App, formContainer, onDone)
			return
		}

		// ── Form path ──
		if step.Form != nil {
			onDone := func() {
				runWithSpinner(step, func() {
					advanceAfterDone(step)
					if !result.Completed {
						doRenderStep(currentStep)
					}
				})
			}

			form := step.Form(shell.App, onDone)
			if form != nil {
				// Apply theme to the form
				form.SetBackgroundColor(theme.BgPanel)
				form.SetFieldBackgroundColor(theme.BgElement)
				form.SetFieldTextColor(theme.FgPrimary)
				form.SetLabelColor(theme.FgPrimary)
				form.SetButtonBackgroundColor(theme.Accent)
				form.SetButtonTextColor(theme.BgPanel)
				form.SetBorder(false)

			// Esc handling: Required steps block skip; optional steps use double-Esc
			form.SetCancelFunc(func() {
				// Required steps cannot be skipped
				if step.Required {
					shell.StatusBar.SetHints("This step is required — press ctrl+c to quit")
					time.AfterFunc(2*time.Second, func() {
						shell.App.QueueUpdateDraw(func() {
							shell.StatusBar.SetHints(originalHints)
						})
					})
					return
				}

				if !escPending {
					// First Esc: show confirmation hint
					escPending = true
					shell.StatusBar.SetHints("Press Esc again to skip this step")
					escTimer = time.AfterFunc(2*time.Second, func() {
						shell.App.QueueUpdateDraw(func() {
							escPending = false
							shell.StatusBar.SetHints(originalHints)
						})
					})
					return
				}

				// Second Esc within 2s: confirm skip
				escPending = false
				if escTimer != nil {
					escTimer.Stop()
				}
				shell.StatusBar.SetHints(originalHints)
				skipCurrent()
				if !result.Completed {
					doRenderStep(currentStep)
				}
			})

				// Ctrl+S = submit (same as pressing the button)
				form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
					if event.Key() == tcell.KeyCtrlS {
						onDone()
						return nil
					}
					return event
				})

				formContainer.AddItem(form, 0, 1, true)
				shell.App.SetFocus(form)
			}
		} else {
			// No form, no CustomView — directly run OnDone with spinner
			runWithSpinner(step, func() {
				advanceAfterDone(step)
				if !result.Completed {
					doRenderStep(currentStep)
				}
			})
		}
	}
	doRenderStep = renderStep

	// Initial render
	renderStep(currentStep)

	// Run
	if err := shell.App.SetRoot(shell.Root, true).EnableMouse(true).Run(); err != nil {
		return WizardResult{Err: err}
	}

	return result
}
