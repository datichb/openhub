// Package shell provides the unified TUI application shell with an
// omnibar-first design. The shell is composed of a full-screen content
// area and a persistent omnibar at the bottom for command input.
package shell

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

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
}

// shellAware is an optional interface that views can implement to receive
// a reference to the shell for toast/prompt interactions.
type shellAware interface {
	SetShell(s views.ShellAccess)
}

// Shell is the top-level TUI container with omnibar-first design.
// Layout: content (fills screen) + suggestions (dynamic) + omnibar (3 rows at bottom).
type Shell struct {
	app              *tview.Application
	pages            *tview.Pages
	root             *tview.Flex // main vertical layout (content + suggestions + omnibar)
	content          *tview.Flex
	omnibar          *Omnibar
	router           *router.Router
	registry         *CommandRegistry
	suggestionsShown bool

	// ctx is cancelled when the shell exits (SIGINT, q, or tview Stop).
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a configured Shell ready to run.
func New(cfg Config) *Shell {
	app := tview.NewApplication()
	tview.Styles = theme.TviewTheme()

	ctx, cancel := context.WithCancel(context.Background())

	s := &Shell{
		app:    app,
		ctx:    ctx,
		cancel: cancel,
	}

	// Build content panel — full screen, clean
	s.content = tview.NewFlex().SetDirection(tview.FlexRow)
	s.content.SetBackgroundColor(theme.BgPanel)
	s.content.SetBorderPadding(1, 0, 2, 2)

	// Build command registry
	s.registry = NewCommandRegistry(cfg.Commands)

	// Build omnibar
	s.omnibar = NewOmnibar(s, s.registry)

	// Build router — callback updates omnibar hints
	s.router = router.New(s.content, app, func(v views.View) {
		s.omnibar.SetHints(v.StatusHints())
	})

	// Register all views and wire ShellAccess
	for _, v := range cfg.Views {
		s.router.Register(v)
		if sv, ok := v.(shellAware); ok {
			sv.SetShell(s)
		}
	}

	// Build layout: content fills space, omnibar is 3 rows at bottom.
	// Suggestions list is inserted dynamically between content and omnibar when active.
	s.root = tview.NewFlex().SetDirection(tview.FlexRow)
	s.root.AddItem(s.content, 0, 1, true)
	s.root.AddItem(s.omnibar.Primitive(), 5, 0, false)

	// Wrap in Pages for overlay support (toasts, inline prompts)
	s.pages = tview.NewPages()
	s.pages.AddPage("main", s.root, true, true)

	// Global keybindings
	app.SetInputCapture(s.globalKeyHandler)

	return s
}

// showSuggestionsInLayout inserts the suggestions list into the root Flex
// between content and omnibar.
func (s *Shell) showSuggestionsInLayout() {
	if s.suggestionsShown {
		return
	}
	s.suggestionsShown = true
	// Remove omnibar, add suggestions, re-add omnibar
	s.root.RemoveItem(s.omnibar.Primitive())
	s.root.AddItem(s.omnibar.SuggestionsPrimitive(), s.omnibar.SuggestionsHeight(), 0, false)
	s.root.AddItem(s.omnibar.Primitive(), 5, 0, false)
}

// hideSuggestionsFromLayout removes the suggestions list from the root Flex.
func (s *Shell) hideSuggestionsFromLayout() {
	if !s.suggestionsShown {
		return
	}
	s.suggestionsShown = false
	s.root.RemoveItem(s.omnibar.SuggestionsPrimitive())
}

// updateSuggestionsHeight recalculates the suggestions height in the layout.
func (s *Shell) updateSuggestionsHeight() {
	if !s.suggestionsShown {
		return
	}
	// Rebuild: remove suggestions + omnibar, re-add with new height
	s.root.RemoveItem(s.omnibar.SuggestionsPrimitive())
	s.root.RemoveItem(s.omnibar.Primitive())
	s.root.AddItem(s.omnibar.SuggestionsPrimitive(), s.omnibar.SuggestionsHeight(), 0, false)
	s.root.AddItem(s.omnibar.Primitive(), 5, 0, false)
}

// Run starts the tview event loop. Blocks until quit.
func (s *Shell) Run() error {
	s.app.SetRoot(s.pages, true).EnableMouse(true)
	err := s.app.Run()
	s.cancel() // signal all goroutines to stop
	return err
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

// NavigateHome navigates to the registered home view by ID.
func (s *Shell) NavigateHome(homeID string) {
	s.router.NavigateTo(homeID)
}

// ─────────────────────────────────────────────────────────────────────────────
// ShellAccess implementation (views.ShellAccess)
// ─────────────────────────────────────────────────────────────────────────────

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

// ShowInputModal displays an inline input prompt in the content area.
// The content is temporarily replaced with a focused input field.
func (s *Shell) ShowInputModal(title, currentValue string, onConfirm func(newValue string)) {
	s.showInlineInput(title, currentValue, false, onConfirm)
}

// ShowPasswordModal displays an inline masked input prompt.
func (s *Shell) ShowPasswordModal(title string, onConfirm func(value string)) {
	s.showInlineInput(title, "", true, func(v string) {
		if onConfirm != nil {
			onConfirm(v)
		}
	})
}

// ShowSelectModal displays an inline select list (fzf-style).
func (s *Shell) ShowSelectModal(title string, options []views.SelectOption, currentValue string, onConfirm func(value string)) {
	s.showInlineSelect(title, options, currentValue, onConfirm)
}

// ShowMultiSelectModal displays an inline multi-select list.
func (s *Shell) ShowMultiSelectModal(title string, options []views.SelectOption, selected []string, onConfirm func(selected []string)) {
	s.showInlineMultiSelect(title, options, selected, onConfirm)
}

// ShowScrollableModal displays an inline scrollable content view with action buttons.
func (s *Shell) ShowScrollableModal(title, content string, actions []views.ModalAction) {
	s.showInlineScrollable(title, content, actions)
}

// ShowInlineForm displays a multi-field form as a centered modal.
//
// UX design:
//   - ↑/↓ navigate between fields (translated to Shift-Tab/Tab)
//   - ←/→ cycle options on FieldSelect fields
//   - Enter on FieldSelect opens a full list sub-modal (showSubSelect)
//   - Enter on FieldMultiSelect opens a multi-select sub-modal (showSubMultiSelect)
//   - Esc / "Annuler" button cancel and close
//   - "Confirmer" button submits
//   - Background dimmed to signal modal context
func (s *Shell) ShowInlineForm(cfg views.InlineFormConfig) {
	form := tview.NewForm()
	form.SetBackgroundColor(theme.BgPanel)
	form.SetLabelColor(theme.FgPrimary)
	form.SetFieldBackgroundColor(theme.BgElement)
	form.SetFieldTextColor(theme.FgPrimary)
	form.SetButtonBackgroundColor(theme.BgElement)
	form.SetButtonTextColor(theme.FgPrimary)
	form.SetButtonActivatedStyle(
		tcell.StyleDefault.Background(theme.Accent).Foreground(theme.BgPanel))
	form.SetBorder(true)
	form.SetBorderColor(theme.Accent)
	form.SetTitle(fmt.Sprintf("  %s  ", cfg.Title))
	form.SetTitleColor(theme.Accent)

	values := make(map[string]string)
	multi := make(map[string][]string)

	// fieldMeta tracks per-field state needed by the InputCapture handler.
	type fieldMeta struct {
		field      *views.FormField
		inputField *tview.InputField // non-nil for FieldSelect / FieldMultiSelect
		selectIdx  int               // current option index (FieldSelect only)
	}
	var metas []fieldMeta

	// ── Build form items ──────────────────────────────────────────────────────
	for i := range cfg.Fields {
		field := &cfg.Fields[i]

		switch field.Type {

		case views.FieldSelect:
			// Find initial option index.
			idx := 0
			for j, opt := range field.Options {
				if opt.Value == field.Default {
					idx = j
					break
				}
			}
			values[field.Key] = field.Default
			label := selectLabel(field, idx)
			inp := tview.NewInputField().
				SetLabel(field.Label).
				SetText(label).
				SetFieldWidth(24).
				SetAcceptanceFunc(func(string, rune) bool { return false }) // read-only
			inp.SetBackgroundColor(theme.BgPanel)
			inp.SetFieldBackgroundColor(theme.BgElement)
			inp.SetFieldTextColor(theme.FgPrimary)
			inp.SetLabelColor(theme.FgPrimary)
			form.AddFormItem(inp)
			metas = append(metas, fieldMeta{field: field, inputField: inp, selectIdx: idx})

		case views.FieldMultiSelect:
			if field.DefaultMulti != nil {
				multi[field.Key] = append([]string{}, field.DefaultMulti...)
			}
			inp := tview.NewInputField().
				SetLabel(field.Label).
				SetText(multiSummary(multi[field.Key], field.Options)).
				SetFieldWidth(30).
				SetAcceptanceFunc(func(string, rune) bool { return false }) // read-only
			inp.SetBackgroundColor(theme.BgPanel)
			inp.SetFieldBackgroundColor(theme.BgElement)
			inp.SetFieldTextColor(theme.FgPrimary)
			inp.SetLabelColor(theme.FgPrimary)
			form.AddFormItem(inp)
			metas = append(metas, fieldMeta{field: field, inputField: inp})

		case views.FieldText:
			values[field.Key] = field.Default
			form.AddInputField(field.Label, field.Default, 40, nil,
				func(text string) { values[field.Key] = text })
			metas = append(metas, fieldMeta{field: field})

		case views.FieldPassword:
			values[field.Key] = field.Default
			form.AddPasswordField(field.Label, field.Default, 40, '*',
				func(text string) { values[field.Key] = text })
			metas = append(metas, fieldMeta{field: field})

		case views.FieldBool:
			checked := field.Default == "true"
			values[field.Key] = field.Default
			form.AddCheckbox(field.Label, checked, func(c bool) {
				if c {
					values[field.Key] = "true"
				} else {
					values[field.Key] = "false"
				}
			})
			metas = append(metas, fieldMeta{field: field})
		}
	}

	// ── InputCapture ─────────────────────────────────────────────────────────
	// Handles ↑↓ navigation and ←/→/Enter on Select/MultiSelect fields.
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Sub-overlay open (select/multiselect list) → don't intercept anything.
		if s.pages.HasPage("sub-overlay") {
			return event
		}

		// ↑ / ↓  →  navigate between form fields (and buttons)
		if event.Key() == tcell.KeyUp {
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
		}
		if event.Key() == tcell.KeyDown {
			return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
		}

		itemIdx, btnIdx := form.GetFocusedItemIndex()

		// When focus is on a button, ←/→ cycle directly between buttons using
		// app.SetFocus so we stay within the button row (Tab/BackTab would
		// escape to the field list).
		if btnIdx >= 0 {
			n := form.GetButtonCount()
			switch event.Key() {
			case tcell.KeyLeft:
				prev := btnIdx - 1
				if prev < 0 {
					prev = n - 1
				}
				s.app.SetFocus(form.GetButton(prev))
				return nil
			case tcell.KeyRight:
				s.app.SetFocus(form.GetButton((btnIdx + 1) % n))
				return nil
			}
			return event
		}

		if itemIdx < 0 || itemIdx >= len(metas) {
			return event
		}
		meta := &metas[itemIdx]

		switch meta.field.Type {

		case views.FieldSelect:
			opts := meta.field.Options
			n := len(opts)
			if n == 0 {
				return nil
			}
			switch event.Key() {
			case tcell.KeyLeft:
				meta.selectIdx = (meta.selectIdx - 1 + n) % n
				values[meta.field.Key] = opts[meta.selectIdx].Value
				meta.inputField.SetText(selectLabel(meta.field, meta.selectIdx))
				return nil
			case tcell.KeyRight:
				meta.selectIdx = (meta.selectIdx + 1) % n
				values[meta.field.Key] = opts[meta.selectIdx].Value
				meta.inputField.SetText(selectLabel(meta.field, meta.selectIdx))
				return nil
			case tcell.KeyEnter:
				s.showSubSelect(
					meta.field.Label,
					meta.field.Options,
					values[meta.field.Key],
					form,
					func(value string) {
						for j, opt := range meta.field.Options {
							if opt.Value == value {
								meta.selectIdx = j
								break
							}
						}
						values[meta.field.Key] = value
						meta.inputField.SetText(selectLabel(meta.field, meta.selectIdx))
					},
				)
				return nil
			}

		case views.FieldMultiSelect:
			if event.Key() == tcell.KeyEnter {
				s.showSubMultiSelect(
					meta.field.Label,
					meta.field.Options,
					multi[meta.field.Key],
					form,
					func(selected []string) {
						multi[meta.field.Key] = selected
						meta.inputField.SetText(multiSummary(selected, meta.field.Options))
					},
				)
				return nil
			}
		}

		return event
	})

	// ── Buttons ───────────────────────────────────────────────────────────────
	form.AddButton("Confirmer", func() {
		s.pages.RemovePage("inline-overlay")
		s.app.SetFocus(s.content)
		if cfg.OnSubmit != nil {
			cfg.OnSubmit(values, multi)
		}
	})
	form.AddButton("Annuler", func() {
		s.pages.RemovePage("inline-overlay")
		s.app.SetFocus(s.content)
		if cfg.OnCancel != nil {
			cfg.OnCancel()
		}
	})
	form.SetCancelFunc(func() {
		s.pages.RemovePage("inline-overlay")
		s.app.SetFocus(s.content)
		if cfg.OnCancel != nil {
			cfg.OnCancel()
		}
	})

	// ── Layout ────────────────────────────────────────────────────────────────
	// Wrap form + hints in a Flex so overlayGrid can handle the backdrop.
	// overlayGrid fills surrounding cells with opaque dim boxes — the only
	// reliable way to render a backdrop in tview (Grid.SetBackgroundColor is
	// a no-op due to dontClear=true in the tview Grid constructor).
	hints := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter)
	hints.SetBackgroundColor(theme.BgDimOverlay)
	hints.SetText(fmt.Sprintf(
		"%s↑↓%s naviguer  %s←→%s cycler · boutons  %sEnter%s ouvrir  %sEsc%s annuler",
		theme.ColorTag(theme.AccentHex), theme.TagColor,
		theme.ColorTag(theme.AccentHex), theme.TagColor,
		theme.ColorTag(theme.AccentHex), theme.TagColor,
		theme.ColorTag(theme.AccentHex), theme.TagColor,
	))

	// No light-dismiss on the form — accidental click would discard in-progress edits.
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	container.SetBackgroundColor(theme.BgPanel)
	container.AddItem(form, 0, 1, true)
	container.AddItem(hints, 1, 0, false)

	grid := s.overlayGrid(container, []int{0, 64, 0}, []int{0, 0, 0}, theme.BgDimOverlay, "", nil)

	s.pages.AddPage("inline-overlay", grid, true, true)
	s.app.SetFocus(form)
}

