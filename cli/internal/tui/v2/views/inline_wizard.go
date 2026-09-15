package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// InlineWizard — multi-step wizard view that runs inside the TUI shell
// ─────────────────────────────────────────────────────────────────────────────
//
// Unlike RunWizard (wizard.go), which creates a separate tview.Application,
// InlineWizardView implements the View interface and runs within the existing
// shell. This avoids the jarring alt-screen switch and keeps the omnibar,
// toasts, and shell chrome accessible throughout the wizard.
//
// The component reuses the same WizardStep struct as the standalone wizard
// for full step-definition compatibility.

// InlineWizardConfig configures an inline wizard view.
type InlineWizardConfig struct {
	// ID is the unique view identifier for the router (e.g. "wizard.team.init").
	ID string
	// Title is displayed in the shell breadcrumb.
	Title string
	// Steps defines the wizard steps. Same struct as the standalone wizard.
	Steps []WizardStep
	// OnComplete is called when the wizard finishes.
	// completed=true means all steps done; completed=false means user aborted.
	OnComplete func(completed bool, err error)
	// SummaryTargetView is the view ID to propose navigating to after completion
	// (e.g. "team.detail", "settings"). If empty, only "return home" is offered.
	SummaryTargetView string
	// SummaryTargetLabel is the label for the navigation proposal
	// (e.g. "Voir la configuration de l'équipe"). If empty, a default is used.
	SummaryTargetLabel string
}

// InlineWizardView is a multi-step wizard that runs inside the TUI shell
// as a regular View, pushed onto the router stack.
type InlineWizardView struct {
	cfg InlineWizardConfig

	// shell is injected via SetShell (shellAware interface).
	shell ShellAccess

	// tview primitives — nil when unmounted.
	app         *tview.Application
	mainFlex    *tview.Flex     // root layout: stepBar + header + content + info + hints
	stepBar     *widgets.StepBar
	stepHeader  *tview.TextView
	stepContent *tview.Flex     // swappable area for form/customview/spinner
	infoPanel   *tview.TextView
	hintsBar    *widgets.StatusBar

	// wizard state
	currentStep int
	stepStates  []widgets.Step
	infoAccum   []inlineStepInfoEntry
	escPending  bool
	completed   bool
	aborted     bool
	wizardErr   error

	// spinner for async OnDone operations
	spinner *widgets.Spinner

	// forward declaration for recursive step rendering
	doRenderStep func(int)
}

type inlineStepInfoEntry struct {
	label  string
	fields []InfoField
}

// Ensure InlineWizardView implements View.
var _ View = (*InlineWizardView)(nil)

// NewInlineWizardView creates an inline wizard view.
// Push it onto the shell router via shell.PushView(v).
func NewInlineWizardView(cfg InlineWizardConfig) *InlineWizardView {
	return &InlineWizardView{
		cfg: cfg,
	}
}

// SetShell provides the shell reference (shellAware interface).
func (w *InlineWizardView) SetShell(s ShellAccess) { w.shell = s }

// ─────────────────────────────────────────────────────────────────────────────
// View interface
// ─────────────────────────────────────────────────────────────────────────────

func (w *InlineWizardView) ID() string    { return w.cfg.ID }
func (w *InlineWizardView) Title() string { return w.cfg.Title }

func (w *InlineWizardView) StatusHints() string {
	return "ctrl+s " + i18n.T("wizard.hint.submit") +
		" · ctrl+b " + i18n.T("wizard.hint.back") +
		" · esc " + i18n.T("wizard.hint.skip")
}

