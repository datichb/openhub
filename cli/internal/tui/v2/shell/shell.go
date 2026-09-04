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
//       app.QueueUpdateDraw(func() {
//           app.QueueUpdateDraw(func() { ... })  // blocks forever
//       })
//
//     ✅ CORRECT (if you must show a toast from inside a callback):
//       app.QueueUpdateDraw(func() {
//           shell.ShowToast(...)  // direct call — ShowToast does pages.AddPage, no QueueUpdateDraw
//       })
//
//  2. NEVER call QueueUpdateDraw from inside a tview InputCapture / SetSelectedFunc
//     handler. These handlers run on the event loop — same deadlock as above.
//
//     ✅ CORRECT pattern for "show something after a handler":
//       go func() {
//           app.QueueUpdateDraw(func() { ... })  // goroutine blocks OUTSIDE the event loop
//       }()
//
//  3. When multiple sequential UI operations are needed after a handler, use a
//     single goroutine with time.Sleep(50ms) + sequential QueueUpdateDraw calls.
//     Launching parallel goroutines that both call QueueUpdateDraw can race and
//     cause two page mutations in the same draw cycle → hard freeze.
//
//     ✅ CORRECT sequential pattern:
//       go func() {
//           time.Sleep(50 * time.Millisecond) // let event loop finish the handler cycle
//           app.QueueUpdateDraw(func() { ShowToast("step 1") }) // blocks until drawn
//           app.QueueUpdateDraw(func() { ShowNextModal() })     // blocks until drawn
//       }()
package shell

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
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
	// Notifications is an optional pre-created notification store. When nil,
	// a default store with capacity 50 is created automatically.
	Notifications *NotificationStore
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

	// activeProject is non-nil when project mode is active.
	activeProject *views.ActiveProject

	// notifications holds the last N toast messages for the Notifications view.
	notifications *NotificationStore

	// activeToasts tracks the number of currently visible toast notifications.
	// Used to offset each new toast vertically so they stack instead of overlapping.
	activeToasts int

	// selection manages screen-level text selection (mouse drag → clipboard copy).
	selection *SelectionManager

	// screen is set on every draw cycle via SetAfterDrawFunc and used by the
	// mouse handler for text extraction. Guarded by the tview draw lock.
	screen tcell.Screen

	// ctx is cancelled when the shell exits (SIGINT, q, or tview Stop).
	ctx    context.Context
	cancel context.CancelFunc
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
	}

	// Wire text-selection manager — calls back into the shell for the toast.
	s.selection = NewSelectionManager(func(text string) {
		go func() {
			s.app.QueueUpdateDraw(func() {
				s.ShowToast(i18n.T("tui.shell.copied_clipboard"), ToastSuccess)
			})
		}()
	})

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

// showSuggestionsInLayout adds the suggestions list as a partial overlay
// (resize=false) so it floats just above the omnibar without pushing the content.
func (s *Shell) showSuggestionsInLayout() {
	if s.suggestionsShown {
		return
	}
	s.suggestionsShown = true
	// resize=false: tview will not auto-resize this page — we control placement via SetRect.
	s.pages.AddPage("suggestions-overlay", s.omnibar.SuggestionsPrimitive(), false, true)
	s.repositionSuggestions()
}

// hideSuggestionsFromLayout removes the suggestions overlay.
func (s *Shell) hideSuggestionsFromLayout() {
	if !s.suggestionsShown {
		return
	}
	s.suggestionsShown = false
	s.pages.RemovePage("suggestions-overlay")
}

// updateSuggestionsHeight recalculates and repositions the suggestions overlay.
func (s *Shell) updateSuggestionsHeight() {
	if !s.suggestionsShown {
		return
	}
	s.repositionSuggestions()
}