// selectLabel formats the display text for a FieldSelect input: "◂ Label ▸".
func selectLabel(field *views.FormField, idx int) string {
	if idx < 0 || idx >= len(field.Options) {
		return "◂ — ▸"
	}
	return "◂ " + field.Options[idx].Label + " ▸"
}

// multiSummary formats the display text for a FieldMultiSelect input.
// Shows up to 2 labels then "+N more", with a ⏎ hint to open the selector.
func multiSummary(selected []string, options []views.SelectOption) string {
	if len(selected) == 0 {
		return "— (Enter sélectionner)"
	}
	labelOf := make(map[string]string, len(options))
	for _, opt := range options {
		labelOf[opt.Value] = opt.Label
	}
	var labels []string
	for _, v := range selected {
		if l, ok := labelOf[v]; ok {
			labels = append(labels, l)
		}
	}
	const max = 2
	var summary string
	if len(labels) <= max {
		summary = strings.Join(labels, ", ")
	} else {
		summary = strings.Join(labels[:max], ", ") + fmt.Sprintf(", +%d", len(labels)-max)
	}
	return summary + " ⏎"
}

// showSubSelect opens a list sub-modal on top of an existing overlay (the form).
// On confirmation or cancellation the focus returns to the form.
func (s *Shell) showSubSelect(title string, options []views.SelectOption, currentValue string, returnTo tview.Primitive, onConfirm func(string)) {
	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSelectedBackgroundColor(theme.BgElement).
		SetSelectedTextColor(theme.FgPrimary)
	list.SetBackgroundColor(theme.BgPanel)
	list.SetBorder(true)
	list.SetBorderColor(theme.Accent)
	list.SetTitle(fmt.Sprintf("  %s · Enter choisir · Esc retour  ", title))
	list.SetTitleColor(theme.Accent)

	currentIdx := 0
	for i, opt := range options {
		list.AddItem(opt.Label, "", 0, nil)
		if opt.Value == currentValue {
			currentIdx = i
		}
	}
	list.SetCurrentItem(currentIdx)

	dismiss := func() {
		s.pages.RemovePage("sub-overlay")
		s.app.SetFocus(returnTo)
	}

	confirm := func(idx int) {
		if onConfirm != nil && idx >= 0 && idx < len(options) {
			onConfirm(options[idx].Value)
		}
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			dismiss()
			return nil
		case tcell.KeyEnter:
			idx := list.GetCurrentItem()
			dismiss()
			confirm(idx)
			return nil
		}
		return event
	})

	// Click on item = immediate selection.
	s.listMouseSelect(list, "sub-overlay", returnTo, confirm)

	height := len(options) + 2
	if height > 16 {
		height = 16
	}
	grid := s.overlayGrid(list, []int{0, -3, 0}, []int{0, height, 0}, theme.BgDimSubOverlay, "sub-overlay", returnTo)

	s.pages.AddPage("sub-overlay", grid, true, true)
	s.app.SetFocus(list)
}

