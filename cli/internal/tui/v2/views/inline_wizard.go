package views

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"golang.org/x/term"

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
	// SummaryTargetViewFunc, if set, is called at summary render time instead of
	// using the static SummaryTargetView field. Use this when the target depends
	// on wizard choices (e.g. project creation).
	SummaryTargetViewFunc func() string
	// SummaryTargetLabelFunc, if set, is called at summary render time instead of
	// using the static SummaryTargetLabel field.
	SummaryTargetLabelFunc func() string
	// Groups, when non-empty, switches the wizard to a "grouped" layout:
	// centered content, right-side info sidebar, and the step bar shows
	// group labels instead of individual step labels. Each group starts
	// at the step index specified by StartIdx. When empty, the classic
	// full-width layout with bottom info panel is used.
	Groups []StepGroup

	// RefreshLabels, when set, is called at the beginning of each step
	// render to re-evaluate locale-dependent labels. Use this to support
	// live locale switching: the callback should update Steps[i].Label,
	// Steps[i].Processing, Groups[i].Label, and Title with fresh i18n.T()
	// values. The engine then syncs stepStates and groupStates.
	RefreshLabels func()

	// FocusButtonAfterRender, when non-nil and pointing to true, causes the
	// engine to focus the external buttonForm (grouped mode) instead of the
	// form after the next renderStep. The flag is reset to false after use.
	// Use this when a DropDown callback triggers a rerender and the user
	// should land on the submit button rather than the DropDown again.
	FocusButtonAfterRender *bool

	// SummaryOnAction, when set, replaces the default NavigateTo call when
	// the summary button is clicked. Use this when the target view requires
	// additional setup (e.g. setting the active project before navigating
	// to project mode). The shell is passed so the callback can call
	// SetProjectMode, SetMode, or NavigateTo as needed.
	SummaryOnAction func(shell ShellAccess)
}

// InlineWizardView is a multi-step wizard that runs inside the TUI shell
// as a regular View, pushed onto the router stack.
type InlineWizardView struct {
	cfg InlineWizardConfig

	// shell is injected via SetShell (shellAware interface).
	shell ShellAccess

	// tview primitives — nil when unmounted.
	app              *tview.Application
	mainFlex         *tview.Flex // root layout (classic: stepBar+header+content+info+hints)
	stepBar          *widgets.StepBar
	stepHeader       *tview.TextView
	stepContent      *tview.Flex // outer container — keeps DrawFunc + InputCapture
	stepContentInner *tview.Flex // inner content area — all mutations (Clear/AddItem) go here
	infoPanel        *tview.TextView // classic: bottom info panel  /  grouped: right sidebar
	hintsBar         *widgets.StatusBar

	// grouped layout primitives (non-nil only when cfg.Groups is set)
	bodyRow *tview.Flex // horizontal: mainPanel + sidebar

	// wizard state
	currentStep int
	stepStates  []widgets.Step
	groupStates []widgets.Step // derived from stepStates when Groups is set
	infoAccum   []inlineStepInfoEntry
	escPending  bool
	completed   bool
	aborted     bool
	wizardErr   error

	// spinner for async OnDone operations
	spinner *widgets.Spinner

	// activeTimers tracks pending time.AfterFunc timers so they can be
	// cancelled on Unmount (avoids referencing a dead wizard).
	activeTimers []*time.Timer

	// forward declaration for recursive step rendering
	doRenderStep func(int)
}

type inlineStepInfoEntry struct {
	stepIdx int
	label   string
	fields  []InfoField
}

// Ensure InlineWizardView implements View.
var _ View = (*InlineWizardView)(nil)

// Ensure InlineWizardView implements InputCapturing.
var _ InputCapturing = (*InlineWizardView)(nil)

// NewInlineWizardView creates an inline wizard view.
// Push it onto the shell router via shell.PushView(v).
func NewInlineWizardView(cfg InlineWizardConfig) *InlineWizardView {
	return &InlineWizardView{
		cfg: cfg,
	}
}

// trackTimer registers a timer so it can be cancelled in Unmount.
func (w *InlineWizardView) trackTimer(t *time.Timer) {
	w.activeTimers = append(w.activeTimers, t)
}

// SetShell provides the shell reference (shellAware interface).
func (w *InlineWizardView) SetShell(s ShellAccess) { w.shell = s }

// CapturesInput implements InputCapturing. Returns true while the wizard
// is mounted and active — the shell delegates all key handling to us,
// bypassing omnibar activation, vim-style navigation, and auto Esc-to-pop.
func (w *InlineWizardView) CapturesInput() bool {
	return w.app != nil && !w.aborted
}

// ─────────────────────────────────────────────────────────────────────────────
// View interface
// ─────────────────────────────────────────────────────────────────────────────

func (w *InlineWizardView) ID() string    { return w.cfg.ID }
func (w *InlineWizardView) Title() string { return w.cfg.Title }

func (w *InlineWizardView) StatusHints() string {
	return w.statusHintsForStep(w.currentStep)
}

// statusHintsForStep returns context-sensitive keybind hints for the given step.
func (w *InlineWizardView) statusHintsForStep(idx int) string {
	if idx < 0 || idx >= len(w.cfg.Steps) {
		return "enter " + i18n.T("wizard.hint.submit")
	}
	step := w.cfg.Steps[idx]
	hasPrev := w.findPrev(idx) != -1

	var parts []string
	parts = append(parts, "ctrl+s "+i18n.T("wizard.hint.submit"))
	if hasPrev {
		parts = append(parts, "ctrl+b "+i18n.T("wizard.hint.back"))
	}
	if !step.Required {
		parts = append(parts, "esc×2 "+i18n.T("wizard.hint.skip"))
	}
	return strings.Join(parts, " · ")
}

