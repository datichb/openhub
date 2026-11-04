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
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// ShellAccess: inline prompts (replace modal overlays)
// ─────────────────────────────────────────────────────────────────────────────

// ShowInputModal displays an inline input prompt in the content area.
// The content is temporarily replaced with a focused input field.
func (s *Shell) ShowInputModal(title, currentValue string, onConfirm func(newValue string)) {
	s.showInlineInput(title, currentValue, false, nil, onConfirm)
}

// ShowCheckedInputModal is ShowInputModal that refuses a value (views.CheckedInputShell):
// check returns the reason of the refusal, shown as a warning while the input
// stays open with its text.
func (s *Shell) ShowCheckedInputModal(title, currentValue string, check func(string) string, onConfirm func(string)) {
	s.showInlineInput(title, currentValue, false, check, onConfirm)
}

// ShowPasswordModal displays an inline masked input prompt.
func (s *Shell) ShowPasswordModal(title string, onConfirm func(value string)) {
	s.showInlineInput(title, "", true, nil, func(v string) {
		if onConfirm != nil {
			onConfirm(v)
		}
	})
}

// ShowSelectModal displays an inline select list (fzf-style).
func (s *Shell) ShowSelectModal(title string, options []views.SelectOption, currentValue string, onConfirm func(value string)) {
	s.showSelectList(s.openPrompt(), title, options, currentValue, onConfirm)
}

// ShowMultiSelectModal displays an inline multi-select list.
func (s *Shell) ShowMultiSelectModal(title string, options []views.SelectOption, selected []string, onConfirm func(selected []string)) {
	s.showCheckList(s.openPrompt(), title, options, selected, onConfirm)
}

// ShowScrollableModal displays an inline scrollable content view with action buttons.
// Shown while a window is already open (a reader opened from the decision
// card), it opens over it: closing it gives the window its focus back.
func (s *Shell) ShowScrollableModal(title, content string, actions []views.ModalAction) {
	textView := tview.NewTextView().SetText(content)
	styleScrollableText(textView)

	hints := i18n.T("tui.modal.hints.scrollable")
	if len(actions) == 0 {
		hints = i18n.T("tui.modal.hints.reader")
	}
	s.showModal(modalConfig{
		Title:        title,
		Content:      textView,
		Actions:      actions,
		Size:         modalSizeHint{ContentLines: countLines(content), HasButtons: len(actions) > 0, ButtonCount: len(actions), ProportionalWidth: -4},
		Hints:        hints,
		LightDismiss: true,
		Win:          s.openWindow(nil),
		FocusTarget:  textView,
		FocusAction:  min(len(actions), 1), // the primary action, if any
	})
}

func (s *Shell) showInlineInput(title, currentValue string, masked bool, check func(string) string, onConfirm func(string)) {
	w := s.openPrompt()
	input := tview.NewInputField().SetText(currentValue).SetFieldWidth(0)
	styleInputField(input)

	if masked {
		input.SetMaskCharacter('*')
	}

	// Enter confirms, Esc cancels; Tab and Shift+Tab would end the input
	// too and lose what was typed: they do nothing.
	input.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			if check != nil {
				if msg := check(input.GetText()); msg != "" {
					s.ShowToast(msg, ToastWarning)
					return
				}
			}
			s.answerWindow(w, func() {
				if onConfirm != nil {
					onConfirm(input.GetText())
				}
			})
		case tcell.KeyEscape:
			s.closeWindow(w)
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

	// Content-aware sizing: a share of the screen, wide enough for the title
	size := s.computeModalSize(modalSizeHint{
		WidthPct: theme.ModalWidthPct, FrameWidth: frameWidth(title, i18n.T("tui.modal.hints.input")), FixedHeight: theme.ModalMinHeight,
	})

	pad := strings.Repeat(" ", theme.ModalTitlePad)
	frame.SetTitle(fmt.Sprintf("%s%s%s", pad, fitTitle(title, size), pad))
	frame.SetTitleColor(theme.ActiveMode.Accent)

	frame.AddItem(container, 0, 1, true)

	// Hints line at the bottom
	hints := tview.NewTextView().
		SetText("  " + i18n.T("tui.modal.hints.input")).
		SetTextColor(theme.FgMuted).
		SetDynamicColors(false)
	hints.SetBackgroundColor(theme.BgModal)
	frame.AddItem(hints, 1, 0, false)

	grid := s.overlayGrid(frame, size.cols, size.rows, w.backdrop(), nil)

	s.pages.AddPage(w.page, grid, true, true)
	s.app.SetFocus(input)
}

// ─────────────────────────────────────────────────────────────────────────────
// Option lists: the select and the multi-select, as a window of their own or
// as a window over a form
// ─────────────────────────────────────────────────────────────────────────────