// showSubMultiSelect opens a multi-select list sub-modal on top of the form.
func (s *Shell) showSubMultiSelect(title string, options []views.SelectOption, selected []string, returnTo tview.Primitive, onConfirm func([]string)) {
	checked := make([]bool, len(options))
	selectedSet := make(map[string]bool, len(selected))
	for _, v := range selected {
		selectedSet[v] = true
	}
	for i, opt := range options {
		checked[i] = selectedSet[opt.Value]
	}

	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSelectedBackgroundColor(theme.BgElement).
		SetSelectedTextColor(theme.FgPrimary)
	list.SetBackgroundColor(theme.BgPanel)
	list.SetBorder(true)
	list.SetBorderColor(theme.Accent)
	list.SetBorderPadding(0, 0, 1, 1)
	list.SetTitle(fmt.Sprintf("  %s · Space/x cocher · Enter confirmer · Esc retour  ", title))
	list.SetTitleColor(theme.Accent)

	renderItems := func() {
		list.Clear()
		for i, opt := range options {
			prefix := "☐ "
			if checked[i] {
				prefix = "☑ "
			}
			list.AddItem(prefix+opt.Label, "", 0, nil)
		}
	}
	renderItems()

	dismiss := func() {
		s.pages.RemovePage("sub-overlay")
		s.app.SetFocus(returnTo)
	}

	toggleItem := func(idx int) {
		if idx >= 0 && idx < len(checked) {
			checked[idx] = !checked[idx]
			renderItems()
			list.SetCurrentItem(idx)
		}
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			dismiss()
			return nil
		case tcell.KeyEnter:
			var result []string
			for i, opt := range options {
				if checked[i] {
					result = append(result, opt.Value)
				}
			}
			dismiss()
			if onConfirm != nil {
				onConfirm(result)
			}
			return nil
		}
		if event.Rune() == ' ' || event.Rune() == 'x' {
			toggleItem(list.GetCurrentItem())
			return nil
		}
		return event
	})

	// Click on item = toggle its checked state.
	s.listMouseToggle(list, toggleItem)

	height := len(options) + 2
	if height > 16 {
		height = 16
	}
	grid := s.overlayGrid(list, []int{0, -3, 0}, []int{0, height, 0}, theme.BgDimSubOverlay, "sub-overlay", returnTo)

	s.pages.AddPage("sub-overlay", grid, true, true)
	s.app.SetFocus(list)
}

