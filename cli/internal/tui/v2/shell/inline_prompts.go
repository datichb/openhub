package shell

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// ShellAccess: inline prompts (replace modal overlays)
// ─────────────────────────────────────────────────────────────────────────────

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
	s.showSelectList(s.inlineListWindow(), title, options, currentValue, onConfirm)
}

// ShowMultiSelectModal displays an inline multi-select list.
func (s *Shell) ShowMultiSelectModal(title string, options []views.SelectOption, selected []string, onConfirm func(selected []string)) {
	s.showCheckList(s.inlineListWindow(), title, options, selected, onConfirm)
}

// ShowScrollableModal displays an inline scrollable content view with action buttons.
func (s *Shell) ShowScrollableModal(title, content string, actions []views.ModalAction) {
	textView := tview.NewTextView().SetText(content)
	styleScrollableText(textView)

	s.showModal(modalConfig{
		Title:        title,
		Content:      textView,
		Actions:      actions,
		Size:         modalSizeHint{ContentLines: countLines(content), HasButtons: len(actions) > 0, ButtonCount: len(actions), ProportionalWidth: -4},
		Hints:        i18n.T("tui.modal.hints.scrollable"),
		LightDismiss: true,
		PageName:     "inline-overlay",
		FocusReturn:  s.content,
		FocusTarget:  textView,
		FocusAction:  min(len(actions), 1), // the primary action, if any
	})
}

func (s *Shell) showInlineInput(title, currentValue string, masked bool, onConfirm func(string)) {
	input := tview.NewInputField().SetText(currentValue).SetFieldWidth(0)
	styleInputField(input)

	if masked {
		input.SetMaskCharacter('*')
	}

	input.SetDoneFunc(func(key tcell.Key) {
		s.pages.RemovePage("inline-overlay")
		s.app.SetFocus(s.content)
		if key == tcell.KeyEnter && onConfirm != nil {
			onConfirm(input.GetText())
		}
	})

	// Wrap input in a container with vertical centering
	container := tview.NewFlex().SetDirection(tview.FlexRow)
	container.SetBackgroundColor(theme.BgModal)
	container.AddItem(tview.NewBox().SetBackgroundColor(theme.BgModal), 0, 1, false) // top spacer
	container.AddItem(input, 1, 0, true)
	container.AddItem(tview.NewBox().SetBackgroundColor(theme.BgModal), 0, 1, false) // bottom spacer

	// Build frame with rounded corners
	frame := tview.NewFlex().SetDirection(tview.FlexRow)
	frame.SetBackgroundColor(theme.BgModal)
	frame.SetBorder(true)
	frame.SetBorderColor(theme.ActiveMode.Accent)
	applyRoundedCorners(frame, theme.ActiveMode.Accent, theme.BgModal)

	pad := strings.Repeat(" ", theme.ModalTitlePad)
	frame.SetTitle(fmt.Sprintf("%s%s%s", pad, title, pad))
	frame.SetTitleColor(theme.ActiveMode.Accent)

	frame.AddItem(container, 0, 1, true)

	// Hints line at the bottom
	hints := tview.NewTextView().
		SetText("  " + i18n.T("tui.modal.hints.input")).
		SetTextColor(theme.FgMuted).
		SetDynamicColors(false)
	hints.SetBackgroundColor(theme.BgModal)
	frame.AddItem(hints, 1, 0, false)

	// Content-aware sizing
	size := s.computeModalSize(modalSizeHint{FixedWidth: theme.ModalMaxWidth, FixedHeight: theme.ModalMinHeight})
	grid := s.overlayGrid(frame, size.cols, size.rows, theme.BgDimOverlay, "", nil)

	s.pages.AddPage("inline-overlay", grid, true, true)
	s.app.SetFocus(input)
}

// ─────────────────────────────────────────────────────────────────────────────
// Option lists: the select and the multi-select, as a window of their own or
// as a sub-window of a form
// ─────────────────────────────────────────────────────────────────────────────

// listWindow says where an option list is shown and how it goes away.
type listWindow struct {
	page      string          // name of the overlay page
	bg        tcell.Color     // dimmed backdrop
	returnTo  tview.Primitive // focus when the list closes
	extraRows int             // rows of the window besides the options
}

// inlineListWindow is the list shown over the content.
func (s *Shell) inlineListWindow() listWindow {
	return listWindow{page: "inline-overlay", bg: theme.BgDimOverlay, returnTo: s.content, extraRows: theme.ModalHintsHeight}
}

// subListWindow is the list shown over a form, which gets the focus back.
func subListWindow(returnTo tview.Primitive) listWindow {
	return listWindow{page: "sub-overlay", bg: theme.BgDimSubOverlay, returnTo: returnTo}
}

func (s *Shell) closeList(w listWindow) {
	s.pages.RemovePage(w.page)
	s.app.SetFocus(w.returnTo)
}

// showList puts the list over the screen, sized for the options.
func (s *Shell) showList(w listWindow, list *tview.List, optionCount int) {
	size := s.computeModalSize(modalSizeHint{OptionCount: optionCount + w.extraRows, ProportionalWidth: -3})
	grid := s.overlayGrid(list, size.cols, size.rows, w.bg, w.page, w.returnTo)
	s.pages.AddPage(w.page, grid, true, true)
	s.app.SetFocus(list)
}

