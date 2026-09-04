package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
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

	// Validate is called before OnDone when the user submits the form.
	// If it returns a non-empty string, the submission is blocked and
	// the error message is displayed in the StatusBar for 3 seconds.
	// If nil, no validation is performed (always passes).
	Validate func() string
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
	cfg.Layout.StatusHints += "ctrl+s submit · ctrl+b back"

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

	// ── Helper: count visible steps and position of current step ──
	countVisibleSteps := func(idx int) (total int, position int) {
		pos := 0
		for i := range cfg.Steps {
			if cfg.Steps[i].Skip {
				continue
			}
			if cfg.Steps[i].SkipIf != nil && cfg.Steps[i].SkipIf() {
				continue
			}
			total++
			if i < idx {
				pos++
			} else if i == idx {
				pos++
				position = pos
			}
		}
		if position == 0 {
			position = total
		}
		return total, position
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

	// ── Helper: find previous completed/active step ──
	findPrev := func(from int) int {
		for i := from - 1; i >= 0; i-- {
			if !shouldSkip(i) && (steps[i].Status == widgets.StepDone || steps[i].Status == widgets.StepActive) {
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

	// ── Step rendering (forward declaration for goBack) ──
	var doRenderStep func(int)

	// ── Helper: go back to previous step (Ctrl+B) ──
	goBack := func() {
		prev := findPrev(currentStep)
		if prev == -1 {
			return // can't go further back
		}
		// Reset current step to pending
		steps[currentStep] = widgets.Step{Label: steps[currentStep].Label, Status: widgets.StepPending}
		// Reset previous step to active
		steps[prev] = widgets.Step{Label: steps[prev].Label, Status: widgets.StepActive}
		// Remove info entry for the previous step (it was completed, now we're re-doing it)
		if len(infoAccumulator) > 0 {
			infoAccumulator = infoAccumulator[:len(infoAccumulator)-1]
		}
		currentStep = prev
		renderInfoPanel()
		doRenderStep(currentStep)
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
		msg := i18n.T("wizard.processing")
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

		// Step counter header (e.g. "◆ 2/5 — Identité")
		totalSteps, stepPos := countVisibleSteps(idx)
		stepHeader := tview.NewTextView().SetDynamicColors(true)
		stepHeader.SetBackgroundColor(theme.BgPanel)
		stepHeader.SetText(fmt.Sprintf("  %s%s %d/%d — %s[-]",
			widgets.ColorTag(theme.Accent), theme.IconActive, stepPos, totalSteps, step.Label))
		formContainer.AddItem(stepHeader, 2, 0, false)

		formContainer.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyCtrlB {
				goBack()
				return nil
			}
			return event
		})

		// Reset double-Esc state when switching steps
		escPending = false
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
				// Run validation if defined
				if step.Validate != nil {
					if errMsg := step.Validate(); errMsg != "" {
						shell.StatusBar.SetHints(fmt.Sprintf("%s%s[-]", widgets.ColorTag(theme.Error), errMsg))
						time.AfterFunc(3*time.Second, func() {
							shell.App.QueueUpdateDraw(func() {
								shell.StatusBar.SetHints(originalHints)
							})
						})
						return
					}
				}
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
					shell.StatusBar.SetHints(i18n.T("wizard.step_required"))
					time.AfterFunc(2*time.Second, func() {
						shell.App.QueueUpdateDraw(func() {
							shell.StatusBar.SetHints(originalHints)
						})
					})
					return
				}

				if !escPending {
					// First Esc: show persistent confirmation hint
					escPending = true
					shell.StatusBar.SetHints(i18n.T("wizard.esc_to_skip"))
					return
				}

				// Second Esc: confirm skip
				escPending = false
				shell.StatusBar.SetHints(originalHints)
				skipCurrent()
				if !result.Completed {
					doRenderStep(currentStep)
				}
			})

				// Ctrl+S = submit (same as pressing the button)
				form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
					// Any non-Esc key resets the double-Esc pending state
					if escPending && event.Key() != tcell.KeyEscape {
						escPending = false
						shell.StatusBar.SetHints(originalHints)
					}
					if event.Key() == tcell.KeyCtrlS {
						onDone()
						return nil
					}
					if event.Key() == tcell.KeyCtrlB {
						goBack()
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