// ShowSessionLauncher displays session options as an inline select.
func (s *Shell) ShowSessionLauncher(cfg SessionLaunchConfig) {
	options := make([]views.SelectOption, len(cfg.Options))
	for i, opt := range cfg.Options {
		options[i] = views.SelectOption{
			Label: fmt.Sprintf("%s — %s", opt.Label, opt.Description),
			Value: opt.Label,
		}
	}
	s.showInlineSelect(cfg.Title, options, "", func(value string) {
		for _, opt := range cfg.Options {
			if opt.Label == value {
				if cfg.OnLaunch != nil {
					cfg.OnLaunch(opt)
				}
				return
			}
		}
	})
}

// SuspendAndExec suspends the TUI, runs a function, then resumes.
// After resume it forces a full terminal re-sync (Sync) and restores focus to
// the main content area to avoid a permanent freeze caused by tcell's
// screen.Resume() silently failing on macOS after a subprocess (e.g., opencode)
// that manipulates the tty.
func (s *Shell) SuspendAndExec(fn func() error) error {
	var execErr error
	ok := s.app.Suspend(func() {
		execErr = fn()
	})
	if !ok {
		return fmt.Errorf("TUI suspend failed — session not launched")
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

// ─────────────────────────────────────────────────────────────────────────────
// Global key handler — minimal and predictable
// ─────────────────────────────────────────────────────────────────────────────

func (s *Shell) globalKeyHandler(event *tcell.EventKey) *tcell.EventKey {
	// Ctrl+Q / Ctrl+C: quit (always available)
	if event.Key() == tcell.KeyCtrlQ || event.Key() == tcell.KeyCtrlC {
		s.app.Stop()
		return nil
	}

	// If omnibar is active, let it handle keys
	if s.omnibar.IsActive() {
		return event // input field captures its own keys
	}

	// If an inline overlay or sub-overlay is active, let it handle keys
	if s.pages.HasPage("sub-overlay") || s.pages.HasPage("inline-overlay") {
		return event
	}

	// Ctrl+P or /: activate omnibar
	if event.Key() == tcell.KeyCtrlP {
		s.omnibar.Activate()
		return nil
	}

	// Esc: pop view (go back) or do nothing
	if event.Key() == tcell.KeyEsc {
		if s.router.StackDepth() > 1 {
			s.router.Pop()
			return nil
		}
		return nil
	}

	// Tab: reserved for future multi-panel cycling
	if event.Key() == tcell.KeyTab {
		return event
	}

	// Delegate to current view's key handler
	if cur := s.router.Current(); cur != nil {
		result := cur.HandleKey(event)
		if result == nil {
			return nil // view consumed the event
		}
	}

	// If the view didn't consume a printable rune, activate omnibar with it.
	// Exception: j/k are universal navigation keys — translate them to
	// Down/Up arrow so every list/table widget responds natively.
	// Exception: ? opens the help overlay instantly (lazygit / htop convention).
	if event.Key() == tcell.KeyRune {
		switch event.Rune() {
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case '?':
			s.showHelpOverlay()
			return nil
		}
		s.omnibar.ActivateWithRune(event.Rune())
		return nil
	}

	// F1: same as '?' — show help overlay (Windows/htop/mc convention).
	if event.Key() == tcell.KeyF1 {
		s.showHelpOverlay()
		return nil
	}

	// Enter: active l'omnibar si aucune vue n'a consommé l'event.
	if event.Key() == tcell.KeyEnter {
		s.omnibar.Activate()
		return nil
	}

	return event
}

// ─────────────────────────────────────────────────────────────────────────────
// Modal overlay helpers
// ─────────────────────────────────────────────────────────────────────────────

// overlayGrid creates a centered 3×3 grid with a dimmed backdrop.
// The content primitive is placed at cell (1,1).
//
// tview.Grid never paints its backgroundColor (dontClear=true in the tview
// constructor), so we explicitly fill all 8 surrounding cells with opaque Box
// primitives — the only reliable way to render the backdrop in tview.
//
// When dismissPage is non-empty, clicking anywhere on the backdrop (outside
// the content cell) dismisses the overlay — the "light dismiss" pattern
// familiar from macOS and web dialogs.
func (s *Shell) overlayGrid(content tview.Primitive, cols, rows []int, bg tcell.Color, dismissPage string, focusReturn tview.Primitive) *tview.Grid {
	grid := tview.NewGrid().SetColumns(cols...).SetRows(rows...)

	// Fill every cell except (1,1) with an opaque backdrop box.
	// tview.Grid.SetBackgroundColor is ineffective due to dontClear=true.
	nRows := len(rows)
	nCols := len(cols)
	for r := 0; r < nRows; r++ {
		for c := 0; c < nCols; c++ {
			if r == 1 && c == 1 {
				continue // content cell — added below
			}
			box := tview.NewBox().SetBackgroundColor(bg)
			grid.AddItem(box, r, c, 1, 1, 0, 0, false)
		}
	}
	grid.AddItem(content, 1, 1, 1, 1, 0, 0, true)

	// Always intercept backdrop clicks — two behaviours depending on dismissPage:
	// - non-empty: light dismiss (close modal, restore focus)
	// - empty:     keep modal open but consume the click to prevent focus theft
	//
	// We intercept BOTH MouseLeftDown and MouseLeftClick because Box.MouseHandler
	// calls setFocus(b) on MouseLeftDown — which fires BEFORE MouseLeftClick.
	// Blocking only MouseLeftClick is too late; focus is already stolen by then.
	grid.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftDown || action == tview.MouseLeftClick {
			cx, cy, cw, ch := content.GetRect()
			mx, my := event.Position()
			// Click is on the dimmed backdrop (outside content bounds)
			if mx < cx || mx >= cx+cw || my < cy || my >= cy+ch {
				// Dismiss only on Click (not Down) — natural "release to dismiss" UX.
				if action == tview.MouseLeftClick && dismissPage != "" {
					s.pages.RemovePage(dismissPage)
					if focusReturn != nil {
						s.app.SetFocus(focusReturn)
					} else {
						s.app.SetFocus(s.content)
					}
				}
				// Consume the event in both cases — prevents backdrop Box from
				// calling setFocus and stealing focus from the modal content.
				return tview.MouseConsumed, nil
			}
		}
		return action, event
	})
	return grid
}

