package views

import (
	"github.com/rivo/tview"
)

// ─────────────────────────────────────────────────────────────────────────────
// Wizard steps — what InlineWizardView runs, in the shell of the TUI or alone
// (RunInlineWizardStandalone) for the wizards of the CLI
// ─────────────────────────────────────────────────────────────────────────────

// InfoField represents a key/value pair displayed in the info panel.
type InfoField struct {
	Label string
	Value string
}

// WizardStep defines a single step in the wizard.
type WizardStep struct {
	// ID is an optional unique identifier for this step, used by RefreshLabels
	// to update labels by name instead of fragile positional indices.
	// Zero-value (empty string) is valid and means the step has no ID.
	ID string

	// Label is displayed in the info panel progression.
	Label string

	// Form builds the tview form for this step.
	// It receives the app and an onDone callback. The step MUST call onDone()
	// when the user confirms (typically from a button callback).
	// Esc asks whether to quit the wizard, or to skip the step (unless
	// Required is true).
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

	// SidebarHidden, when true, excludes this step from the sidebar in
	// grouped mode. Use for transitional pages (intros, welcome) that
	// don't configure anything.
	SidebarHidden bool

	// Validate is called before OnDone when the user submits the form.
	// If it returns a non-empty string, the submission is blocked and
	// the error message is displayed in the hints bar for 3 seconds.
	// If nil, no validation is performed (always passes).
	Validate func() string

	// OnSkip runs on the event loop when the user skips this step (Esc →
	// « Passer l'étape », or an ErrorAction with Skip), after the error of
	// the step is forgotten and before the next step is chosen: it sets the
	// state the next SkipIf read, so that a step can skip its whole group.
	OnSkip func()

	// ErrorActions, when set, adds buttons to the error screen of this step
	// (the failure of its OnDone): err is the error shown. The list is the
	// whole row, in order; a « retry » action (Retry) is put first when the
	// list has none. The focus starts on the retry button, so that Enter
	// keeps meaning « retry ». Without it the screen only offers Ctrl+B
	// (back) and Enter (retry).
	ErrorActions func(err error) []ErrorAction

	// Rerender is injected automatically by the wizard engine before calling
	// Form. Dynamic forms (e.g. a DropDown that changes the set of fields)
	// can call it to safely rebuild the current step from scratch.
	//
	// IMPORTANT: never call Rerender synchronously from a tview handler
	// callback (InputCapture, SetSelectedFunc, etc.) — this would mutate
	// the widget tree mid-handler. Always defer via a goroutine:
	//
	//   go func() { app.QueueUpdateDraw(func() { step.Rerender() }) }()
	//
	// This field is set automatically — do not set it in step definitions.
	Rerender func()
}

// ErrorAction is a button of the error screen of a step (WizardStep.ErrorActions).
// Exactly one of Retry, GoToStep, Skip and Quit says what it does; Before
// runs first, on the event loop (it sets the state the target step reads).
type ErrorAction struct {
	// Label is the text of the button.
	Label string
	// Key is an optional letter that triggers the action (any case).
	Key rune
	// Before runs before the action itself.
	Before func()
	// Retry draws the step again (what Enter does on the old error screen).
	Retry bool
	// GoToStep goes back to the step of that ID, which is drawn again; the
	// steps after it, this one included, are to be done again.
	GoToStep string
	// Skip skips the step like « Passer l'étape » (OnSkip included), even when
	// the step is Required.
	Skip bool
	// Quit leaves the wizard like « Quitter ».
	Quit bool
}

// StepByID returns a pointer to the first step with the given ID, or nil if not found.
// Use this in RefreshLabels to avoid hardcoded indices.
func StepByID(steps []WizardStep, id string) *WizardStep {
	for i := range steps {
		if steps[i].ID == id {
			return &steps[i]
		}
	}
	return nil
}

// StepGroup defines a named group of wizard steps for the step bar.
// When groups are configured on InlineWizardConfig, the step bar displays
// group labels instead of individual step labels, providing a cleaner
// high-level progression view.
type StepGroup struct {
	// Label is the display name shown in the step bar (e.g. "Provider").
	Label string
	// StartIdx is the 0-based index of the first step in this group.
	StartIdx int
	// Hidden, when non-nil and true, leaves the group out of the sidebar
	// (e.g. the team group of a solo setup). It is evaluated on every render.
	Hidden func() bool
}
