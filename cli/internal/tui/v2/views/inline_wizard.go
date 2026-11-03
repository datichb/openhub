package views

import (
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// InlineWizard — multi-step wizard view that runs inside the TUI shell
// ─────────────────────────────────────────────────────────────────────────────
//
// InlineWizardView implements the View interface and runs within the existing
// shell. This avoids the jarring alt-screen switch and keeps the omnibar,
// toasts, and shell chrome accessible throughout the wizard. The wizards of
// the CLI run it alone, in an application of their own
// (RunInlineWizardStandalone): same engine, same rendering.
//
// The steps are WizardStep (wizard_step.go).

// InlineWizardConfig configures an inline wizard view.
type InlineWizardConfig struct {
	// ID is the unique view identifier for the router (e.g. "wizard.team.init").
	ID string
	// Title is displayed in the shell breadcrumb.
	Title string
	// Steps defines the wizard steps.
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

	// SummaryNote, when set, returns text (tview tags allowed) shown under the
	// recap on the summary screen, e.g. a suggested next command. Empty = none.
	SummaryNote func() string
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
	stepContent      *tview.Flex     // outer container — keeps DrawFunc + InputCapture
	stepContentInner *tview.Flex     // inner content area — all mutations (Clear/AddItem) go here
	infoPanel        *tview.TextView // classic: bottom info panel  /  grouped: right sidebar
	hintsBar         *widgets.StatusBar

	// grouped layout primitives (non-nil only when cfg.Groups is set)
	bodyRow *tview.Flex // horizontal: mainPanel + sidebar

	// wizard state
	currentStep int
	stepStates  []widgets.Step
	groupStates []widgets.Step // derived from stepStates when Groups is set
	infoAccum   []inlineStepInfoEntry
	completed   bool
	aborted     bool
	wizardErr   error

	// spinner for async OnDone operations
	spinner    *widgets.Spinner
	processing bool // a step's OnDone is running

	// form is the form of the current step (nil for other steps).
	form *tview.Form
	// hintsHost is the flex holding the hints bar; the « quit the wizard? »
	// prompt (quitBar) takes its row while shown.
	hintsHost *tview.Flex
	quitBar   *tview.Flex
	quitBack  tview.Primitive

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

// notifyHold is how long a notification stays in the hints bar.
var notifyHold = 3 * time.Second

// Notify shows msg in the hints bar for a few seconds, then the hints of the
// step come back. It does not take the focus and does not block the wizard,
// and it may be called from any goroutine.
func (w *InlineWizardView) Notify(msg string, isErr bool) {
	app := w.app
	if app == nil {
		return
	}
	color := theme.Success
	icon := theme.IconSuccess
	if isErr {
		color, icon = theme.Error, theme.IconError
	}
	text := theme.Paint(color, icon+" "+msg)
	safego.Go(func() {
		app.QueueUpdateDraw(func() {
			if w.app == nil || w.hintsBar == nil {
				return
			}
			if w.quitBar == nil {
				w.hintsBar.SetHints(text)
			}
			w.trackTimer(safego.AfterFunc(notifyHold, func() {
				app.QueueUpdateDraw(func() {
					if w.app != nil && w.hintsBar != nil && w.quitBar == nil {
						w.hintsBar.SetHints(w.statusHintsForStep(w.currentStep))
					}
				})
			}))
		})
	})
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
	hasPrev := w.findPrev(idx) != -1

	var parts []string
	parts = append(parts, "ctrl+s "+i18n.T("wizard.hint.submit"))
	if hasPrev {
		parts = append(parts, "ctrl+b/alt+← "+i18n.T("wizard.hint.back"))
	}
	parts = append(parts, "esc "+i18n.T("tui.wizard.quit.hint"))
	return strings.Join(parts, " · ")
}

// updateDropdownHints swaps the hints bar text when a DropDown gains or loses
// focus. When a DropDown is focused, users see navigation-specific hints
// (↑↓ navigate · enter select · tab next field) instead of the generic
// wizard hints. Also refreshes the label visual hierarchy via fixFormLabelFocus.
func (w *InlineWizardView) updateDropdownHints(form *tview.Form) {
	if w.hintsBar == nil || w.quitBar != nil {
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
	w.quitBar, w.quitBack, w.form, w.processing = nil, nil, nil, false

	if !w.initStepStates() {
		w.completed = true
		w.fireComplete()
		return
	}
	w.newSharedWidgets()

	// ── Layout: grouped (centered + sidebar) or classic (full-width) ──
	w.paintContentBackground(content)
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

// initStepStates sets the state of every step and activates the first one that
// is not skipped. It reports false when there is no step to run.
func (w *InlineWizardView) initStepStates() bool {
	if len(w.cfg.Steps) == 0 {
		return false
	}
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
	return w.currentStep != -1
}

// newSharedWidgets builds the widgets both layouts use.
func (w *InlineWizardView) newSharedWidgets() {
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
		fillBackground(screen, x, y, width, height)
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
}

// paintContentBackground force-clears the shell content container's full rect
// including its padding (top=1, left=2, right=2). The container is a Flex with
// dontClear=true, so its padding areas are never repainted by default.
// The router clears any previous DrawFunc before Mount, so this one
// is installed fresh and only lives while the wizard is mounted.
func (w *InlineWizardView) paintContentBackground(content *tview.Flex) {
	content.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		fillBackground(screen, x, y, width, height)
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
	w.form, w.hintsHost, w.quitBar, w.quitBack = nil, nil, nil, nil
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

	// ── The « quit the wizard? » prompt has the keys (Esc = continue) ──
	if w.quitBar != nil {
		switch event.Key() {
		case tcell.KeyEscape:
			w.closeQuitPrompt()
			return nil
		case tcell.KeyLeft, tcell.KeyBacktab:
			w.moveQuitFocus(-1)
			return nil
		case tcell.KeyRight, tcell.KeyTab:
			w.moveQuitFocus(1)
			return nil
		}
		return event
	}

	// ── Ctrl+B, Alt+←, PgUp: go back (all step types) ──
	if isBackKey(event) {
		w.goBack()
		return nil
	}

	// ── Esc: ask whether to quit (an open drop-down list closes first) ──
	if event.Key() == tcell.KeyEscape {
		if step.CustomView == nil && formDropDownOpen(w.form) {
			return event
		}
		if !w.processing {
			w.askQuit()
		}
		return nil
	}

	// ── CustomView steps: forward all other events to the widget tree ──
	if step.CustomView != nil {
		if handler := w.stepContent.InputHandler(); handler != nil {
			handler(event, func(p tview.Primitive) { w.app.SetFocus(p) })
			return nil
		}
		return nil
	}

	// ── Form steps: let tview deliver to the focused form widget ──
	// The form's own InputCapture handles Ctrl+S (submit).
	return event
}
