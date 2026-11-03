// Package shell provides the unified TUI application shell with an
// omnibar-first design. The shell is composed of a full-screen content
// area and a persistent omnibar at the bottom for command input.
//
// # QueueUpdateDraw convention
//
// tview.Application.QueueUpdateDraw (and QueueUpdate) are SYNCHRONOUS — they
// block the calling goroutine via an unbuffered done-channel until the event
// loop picks up the callback, executes it, and signals completion. This has
// two critical consequences:
//
//  1. NEVER call QueueUpdateDraw from inside a running QueueUpdateDraw callback.
//     The event loop cannot drain the queue while executing the current callback,
//     so the inner QueueUpdateDraw blocks forever → hard TUI freeze.
//
//     ❌ DEADLOCK:
//     app.QueueUpdateDraw(func() {
//     app.QueueUpdateDraw(func() { ... })  // blocks forever
//     })
//
//     ✅ CORRECT (if you must show a toast from inside a callback):
//     app.QueueUpdateDraw(func() {
//     shell.ShowToast(...)  // direct call — ShowToast does pages.AddPage, no QueueUpdateDraw
//     })
//
//  2. NEVER call QueueUpdateDraw from inside a tview InputCapture / SetSelectedFunc
//     handler. These handlers run on the event loop — same deadlock as above.
//
//     ✅ CORRECT pattern for "show something after a handler":
//     safego.Go(func() {
//     app.QueueUpdateDraw(func() { ... })  // goroutine blocks OUTSIDE the event loop
//     })
//
//  3. When multiple sequential UI operations are needed after a handler, use a
//     single goroutine with time.Sleep(50ms) + sequential QueueUpdateDraw calls.
//     Launching parallel goroutines that both call QueueUpdateDraw can race and
//     cause two page mutations in the same draw cycle → hard freeze.
//
//     ✅ CORRECT sequential pattern:
//     safego.Go(func() {
//     time.Sleep(50 * time.Millisecond) // let event loop finish the handler cycle
//     app.QueueUpdateDraw(func() { ShowToast("step 1") }) // blocks until drawn
//     app.QueueUpdateDraw(func() { ShowNextModal() })     // blocks until drawn
//     })
package shell

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/router"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Config holds the shell initialization parameters.
type Config struct {
	// ProjectName is displayed in the splash screen.
	ProjectName string
	// Commands defines the flat command registry for the omnibar.
	Commands []Command
	// Views is the list of views to register with the router.
	Views []views.View
	// HomeViewID is the ID of the initial view to display.
	HomeViewID string
	// InitialViewID, when set, is a view opened on top of the home view at
	// start (`oh board`, `oh team board`, `oh dashboard`): Esc goes back to
	// the home view of the mode, as from the menu of that mode.
	InitialViewID string
	// Notifications is an optional pre-created notification store. When nil,
	// a default store with capacity 50 is created automatically.
	Notifications *NotificationStore
	// TeamsProvider returns the list of configured teams as selectable options.
	// Used by Ctrl+T to switch to team mode (ADR-032 Phase 3).
	// If nil, Ctrl+T falls back to navigating to the teams list view.
	TeamsProvider func() []views.SelectOption
	// ProjectsProvider returns the active projects (ID in Value, name in
	// Label), and ProjectResolver completes the one that is chosen (path,
	// branch). Ctrl+T enters the project mode through them; both are
	// called off the event loop. Without ProjectsProvider, Ctrl+T skips the
	// project mode.
	ProjectsProvider func() []views.SelectOption
	ProjectResolver  func(id string) *views.ActiveProject
	// BeforeQuit, when set, is called on Ctrl+Q/Ctrl+C instead of quitting
	// right away (e.g. to ask what to do with running sessions). It must call
	// quit to stop the TUI. A second Ctrl+Q while it is pending forces the quit.
	BeforeQuit func(quit func())
	// Compact returns the `[cli] compact` setting (auto | always | never;
	// nil or "" = auto), read at every draw.
	Compact func() string
}