// listMouseSelect wires a single-click-to-confirm behaviour on a tview.List
// inside a modal overlay. A click moves the cursor to the clicked item and
// immediately confirms the selection — matching the macOS Finder / lazygit
// "click to pick" pattern.
func (s *Shell) listMouseSelect(list *tview.List, pageName string, focusReturn tview.Primitive, onSelect func(idx int)) {
	list.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftClick {
			// Let tview update the highlighted item first, then confirm.
			s.app.QueueUpdateDraw(func() {
				idx := list.GetCurrentItem()
				if idx >= 0 {
					s.pages.RemovePage(pageName)
					if focusReturn != nil {
						s.app.SetFocus(focusReturn)
					} else {
						s.app.SetFocus(s.content)
					}
					if onSelect != nil {
						onSelect(idx)
					}
				}
			})
			return action, event
		}
		return action, event
	})
}

// listMouseToggle wires a single-click-to-toggle behaviour on a tview.List
// used for multi-select overlays. A click flips the checked state of the
// clicked item without closing the modal — matching standard checkbox UX.
func (s *Shell) listMouseToggle(list *tview.List, toggle func(idx int)) {
	list.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftClick {
			s.app.QueueUpdateDraw(func() {
				idx := list.GetCurrentItem()
				if idx >= 0 && toggle != nil {
					toggle(idx)
				}
			})
			return action, event
		}
		return action, event
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// Inline prompts (replace modal overlays)
// ─────────────────────────────────────────────────────────────────────────────

func (s *Shell) showInlineInput(title, currentValue string, masked bool, onConfirm func(string)) {
	input := tview.NewInputField().
		SetLabel(fmt.Sprintf("  %s: ", title)).
		SetLabelColor(theme.Accent).
		SetText(currentValue).
		SetFieldWidth(50).
		SetFieldBackgroundColor(theme.BgElement).
		SetFieldTextColor(theme.FgPrimary)
	input.SetBackgroundColor(theme.BgPanel)

	if masked {
		input.SetMaskCharacter('*')
	}

	// Hints below input
	hint := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	hint.SetBackgroundColor(theme.BgPanel)
	hint.SetText(fmt.Sprintf("  %sEnter%s confirmer  %sEsc%s annuler",
		theme.ColorTag(theme.AccentHex), theme.TagColor,
		theme.ColorTag(theme.TextMutedHex), theme.TagColor))

	// Layout: centered vertically
	frame := tview.NewFlex().SetDirection(tview.FlexRow)
	frame.SetBackgroundColor(theme.BgPanel)
	frame.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false) // top spacer
	frame.AddItem(input, 1, 0, true)
	frame.AddItem(hint, 1, 0, false)
	frame.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false) // bottom spacer

	input.SetDoneFunc(func(key tcell.Key) {
		s.pages.RemovePage("inline-overlay")
		s.app.SetFocus(s.content)
		if key == tcell.KeyEnter && onConfirm != nil {
			onConfirm(input.GetText())
		}
	})

	// No light-dismiss on input — accidental backdrop click would lose in-progress text.
	grid := s.overlayGrid(frame, []int{0, 60, 0}, []int{0, 4, 0}, theme.BgDimOverlay, "", nil)

	s.pages.AddPage("inline-overlay", grid, true, true)
	s.app.SetFocus(input)
}