func (w *InlineWizardView) Mount(content *tview.Flex, app *tview.Application) {
	w.app = app
	w.completed = false
	w.aborted = false
	w.wizardErr = nil
	w.escPending = false

	if len(w.cfg.Steps) == 0 {
		w.completed = true
		w.fireComplete()
		return
	}

	// ── Initialize step states ──
	w.stepStates = make([]widgets.Step, len(w.cfg.Steps))
	for i, s := range w.cfg.Steps {
		if s.Skip {
			w.stepStates[i] = widgets.Step{Label: s.Label, Status: widgets.StepDone}
		} else {
			w.stepStates[i] = widgets.Step{Label: s.Label, Status: widgets.StepPending}
		}
	}
	w.infoAccum = make([]inlineStepInfoEntry, 0)

	// Find first non-skipped step
	w.currentStep = -1
	for i, s := range w.cfg.Steps {
		if !s.Skip {
			w.currentStep = i
			w.stepStates[i].Status = widgets.StepActive
			break
		}
	}
	if w.currentStep == -1 {
		w.completed = true
		w.fireComplete()
		return
	}

	// ── Build layout ──

	// Step bar (horizontal progress)
	w.stepBar = widgets.NewStepBar(w.stepStates)

	// Step header ("◆ 2/5 — Label")
	w.stepHeader = tview.NewTextView().SetDynamicColors(true)
	w.stepHeader.SetBackgroundColor(theme.BgPanel)

	// Swappable step content area
	w.stepContent = tview.NewFlex().SetDirection(tview.FlexRow)
	w.stepContent.SetBackgroundColor(theme.BgPanel)

	// Info panel (accumulated results)
	w.infoPanel = tview.NewTextView().SetDynamicColors(true)
	w.infoPanel.SetBackgroundColor(theme.BgPanel)

	// Hints bar
	w.hintsBar = widgets.NewStatusBar(w.StatusHints())

	// Spinner (reused across steps)
	w.spinner = widgets.NewSpinner(i18n.T("wizard.processing"))
	w.spinner.SetBackgroundColor(theme.BgPanel)

	// Assemble the main layout
	w.mainFlex = tview.NewFlex().SetDirection(tview.FlexRow)
	w.mainFlex.SetBackgroundColor(theme.BgPanel)
	w.mainFlex.
		AddItem(w.stepBar.TextView, 1, 0, false).       // step bar (1 row)
		AddItem(w.stepHeader, 2, 0, false).              // step header (2 rows)
		AddItem(w.stepContent, 0, 1, true).              // step content (fills)
		AddItem(w.infoPanel, 0, 0, false).               // info panel (dynamic, starts hidden)
		AddItem(w.hintsBar.TextView, 1, 0, false)        // hints bar (1 row)

	content.AddItem(w.mainFlex, 0, 1, true)

	// ── Wire forward declaration and render first step ──
	w.doRenderStep = w.renderStep
	w.renderInfoPanel()
	w.renderStep(w.currentStep)
}

func (w *InlineWizardView) Unmount() {
	if w.spinner != nil {
		w.spinner.Stop()
	}
	if !w.completed && !w.aborted {
		w.aborted = true
		w.fireComplete()
	}
	w.app = nil
	w.mainFlex = nil
	w.stepBar = nil
	w.stepHeader = nil
	w.stepContent = nil
	w.infoPanel = nil
	w.hintsBar = nil
	w.spinner = nil
	w.doRenderStep = nil
}

func (w *InlineWizardView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if w.app == nil || w.stepContent == nil {
		return event
	}
	// The step content (form/customview) has its own InputCapture that handles
	// Ctrl+S, Ctrl+B, and Esc. We only need to handle Esc at the wizard level
	// when in the summary screen (completed state).
	if w.completed {
		return w.handleSummaryKey(event)
	}
	return event
}

// ─────────────────────────────────────────────────────────────────────────────
// Step navigation helpers
// ─────────────────────────────────────────────────────────────────────────────

func (w *InlineWizardView) shouldSkip(idx int) bool {
	s := w.cfg.Steps[idx]
	if s.SkipIf != nil {
		return s.SkipIf()
	}
	return s.Skip
}