// updateDropdownHints swaps the hints bar text when a DropDown gains or loses
// focus. When a DropDown is focused, users see navigation-specific hints
// (↑↓ navigate · enter select · tab next field) instead of the generic
// wizard hints. Also refreshes the label visual hierarchy via fixFormLabelFocus.
func (w *InlineWizardView) updateDropdownHints(form *tview.Form) {
	if w.hintsBar == nil || w.escPending {
		return
	}
	if isDropDownFocused(form) {
		w.hintsBar.SetHints(i18n.T("wizard.hint.dropdown"))
	} else {
		w.hintsBar.SetHints(w.statusHintsForStep(w.currentStep))
	}
	fixFormLabelFocus(form)
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

	// ── Build shared widgets ──

	// Step header ("◆ 2/5 — Label")
	w.stepHeader = tview.NewTextView().SetDynamicColors(true)
	w.stepHeader.SetBackgroundColor(theme.BgPanel)

	// Swappable step content area
	w.stepContent = tview.NewFlex().SetDirection(tview.FlexRow)
	w.stepContent.SetBackgroundColor(theme.BgPanel)
	// Force-clear the entire area on every draw. tview.Flex sets dontClear=true
	// by default, so it never repaints its own background. When stepContent is
	// cleared (e.g. during spinner transitions), stale pixels from the previous
	// step remain on screen — form input fields (#313244 BgElement) and the
	// initial omnibar render produce visible artifacts. This DrawFunc paints
	// BgPanel over the full rect before children draw, eliminating the issue.
	w.stepContent.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		bg := tcell.StyleDefault.Background(theme.BgPanel)
		for row := y; row < y+height; row++ {
			for col := x; col < x+width; col++ {
				screen.SetContent(col, row, ' ', nil, bg)
			}
		}
		return x, y, width, height
	})
	// Default: inner = outer (classic mode). Grouped mode overrides this
	// in mountGroupedLayout by inserting a centering wrapper.
	w.stepContentInner = w.stepContent

	// Info panel (accumulated results — bottom in classic, sidebar in grouped)
	w.infoPanel = tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	w.infoPanel.SetBackgroundColor(theme.BgPanel)

	// Hints bar
	w.hintsBar = widgets.NewStatusBar(w.statusHintsForStep(w.currentStep))

	// Spinner (reused across steps)
	w.spinner = widgets.NewSpinner(i18n.T("wizard.processing"))
	w.spinner.SetBackgroundColor(theme.BgPanel)

	// ── Layout: grouped (centered + sidebar) or classic (full-width) ──
	// Force-clear the shell content container's full rect including its
	// padding (top=1, left=2, right=2). The container is a Flex with
	// dontClear=true, so its padding areas are never repainted by default.
	// The router clears any previous DrawFunc before Mount, so this one
	// is installed fresh and only lives while the wizard is mounted.
	content.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		bg := tcell.StyleDefault.Background(theme.BgPanel)
		for row := y; row < y+height; row++ {
			for col := x; col < x+width; col++ {
				screen.SetContent(col, row, ' ', nil, bg)
			}
		}
		// Return inner rect matching the shell's SetBorderPadding(1, 0, 2, 2).
		innerW := width - 2 - 2
		if innerW < 0 {
			innerW = 0
		}
		innerH := height - 1
		if innerH < 0 {
			innerH = 0
		}
		return x + 2, y + 1, innerW, innerH
	})

	if len(w.cfg.Groups) > 0 {
		w.mountGroupedLayout(content)
	} else {
		w.mountClassicLayout(content)
	}

	// ── Wire forward declaration and render first step ──
	w.doRenderStep = w.renderStep
	w.renderInfoPanel()
	w.renderStep(w.currentStep)

	// Hide the shell omnibar — the wizard has its own hints bar.
	if w.shell != nil {
		w.shell.SetOmnibarVisible(false)
	}
}

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

	content.AddItem(w.mainFlex, 0, 1, true)
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

	// ── Centered content wrapper (flexbox-style) ──
	// All wizard pages get the same width via proportional spacers,
	// like CSS `max-width` + `margin: auto`. Pages fill stepContentInner
	// without knowing their width — the wrapper decides.
	bg := theme.BgPanel
	w.stepContentInner = tview.NewFlex().SetDirection(tview.FlexRow)
	w.stepContentInner.SetBackgroundColor(bg)
	centeredWrapper := tview.NewFlex().SetDirection(tview.FlexColumn)
	centeredWrapper.SetBackgroundColor(bg)
	centeredWrapper.
		AddItem(tview.NewBox().SetBackgroundColor(bg), 0, 1, false).  // left spacer
		AddItem(w.stepContentInner, 0, 5, true).                      // content (5/7 of width)
		AddItem(tview.NewBox().SetBackgroundColor(bg), 0, 1, false)   // right spacer
	w.stepContent.AddItem(centeredWrapper, 0, 1, true)

	// leftCol wraps stepContent + hintsBar so that hints are centered
	// relative to the content area, not the full width including the sidebar.
	leftCol := tview.NewFlex().SetDirection(tview.FlexRow)
	leftCol.SetBackgroundColor(theme.BgPanel)
	leftCol.SetBorderPadding(0, 0, 2, 1)
	// Force-clear the full rect including padding columns. Flex has
	// dontClear=true by default, so the 2-col left + 1-col right padding
	// would never be repainted — stale pixels bleed through as a visible
	// vertical strip.
	leftCol.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		bg := tcell.StyleDefault.Background(theme.BgPanel)
		for row := y; row < y+height; row++ {
			for col := x; col < x+width; col++ {
				screen.SetContent(col, row, ' ', nil, bg)
			}
		}
		// Return inner rect matching SetBorderPadding(0, 0, 2, 1).
		innerW := width - 2 - 1
		if innerW < 0 {
			innerW = 0
		}
		return x + 2, y, innerW, height
	})
	leftCol.AddItem(w.stepContent, 0, 1, true)
	leftCol.AddItem(w.hintsBar.TextView, 1, 0, false)

	// Vertical separator (│) between left column and sidebar
	sep := tview.NewBox()
	sep.SetBackgroundColor(theme.BgPanel)
	sep.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		style := tcell.StyleDefault.Foreground(theme.BorderCard).Background(theme.BgPanel)
		for row := y; row < y+height; row++ {
			screen.SetContent(x, row, '│', nil, style)
		}
		return x + 1, y, width - 1, height
	})

	// bodyRow: leftCol (left, 3/4) + separator (1 col) + sidebar (right, 1/4)
	w.bodyRow = tview.NewFlex().SetDirection(tview.FlexColumn)
	w.bodyRow.SetBackgroundColor(theme.BgPanel)
	w.bodyRow.
		AddItem(leftCol, 0, 3, true).
		AddItem(sep, 1, 0, false).
		AddItem(w.infoPanel, 0, 1, false)

	// Full-screen layout
	w.mainFlex = tview.NewFlex().SetDirection(tview.FlexRow)
	w.mainFlex.SetBackgroundColor(theme.BgPanel)
	w.mainFlex.AddItem(w.bodyRow, 0, 1, true)

	content.AddItem(w.mainFlex, 0, 1, true)
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