// shellAware is an optional interface that views can implement to receive
// a reference to the shell for toast/prompt interactions.
type shellAware interface {
	SetShell(s views.ShellAccess)
}

// Shell is the top-level TUI container with omnibar-first design.
// Layout: content (fills screen) + suggestions (dynamic) + omnibar (3 rows at bottom).
type Shell struct {
	quitPending      bool
	badge            string // sessions badge of the mode bar
	badgeAlert       bool
	app              *tview.Application
	pages            *tview.Pages
	root             *tview.Flex // main vertical layout (content + suggestions + omnibar)
	content          *tview.Flex
	windows          [len(windowPages)]*window // the windows shown, by depth (windows.go)
	omnibar          *Omnibar
	router           *router.Router
	registry         *CommandRegistry
	suggestionsShown bool
	cfg              Config // original config for runtime callbacks (ADR-032)

	// activeProject is non-nil when project mode is active.
	activeProject *views.ActiveProject
	// activeTeam is non-nil when team mode is active (ADR-032 Phase 3).
	activeTeam *views.ActiveTeam
	// lastProject is the project the user was in last: Ctrl+T from the hub
	// goes back to it.
	lastProject string
	// activeMode is the current navigation mode (ADR-032 Phase 3).
	activeMode views.Mode

	// notifications holds the last N toast messages for the Notifications view.
	notifications *NotificationStore

	// activeToasts tracks the number of currently visible toast notifications.
	// Used to offset each new toast vertically so they stack instead of overlapping.
	activeToasts int
	// activeToastIDs tracks the page names of visible toasts for manual dismiss.
	activeToastIDs []string

	// running is true while app.Run() is executing (event loop active).
	// Used by TUILogHandler to avoid QueueUpdateDraw deadlock before app starts.
	running atomic.Bool

	// selection manages screen-level text selection (mouse drag → clipboard copy).
	selection *SelectionManager

	// screen is set on every draw cycle via SetAfterDrawFunc and used by the
	// mouse handler for text extraction. Guarded by the tview draw lock.
	screen tcell.Screen

	// ctx is cancelled when the shell exits (SIGINT, q, or tview Stop).
	ctx    context.Context
	cancel context.CancelFunc

	// Layout level: wantLevel is the level of the last drawn terminal
	// size, applied on the event loop (applyLevel) into level; tooSmall
	// blocks the keys behind the « terminal too small » message.
	wantLevel     atomic.Int32
	level         views.LayoutLevel
	tooSmall      atomic.Bool
	omnibarHidden bool
}

// New creates a configured Shell ready to run.
func New(cfg Config) *Shell {
	app := tview.NewApplication()
	tview.Styles = theme.TviewTheme()

	ctx, cancel := context.WithCancel(context.Background())

	ns := cfg.Notifications
	if ns == nil {
		ns = NewNotificationStore(50)
	}

	s := &Shell{
		app:           app,
		ctx:           ctx,
		cancel:        cancel,
		notifications: ns,
		activeMode:    views.ModeHub, // default mode (ADR-032 Phase 3)
		cfg:           cfg,
	}

	// Wire text-selection manager — calls back into the shell for the toast.
	s.selection = NewSelectionManager(func(text string) {
		safego.Go(func() {
			s.app.QueueUpdateDraw(func() {
				s.ShowToast(i18n.T("tui.shell.copied_clipboard"), ToastSuccess)
			})
		})
	})

	// Build content panel — full screen, clean
	s.content = tview.NewFlex().SetDirection(tview.FlexRow)
	s.content.SetBackgroundColor(theme.BgPanel)
	s.content.SetBorderPadding(1, 0, 2, 2)

	// Build command registry
	s.registry = NewCommandRegistry(cfg.Commands)

	// Build omnibar
	s.omnibar = NewOmnibar(s, s.registry)

	s.initRouter(cfg.Views)

	// Build layout: content fills space, omnibar is 6 rows at bottom
	// (omnibar hints/input + 1-row mode bar + gutter).
	// Suggestions list is inserted dynamically between content and omnibar when active.
	s.root = tview.NewFlex().SetDirection(tview.FlexRow)
	s.root.AddItem(s.content, 0, 1, true)
	s.root.AddItem(s.omnibar.Primitive(), 8, 0, false)

	// Wrap in Pages for overlay support (toasts, inline prompts)
	s.pages = tview.NewPages()
	s.pages.AddPage("main", s.root, true, true)

	// Global keybindings
	app.SetInputCapture(s.globalKeyHandler)

	return s
}