func (w *InlineWizardView) countVisibleSteps(idx int) (total int, position int) {
	pos := 0
	for i := range w.cfg.Steps {
		if w.cfg.Steps[i].Skip {
			continue
		}
		if w.cfg.Steps[i].SkipIf != nil && w.cfg.Steps[i].SkipIf() {
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

func (w *InlineWizardView) findNext(from int) int {
	for i := from + 1; i < len(w.cfg.Steps); i++ {
		if !w.shouldSkip(i) && w.stepStates[i].Status == widgets.StepPending {
			return i
		}
	}
	return -1
}

func (w *InlineWizardView) findPrev(from int) int {
	for i := from - 1; i >= 0; i-- {
		if !w.shouldSkip(i) && (w.stepStates[i].Status == widgets.StepDone || w.stepStates[i].Status == widgets.StepActive) {
			return i
		}
	}
	return -1
}

func (w *InlineWizardView) skipCurrent() {
	w.stepStates[w.currentStep] = widgets.Step{
		Label:  w.stepStates[w.currentStep].Label,
		Status: widgets.StepSkipped,
	}
	w.stepBar.UpdateStatus(w.currentStep, widgets.StepSkipped)

	next := w.findNext(w.currentStep)
	if next == -1 {
		w.wizardComplete()
		return
	}
	w.currentStep = next
	w.stepStates[next] = widgets.Step{Label: w.stepStates[next].Label, Status: widgets.StepActive}
	w.stepBar.UpdateStatus(next, widgets.StepActive)
	w.renderInfoPanel()
}

func (w *InlineWizardView) goBack() {
	prev := w.findPrev(w.currentStep)
	if prev == -1 {
		return
	}
	// Reset current to pending
	w.stepStates[w.currentStep] = widgets.Step{
		Label:  w.stepStates[w.currentStep].Label,
		Status: widgets.StepPending,
	}
	w.stepBar.UpdateStatus(w.currentStep, widgets.StepPending)

	// Reset previous to active
	w.stepStates[prev] = widgets.Step{Label: w.stepStates[prev].Label, Status: widgets.StepActive}
	w.stepBar.UpdateStatus(prev, widgets.StepActive)

	// Pop last info entry
	if len(w.infoAccum) > 0 {
		w.infoAccum = w.infoAccum[:len(w.infoAccum)-1]
	}
	w.currentStep = prev
	w.renderInfoPanel()
	w.doRenderStep(prev)
}

func (w *InlineWizardView) advanceAfterDone(step WizardStep) {
	// Collect info fields
	if step.InfoFields != nil {
		fields := step.InfoFields()
		w.infoAccum = append(w.infoAccum, inlineStepInfoEntry{label: step.Label, fields: fields})
	} else {
		w.infoAccum = append(w.infoAccum, inlineStepInfoEntry{label: step.Label, fields: nil})
	}

	w.stepStates[w.currentStep] = widgets.Step{
		Label:  w.stepStates[w.currentStep].Label,
		Status: widgets.StepDone,
	}
	w.stepBar.UpdateStatus(w.currentStep, widgets.StepDone)

	next := w.findNext(w.currentStep)
	if next == -1 {
		w.wizardComplete()
		return
	}
	w.currentStep = next
	w.stepStates[next] = widgets.Step{Label: w.stepStates[next].Label, Status: widgets.StepActive}
	w.stepBar.UpdateStatus(next, widgets.StepActive)
	w.renderInfoPanel()
}

func (w *InlineWizardView) runWithSpinner(step WizardStep, afterDone func()) {
	msg := i18n.T("wizard.processing")
	if step.Processing != "" {
		msg = step.Processing
	}
	w.spinner.SetMessage(msg)
	w.stepContent.Clear()
	w.stepContent.AddItem(w.spinner.TextView, 3, 0, false)
	w.spinner.Start(w.app)

	go func() {
		var err error
		if step.OnDone != nil {
			err = step.OnDone()
		}
		w.app.QueueUpdateDraw(func() {
			w.spinner.Stop()
			if err != nil {
				w.wizardErr = err
				// Show error in the step content but do NOT mark as completed.
				// The user can press Ctrl+B to go back and fix the issue,
				// or Enter to retry the current step.
				w.stepContent.Clear()
				errView := tview.NewTextView().SetDynamicColors(true)
				errView.SetBackgroundColor(theme.BgPanel)
				errView.SetText(fmt.Sprintf("  %s%s %s[-]\n\n  %s%s[-]\n\n  %sCtrl+B[-] retour  •  %sEnter[-] réessayer",
					widgets.ColorTag(theme.Error), theme.IconError,
					i18n.T("wizard.error"),
					widgets.ColorTag(theme.FgSecondary), err.Error(),
					widgets.ColorTag(theme.Accent),
					widgets.ColorTag(theme.Accent)))
				errView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
					switch {
					case event.Key() == tcell.KeyCtrlB:
						w.goBack()
						return nil
					case event.Key() == tcell.KeyEnter:
						// Retry: re-render the current step
						w.wizardErr = nil
						w.renderStep(w.currentStep)
						return nil
					}
					return event
				})
				w.stepContent.AddItem(errView, 0, 1, true)
				w.app.SetFocus(errView)
				return
			}
			afterDone()
		})
	}()
}

// ─────────────────────────────────────────────────────────────────────────────
// Step rendering
// ─────────────────────────────────────────────────────────────────────────────

func (w *InlineWizardView) renderStep(idx int) {
	step := w.cfg.Steps[idx]

	// Dynamic skip check
	if w.shouldSkip(idx) {
		w.stepStates[idx] = widgets.Step{Label: w.stepStates[idx].Label, Status: widgets.StepDone}
		w.stepBar.UpdateStatus(idx, widgets.StepDone)
		next := w.findNext(idx)
		if next == -1 {
			w.wizardComplete()
			return
		}
		w.currentStep = next
		w.stepStates[next] = widgets.Step{Label: w.stepStates[next].Label, Status: widgets.StepActive}
		w.stepBar.UpdateStatus(next, widgets.StepActive)
		w.renderInfoPanel()
		w.doRenderStep(next)
		return
	}

	// Clear step content
	w.stepContent.Clear()

	// Step counter header ("◆ 2/5 — Label")
	totalSteps, stepPos := w.countVisibleSteps(idx)
	w.stepHeader.SetText(fmt.Sprintf("  %s%s %d/%d — %s[-]",
		widgets.ColorTag(theme.Accent), theme.IconActive, stepPos, totalSteps, step.Label))

	// Wire Ctrl+B on the step content container
	w.stepContent.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlB {
			w.goBack()
			return nil
		}
		return event
	})

	// Reset double-Esc state
	w.escPending = false
	w.hintsBar.SetHints(w.StatusHints())

	// ── CustomView path ──
	if step.CustomView != nil {
		onDone := func() {
			w.runWithSpinner(step, func() {
				w.advanceAfterDone(step)
				if !w.completed {
					w.doRenderStep(w.currentStep)
				}
			})
		}
		step.CustomView(w.app, w.stepContent, onDone)
		return
	}

	// ── Form path ──
	if step.Form != nil {
		onDone := func() {
			// Run validation if defined
			if step.Validate != nil {
				if errMsg := step.Validate(); errMsg != "" {
					w.hintsBar.SetHints(fmt.Sprintf("%s%s[-]",
						widgets.ColorTag(theme.Error), errMsg))
					time.AfterFunc(3*time.Second, func() {
						if w.app != nil {
							w.app.QueueUpdateDraw(func() {
								if w.hintsBar != nil {
									w.hintsBar.SetHints(w.StatusHints())
								}
							})
						}
					})
					return
				}
			}
			w.runWithSpinner(step, func() {
				w.advanceAfterDone(step)
				if !w.completed {
					w.doRenderStep(w.currentStep)
				}
			})
		}

		form := step.Form(w.app, onDone)
		if form != nil {
			// Apply theme
			form.SetBackgroundColor(theme.BgPanel)
			form.SetFieldBackgroundColor(theme.BgElement)
			form.SetFieldTextColor(theme.FgPrimary)
			form.SetLabelColor(theme.FgPrimary)
			form.SetButtonStyle(tcell.StyleDefault.
				Background(theme.Accent).
				Foreground(theme.BgPanel))
			form.SetButtonActivatedStyle(tcell.StyleDefault.
				Background(theme.Action).
				Foreground(theme.BgPanel))
			form.SetBorder(false)

			// Fix DropDown popup list colors (tview bakes them at construction
			// time from tview.Styles, which produces invisible text in our theme).
			fixFormDropDownStyles(form)

			// Esc handling: Required steps block skip; optional use double-Esc
			form.SetCancelFunc(func() {
				if step.Required {
					w.hintsBar.SetHints(i18n.T("wizard.step_required"))
					time.AfterFunc(2*time.Second, func() {
						if w.app != nil {
							w.app.QueueUpdateDraw(func() {
								if w.hintsBar != nil {
									w.hintsBar.SetHints(w.StatusHints())
								}
							})
						}
					})
					return
				}

				if !w.escPending {
					w.escPending = true
					w.hintsBar.SetHints(i18n.T("wizard.esc_to_skip"))
					return
				}

				// Second Esc: confirm skip
				w.escPending = false
				w.hintsBar.SetHints(w.StatusHints())
				w.skipCurrent()
				if !w.completed {
					w.doRenderStep(w.currentStep)
				}
			})

			// Ctrl+S = submit, Ctrl+B = back
			form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				// Any non-Esc key resets double-Esc pending state
				if w.escPending && event.Key() != tcell.KeyEscape {
					w.escPending = false
					w.hintsBar.SetHints(w.StatusHints())
				}
				if event.Key() == tcell.KeyCtrlS {
					onDone()
					return nil
				}
				if event.Key() == tcell.KeyCtrlB {
					w.goBack()
					return nil
				}
				return event
			})

			w.stepContent.AddItem(form, 0, 1, true)
			w.app.SetFocus(form)
		}
	} else {
		// No form, no CustomView — processing-only step
		w.runWithSpinner(step, func() {
			w.advanceAfterDone(step)
			if !w.completed {
				w.doRenderStep(w.currentStep)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Info panel rendering
// ─────────────────────────────────────────────────────────────────────────────

func (w *InlineWizardView) renderInfoPanel() {
	if w.infoPanel == nil {
		return
	}

	var b strings.Builder

	// Completed steps with their info fields
	for _, si := range w.infoAccum {
		fmt.Fprintf(&b, "  %s%s[-] %s%s[-]\n",
			widgets.ColorTag(theme.Success), theme.IconDone,
			widgets.ColorTag(theme.FgPrimary), si.label)
		for _, f := range si.fields {
			if f.Value != "" {
				fmt.Fprintf(&b, "    %s%s:[-] %s\n",
					widgets.ColorTag(theme.FgSecondary), f.Label, f.Value)
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
				widgets.ColorTag(theme.FgMuted), theme.IconSkipped, w.cfg.Steps[i].Label)
			continue
		}
		icon := theme.IconPending
		color := widgets.ColorTag(theme.FgMuted)
		if i == w.currentStep {
			icon = theme.IconActive
			color = widgets.ColorTag(theme.Accent)
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

func (w *InlineWizardView) updateInfoPanelSize(lines int) {
	if w.mainFlex == nil || w.infoPanel == nil {
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

// ─────────────────────────────────────────────────────────────────────────────
// Wizard completion and summary screen
// ─────────────────────────────────────────────────────────────────────────────

func (w *InlineWizardView) wizardComplete() {
	w.completed = true
	w.renderSummaryScreen()
	w.fireComplete()
}

func (w *InlineWizardView) fireComplete() {
	if w.cfg.OnComplete != nil {
		w.cfg.OnComplete(w.completed && !w.aborted, w.wizardErr)
	}
}

func (w *InlineWizardView) renderSummaryScreen() {
	if w.stepContent == nil {
		return
	}

	// Mark all remaining as done in the step bar
	for i := range w.stepStates {
		if w.stepStates[i].Status == widgets.StepActive {
			w.stepStates[i].Status = widgets.StepDone
			w.stepBar.UpdateStatus(i, widgets.StepDone)
		}
	}

	// Update header
	w.stepHeader.SetText(fmt.Sprintf("  %s%s %s[-]",
		widgets.ColorTag(theme.Success), theme.IconSuccess,
		i18n.T("wizard.complete")))

	// Build summary content
	w.stepContent.Clear()
	summary := tview.NewTextView().SetDynamicColors(true)
	summary.SetBackgroundColor(theme.BgPanel)

	var b strings.Builder
	b.WriteString("\n")

	for _, si := range w.infoAccum {
		fmt.Fprintf(&b, "  %s%s[-] %s%s[-]\n",
			widgets.ColorTag(theme.Success), theme.IconDone,
			widgets.ColorTag(theme.FgPrimary), si.label)
		for _, f := range si.fields {
			if f.Value != "" {
				fmt.Fprintf(&b, "      %s%s:[-] %s\n",
					widgets.ColorTag(theme.FgSecondary), f.Label, f.Value)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Navigation proposals
	if w.cfg.SummaryTargetView != "" {
		label := w.cfg.SummaryTargetLabel
		if label == "" {
			label = i18n.T("wizard.summary.goto_detail")
		}
		fmt.Fprintf(&b, "  %s[Enter][-] %s\n",
			widgets.ColorTag(theme.Accent), label)
	}
	fmt.Fprintf(&b, "  %s[Esc][-]   %s\n",
		widgets.ColorTag(theme.FgMuted), i18n.T("wizard.summary.go_home"))

	summary.SetText(b.String())
	w.stepContent.AddItem(summary, 0, 1, true)

	// Update hints
	hints := "enter " + i18n.T("wizard.hint.navigate") + " · esc " + i18n.T("wizard.hint.close")
	w.hintsBar.SetHints(hints)

	// Update info panel to show final state
	w.renderInfoPanel()
}

func (w *InlineWizardView) handleSummaryKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter:
		if w.cfg.SummaryTargetView != "" && w.shell != nil {
			w.shell.NavigateTo(w.cfg.SummaryTargetView)
		}
		return nil
	case tcell.KeyEscape:
		// Let the shell's global handler pop this view
		return event
	}
	return event
}

// ─────────────────────────────────────────────────────────────────────────────
// DropDown style fix
// ─────────────────────────────────────────────────────────────────────────────

// fixFormDropDownStyles iterates over all form items and applies the correct
// list popup styles to any DropDown. This is needed because tview.DropDown
// bakes the popup list colors from tview.Styles at construction time, and
// our shell theme produces invisible text (same color as background).
// See ADR-034 for the full analysis.
func fixFormDropDownStyles(form *tview.Form) {
	if form == nil {
		return
	}
	unselected := tcell.StyleDefault.
		Background(theme.BgElement).
		Foreground(theme.FgPrimary)
	selected := tcell.StyleDefault.
		Background(theme.Accent).
		Foreground(theme.BgPanel)

	for i := 0; i < form.GetFormItemCount(); i++ {
		if dd, ok := form.GetFormItem(i).(*tview.DropDown); ok {
			dd.SetListStyles(unselected, selected)
		}
	}
}
