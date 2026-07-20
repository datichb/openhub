// Package shell provides the unified TUI application shell with
// persistent header, menu sidebar, content panel, and status bar.
// It orchestrates the router, menu, and overlay system (toasts/modals).
package shell

import (
	"fmt"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/menu"
	"github.com/datichb/openhub/cli/internal/tui/v2/router"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Config holds the shell initialization parameters.
type Config struct {
	// ProjectName is displayed in the header.
	ProjectName string
	// MenuItems defines the sidebar menu structure.
	MenuItems []*menu.MenuItem
	// Views is the list of views to register with the router.
	Views []views.View
	// HomeViewID is the ID of the initial view to display.
	HomeViewID string
}

// shellAware is an optional interface that views can implement to receive
// a reference to the shell for modal/toast interactions.
type shellAware interface {
	SetShell(s views.ShellAccess)
}

// Shell is the top-level TUI container that owns the tview.Application
// and manages the lifecycle of views via a Router.
type Shell struct {
	app           *tview.Application
	pages         *tview.Pages
	header        *Header
	menu          *menu.Menu
	menuItems     []*menu.MenuItem
	content       *tview.Flex
	statusBar     *StatusBar
	router        *router.Router
	overlayActive bool
	mu            sync.Mutex
}

// New creates a configured Shell ready to run.
func New(cfg Config) *Shell {
	app := tview.NewApplication()
	tview.Styles = theme.TviewTheme()

	s := &Shell{
		app:       app,
		menuItems: cfg.MenuItems,
	}

	// Build content panel
	s.content = tview.NewFlex().SetDirection(tview.FlexRow)
	s.content.SetBackgroundColor(theme.BgPanel)
	s.content.SetBorder(true)
	s.content.SetBorderColor(theme.BorderFocus)
	s.content.SetBorderPadding(1, 0, 2, 2)

	// Build header
	s.header = NewHeader(cfg.ProjectName)

	// Build status bar
	s.statusBar = NewStatusBar()

	// Build menu
	s.menu = menu.New(cfg.MenuItems, func(item *menu.MenuItem) {
		if item.Action != nil {
			item.Action()
			return
		}
		if item.ViewID != "" {
			s.router.NavigateTo(item.ViewID)
		}
	})

	// Build router
	s.router = router.New(s.content, app, func(v views.View) {
		s.header.SetBreadcrumb(v.Title())
		s.statusBar.SetHints(v.StatusHints())
		s.menu.SetActive(v.ID())
	})

	// Register all views and wire ShellAccess for those that support it
	for _, v := range cfg.Views {
		s.router.Register(v)
		// Wire shell access for views that need modals/toasts
		if sv, ok := v.(shellAware); ok {
			sv.SetShell(s)
		}
	}

	// Build layout
	middle := tview.NewFlex()
	middle.AddItem(s.menu.Primitive(), 0, 1, false)
	middle.AddItem(s.content, 0, 5, true)

	root := tview.NewFlex().SetDirection(tview.FlexRow)
	root.AddItem(s.header.Primitive(), 2, 0, false)
	root.AddItem(middle, 0, 1, true)
	root.AddItem(s.statusBar.Primitive(), 1, 0, false)

	// Wrap in Pages for overlay support
	s.pages = tview.NewPages()
	s.pages.AddPage("main", root, true, true)

	// Global keybindings
	app.SetInputCapture(s.globalKeyHandler)

	// Responsive: adjust layout on each draw based on terminal size
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		s.handleResize(screen)
		return false // continue drawing
	})

	return s
}

// Run starts the tview event loop. Blocks until quit.
func (s *Shell) Run() error {
	// Navigate to home view
	s.app.SetRoot(s.pages, true).EnableMouse(true)
	return s.app.Run()
}

// App returns the underlying tview.Application for external QueueUpdateDraw calls.
func (s *Shell) App() *tview.Application {
	return s.app
}

// NavigateHome navigates to the registered home view by ID.
func (s *Shell) NavigateHome(homeID string) {
	s.router.NavigateTo(homeID)
}

// ShowToast displays a temporary notification message.
func (s *Shell) ShowToast(msg string, level ToastLevel) {
	s.showToast(msg, level, 2500*time.Millisecond)
}

// ShowToastMsg displays a toast with auto-level (success=true → green, false → red).
func (s *Shell) ShowToastMsg(msg string, success bool) {
	if success {
		s.showToast(msg, ToastSuccess, 2500*time.Millisecond)
	} else {
		s.showToast(msg, ToastError, 3500*time.Millisecond)
	}
}