// initRouter builds the router and registers the views, wiring ShellAccess
// into those that ask for it. The router callback updates the omnibar hints
// and the mode bar.
func (s *Shell) initRouter(vs []views.View) {
	s.router = router.New(s.content, s.app, func(v views.View) {
		// Defensive: ensure theme matches active mode on every navigation.
		// This catches any code path that navigates without calling SetMode.
		theme.SetActiveMode(string(s.activeMode))
		s.omnibar.SetHints(v.StatusHints())
		s.updateModeBar(v)

		// Manage omnibar visibility based on the new view: views that
		// capture all input (e.g. inline wizards) hide the omnibar;
		// all other views restore it. This centralised check replaces
		// the previous per-view Unmount/Mount toggle and eliminates the
		// one-frame flash when transitioning between two wizards.
		if ic, ok := v.(views.InputCapturing); ok && ic.CapturesInput() {
			s.SetOmnibarVisible(false)
		} else {
			s.SetOmnibarVisible(true)
		}
	})
	s.router.SetBeforeMount(s.syncModeWithStack)

	for _, v := range vs {
		s.router.Register(v)
		if sv, ok := v.(shellAware); ok {
			sv.SetShell(s)
		}
	}
}

// Run starts the tview event loop. Blocks until quit.
func (s *Shell) Run() error {
	// Rotate the persistent notifications file at startup (best-effort, background).
	// Keeps at most 500 entries and removes entries older than 7 days.
	safego.Go(func() {
		_ = RotateFile(NotificationsFilePath(), 500, 7*24*time.Hour)
	})
	// Apply selection highlight after every draw cycle, before screen.Show().
	// Also cache the screen reference so mouse handlers can read cell content.
	s.app.SetBeforeDrawFunc(s.beforeDraw)
	s.app.SetAfterDrawFunc(func(screen tcell.Screen) {
		s.screen = screen
		s.selection.ApplyHighlight(screen)
	})
	s.app.SetMouseCapture(s.mouseCapture)

	s.app.SetRoot(s.pages, true).EnableMouse(true)
	s.running.Store(true)
	err := s.app.Run()
	s.running.Store(false)
	s.cancel() // signal all goroutines to stop
	return err
}

// IsRunning returns true while the tview event loop is active.
// Used by TUILogHandler to avoid QueueUpdateDraw deadlock before app.Run() starts.
func (s *Shell) IsRunning() bool {
	return s.running.Load()
}

// Context returns the shell's lifecycle context.
// It is cancelled when the shell exits. Use this in goroutines started by action callbacks.
func (s *Shell) Context() context.Context {
	return s.ctx
}

// App returns the underlying tview.Application for external QueueUpdateDraw calls.
func (s *Shell) App() *tview.Application {
	return s.app
}

// Notifications returns the notification store for this session.
// Used by the NotificationsView to display the history of toasts.
func (s *Shell) Notifications() *NotificationStore {
	return s.notifications
}

// CancelQuit clears a pending quit request (BeforeQuit dialog cancelled).
func (s *Shell) CancelQuit() { s.quitPending = false }

// RequestQuit quits through BeforeQuit (Ctrl+Q, Ctrl+C, omnibar "quit"). A
// second request while the BeforeQuit dialog is pending quits at once.
// Event loop only.
func (s *Shell) RequestQuit() {
	if s.cfg.BeforeQuit == nil || s.quitPending {
		s.app.Stop()
		return
	}
	s.quitPending = true
	s.cfg.BeforeQuit(func() { s.app.Stop() })
}

// Commands returns the omnibar command registry (generated commands).
func (s *Shell) Commands() *CommandRegistry { return s.registry }