func (s *Shell) showInlineSelect(title string, options []views.SelectOption, currentValue string, onConfirm func(string)) {
	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSelectedBackgroundColor(theme.BgElement).
		SetSelectedTextColor(theme.FgPrimary)
	list.SetBackgroundColor(theme.BgPanel)
	list.SetBorder(true)
	list.SetBorderColor(theme.Accent)
	list.SetTitle(fmt.Sprintf(" %s · Enter sélectionner · Esc annuler ", title))
	list.SetTitleColor(theme.Accent)

	currentIdx := 0
	for i, opt := range options {
		list.AddItem(opt.Label, "", 0, nil)
		if opt.Value == currentValue {
			currentIdx = i
		}
	}
	list.SetCurrentItem(currentIdx)

	confirm := func(idx int) {
		if onConfirm != nil && idx >= 0 && idx < len(options) {
			onConfirm(options[idx].Value)
		}
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			s.pages.RemovePage("inline-overlay")
			s.app.SetFocus(s.content)
			return nil
		case tcell.KeyEnter:
			idx := list.GetCurrentItem()
			s.pages.RemovePage("inline-overlay")
			s.app.SetFocus(s.content)
			confirm(idx)
			return nil
		}
		return event
	})

	// Click on item = immediate selection (macOS Finder / lazygit convention).
	s.listMouseSelect(list, "inline-overlay", s.content, confirm)

	height := len(options) + 2
	if height > 15 {
		height = 15
	}
	grid := s.overlayGrid(list, []int{0, -3, 0}, []int{0, height, 0}, theme.BgDimOverlay, "inline-overlay", s.content)

	s.pages.AddPage("inline-overlay", grid, true, true)
	s.app.SetFocus(list)
}