// ShowInputModal displays a centered input modal for editing a value.
func (s *Shell) ShowInputModal(title, currentValue string, onConfirm func(newValue string)) {
	s.overlayActive = true
	s.app.EnableMouse(false)

	input := tview.NewInputField().
		SetLabel("  " + title + ": ").
		SetLabelColor(theme.Accent).
		SetText(currentValue).
		SetFieldWidth(40).
		SetFieldBackgroundColor(theme.BgElement).
		SetFieldTextColor(theme.FgPrimary)
	input.SetBackgroundColor(theme.BgPanel)

	input.SetDoneFunc(func(key tcell.Key) {
		s.pages.RemovePage("input-modal")
		s.overlayActive = false
		s.app.EnableMouse(true)
		s.app.SetFocus(s.content)
		if key == tcell.KeyEnter && onConfirm != nil {
			onConfirm(input.GetText())
		}
	})

	// Frame the input with border and title
	frame := tview.NewFlex().SetDirection(tview.FlexRow)
	frame.AddItem(tview.NewTextView().SetText(""), 1, 0, false) // top padding
	frame.AddItem(input, 1, 0, true)
	frame.AddItem(tview.NewTextView().SetText(""), 1, 0, false) // bottom padding
	frame.SetBorder(true)
	frame.SetBorderColor(theme.Accent)
	frame.SetTitle(" " + title + " · Esc annuler ")
	frame.SetTitleColor(theme.Accent)
	frame.SetBackgroundColor(theme.BgPanel)

	// Center it
	grid := tview.NewGrid().
		SetColumns(0, 60, 0).
		SetRows(0, 5, 0)
	grid.AddItem(frame, 1, 1, 1, 1, 0, 0, true)

	s.pages.AddPage("input-modal", grid, true, true)
	s.app.SetFocus(input)
}

// ShowSelectModal displays a centered dropdown modal for selecting a value from a list.
func (s *Shell) ShowSelectModal(title string, options []views.SelectOption, currentValue string, onConfirm func(value string)) {
	s.overlayActive = true
	s.app.EnableMouse(false)

	// Build dropdown options and find current index
	currentIdx := 0
	dd := tview.NewDropDown().
		SetLabel("  " + title + ": ").
		SetLabelColor(theme.Accent).
		SetFieldBackgroundColor(theme.BgElement).
		SetFieldTextColor(theme.FgPrimary).
		SetListStyles(
			tcell.StyleDefault.Background(theme.BgElement).Foreground(theme.FgPrimary),
			tcell.StyleDefault.Background(theme.Accent).Foreground(theme.BgPanel),
		)
	dd.SetBackgroundColor(theme.BgPanel)

	for i, opt := range options {
		dd.AddOption(opt.Label, nil)
		if opt.Value == currentValue {
			currentIdx = i
		}
	}
	dd.SetCurrentOption(currentIdx)

	// Handle selection
	confirmed := false
	dd.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			confirmed = true
			idx, _ := dd.GetCurrentOption()
			s.pages.RemovePage("select-modal")
			s.overlayActive = false
			s.app.EnableMouse(true)
			s.app.SetFocus(s.content)
			if onConfirm != nil && idx >= 0 && idx < len(options) {
				onConfirm(options[idx].Value)
			}
		} else if key == tcell.KeyEscape {
			s.pages.RemovePage("select-modal")
			s.overlayActive = false
			s.app.EnableMouse(true)
			s.app.SetFocus(s.content)
		}
	})

	// Also handle Tab as confirm (tview DropDown closes list on Enter then sends Tab)
	dd.SetSelectedFunc(func(text string, index int) {
		if !confirmed {
			confirmed = true
			s.pages.RemovePage("select-modal")
			s.overlayActive = false
			s.app.EnableMouse(true)
			s.app.SetFocus(s.content)
			if onConfirm != nil && index >= 0 && index < len(options) {
				onConfirm(options[index].Value)
			}
		}
	})

	// Frame the dropdown with border and title
	frame := tview.NewFlex().SetDirection(tview.FlexRow)
	frame.AddItem(tview.NewTextView().SetText(""), 1, 0, false)
	frame.AddItem(dd, 1, 0, true)
	frame.AddItem(tview.NewTextView().SetText(""), 1, 0, false)
	frame.SetBorder(true)
	frame.SetBorderColor(theme.Accent)
	frame.SetTitle(" " + title + " · Enter confirmer · Esc annuler ")
	frame.SetTitleColor(theme.Accent)
	frame.SetBackgroundColor(theme.BgPanel)

	// Center it
	grid := tview.NewGrid().
		SetColumns(0, 60, 0).
		SetRows(0, 5, 0)
	grid.AddItem(frame, 1, 1, 1, 1, 0, 0, true)

	s.pages.AddPage("select-modal", grid, true, true)
	s.app.SetFocus(dd)
}

