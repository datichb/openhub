package shell

import (
	"errors"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Navigation: the stack of views of the router and the terminal hand-over.

// NavigateHome navigates to the registered home view by ID and sets the
// active navigation mode accordingly. This ensures theme.ActiveMode is
// correct when the TUI starts directly in project or team mode. The initial
// view of the configuration, if any, is opened on top of the home view.
func (s *Shell) NavigateHome(homeID string, mode views.Mode) {
	s.activeMode = mode
	theme.SetActiveMode(string(mode))
	s.router.NavigateTo(homeID)
	if id := s.cfg.InitialViewID; id != "" && id != homeID {
		s.router.NavigateTo(id)
	}
}

// SuspendAndExec suspends the TUI, runs a function, then resumes.
// After resume it forces a full terminal re-sync (Sync) and restores focus to
// the main content area to avoid a permanent freeze caused by tcell's
// screen.Resume() silently failing on macOS after a subprocess (e.g. the tool client)
// that manipulates the tty.
func (s *Shell) SuspendAndExec(fn func() error) error {
	var execErr error
	ok := s.app.Suspend(func() {
		execErr = fn()
	})
	if !ok {
		return errors.New(i18n.T("tui.shell.suspend_failed"))
	}
	// Force full redraw from scratch. Marks all cells dirty and flushes them
	// to the terminal. Handles the case where Resume() silently failed.
	s.app.Sync()
	// Restore focus so subsequent key events reach the active view.
	// Without this the focus may sit on an orphaned widget (pre-suspend view)
	// or get stolen by the next ShowToast call.
	s.app.SetFocus(s.content)
	return execErr
}

// RestoreFocus explicitly restores keyboard focus to the main content area.
func (s *Shell) RestoreFocus() {
	s.app.SetFocus(s.content)
}

// NavigateTo navigates to a registered view by ID.
func (s *Shell) NavigateTo(viewID string) {
	s.router.NavigateTo(viewID)
}

// PushView pushes an ephemeral view onto the router stack without prior
// registration. Use this for on-demand views like inline wizards that are
// created dynamically rather than at startup. Esc/Pop returns to the
// previous view. If the view implements shellAware, SetShell is called.
func (s *Shell) PushView(v views.View) {
	if sv, ok := v.(shellAware); ok {
		sv.SetShell(s)
	}
	s.router.Push(v)
}

// PopView pops the current ephemeral view from the router stack,
// returning to the previous view. Returns false if already at root.
func (s *Shell) PopView() bool {
	return s.router.Pop()
}

// RemountIf mounts the current view again when its ID is one of ids (e.g.
// a landing whose cached sessions summary changed). Event loop only.
func (s *Shell) RemountIf(ids ...string) {
	cur := s.router.Current()
	if cur == nil || s.pages.HasPage("inline-overlay") || s.omnibar.IsActive() {
		return
	}
	for _, id := range ids {
		if cur.ID() == id {
			s.router.Remount()
			return
		}
	}
}