// newOptionList is the list of an option window, with the function that sets
// its rows: the rows keep their full text and are ellipsized to the width of
// the list when it is drawn (the window is as wide as the longest row, unless
// the screen is narrower).
func newOptionList() (list *tview.List, setRows func(rows []string)) {
	list = tview.NewList()
	styleSelectList(list)
	list.SetBorderPadding(0, 0, theme.ModalContentPadX, theme.ModalContentPadX)

	var rows []string
	fitted := -1 // width the rows were ellipsized to (-1: not yet)
	list.SetDrawFunc(func(_ tcell.Screen, x, y, width, height int) (int, int, int, int) {
		inner := width - 2*theme.ModalContentPadX
		if inner != fitted && len(rows) == list.GetItemCount() {
			fitted = inner
			for i, row := range rows {
				list.SetItemText(i, widgets.TruncateTagged(row, inner), "")
			}
		}
		return x + theme.ModalContentPadX, y, inner, height
	})
	setRows = func(texts []string) {
		rows, fitted = texts, -1
		list.Clear()
		for _, row := range texts {
			list.AddItem(row, "", 0, nil)
		}
	}
	return list, setRows
}

// rowsWidth is the width of the widest row.
func rowsWidth(rows []string) (width int) {
	for _, row := range rows {
		width = max(width, tview.TaggedStringWidth(row))
	}
	return width
}

// showList puts the list in a window, as wide as its rows need.
func (s *Shell) showList(w *window, title string, list *tview.List, rows []string, hints string) {
	s.showModal(modalConfig{
		Title:   title,
		Content: list,
		Size: modalSizeHint{
			OptionCount:  len(rows) + theme.ModalHintsHeight,
			ContentWidth: rowsWidth(rows), FrameWidth: frameWidth(title, hints),
		},
		Hints:        hints,
		LightDismiss: true,
		Win:          w,
		FocusTarget:  list,
	})
}

// showSelectList shows the options as a list: Enter picks the highlighted
// one, a click too, Esc closes the list.
func (s *Shell) showSelectList(w *window, title string, options []views.SelectOption, currentValue string, onConfirm func(string)) {
	list, setRows := newOptionList()
	rows := make([]string, len(options))
	currentIdx := 0
	for i, opt := range options {
		rows[i] = opt.Label
		if opt.Value == currentValue {
			currentIdx = i
		}
	}
	setRows(rows)
	list.SetCurrentItem(currentIdx)

	confirm := func(idx int) {
		if onConfirm != nil && idx >= 0 && idx < len(options) {
			onConfirm(options[idx].Value)
		}
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			idx := list.GetCurrentItem()
			s.answerWindow(w, func() { confirm(idx) })
			return nil
		}
		return event
	})

	// Click on item = immediate selection (macOS Finder / lazygit convention).
	s.listMouseSelect(list, w, confirm)
	s.showList(w, title, list, rows, i18n.T("tui.modal.hints.select"))
}

// checkList is the state of a multi-select list.
type checkList struct {
	list    *tview.List
	setRows func(rows []string)
	options []views.SelectOption
	checked []bool
}

func newCheckList(options []views.SelectOption, selected []string) *checkList {
	selectedSet := make(map[string]bool, len(selected))
	for _, v := range selected {
		selectedSet[v] = true
	}
	list, setRows := newOptionList()
	c := &checkList{list: list, setRows: setRows, options: options, checked: make([]bool, len(options))}
	for i, opt := range options {
		c.checked[i] = selectedSet[opt.Value]
	}
	c.render()
	return c
}

// rows are the texts of the rows: the label of each option after its box.
func (c *checkList) rows() []string {
	rows := make([]string, len(c.options))
	for i, opt := range c.options {
		prefix := theme.SymUnselectedMulti + " "
		if c.checked[i] {
			prefix = theme.SymSelectedMulti + " "
		}
		rows[i] = prefix + opt.Label
	}
	return rows
}

func (c *checkList) render() { c.setRows(c.rows()) }

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
func (s *Shell) showCheckList(w *window, title string, options []views.SelectOption, selected []string, onConfirm func([]string)) {
	c := newCheckList(options, selected)
	c.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter {
			result := c.values()
			s.answerWindow(w, func() {
				if onConfirm != nil {
					onConfirm(result)
				}
			})
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
	s.showList(w, title, c.list, c.rows(), i18n.T("tui.modal.hints.multiselect"))
}

// showSubSelect opens a list over a form. On confirmation or cancellation the
// focus returns to the form.
func (s *Shell) showSubSelect(title string, options []views.SelectOption, currentValue string, returnTo tview.Primitive, onConfirm func(string)) {
	s.showSelectList(s.openWindow(returnTo), title, options, currentValue, onConfirm)
}

// showSubMultiSelect opens a multi-select list over the form.
func (s *Shell) showSubMultiSelect(title string, options []views.SelectOption, selected []string, returnTo tview.Primitive, onConfirm func([]string)) {
	s.showCheckList(s.openWindow(returnTo), title, options, selected, onConfirm)
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
// When dismiss is set, clicking anywhere on the backdrop (outside the
// content cell) runs it — the "light dismiss" pattern familiar from macOS
// and web dialogs.
func (s *Shell) overlayGrid(content tview.Primitive, cols, rows []int, bg tcell.Color, dismiss func()) *tview.Grid {
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
	// - set:   light dismiss (close modal, restore focus)
	// - nil:   keep modal open but consume the click to prevent focus theft
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
				if action == tview.MouseLeftClick && dismiss != nil {
					dismiss()
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
func (s *Shell) listMouseSelect(list *tview.List, w *window, onSelect func(idx int)) {
	s.onListClick(list, func() {
		idx := list.GetCurrentItem()
		if idx >= 0 && s.windowOpen(w) {
			s.answerWindow(w, func() {
				if onSelect != nil {
					onSelect(idx)
				}
			})
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