func (s *Shell) showInlineMultiSelect(title string, options []views.SelectOption, selected []string, onConfirm func([]string)) {
	selectedSet := make(map[string]bool, len(selected))
	for _, v := range selected {
		selectedSet[v] = true
	}

	checked := make([]bool, len(options))
	for i, opt := range options {
		checked[i] = selectedSet[opt.Value]
	}

	list := tview.NewList().
		ShowSecondaryText(false).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSelectedBackgroundColor(theme.BgElement).
		SetSelectedTextColor(theme.FgPrimary)
	list.SetBackgroundColor(theme.BgPanel)
	list.SetBorder(true)
	list.SetBorderColor(theme.Accent)
	list.SetTitle(fmt.Sprintf(" %s · Space/x toggle · Enter confirmer · Esc annuler ", title))
	list.SetTitleColor(theme.Accent)

	renderItems := func() {
		list.Clear()
		for i, opt := range options {
			prefix := "☐ "
			if checked[i] {
				prefix = "☑ "
			}
			list.AddItem(prefix+opt.Label, "", 0, nil)
		}
	}
	renderItems()

	toggleItem := func(idx int) {
		if idx >= 0 && idx < len(checked) {
			checked[idx] = !checked[idx]
			renderItems()
			list.SetCurrentItem(idx)
		}
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			s.pages.RemovePage("inline-overlay")
			s.app.SetFocus(s.content)
			return nil
		case tcell.KeyEnter:
			var result []string
			for i, opt := range options {
				if checked[i] {
					result = append(result, opt.Value)
				}
			}
			s.pages.RemovePage("inline-overlay")
			s.app.SetFocus(s.content)
			if onConfirm != nil {
				onConfirm(result)
			}
			return nil
		}

		if event.Rune() == ' ' || event.Rune() == 'x' {
			toggleItem(list.GetCurrentItem())
			return nil
		}

		return event
	})

	// Click on item = toggle its checked state (standard checkbox UX).
	s.listMouseToggle(list, toggleItem)

	height := len(options) + 2
	if height > 15 {
		height = 15
	}
	grid := s.overlayGrid(list, []int{0, -3, 0}, []int{0, height, 0}, theme.BgDimOverlay, "inline-overlay", s.content)

	s.pages.AddPage("inline-overlay", grid, true, true)
	s.app.SetFocus(list)
}

