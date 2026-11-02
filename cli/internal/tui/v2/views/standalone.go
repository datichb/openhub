package views

import (
	"context"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// Standalone runner for InlineWizardView
// ─────────────────────────────────────────────────────────────────────────────
//
// RunInlineWizardStandalone mounts an InlineWizardView inside a dedicated
// tview.Application, runs the event loop, and returns the result. This is how
// the wizards of the CLI (oh init, oh provider setup, oh project add…) run:
// the same engine and the same rendering as in the TUI, without the shell.
// Esc asks whether to quit, as in the shell.

// StandaloneResult holds the outcome of a standalone inline wizard run.
type StandaloneResult struct {
	Completed bool  // all steps finished successfully
	Aborted   bool  // the user quit (Esc then « Quit », Ctrl+C) with no step failing
	Err       error // a step failed and the user quit instead of retrying, or the screen could not start
}

// RunInlineWizardStandalone creates a tview.Application, mounts the given
// InlineWizardView, runs the blocking event loop, and returns the result.
//
// The caller builds the wizard via NewInlineWizardView exactly as for the TUI,
// then passes it here instead of shell.PushView. The wizard's OnComplete
// callback is intercepted to capture the result and stop the application.
func RunInlineWizardStandalone(wizard *InlineWizardView) StandaloneResult {
	return RunInlineWizardStandaloneOn(wizard, nil)
}

// RunInlineWizardStandaloneOn is RunInlineWizardStandalone on a given screen
// (nil: the terminal), for the tests that send keys to a simulation screen.
func RunInlineWizardStandaloneOn(wizard *InlineWizardView, screen tcell.Screen) StandaloneResult {
	// Fast path: no steps → completed immediately.
	if len(wizard.cfg.Steps) == 0 {
		if wizard.cfg.OnComplete != nil {
			wizard.cfg.OnComplete(true, nil)
		}
		return StandaloneResult{Completed: true}
	}

	tview.Styles = theme.TviewTheme()
	app := tview.NewApplication()
	if screen != nil {
		app.SetScreen(screen)
	}

	var result StandaloneResult
	// completedBeforeRun tracks whether OnComplete fired during Mount
	// (e.g. all steps skipped). In that case, we skip app.Run() entirely
	// since there is nothing to display.
	completedBeforeRun := false

	// Wrap the caller's OnComplete to capture the result and stop the app.
	originalOnComplete := wizard.cfg.OnComplete
	wizard.cfg.OnComplete = func(completed bool, err error) {
		result.Completed = completed
		result.Err = err
		// Quitting with no step failing is not an error: the caller leaves
		// quietly. A step that failed keeps its error.
		result.Aborted = result.Aborted || (!completed && err == nil)
		completedBeforeRun = true
		// Forward to the original callback (e.g. config.Update, ReloadApp).
		if originalOnComplete != nil {
			originalOnComplete(completed, err)
		}
		// Stop the event loop so Run() returns. Use QueueUpdate to avoid
		// calling Stop from inside a draw cycle.
		safego.Go(func() {
			app.QueueUpdate(func() {
				app.Stop()
			})
		})
	}

	// Provide a minimal ShellAccess stub so the summary screen's
	// NavigateTo/PopView calls work (they call app.Stop via the stub).
	wizard.SetShell(&standaloneShellStub{app: app, wizard: wizard})

	// Build the content flex — mirrors what the shell router provides.
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	content.SetBackgroundColor(theme.BgPanel)

	wizard.Mount(content, app)

	// If the wizard completed during Mount (all steps skipped, or all
	// processing-only steps finished synchronously), skip the event loop.
	if completedBeforeRun {
		wizard.Unmount()
		return result
	}

	// Wire key handling: Ctrl+C to quit, everything else to the wizard.
	content.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC || event.Key() == tcell.KeyCtrlQ {
			result.Aborted = true
			app.Stop()
			return nil
		}
		return wizard.HandleKey(event)
	})

	app.SetRoot(content, true).EnableMouse(true)

	if err := app.Run(); err != nil {
		wizard.Unmount()
		return StandaloneResult{Err: err}
	}

	wizard.Unmount()
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// standaloneShellStub — minimal ShellAccess for InlineWizardView
// ─────────────────────────────────────────────────────────────────────────────
//
// InlineWizardView uses exactly 3 ShellAccess methods:
//   - SetOmnibarVisible (Mount/Unmount)      → no-op
//   - NavigateTo        (summary screen)      → stop the app
//   - PopView           (summary screen, quit) → stop the app
//
// ShowToastMsg is a notification that does not block: it shows in the hints
// bar of the wizard for a few seconds. All other methods panic — they are
// never called by the wizard.

type standaloneShellStub struct {
	app    *tview.Application
	wizard *InlineWizardView
}

var _ ShellAccess = (*standaloneShellStub)(nil)

func (s *standaloneShellStub) Context() context.Context { return context.Background() }
func (s *standaloneShellStub) SetOmnibarVisible(bool)   {}
func (s *standaloneShellStub) NavigateTo(string)        { s.app.Stop() }
func (s *standaloneShellStub) PopView() bool            { s.app.Stop(); return true }
func (s *standaloneShellStub) PushView(View)            {}

// ── Methods never called by InlineWizardView — defensive panics ─────────────

func (s *standaloneShellStub) ShowInputModal(string, string, func(string)) {
	panic("standaloneShellStub: ShowInputModal not supported")
}
func (s *standaloneShellStub) ShowPasswordModal(string, func(string)) {
	panic("standaloneShellStub: ShowPasswordModal not supported")
}
func (s *standaloneShellStub) ShowSelectModal(string, []SelectOption, string, func(string)) {
	panic("standaloneShellStub: ShowSelectModal not supported")
}
func (s *standaloneShellStub) ShowMultiSelectModal(string, []SelectOption, []string, func([]string)) {
	panic("standaloneShellStub: ShowMultiSelectModal not supported")
}
func (s *standaloneShellStub) ShowScrollableModal(string, string, []ModalAction) {
	panic("standaloneShellStub: ShowScrollableModal not supported")
}
func (s *standaloneShellStub) ShowToastMsg(msg string, isErr bool) {
	if s.wizard != nil {
		s.wizard.Notify(msg, isErr)
	}
}
func (s *standaloneShellStub) ShowInlineForm(InlineFormConfig) {
	panic("standaloneShellStub: ShowInlineForm not supported")
}
func (s *standaloneShellStub) Confirm(string, string, bool, func()) {
	panic("standaloneShellStub: Confirm not supported")
}
func (s *standaloneShellStub) SetProjectMode(*ActiveProject) {
	panic("standaloneShellStub: SetProjectMode not supported")
}
func (s *standaloneShellStub) SetActiveProject(*ActiveProject) {
	panic("standaloneShellStub: SetActiveProject not supported")
}
func (s *standaloneShellStub) ActiveProject() *ActiveProject {
	panic("standaloneShellStub: ActiveProject not supported")
}
func (s *standaloneShellStub) SetMode(Mode) {
	panic("standaloneShellStub: SetMode not supported")
}
func (s *standaloneShellStub) Mode() Mode {
	panic("standaloneShellStub: Mode not supported")
}
func (s *standaloneShellStub) SetActiveTeam(*ActiveTeam) {
	panic("standaloneShellStub: SetActiveTeam not supported")
}
func (s *standaloneShellStub) ActiveTeam() *ActiveTeam {
	panic("standaloneShellStub: ActiveTeam not supported")
}