func (w *InlineWizardView) Unmount() {
	// NOTE: omnibar visibility is now managed by the shell's onNavigate
	// callback. This avoids a one-frame flash when transitioning between
	// two wizards (Unmount would restore the omnibar, then Mount would
	// hide it again — the intermediate state was visible for one draw).

	if w.spinner != nil {
		w.spinner.Stop()
	}
	// Cancel pending timers to avoid referencing a dead wizard.
	for _, t := range w.activeTimers {
		t.Stop()
	}
	w.activeTimers = nil
	if !w.completed && !w.aborted {
		w.aborted = true
		w.fireComplete()
	}
	w.app = nil
	w.mainFlex = nil
	w.stepBar = nil
	w.stepHeader = nil
	w.stepContent = nil
	w.stepContentInner = nil
	w.infoPanel = nil
	w.hintsBar = nil
	w.spinner = nil
	w.doRenderStep = nil
	w.bodyRow = nil
	w.groupStates = nil
}

func (w *InlineWizardView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	if w.app == nil || w.stepContent == nil {
		return event
	}
	if w.completed {
		return w.handleSummaryKey(event)
	}

	idx := w.currentStep
	if idx < 0 || idx >= len(w.cfg.Steps) {
		return event
	}
	step := w.cfg.Steps[idx]

	// ── Ctrl+B: go back (all step types) ──
	if event.Key() == tcell.KeyCtrlB {
		w.goBack()
		return nil
	}

	// ── CustomView steps: forward all non-Esc events to the widget tree ──
	if step.CustomView != nil {
		if event.Key() == tcell.KeyEscape {
			if step.Required {
				// Required: show transient message, do not skip.
				w.hintsBar.SetHints(fmt.Sprintf("%s%s %s[-]",
					widgets.ColorTag(theme.Error), theme.IconWarning,
					i18n.T("wizard.step_required")))
				w.trackTimer(time.AfterFunc(2*time.Second, func() {
					if w.app != nil {
						w.app.QueueUpdateDraw(func() {
							if w.hintsBar != nil {
								w.hintsBar.SetHints(w.statusHintsForStep(w.currentStep))
							}
						})
					}
				}))
				return nil
			}
			// Optional: double-Esc to skip (consistent with Form steps).
			if !w.escPending {
				w.escPending = true
				w.hintsBar.SetHints(i18n.T("wizard.esc_to_skip"))
				return nil
			}
			w.escPending = false
			w.hintsBar.SetHints(w.statusHintsForStep(w.currentStep))
			w.skipCurrent()
			if !w.completed {
				w.doRenderStep(w.currentStep)
			}
			return nil
		}
		// Any non-Esc key resets double-Esc pending state
		if w.escPending {
			w.escPending = false
			w.hintsBar.SetHints(w.statusHintsForStep(w.currentStep))
		}
		if handler := w.stepContent.InputHandler(); handler != nil {
			handler(event, func(p tview.Primitive) { w.app.SetFocus(p) })
			return nil
		}
		return nil
	}

	// ── Form steps: let tview deliver to the focused form widget ──
	// The form's own InputCapture handles Ctrl+S (submit) and its CancelFunc
	// handles Esc (double-Esc skip for optional, block for required).
	// We only intercept Ctrl+B (above); everything else passes through.
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

func (w *InlineWizardView) countVisibleSteps(idx int) (total, position int) {
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
	w.syncStepBar(w.currentStep, widgets.StepSkipped)

	next := w.findNext(w.currentStep)
	if next == -1 {
		w.wizardComplete()
		return
	}
	w.currentStep = next
	w.syncStepBar(next, widgets.StepActive)
	w.renderInfoPanel()
}

func (w *InlineWizardView) goBack() {
	prev := w.findPrev(w.currentStep)
	if prev == -1 {
		return
	}
	// Reset current to pending
	w.syncStepBar(w.currentStep, widgets.StepPending)

	// Reset previous to active
	w.syncStepBar(prev, widgets.StepActive)

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
		w.infoAccum = append(w.infoAccum, inlineStepInfoEntry{stepIdx: w.currentStep, label: step.Label, fields: fields})
	} else {
		w.infoAccum = append(w.infoAccum, inlineStepInfoEntry{stepIdx: w.currentStep, label: step.Label, fields: nil})
	}

	w.syncStepBar(w.currentStep, widgets.StepDone)

	next := w.findNext(w.currentStep)
	if next == -1 {
		w.wizardComplete()
		return
	}
	w.currentStep = next
	w.syncStepBar(next, widgets.StepActive)
	w.renderInfoPanel()
}