// ShowMultiSelectModal displays a centered modal with a checkbox list for multi-selection.
// Each option can be toggled with Space; Enter confirms, Esc cancels.
func (s *Shell) ShowMultiSelectModal(title string, options []views.SelectOption, selected []string, onConfirm func(selected []string)) {
	s.overlayActive = true
	s.app.EnableMouse(false)

	// Build selected set for quick lookup
	selectedSet := make(map[string]bool, len(selected))
	for _, v := range selected {
		selectedSet[v] = true
	}

	// Track checked state per option
	checked := make([]bool, len(options))
	for i, opt := range options {
		checked[i] = selectedSet[opt.Value]
	}

	// Build the list widget
	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSelectedBackgroundColor(theme.BgElement).
		SetSelectedTextColor(theme.FgPrimary)
	list.SetBackgroundColor(theme.BgPanel)

	// Render function to update labels with checkmarks
	renderItems := func() {
		list.Clear()
		for i, opt := range options {
			prefix := "[ ] "
			if checked[i] {
				prefix = "[x] "
			}
			list.AddItem(prefix+opt.Label, "", 0, nil)
		}
	}
	renderItems()

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			s.pages.RemovePage("multiselect-modal")
			s.overlayActive = false
			s.app.EnableMouse(true)
			s.app.SetFocus(s.content)
			return nil
		case tcell.KeyEnter:
			// Confirm: collect selected values
			var result []string
			for i, opt := range options {
				if checked[i] {
					result = append(result, opt.Value)
				}
			}
			s.pages.RemovePage("multiselect-modal")
			s.overlayActive = false
			s.app.EnableMouse(true)
			s.app.SetFocus(s.content)
			if onConfirm != nil {
				onConfirm(result)
			}
			return nil
		}

		// Space or 'x' to toggle
		if event.Rune() == ' ' || event.Rune() == 'x' {
			idx := list.GetCurrentItem()
			if idx >= 0 && idx < len(checked) {
				checked[idx] = !checked[idx]
				renderItems()
				list.SetCurrentItem(idx)
			}
			return nil
		}

		return event
	})

	// Frame
	frame := tview.NewFlex().SetDirection(tview.FlexRow)
	frame.AddItem(list, 0, 1, true)
	frame.SetBorder(true)
	frame.SetBorderColor(theme.Accent)
	frame.SetTitle(" " + title + " · Space toggle · Enter confirmer · Esc annuler ")
	frame.SetTitleColor(theme.Accent)
	frame.SetBackgroundColor(theme.BgPanel)

	// Center (60% width, 60% height)
	grid := tview.NewGrid().
		SetColumns(0, -3, 0).
		SetRows(2, -3, 2)
	grid.AddItem(frame, 1, 1, 1, 1, 0, 0, true)

	s.pages.AddPage("multiselect-modal", grid, true, true)
	s.app.SetFocus(list)
}