// newTitledList is the bordered list of an option window.
func newTitledList(title string) *tview.List {
	list := tview.NewList()
	styleSelectList(list)
	list.SetBorder(true)
	list.SetBorderColor(theme.ActiveMode.Accent)
	list.SetBorderPadding(0, 0, theme.ModalContentPadX, theme.ModalContentPadX)
	applyRoundedCornersBox(list.Box, theme.ActiveMode.Accent, theme.BgModal)

	pad := strings.Repeat(" ", theme.ModalTitlePad)
	list.SetTitle(fmt.Sprintf("%s%s%s", pad, title, pad))
	list.SetTitleColor(theme.ActiveMode.Accent)
	return list
}

// showSelectList shows the options as a list: Enter picks the highlighted
// one, a click too, Esc closes the list.
func (s *Shell) showSelectList(w listWindow, title string, options []views.SelectOption, currentValue string, onConfirm func(string)) {
	list := newTitledList(title)
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
			s.closeList(w)
			return nil
		case tcell.KeyEnter:
			idx := list.GetCurrentItem()
			s.closeList(w)
			confirm(idx)
			return nil
		}
		return event
	})

	// Click on item = immediate selection (macOS Finder / lazygit convention).
	s.listMouseSelect(list, w.page, w.returnTo, confirm)
	s.showList(w, list, len(options))
}

// checkList is the state of a multi-select list.
type checkList struct {
	list    *tview.List
	options []views.SelectOption
	checked []bool
}

func newCheckList(title string, options []views.SelectOption, selected []string) *checkList {
	selectedSet := make(map[string]bool, len(selected))
	for _, v := range selected {
		selectedSet[v] = true
	}
	c := &checkList{list: newTitledList(title), options: options, checked: make([]bool, len(options))}
	for i, opt := range options {
		c.checked[i] = selectedSet[opt.Value]
	}
	c.render()
	return c
}

func (c *checkList) render() {
	c.list.Clear()
	for i, opt := range c.options {
		prefix := theme.SymUnselectedMulti + " "
		if c.checked[i] {
			prefix = theme.SymSelectedMulti + " "
		}
		c.list.AddItem(prefix+opt.Label, "", 0, nil)
	}
}

func (c *checkList) toggle(idx int) {
	if idx >= 0 && idx < len(c.checked) {
		c.checked[idx] = !c.checked[idx]
		c.render()
		c.list.SetCurrentItem(idx)
	}
}

// values are the values of the options checked.
func (c *checkList) values() (result []string) {
	for i, opt := range c.options {
		if c.checked[i] {
			result = append(result, opt.Value)
		}
	}
	return result
}

// showCheckList shows the options as a list to check: space (or a click)
// toggles the highlighted one, Enter validates, Esc closes the list.
func (s *Shell) showCheckList(w listWindow, title string, options []views.SelectOption, selected []string, onConfirm func([]string)) {
	c := newCheckList(title, options, selected)
	c.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			s.closeList(w)
			return nil
		case tcell.KeyEnter:
			result := c.values()
			s.closeList(w)
			if onConfirm != nil {
				onConfirm(result)
			}
			return nil
		}
		if keys.Toggle.Matches(event) {
			c.toggle(c.list.GetCurrentItem())
			return nil
		}
		return event
	})

	// Click on item = toggle its checked state (standard checkbox UX).
	s.listMouseToggle(c.list, c.toggle)
	s.showList(w, c.list, len(options))
}

// showSubSelect opens a list sub-modal on top of an existing overlay (the form).
// On confirmation or cancellation the focus returns to the form.
func (s *Shell) showSubSelect(title string, options []views.SelectOption, currentValue string, returnTo tview.Primitive, onConfirm func(string)) {
	s.showSelectList(subListWindow(returnTo), title, options, currentValue, onConfirm)
}

// showSubMultiSelect opens a multi-select list sub-modal on top of the form.
func (s *Shell) showSubMultiSelect(title string, options []views.SelectOption, selected []string, returnTo tview.Primitive, onConfirm func([]string)) {
	s.showCheckList(subListWindow(returnTo), title, options, selected, onConfirm)
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
	s.onListClick(list, func() {
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
}

// listMouseToggle wires a single-click-to-toggle behaviour on a tview.List
// used for multi-select overlays. A click flips the checked state of the
// clicked item without closing the modal — matching standard checkbox UX.
func (s *Shell) listMouseToggle(list *tview.List, toggle func(idx int)) {
	s.onListClick(list, func() {
		idx := list.GetCurrentItem()
		if idx >= 0 && toggle != nil {
			toggle(idx)
		}
	})
}

// onListClick runs fn on the event loop after a click on the list, once
// tview has moved the cursor to the clicked item. The mouse capture runs on
// the tview event loop, where QueueUpdateDraw would deadlock (the unbuffered
// done-channel blocks forever): fn is queued from a goroutine, which blocks
// outside the event loop.
func (s *Shell) onListClick(list *tview.List, fn func()) {
	list.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftClick {
			safego.Go(func() { s.app.QueueUpdateDraw(fn) })
		}
		return action, event
	})
}