func (w *InlineWizardView) runWithSpinner(step WizardStep, afterDone func()) {
	// Fast path: no async work — transition directly without goroutine.
	if step.OnDone == nil {
		afterDone()
		return
	}

	// Block input on the current step content while OnDone executes.
	// The current content stays visible (no flash) until either OnDone
	// completes (direct transition) or the spinner delay expires.
	prevCapture := w.stepContent.GetInputCapture()
	w.stepContent.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return nil // absorb all keys during processing
	})

	msg := i18n.T("wizard.processing")
	if step.Processing != "" {
		msg = step.Processing
	}
	w.spinner.SetMessage(msg)

	// Capture app reference before launching goroutine to avoid nil
	// dereference if Unmount() runs before QueueUpdateDraw fires.
	tvApp := w.app

	// showSpinner replaces the current step content with the spinner.
	// Called either by the delayed timer or not at all if OnDone is fast.
	spinnerShown := false
	showSpinner := func() {
		if w.app == nil {
			return
		}
		spinnerShown = true
		w.stepContent.SetInputCapture(prevCapture)
		w.stepContentInner.Clear()
		w.stepContentInner.AddItem(w.spinner.TextView, 3, 0, false)
		bgFill := tview.NewBox()
		bgFill.SetBackgroundColor(theme.BgPanel)
		w.stepContentInner.AddItem(bgFill, 0, 1, false)
		w.spinner.Start(w.app)
	}

	// Delayed spinner: only show after 1s if OnDone hasn't finished yet.
	// This keeps the previous step content visible during short operations,
	// eliminating the flash that occurred when the spinner appeared briefly
	// for fast OnDone callbacks.
	spinnerTimer := time.AfterFunc(1*time.Second, func() {
		tvApp.QueueUpdateDraw(func() {
			showSpinner()
		})
	})
	w.trackTimer(spinnerTimer)

	go func() {
		var err error
		if step.OnDone != nil {
			err = step.OnDone()
		}
		spinnerTimer.Stop()
		tvApp.QueueUpdateDraw(func() {
			// Guard: wizard may have been unmounted while OnDone was running.
			if w.app == nil {
				return
			}
			if spinnerShown {
				w.spinner.Stop()
			}
			// Restore input capture.
			w.stepContent.SetInputCapture(prevCapture)

			if err != nil {
				w.wizardErr = err
				// Show error in the step content but do NOT mark as completed.
				// The user can press Ctrl+B to go back and fix the issue,
				// or Enter to retry the current step.
			w.stepContentInner.Clear()
			errView := tview.NewTextView().SetDynamicColors(true)
				errView.SetBackgroundColor(theme.BgPanel)
				errView.SetText(fmt.Sprintf("  %s%s %s[-]\n\n  %s%s[-]\n\n  %sCtrl+B[-] %s  •  %sEnter[-] %s",
					widgets.ColorTag(theme.Error), theme.IconError,
					i18n.T("wizard.error"),
					widgets.ColorTag(theme.FgSecondary), err.Error(),
					widgets.ColorTag(theme.ActiveMode.Primary), i18n.T("wizard.error.back"),
					widgets.ColorTag(theme.ActiveMode.Primary), i18n.T("wizard.error.retry")))
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
				w.stepContentInner.AddItem(errView, 0, 1, true)
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
	// Re-evaluate locale-dependent labels before rendering.
	if w.cfg.RefreshLabels != nil {
		w.cfg.RefreshLabels()
		// Sync stepStates and groupStates with updated labels.
		for i, s := range w.cfg.Steps {
			if i < len(w.stepStates) {
				w.stepStates[i].Label = s.Label
			}
		}
		if len(w.cfg.Groups) > 0 {
			for gi, g := range w.cfg.Groups {
				if gi < len(w.groupStates) {
					w.groupStates[gi].Label = g.Label
				}
			}
		}
	}

	// Inject Rerender before reading the step so the Form callback can use it.
	w.cfg.Steps[idx].Rerender = func() { w.doRenderStep(w.currentStep) }

	step := w.cfg.Steps[idx]

	// Dynamic skip check
	if w.shouldSkip(idx) {
		w.syncStepBar(idx, widgets.StepDone)
		next := w.findNext(idx)
		if next == -1 {
			w.wizardComplete()
			return
		}
		w.currentStep = next
		w.syncStepBar(next, widgets.StepActive)
		w.renderInfoPanel()
		w.doRenderStep(next)
		return
	}

	// Clear step content (inner only — outer keeps its wrapper/DrawFunc)
	w.stepContentInner.Clear()

	// Step counter header — classic mode only (grouped mode uses sidebar)
	if len(w.cfg.Groups) == 0 {
		totalSteps, stepPos := w.countVisibleSteps(idx)
		w.stepHeader.SetText(fmt.Sprintf("  %s%s %d/%d — %s[-]",
			widgets.ColorTag(theme.ActiveMode.Primary), theme.IconActive, stepPos, totalSteps, step.Label))
	}

	// Refresh sidebar in grouped mode
	w.renderInfoPanel()

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
	w.hintsBar.SetHints(w.statusHintsForStep(w.currentStep))

	// ── Warn if both Form and CustomView are set (mutually exclusive) ──
	if step.Form != nil && step.CustomView != nil {
		slog.Warn("wizard step has both Form and CustomView set; CustomView takes precedence",
			"step", step.Label, "idx", idx)
	}

	// ── CustomView path ──
	if step.CustomView != nil {
		onDone := func() {
			// Run validation if defined (same contract as the Form path).
			if step.Validate != nil {
				if errMsg := step.Validate(); errMsg != "" {
					w.hintsBar.SetHints(fmt.Sprintf("%s%s %s[-]",
						widgets.ColorTag(theme.Error), theme.IconWarning, errMsg))
					w.trackTimer(time.AfterFunc(3*time.Second, func() {
						if w.app != nil {
							w.app.QueueUpdateDraw(func() {
								if w.hintsBar != nil {
									w.hintsBar.SetHints(w.statusHintsForStep(w.currentStep))
								}
							})
						}
					}))
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
		step.CustomView(w.app, w.stepContentInner, onDone)
		return
	}

	// ── Form path ──
	if step.Form != nil {
		var form *tview.Form // pre-declare so onDone closure can reference it

		onDone := func() {
			// Run validation if defined
			if step.Validate != nil {
				if errMsg := step.Validate(); errMsg != "" {
					w.hintsBar.SetHints(fmt.Sprintf("%s%s %s[-]",
						widgets.ColorTag(theme.Error), theme.IconWarning, errMsg))
					// Mark the first form field with a ✗ error indicator.
					if form != nil && form.GetFormItemCount() > 0 {
						if input, ok := form.GetFormItem(0).(*tview.InputField); ok {
							origLabel := input.GetLabel()
							input.SetLabel(fmt.Sprintf("%s%s[-] %s",
								widgets.ColorTag(theme.Error), theme.IconError, origLabel))
							w.trackTimer(time.AfterFunc(3*time.Second, func() {
								if w.app != nil {
									w.app.QueueUpdateDraw(func() { input.SetLabel(origLabel) })
								}
							}))
						}
					}
					w.trackTimer(time.AfterFunc(3*time.Second, func() {
						if w.app != nil {
							w.app.QueueUpdateDraw(func() {
								if w.hintsBar != nil {
									w.hintsBar.SetHints(w.statusHintsForStep(w.currentStep))
								}
							})
						}
					}))
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

		form = step.Form(w.app, onDone)
		if form != nil {
			// Apply theme
			form.SetBackgroundColor(theme.BgPanel)
			form.SetFieldBackgroundColor(theme.BgElement)
			form.SetFieldTextColor(theme.FgPrimary)
			form.SetLabelColor(theme.FgPrimary)
			form.SetButtonStyle(tcell.StyleDefault.
				Background(theme.ActiveMode.Primary).
				Foreground(theme.BgPanel))
			form.SetButtonActivatedStyle(tcell.StyleDefault.
				Background(theme.ActiveMode.Secondary).
				Foreground(theme.BgPanel))
			form.SetBorder(false)

			// Fix DropDown popup list colors (tview bakes them at construction
			// time from tview.Styles, which produces invisible text in our theme).
			fixFormDropDownStyles(form)

			// Apply initial label focus hierarchy (bold on focused, muted on rest).
			fixFormLabelFocus(form)

			// Esc handling: Required steps block skip; optional use double-Esc.
			// When a DropDown has focus, the first Escape closes its popup
			// rather than initiating the skip sequence (tview forwards the
			// Escape via finishedFunc, so CancelFunc always fires).
			form.SetCancelFunc(func() {
				// If a DropDown is focused, swallow this Escape — it was
				// triggered by closing the popup, not by the user wanting
				// to skip the step.
				if isDropDownFocused(form) {
					w.updateDropdownHints(form)
					return
				}

				if step.Required {
					w.hintsBar.SetHints(fmt.Sprintf("%s%s %s[-]",
						widgets.ColorTag(theme.Error), theme.IconWarning,
						i18n.T("wizard.step_required")))
					w.trackTimer(time.AfterFunc(2*time.Second, func() {
						if w.app != nil {
							w.app.QueueUpdateDraw(func() {
								if w.hintsBar != nil {
									w.hintsBar.SetHints(w.statusHintsForStep(w.currentStep))
								}
							})
						}
					}))
					return
				}

				if !w.escPending {
					w.escPending = true
					w.hintsBar.SetHints(i18n.T("wizard.esc_to_skip"))
					return
				}

				// Second Esc: confirm skip
				w.escPending = false
				w.hintsBar.SetHints(w.statusHintsForStep(w.currentStep))
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
					w.hintsBar.SetHints(w.statusHintsForStep(w.currentStep))
				}
				// Arrow keys → Tab/Backtab (except on DropDowns)
				if remapped := remapArrowToTab(form, event); remapped != nil {
					defer w.updateDropdownHints(form)
					return remapped
				}
				if event.Key() == tcell.KeyCtrlS {
					onDone()
					return nil
				}
				if event.Key() == tcell.KeyCtrlB {
					w.goBack()
					return nil
				}
				// Update hints after any navigation key (Tab, Backtab, Enter)
				// that may shift focus to or from a DropDown.
				if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab || event.Key() == tcell.KeyEnter {
					defer w.updateDropdownHints(form)
				}
				return event
			})

			if len(w.cfg.Groups) > 0 {
				// Detect terminal height early to decide layout mode.
				_, termH, _ := term.GetSize(int(os.Stdout.Fd()))
				if termH <= 0 {
					termH = 50 // safe fallback
				}
				groupedAvailH := termH - 7 // shell overhead: omnibar(5) + border_padding(1) + hints(1)
				minimalLayout := groupedAvailH < 20

				// Grouped mode: remove buttons from the form and place them
				// in a separate buttonForm at a fixed position (same as intros).
				// In minimal layout, keep buttons inside the form to save space.
				if !minimalLayout {
					form.ClearButtons()
				}

			if minimalLayout {
				// ── Minimal layout — form only with inline buttons ──
				w.stepContentInner.AddItem(form, 0, 1, true)

					form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
						if w.escPending && event.Key() != tcell.KeyEscape {
							w.escPending = false
							w.hintsBar.SetHints(w.statusHintsForStep(w.currentStep))
						}
						if event.Key() == tcell.KeyCtrlS {
							onDone()
							return nil
						}
						if event.Key() == tcell.KeyCtrlB {
							w.goBack()
							return nil
						}
						if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab || event.Key() == tcell.KeyEnter {
							defer w.updateDropdownHints(form)
						}
						return event
					})
				} else {
					// ── Normal grouped layout — BuildWizardPage ──

				// Compute dynamic max width from form content.
				formMaxWidth := 60
				for i := 0; i < form.GetFormItemCount(); i++ {
					label := form.GetFormItem(i).GetLabel()
					w2 := tview.TaggedStringWidth(label) + 40
					if w2 > formMaxWidth {
						formMaxWidth = w2
					}
				}
				if formMaxWidth > 80 {
					formMaxWidth = 80
				}
				// Cap at available width to prevent clipping on small terminals.
				if innerW := StepContentInnerWidth(); innerW > 0 && formMaxWidth > innerW {
					formMaxWidth = innerW
				}

					form.SetBorderPadding(1, 1, 2, 2) // match styleWizardForm for visual consistency

					// Separate button form at fixed position.
					buttonForm := NewStyledButtonForm()
					buttonForm.AddButton("  "+i18n.T("wizard.hint.submit")+"  ", onDone)

					// Assemble the page layout via BuildWizardPage.
					pageResult := BuildWizardPage(nil, w.stepContentInner, WizardPageLayout{
						Badge:           step.Label,
						Content:         form,
						ContentMaxWidth: formMaxWidth,
						Buttons:         buttonForm,
						// FocusTarget: nil — we handle focus below.
					})

					// Install navigation: arrow remap + cross-section.
					SetupFormNavigation(form)
					SetupCrossSectionNav(CrossSectionNavConfig{
						App:     w.app,
						Content: form,
						Buttons: buttonForm,
					})

					// Compose engine-specific captures ON TOP of navigation captures.
					// Engine wraps nav (saves nav capture, installs its own that delegates).
					navFormCapture := form.GetInputCapture()
					form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
						// Reset double-Esc on non-Esc key.
						if w.escPending && event.Key() != tcell.KeyEscape {
							w.escPending = false
							w.hintsBar.SetHints(w.statusHintsForStep(w.currentStep))
						}
						if event.Key() == tcell.KeyCtrlS {
							onDone()
							return nil
						}
						if event.Key() == tcell.KeyCtrlB {
							w.goBack()
							return nil
						}
						// Delegate to navigation capture.
						var result *tcell.EventKey
						if navFormCapture != nil {
							result = navFormCapture(event)
						} else {
							result = event
						}
						// Update hints after navigation keys.
						if result != nil && (event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab || event.Key() == tcell.KeyEnter ||
							event.Key() == tcell.KeyDown || event.Key() == tcell.KeyUp) {
							w.updateDropdownHints(form)
						}
						return result
					})

					navBtnCapture := buttonForm.GetInputCapture()
					buttonForm.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
						if event.Key() == tcell.KeyCtrlS {
							onDone()
							return nil
						}
						if navBtnCapture != nil {
							return navBtnCapture(event)
						}
						return event
					})

					// Store the buttonForm reference for FocusButtonAfterRender.
					_ = pageResult // buttonForm is used directly below.

					// FocusButtonAfterRender: use direct reference instead of scanning.
					w.app.SetFocus(form)
					if w.cfg.FocusButtonAfterRender != nil && *w.cfg.FocusButtonAfterRender {
						*w.cfg.FocusButtonAfterRender = false
						w.app.SetFocus(buttonForm)
					}
				}
			} else {
			w.stepContentInner.AddItem(form, 0, 1, true)
			w.app.SetFocus(form)
			}
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

	accent := widgets.ColorTag(theme.ActiveMode.Primary)
	success := widgets.ColorTag(theme.Success)
	muted := widgets.ColorTag(theme.FgMuted)
	secondary := widgets.ColorTag(theme.FgSecondary)
	primary := widgets.ColorTag(theme.FgPrimary)
	reset := "[-]"

	fmt.Fprintf(&b, " %s%s %s%s\n", accent, theme.IconActive, i18n.T("wizard.sidebar.title"), reset)
	fmt.Fprintf(&b, " %s─────────────────%s\n\n", muted, reset)

	// Build a map of step index → accumulated info (using stored stepIdx)
	infoByStep := make(map[int][]InfoField)
	for _, si := range w.infoAccum {
		infoByStep[si.stepIdx] = si.fields
	}

	for gi, g := range w.cfg.Groups {
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
		color = widgets.ColorTag(theme.ActiveMode.Primary)
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

// summaryTarget returns the target view ID and label for the summary screen,
// preferring the Func callbacks (evaluated at render time) over static fields.
func (w *InlineWizardView) summaryTarget() (viewID, label string) {
	if w.cfg.SummaryTargetViewFunc != nil {
		viewID = w.cfg.SummaryTargetViewFunc()
	} else {
		viewID = w.cfg.SummaryTargetView
	}
	if w.cfg.SummaryTargetLabelFunc != nil {
		label = w.cfg.SummaryTargetLabelFunc()
	} else {
		label = w.cfg.SummaryTargetLabel
	}
	return
}

func (w *InlineWizardView) renderSummaryScreen() {
	if w.stepContentInner == nil {
		return
	}

	// Mark all remaining as done in the step bar
	for i := range w.stepStates {
		if w.stepStates[i].Status == widgets.StepActive {
			w.syncStepBar(i, widgets.StepDone)
		}
	}

	// Update header
	w.stepHeader.SetText(fmt.Sprintf("  %s%s %s[-]",
		widgets.ColorTag(theme.Success), theme.IconSuccess,
		i18n.T("wizard.complete")))

	// Build summary content
	w.stepContentInner.Clear()

	// ── Recap text ──
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
				if strings.Contains(f.Value, "\n") {
					// Multi-line value: label on its own line, content indented below.
					fmt.Fprintf(&b, "      %s%s:[-]\n",
						widgets.ColorTag(theme.FgSecondary), f.Label)
					for _, line := range strings.Split(f.Value, "\n") {
						fmt.Fprintf(&b, "        %s\n", line)
					}
				} else {
					fmt.Fprintf(&b, "      %s%s:[-] %s\n",
						widgets.ColorTag(theme.FgSecondary), f.Label, f.Value)
				}
			}
		}
		b.WriteString("\n")
	}

	// Show skipped steps so the user knows what was not configured.
	for i, ss := range w.stepStates {
		if ss.Status != widgets.StepSkipped {
			continue
		}
		step := w.cfg.Steps[i]
		if step.SidebarHidden {
			continue
		}
		fmt.Fprintf(&b, "  %s%s %s[-]\n",
			widgets.ColorTag(theme.FgMuted), theme.IconSkipped, step.Label)
	}

	summary.SetText(b.String())
	w.stepContentInner.AddItem(summary, 0, 1, false)

	// ── Action button ──
	targetView, targetLabel := w.summaryTarget()
	buttonLabel := targetLabel
	if buttonLabel == "" {
		buttonLabel = i18n.T("wizard.summary.start")
	}
	summaryForm := NewStyledButtonForm()

	summaryForm.AddButton(buttonLabel, func() {
		if w.cfg.SummaryOnAction != nil && w.shell != nil {
			w.cfg.SummaryOnAction(w.shell)
		} else if targetView != "" && w.shell != nil {
			w.shell.NavigateTo(targetView)
		} else if w.shell != nil {
			w.shell.PopView()
		}
	})

	w.stepContentInner.AddItem(summaryForm, 3, 0, true)
	w.app.SetFocus(summaryForm)

	// Update hints
	w.hintsBar.SetHints("enter " + i18n.T("wizard.hint.submit") + " · ctrl+b " + i18n.T("wizard.hint.back"))

	// Update info panel to show final state
	w.renderInfoPanel()
}

func (w *InlineWizardView) handleSummaryKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyEnter, tcell.KeyTab, tcell.KeyBacktab:
		// Let tview deliver to the focused form button.
		return event
	case tcell.KeyCtrlB:
		// Allow going back from summary to review/edit the last step.
		w.completed = false
		w.wizardErr = nil
		// Find the last completed step to return to.
		lastDone := -1
		for i := len(w.stepStates) - 1; i >= 0; i-- {
			if w.stepStates[i].Status == widgets.StepDone && !w.cfg.Steps[i].SidebarHidden {
				lastDone = i
				break
			}
		}
		if lastDone >= 0 {
			w.syncStepBar(lastDone, widgets.StepActive)
			w.currentStep = lastDone
			if len(w.infoAccum) > 0 {
				w.infoAccum = w.infoAccum[:len(w.infoAccum)-1]
			}
			w.renderInfoPanel()
			w.doRenderStep(lastDone)
		}
		return nil
	}
	// Block everything else (Esc, runes, etc.).
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// DropDown style fix
// ─────────────────────────────────────────────────────────────────────────────

// BuildStepBadge creates a centered rounded badge for use above form steps
// in grouped mode. When compact is false, returns a 5-row badge with vertical
// padding; when true, returns a tight 3-row badge.
func BuildStepBadge(label string, compact bool) *tview.TextView {
	accent := widgets.ColorTag(theme.ActiveMode.Primary)
	border := widgets.ColorTag(theme.BorderCard)
	reset := "[-]"

	badgeText := fmt.Sprintf("   ◇  %s   ", label)
	badgeWidth := len([]rune(badgeText))
	top := "╭" + strings.Repeat("─", badgeWidth) + "╮"
	bot := "╰" + strings.Repeat("─", badgeWidth) + "╯"

	var text string
	if compact {
		text = fmt.Sprintf("%s%s%s\n%s│%s%s%s%s│%s\n%s%s%s",
			border, top, reset,
			border, reset, accent, badgeText, border, reset,
			border, bot, reset)
	} else {
		text = fmt.Sprintf("\n%s%s%s\n%s│%s%s%s%s│%s\n%s%s%s\n",
			border, top, reset,
			border, reset, accent, badgeText, border, reset,
			border, bot, reset)
	}

	tv := tview.NewTextView().SetDynamicColors(true).SetTextAlign(tview.AlignCenter)
	tv.SetBackgroundColor(theme.BgPanel)
	tv.SetText(text)
	return tv
}

// NewStyledButtonForm creates a themed form with centered buttons for use
// as the fixed-position button bar in grouped mode.
func NewStyledButtonForm() *tview.Form {
	f := tview.NewForm()
	f.SetBackgroundColor(theme.BgPanel)
	f.SetButtonsAlign(tview.AlignCenter)
	f.SetButtonStyle(tcell.StyleDefault.
		Background(theme.ActiveMode.Primary).
		Foreground(theme.BgPanel))
	f.SetButtonActivatedStyle(tcell.StyleDefault.
		Background(theme.ActiveMode.Secondary).
		Foreground(theme.BgPanel))
	f.SetBorder(false)
	f.SetBorderPadding(1, 1, 1, 1) // must match estimateButtonFormHeight assumptions
	return f
}

// remapArrowToTab converts Up/Down arrow keys to Backtab/Tab for inter-field
// navigation in forms. DropDown fields are excluded because they use Down/Up
// to open/navigate their popup list.
// Returns the remapped event, or nil if no remapping was needed.
func remapArrowToTab(form *tview.Form, event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyDown && event.Key() != tcell.KeyUp {
		return nil
	}
	itemIdx, _ := form.GetFocusedItemIndex()
	if itemIdx >= 0 && itemIdx < form.GetFormItemCount() {
		if _, isDD := form.GetFormItem(itemIdx).(*tview.DropDown); isDD {
			return nil // let DropDown handle Down/Up natively
		}
	}
	if event.Key() == tcell.KeyDown {
		return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	}
	return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
}

// isLastFocusableFormItem returns true if itemIdx is the last focusable
// (interactive) item in the form. Non-scrollable TextViews are considered
// non-focusable because tview's form handler skips them during Tab cycling.
// This is used in grouped mode to decide when Tab should jump from the form
// to the external buttonForm.
func isLastFocusableFormItem(form *tview.Form, itemIdx int) bool {
	if form == nil || itemIdx < 0 {
		return false
	}
	count := form.GetFormItemCount()
	if itemIdx >= count {
		return false
	}
	// Check whether any item after itemIdx is focusable (interactive).
	for i := itemIdx + 1; i < count; i++ {
		item := form.GetFormItem(i)
		// tview.TextView form items (added via AddTextView) are not interactive.
		if _, isTV := item.(*tview.TextView); isTV {
			continue
		}
		// Any other item type (InputField, DropDown, Checkbox, etc.) is focusable.
		return false
	}
	return true
}

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
		Background(theme.ActiveMode.Primary).
		Foreground(theme.BgPanel)

	for i := 0; i < form.GetFormItemCount(); i++ {
		if dd, ok := form.GetFormItem(i).(*tview.DropDown); ok {
			dd.SetListStyles(unselected, selected)
			// Add inner padding to the dropdown field and popup list items.
			// The suffix " ▼ " provides a visual affordance that the field is
			// a selectable dropdown (not static text).
			dd.SetTextOptions("  ", "  ", "  ", "  ▼ ", "")
		}
	}
}

// isDropDownFocused returns true if the currently focused form item is a DropDown.
func isDropDownFocused(form *tview.Form) bool {
	if form == nil {
		return false
	}
	itemIdx, _ := form.GetFocusedItemIndex()
	if itemIdx < 0 || itemIdx >= form.GetFormItemCount() {
		return false
	}
	_, isDD := form.GetFormItem(itemIdx).(*tview.DropDown)
	return isDD
}

// tviewTagRe matches tview color/style tags like [#hex], [::b], [-:-:-], etc.
var tviewTagRe = regexp.MustCompile(`\[[^\[\]]*\]`)

// stripTviewTags removes all tview color/style tags from a string.
func stripTviewTags(s string) string {
	return tviewTagRe.ReplaceAllString(s, "")
}

// fixFormLabelFocus applies visual hierarchy to form labels based on which
// field currently has focus. The focused field's label is rendered bold with
// the primary text color; all other labels use the secondary text color.
// This creates a clear visual trail guiding the user through the form.
func fixFormLabelFocus(form *tview.Form) {
	if form == nil {
		return
	}
	focusedIdx, _ := form.GetFocusedItemIndex()
	primary := widgets.ColorTag(theme.FgPrimary)
	secondary := widgets.ColorTag(theme.FgSecondary)

	for i := 0; i < form.GetFormItemCount(); i++ {
		item := form.GetFormItem(i)
		// Skip TextViews — they are info/hint elements, not interactive fields.
		if _, isTV := item.(*tview.TextView); isTV {
			continue
		}
		raw := stripTviewTags(item.GetLabel())
		if raw == "" {
			continue
		}

		var styled string
		if i == focusedIdx {
			styled = primary + "[::b]" + raw + "[::-]" + theme.TagReset
		} else {
			styled = secondary + raw + theme.TagReset
		}

		// SetLabel is not on the FormItem interface — type-assert to concrete types.
		switch v := item.(type) {
		case *tview.InputField:
			v.SetLabel(styled)
		case *tview.DropDown:
			v.SetLabel(styled)
		case *tview.Checkbox:
			v.SetLabel(styled)
		}
	}
}

// AutoAdvanceFromDropDown moves focus to the next interactive form field
// after a DropDown selection. Call this at the end of your AddDropDown
// callback to get auto-advance behavior without overwriting the callback.
//
// Example:
//
//	form.AddDropDown("Lang", opts, 0, func(text string, idx int) {
//	    selectedLang = text
//	    views.AutoAdvanceFromDropDown(app, form, 0) // 0 = this DD's index
//	})
func AutoAdvanceFromDropDown(app *tview.Application, form *tview.Form, currentIdx int) {
	if app == nil || form == nil {
		return
	}
	// Wrap in a goroutine: QueueUpdateDraw blocks on a done-channel until
	// the event loop processes the update. If called directly from a
	// SetSelectedFunc callback (which runs ON the event loop), this would
	// deadlock. The goroutine moves the blocking <-ch off the event loop.
	go func() {
		app.QueueUpdateDraw(func() {
			next := currentIdx + 1
			count := form.GetFormItemCount()
			for next < count {
				if _, isTV := form.GetFormItem(next).(*tview.TextView); !isTV {
					break
				}
				next++
			}
			if next < count {
				form.SetFocus(next)
				app.SetFocus(form)
			}
		})
	}()
}

// ─────────────────────────────────────────────────────────────────────────────
// Exported wrappers for use by cmd package (hybrid steps that embed forms
// inside CustomViews need access to the same theming helpers).
// ─────────────────────────────────────────────────────────────────────────────

// FixFormDropDownStyles is the exported wrapper for fixFormDropDownStyles.
func FixFormDropDownStyles(form *tview.Form) { fixFormDropDownStyles(form) }

// FixFormLabelFocus is the exported wrapper for fixFormLabelFocus.
func FixFormLabelFocus(form *tview.Form) { fixFormLabelFocus(form) }

// IsLastFocusableFormItem is the exported wrapper for isLastFocusableFormItem.
func IsLastFocusableFormItem(form *tview.Form, itemIdx int) bool {
	return isLastFocusableFormItem(form, itemIdx)
}

// RemapArrowToTab is the exported wrapper for remapArrowToTab.
func RemapArrowToTab(form *tview.Form, event *tcell.EventKey) *tcell.EventKey {
	return remapArrowToTab(form, event)
}
