package views

import (
	"context"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// Standalone runner for InlineWizardView
// ─────────────────────────────────────────────────────────────────────────────
//
// RunInlineWizardStandalone mounts an InlineWizardView inside a dedicated
// tview.Application, runs the event loop, and returns the result. This allows
// CLI commands (e.g. `oh init`) to reuse the same wizard builder as the TUI
// without requiring the full shell infrastructure.
//
// The 10+ existing RunWizard call sites are NOT affected — they continue to
// use the classic wizard engine (wizard.go). Only callers that explicitly opt
// in to the inline engine go through this path.

// StandaloneResult holds the outcome of a standalone inline wizard run.
type StandaloneResult struct {
	Completed bool  // all steps finished successfully
	Aborted   bool  // user pressed Ctrl+C or quit
	Err       error // a step's OnDone returned an error (with retry declined)
}

// RunInlineWizardStandalone creates a tview.Application, mounts the given
// InlineWizardView, runs the blocking event loop, and returns the result.
//
// The caller builds the wizard via NewInlineWizardView exactly as for the TUI,
// then passes it here instead of shell.PushView. The wizard's OnComplete
// callback is intercepted to capture the result and stop the application.
func RunInlineWizardStandalone(wizard *InlineWizardView) StandaloneResult {
	// Fast path: no steps → completed immediately.
	if len(wizard.cfg.Steps) == 0 {
		if wizard.cfg.OnComplete != nil {
			wizard.cfg.OnComplete(true, nil)
		}
		return StandaloneResult{Completed: true}
	}

	tview.Styles = theme.TviewTheme()
	app := tview.NewApplication()

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
		completedBeforeRun = true
		// Forward to the original callback (e.g. config.Update, ReloadApp).
		if originalOnComplete != nil {
			originalOnComplete(completed, err)
		}
		// Stop the event loop so Run() returns. Use QueueUpdate to avoid
		// calling Stop from inside a draw cycle.
		go func() {
			app.QueueUpdate(func() {
				app.Stop()
			})
		}()
	}

	// Provide a minimal ShellAccess stub so the summary screen's
	// NavigateTo/PopView calls work (they call app.Stop via the stub).
	wizard.SetShell(&standaloneShellStub{app: app})

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
//   - PopView           (summary screen)      → stop the app
//
// All other methods panic — they are never called by the wizard.

type standaloneShellStub struct {
	app *tview.Application
}

func (s *standaloneShellStub) Context() context.Context { return context.Background() }
func (s *standaloneShellStub) SetOmnibarVisible(bool)   {}
func (s *standaloneShellStub) NavigateTo(string)         { s.app.Stop() }
func (s *standaloneShellStub) PopView() bool             { s.app.Stop(); return true }
func (s *standaloneShellStub) PushView(View)             {}

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
func (s *standaloneShellStub) ShowToastMsg(string, bool) {
	panic("standaloneShellStub: ShowToastMsg not supported")
}
func (s *standaloneShellStub) ShowInlineForm(InlineFormConfig) {
	panic("standaloneShellStub: ShowInlineForm not supported")
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