// repositionSuggestions calculates and applies the exact screen position of
// the suggestions widget. It spans the full width, sits just above the omnibar,
// and never covers more than (screenH - omnibarHeight) rows.
func (s *Shell) repositionSuggestions() {
	// GetInnerRect returns (x, y, width, height) of the pages container.
	_, _, w, screenH := s.pages.GetInnerRect()
	if w == 0 || screenH == 0 {
		// Layout not yet computed (first draw) — let tview call us again.
		return
	}

	height := s.omnibar.SuggestionsHeight()
	const omnibarHeight = 5

	y := screenH - omnibarHeight - height
	if y < 0 {
		y = 0
		height = screenH - omnibarHeight
	}
	if height < 1 {
		return
	}

	s.omnibar.SuggestionsList().SetRect(0, y, w, height)
}

// Run starts the tview event loop. Blocks until quit.
func (s *Shell) Run() error {
	// Rotate the persistent notifications file at startup (best-effort, background).
	// Keeps at most 500 entries and removes entries older than 7 days.
	go func() {
		_ = RotateFile(NotificationsFilePath(), 500, 7*24*time.Hour)
	}()
	// Apply selection highlight after every draw cycle, before screen.Show().
	// Also cache the screen reference so mouse handlers can read cell content.
	s.app.SetAfterDrawFunc(func(screen tcell.Screen) {
		s.screen = screen
		s.selection.ApplyHighlight(screen)
	})

	// Global mouse capture for text selection.
	// Events in interactive zones (omnibar, modals) are passed through unchanged.
	s.app.SetMouseCapture(func(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
		x, y := event.Position()

		switch action {
		case tview.MouseLeftDown:
			if s.isInteractiveZone(x, y) {
				s.selection.Clear()
				return event, action // pass through to tview widgets
			}
			if screen := s.screen; screen != nil {
				s.selection.HandleMouseDown(x, y, screen)
			}
			return nil, tview.MouseConsumed

		case tview.MouseMove:
			// Only intercept drag (left button held) outside interactive zones
			if event.Buttons()&tcell.Button1 != 0 {
				if s.selection.IsActive() {
					s.selection.HandleMouseDrag(x, y)
					s.app.ForceDraw()
					return nil, tview.MouseConsumed
				}
			}

		case tview.MouseLeftUp:
			if s.selection.IsActive() {
				if screen := s.screen; screen != nil {
					s.selection.HandleMouseUp(x, y, screen)
				}
				return nil, tview.MouseConsumed
			}
		}

		return event, action
	})

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

// Notifications returns the notification store for this session.
// Used by the NotificationsView to display the history of toasts.
func (s *Shell) Notifications() *NotificationStore {
	return s.notifications
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
		s.showToast(msg, ToastError, 6*time.Second)
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

	// itemToMeta maps a form item index → metas slice index.
	// This is necessary because AddTextView (hints) inserts non-interactive items
	// into the form, shifting the item indices so that metas[itemIdx] would be wrong.
	// We populate this map after each interactive field is added so hints are excluded.
	itemToMeta := make(map[int]int)

	// ── Build form items ──────────────────────────────────────────────────────
	for i := range cfg.Fields {
		field := &cfg.Fields[i]
		// Skip fields whose condition is not met
		if field.Conditional != nil && !field.Conditional(values) {
			continue
		}

		// hintLines computes the hint height: 2 lines if >50 chars, else 1.
		hintLines := func(hint string) int {
			if len("  "+hint) > 50 {
				return 2
			}
			return 1
		}

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
			itemToMeta[form.GetFormItemCount()-1] = len(metas)
			metas = append(metas, fieldMeta{field: field, inputField: inp, selectIdx: idx})
			if field.Hint != "" {
				form.AddTextView("", "  "+field.Hint, 0, hintLines(field.Hint), true, false)
			}

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
			itemToMeta[form.GetFormItemCount()-1] = len(metas)
			metas = append(metas, fieldMeta{field: field, inputField: inp})
			if field.Hint != "" {
				form.AddTextView("", "  "+field.Hint, 0, hintLines(field.Hint), true, false)
			}

		case views.FieldText:
			values[field.Key] = field.Default
			form.AddInputField(field.Label, field.Default, 40, nil,
				func(text string) { values[field.Key] = text })
			itemToMeta[form.GetFormItemCount()-1] = len(metas)
			metas = append(metas, fieldMeta{field: field})
			if field.Hint != "" {
				form.AddTextView("", "  "+field.Hint, 0, hintLines(field.Hint), true, false)
			}

		case views.FieldPassword:
			values[field.Key] = field.Default
			form.AddPasswordField(field.Label, field.Default, 40, '*',
				func(text string) { values[field.Key] = text })
			itemToMeta[form.GetFormItemCount()-1] = len(metas)
			metas = append(metas, fieldMeta{field: field})
			if field.Hint != "" {
				form.AddTextView("", "  "+field.Hint, 0, hintLines(field.Hint), true, false)
			}

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
			itemToMeta[form.GetFormItemCount()-1] = len(metas)
			metas = append(metas, fieldMeta{field: field})
			if field.Hint != "" {
				form.AddTextView("", "  "+field.Hint, 0, hintLines(field.Hint), true, false)
			}
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

		// Look up the meta for the focused item, skipping hint TextViews.
		metaIdx, ok := itemToMeta[itemIdx]
		if !ok {
			return event // focus is on a hint TextView — pass through
		}
		meta := &metas[metaIdx]

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
				if n <= 5 {
					// Few options: cycle to next (no sub-modal, more fluent)
					meta.selectIdx = (meta.selectIdx + 1) % n
					values[meta.field.Key] = opts[meta.selectIdx].Value
					meta.inputField.SetText(selectLabel(meta.field, meta.selectIdx))
					return nil
				}
				// Many options: open sub-select modal
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
		// Validate required fields
		for _, f := range cfg.Fields {
			if f.Required {
				switch f.Type {
				case views.FieldBool:
					// A bool is always set (true/false) — skip validation
				case views.FieldMultiSelect:
					if len(multi[f.Key]) == 0 {
						s.ShowToast(fmt.Sprintf("Champ requis : %s", f.Label), ToastWarning)
						return
					}
				default:
					if values[f.Key] == "" {
						s.ShowToast(fmt.Sprintf("Champ requis : %s", f.Label), ToastWarning)
						return
					}
				}
			}
		}
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
		i18n.T("tui.shell.form_hints"),
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
		return i18n.T("tui.shell.enter_select")
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
	list.SetTitle(fmt.Sprintf("  %s · %s  ", title, i18n.T("tui.shell.select_hints")))
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
	list.SetTitle(fmt.Sprintf("  %s · %s  ", title, i18n.T("tui.shell.multiselect_hints")))
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

// NavigateTo navigates to a registered view by ID.
func (s *Shell) NavigateTo(viewID string) {
	s.router.NavigateTo(viewID)
}

// SetProjectMode activates or deactivates project mode.
// Passing nil deactivates project mode (hub mode).
func (s *Shell) SetProjectMode(project *views.ActiveProject) {
	s.activeProject = project
	if project != nil {
		s.router.NavigateTo("project.mode")
	} else {
		s.router.NavigateTo("home")
	}
}

// SetActiveProject sets the active project without triggering navigation.
// Use this to pre-set the project before NavigateHome is called.
func (s *Shell) SetActiveProject(project *views.ActiveProject) {
	s.activeProject = project
}

// ActiveProject returns the currently active project, or nil if in hub mode.
func (s *Shell) ActiveProject() *views.ActiveProject {
	return s.activeProject
}

// ─────────────────────────────────────────────────────────────────────────────
// Selection helpers
// ─────────────────────────────────────────────────────────────────────────────

// isInteractiveZone reports whether the cell at (x, y) belongs to a widget
// that handles its own mouse events. Mouse events in interactive zones are
// passed through to tview unmodified; events outside are routed to the
// SelectionManager for text selection.
//
// Interactive zones:
//   - Omnibar container (always, whether active or not)
//   - Suggestions overlay (when visible)
//   - Any inline-overlay page (modals, prompts) — the entire screen is treated
//     as interactive when a modal is open so that the user cannot accidentally
//     start a selection while interacting with the modal.
func (s *Shell) isInteractiveZone(x, y int) bool {
	// If any overlay modal is active, the whole screen is "interactive" —
	// pass all events to tview so the modal works normally.
	if s.pages.HasPage("inline-overlay") || s.pages.HasPage("sub-overlay") {
		return true
	}

	// Omnibar container
	ox, oy, ow, oh := s.omnibar.container.GetRect()
	if x >= ox && x < ox+ow && y >= oy && y < oy+oh {
		return true
	}

	// Suggestions list (when visible)
	if s.suggestionsShown {
		sx, sy, sw, sh := s.omnibar.SuggestionsList().GetRect()
		if x >= sx && x < sx+sw && y >= sy && y < sy+sh {
			return true
		}
	}

	return false
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

	// Esc: if a text selection is active, clear it first.
	// This avoids accidentally triggering navigation or modal-close while
	// the user is dismissing a selection.
	if event.Key() == tcell.KeyEscape && s.selection.IsActive() {
		s.selection.Clear()
		s.app.ForceDraw()
		return nil // consume the Esc — do not propagate
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

	// Ctrl+T: toggle between project mode and hub mode
	if event.Key() == tcell.KeyCtrlT {
		if s.activeProject != nil {
			s.SetProjectMode(nil)
		} else {
			s.router.NavigateTo("projects.list")
		}
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

	// Delegate to current view's key handler.
	// Note: Tab is intentionally NOT intercepted here — views that want to
	// use Tab for panel switching (e.g. MetricsView) capture it in HandleKey.
	// Views that don't handle Tab return the event, and tview's native focus
	// cycling takes over (Phase 2).
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
		case 'g':
			return tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)
		case 'G':
			return tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)
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
		SetLabelColor(theme.Accent).
		SetText(currentValue).
		SetFieldWidth(52).
		SetFieldBackgroundColor(theme.BgElement).
		SetFieldTextColor(theme.FgPrimary)
	input.SetBackgroundColor(theme.BgPanel)

	if masked {
		input.SetMaskCharacter('*')
	}

	// Hint bar at the bottom of the frame
	hint := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	hint.SetBackgroundColor(theme.BgPanel)
	hint.SetText(fmt.Sprintf("  "+i18n.T("tui.shell.input_hints"),
		theme.ColorTag(theme.AccentHex), theme.TagColor,
		theme.ColorTag(theme.TextMutedHex), theme.TagColor))

	// Frame with border + title — same design as showInlineSelect and ShowInlineForm.
	frame := tview.NewFlex().SetDirection(tview.FlexRow)
	frame.SetBackgroundColor(theme.BgPanel)
	frame.SetBorder(true)
	frame.SetBorderColor(theme.Accent)
	frame.SetTitle(fmt.Sprintf("  %s  ", title))
	frame.SetTitleColor(theme.Accent)
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

	// Width 64 matches ShowInlineForm for visual consistency across a flow.
	// No light-dismiss — accidental backdrop click would lose in-progress text.
	grid := s.overlayGrid(frame, []int{0, 64, 0}, []int{0, 7, 0}, theme.BgDimOverlay, "", nil)

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
	list.SetTitle(fmt.Sprintf(" %s · %s ", title, i18n.T("tui.shell.select_modal_hints")))
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
	list.SetTitle(fmt.Sprintf(" %s · %s ", title, i18n.T("tui.shell.multiselect_modal_hints")))
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
	frame.AddItem(buttons, 3, 0, false)
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