// ShowScrollableModal displays a centered modal with scrollable text content and action buttons.
func (s *Shell) ShowScrollableModal(title, content string, actions []views.ModalAction) {
	s.overlayActive = true
	s.app.EnableMouse(false)

	// Scrollable text content
	textView := tview.NewTextView().
		SetText(content).
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	textView.SetBackgroundColor(theme.BgPanel)
	textView.SetTextColor(theme.FgPrimary)
	textView.SetBorderPadding(0, 0, 1, 1)

	// Button bar at the bottom
	buttons := tview.NewFlex()
	buttons.SetBackgroundColor(theme.BgPanel)

	focusables := []tview.Primitive{textView}

	for _, act := range actions {
		a := act // capture
		btn := tview.NewButton(a.Label).
			SetSelectedFunc(func() {
				s.pages.RemovePage("scrollable-modal")
				s.overlayActive = false
				s.app.EnableMouse(true)
				s.app.SetFocus(s.content)
				if a.Callback != nil {
					a.Callback()
				}
			})
		btn.SetBackgroundColor(theme.BgElement)
		btn.SetLabelColor(theme.FgPrimary)
		btn.SetBackgroundColorActivated(theme.Accent)
		btn.SetLabelColorActivated(theme.BgPanel)
		buttons.AddItem(btn, len(a.Label)+4, 0, false)
		buttons.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 1, 0, false)
		focusables = append(focusables, btn)
	}

	// Layout: text area + button bar
	frame := tview.NewFlex().SetDirection(tview.FlexRow)
	frame.AddItem(textView, 0, 1, true)
	frame.AddItem(buttons, 1, 0, false)
	frame.SetBorder(true)
	frame.SetBorderColor(theme.Accent)
	frame.SetTitle(" " + title + " · Tab switch · Esc annuler ")
	frame.SetTitleColor(theme.Accent)
	frame.SetBackgroundColor(theme.BgPanel)

	// Focus cycling with Tab
	currentFocus := 0
	frame.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			s.pages.RemovePage("scrollable-modal")
			s.overlayActive = false
			s.app.EnableMouse(true)
			s.app.SetFocus(s.content)
			return nil
		case tcell.KeyTab, tcell.KeyBacktab:
			if event.Key() == tcell.KeyTab {
				currentFocus = (currentFocus + 1) % len(focusables)
			} else {
				currentFocus = (currentFocus - 1 + len(focusables)) % len(focusables)
			}
			s.app.SetFocus(focusables[currentFocus])
			return nil
		}
		return event
	})

	// Center (70% width, 70% height)
	grid := tview.NewGrid().
		SetColumns(0, -4, 0).
		SetRows(1, -4, 1)
	grid.AddItem(frame, 1, 1, 1, 1, 0, 0, true)

	s.pages.AddPage("scrollable-modal", grid, true, true)
	s.app.SetFocus(textView)
}

// ShowModal displays a centered confirmation modal.
func (s *Shell) ShowModal(title, message string, onConfirm func()) {
	s.overlayActive = true
	s.app.EnableMouse(false)

	modal := tview.NewModal().
		SetText(fmt.Sprintf("%s\n\n%s", title, message)).
		AddButtons([]string{"Annuler", "Confirmer"}).
		SetDoneFunc(func(buttonIndex int, _ string) {
			s.pages.RemovePage("modal")
			s.overlayActive = false
			s.app.EnableMouse(true)
			s.app.SetFocus(s.content)
			if buttonIndex == 1 && onConfirm != nil {
				onConfirm()
			}
		})
	modal.SetBackgroundColor(theme.BgElement)
	modal.SetBorderColor(theme.Accent)

	s.pages.AddPage("modal", modal, false, true)
	s.app.SetFocus(modal)
}

// DismissModal removes a named overlay page.
func (s *Shell) DismissModal(name string) {
	s.pages.RemovePage(name)
	s.app.SetFocus(s.content)
}

// SuspendAndExec suspends the TUI, runs a function, then resumes.
// Used to launch external processes like opencode.
func (s *Shell) SuspendAndExec(fn func() error) error {
	var execErr error
	s.app.Suspend(func() {
		execErr = fn()
	})
	return execErr
}

func (s *Shell) globalKeyHandler(event *tcell.EventKey) *tcell.EventKey {
	// Ctrl+Q: quit (always available, even with overlay)
	if event.Key() == tcell.KeyCtrlQ {
		s.app.Stop()
		return nil
	}

	// If an overlay is active, let the modal widget handle all other keys
	if s.overlayActive {
		return event
	}

	// Ctrl+P: command palette
	if event.Key() == tcell.KeyCtrlP {
		s.ShowCommandPalette()
		return nil
	}

	// Ctrl+N: toggle focus between menu and content
	if event.Key() == tcell.KeyCtrlN {
		s.toggleMenuFocus()
		return nil
	}

	// ?: show help view
	if event.Rune() == '?' {
		s.router.NavigateTo("help")
		return nil
	}

	// Esc: pop view (go back) when content is focused
	if event.Key() == tcell.KeyEsc {
		if s.router.StackDepth() > 1 {
			s.router.Pop()
			return nil
		}
	}

	// Delegate to current view's key handler
	if cur := s.router.Current(); cur != nil {
		return cur.HandleKey(event)
	}

	return event
}

func (s *Shell) toggleMenuFocus() {
	menuPrimitive := s.menu.Primitive()
	if s.app.GetFocus() == menuPrimitive {
		s.app.SetFocus(s.content)
		s.content.SetBorderColor(theme.BorderFocus)
	} else {
		s.app.SetFocus(menuPrimitive)
		s.content.SetBorderColor(theme.BorderNormal)
	}
}