func (s *Shell) showInlineScrollable(title, content string, actions []views.ModalAction) {
	textView := tview.NewTextView().
		SetText(content).
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true)
	textView.SetBackgroundColor(theme.BgPanel)
	textView.SetTextColor(theme.FgPrimary)
	textView.SetBorderPadding(0, 0, 1, 1)

	// Button bar
	buttons := tview.NewFlex()
	buttons.SetBackgroundColor(theme.BgPanel)

	focusables := []tview.Primitive{textView}

	for _, act := range actions {
		a := act
		btn := tview.NewButton(a.Label).
			SetSelectedFunc(func() {
				s.pages.RemovePage("inline-overlay")
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

	frame := tview.NewFlex().SetDirection(tview.FlexRow)
	frame.AddItem(textView, 0, 1, true)
	frame.AddItem(buttons, 1, 0, false)
	frame.SetBorder(true)
	frame.SetBorderColor(theme.Accent)
	frame.SetTitle(fmt.Sprintf(" %s · Tab switch · Esc fermer ", title))
	frame.SetTitleColor(theme.Accent)
	frame.SetBackgroundColor(theme.BgPanel)

	// Focus cycling
	currentFocus := 0
	frame.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			s.pages.RemovePage("inline-overlay")
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

	// Light dismiss: click outside the frame closes the modal.
	grid := s.overlayGrid(frame, []int{0, -4, 0}, []int{1, -4, 1}, theme.BgDimOverlay, "inline-overlay", s.content)

	s.pages.AddPage("inline-overlay", grid, true, true)
	s.app.SetFocus(textView)
}
