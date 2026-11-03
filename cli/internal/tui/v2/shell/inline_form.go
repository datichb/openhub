package shell

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

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
	f := newInlineForm(s, cfg)
	f.addFields()
	f.form.SetInputCapture(f.captureFieldKeys)
	f.form.SetCancelFunc(f.cancel)
	frame := f.buildFrame()
	frame.SetInputCapture(f.captureFocusCycle)
	f.show(frame)
}

// maxCycledOptions is the largest select that Enter cycles through; beyond,
// Enter opens the list of the options.
const maxCycledOptions = 5

// formFieldMeta is the state of an interactive field that the key handler
// needs.
type formFieldMeta struct {
	field      *views.FormField
	inputField *tview.InputField // non-nil for FieldSelect / FieldMultiSelect
	selectIdx  int               // current option index (FieldSelect only)
}

// inlineForm is one form being shown by ShowInlineForm.
type inlineForm struct {
	s      *Shell
	win    *window
	cfg    views.InlineFormConfig
	form   *tview.Form
	values map[string]string
	multi  map[string][]string

	metas []*formFieldMeta
	// itemToMeta maps a form item index → metas index. AddTextView (hints and
	// validation messages) inserts non-interactive items into the form,
	// shifting the item indices, so metas[itemIdx] would be wrong: only the
	// interactive fields are registered.
	itemToMeta map[int]int

	// buttons are the focusables of the button bar (Confirm, Cancel).
	buttons []tview.Primitive
}

func newInlineForm(s *Shell, cfg views.InlineFormConfig) *inlineForm {
	form := tview.NewForm()
	form.SetBackgroundColor(theme.BgModal)
	form.SetLabelColor(theme.FgPrimary)
	form.SetFieldBackgroundColor(theme.BgModalField)
	form.SetFieldTextColor(theme.FgPrimary)
	form.SetBorder(false) // the outer frame has the border
	return &inlineForm{
		s: s, win: s.openPrompt(), cfg: cfg, form: form,
		values:     make(map[string]string),
		multi:      make(map[string][]string),
		itemToMeta: make(map[int]int),
	}
}

// ── Fields ───────────────────────────────────────────────────────────────────

// addFields adds the fields whose condition is met.
func (f *inlineForm) addFields() {
	for i := range f.cfg.Fields {
		field := &f.cfg.Fields[i]
		if field.Conditional != nil && !field.Conditional(f.values) {
			continue
		}
		switch field.Type {
		case views.FieldSelect:
			f.addSelect(field)
		case views.FieldMultiSelect:
			f.addMultiSelect(field)
		case views.FieldText:
			f.addText(field)
		case views.FieldPassword:
			f.addPassword(field)
		case views.FieldBool:
			f.addBool(field)
		}
	}
}

// register records the interactive field that was just added to the form.
func (f *inlineForm) register(meta *formFieldMeta) {
	f.itemToMeta[f.form.GetFormItemCount()-1] = len(f.metas)
	f.metas = append(f.metas, meta)
}

// addHint adds the hint of a field under it: 2 lines if it is longer than
// 50 characters, else 1.
func (f *inlineForm) addHint(field *views.FormField) {
	if field.Hint == "" {
		return
	}
	lines := 1
	if len("  "+field.Hint) > 50 {
		lines = 2
	}
	f.form.AddTextView("", "  "+field.Hint, 0, lines, true, false)
}

// readOnlyInput is the field that shows the choice of a select: it takes no
// typing.
func readOnlyInput(label, text string, width int) *tview.InputField {
	inp := tview.NewInputField().
		SetLabel(label).
		SetText(text).
		SetFieldWidth(width).
		SetAcceptanceFunc(func(string, rune) bool { return false })
	inp.SetBackgroundColor(theme.BgModal)
	inp.SetFieldBackgroundColor(theme.BgModalField)
	inp.SetFieldTextColor(theme.FgPrimary)
	inp.SetLabelColor(theme.FgPrimary)
	return inp
}

func (f *inlineForm) addSelect(field *views.FormField) {
	idx := 0
	for j, opt := range field.Options {
		if opt.Value == field.Default {
			idx = j
			break
		}
	}
	f.values[field.Key] = field.Default
	inp := readOnlyInput(field.Label, selectLabel(field, idx), 24)
	f.form.AddFormItem(inp)
	f.register(&formFieldMeta{field: field, inputField: inp, selectIdx: idx})
	f.addHint(field)
}

func (f *inlineForm) addMultiSelect(field *views.FormField) {
	if field.DefaultMulti != nil {
		f.multi[field.Key] = append([]string{}, field.DefaultMulti...)
	}
	inp := readOnlyInput(field.Label, multiSummary(f.multi[field.Key], field.Options), 30)
	f.form.AddFormItem(inp)
	f.register(&formFieldMeta{field: field, inputField: inp})
	f.addHint(field)
}

func (f *inlineForm) addText(field *views.FormField) {
	f.values[field.Key] = field.Default
	var check *tview.TextView
	f.form.AddInputField(field.Label, field.Default, 40, nil,
		func(text string) {
			f.values[field.Key] = text
			if check != nil {
				check.SetText(views.ValidationText(field.Validate(text)))
			}
		})
	f.register(&formFieldMeta{field: field})
	if field.Validate != nil {
		// The message of the check, under the field, as it is typed.
		f.form.AddTextView("", views.ValidationText(field.Validate(field.Default)), 0, 2, true, false)
		check, _ = f.form.GetFormItem(f.form.GetFormItemCount() - 1).(*tview.TextView)
	}
	f.addHint(field)
}

func (f *inlineForm) addPassword(field *views.FormField) {
	f.values[field.Key] = field.Default
	f.form.AddPasswordField(field.Label, field.Default, 40, '*',
		func(text string) { f.values[field.Key] = text })
	f.register(&formFieldMeta{field: field})
	f.addHint(field)
}

func (f *inlineForm) addBool(field *views.FormField) {
	f.values[field.Key] = field.Default
	f.form.AddCheckbox(field.Label, field.Default == "true", func(checked bool) {
		if checked {
			f.values[field.Key] = "true"
		} else {
			f.values[field.Key] = "false"
		}
	})
	f.register(&formFieldMeta{field: field})
	f.addHint(field)
}

// ── Keys of the fields ───────────────────────────────────────────────────────

// captureFieldKeys handles ↑/↓ (previous and next field) and ←/→/Enter on the
// Select and MultiSelect fields.
func (f *inlineForm) captureFieldKeys(event *tcell.EventKey) *tcell.EventKey {
	// A list open over the form (select/multiselect) → don't intercept anything.
	if f.s.hasWindowOver(f.win) {
		return event
	}
	switch event.Key() {
	case tcell.KeyUp:
		return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
	case tcell.KeyDown:
		return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
	}

	// The meta of the focused item; a hint or a message has none.
	itemIdx, _ := f.form.GetFocusedItemIndex()
	metaIdx, ok := f.itemToMeta[itemIdx]
	if !ok {
		return event
	}
	meta := f.metas[metaIdx]
	switch meta.field.Type {
	case views.FieldSelect:
		return f.selectKey(meta, event)
	case views.FieldMultiSelect:
		return f.multiSelectKey(meta, event)
	}
	return event
}

func (f *inlineForm) selectKey(meta *formFieldMeta, event *tcell.EventKey) *tcell.EventKey {
	n := len(meta.field.Options)
	if n == 0 {
		return nil
	}
	switch event.Key() {
	case tcell.KeyLeft:
		f.stepSelect(meta, -1)
		return nil
	case tcell.KeyRight:
		f.stepSelect(meta, 1)
		return nil
	case tcell.KeyEnter:
		if n <= maxCycledOptions {
			// Few options: cycle to next (no sub-modal, more fluent)
			f.stepSelect(meta, 1)
			return nil
		}
		f.openSelectList(meta)
		return nil
	}
	return event
}

// stepSelect moves the choice of a select by delta options, wrapping.
func (f *inlineForm) stepSelect(meta *formFieldMeta, delta int) {
	n := len(meta.field.Options)
	meta.selectIdx = (meta.selectIdx + delta + n) % n
	f.values[meta.field.Key] = meta.field.Options[meta.selectIdx].Value
	meta.inputField.SetText(selectLabel(meta.field, meta.selectIdx))
}

// openSelectList lets the user pick the option from a list (many options).
func (f *inlineForm) openSelectList(meta *formFieldMeta) {
	field := meta.field
	f.s.showSubSelect(field.Label, field.Options, f.values[field.Key], f.form, func(value string) {
		for j, opt := range field.Options {
			if opt.Value == value {
				meta.selectIdx = j
				break
			}
		}
		f.values[field.Key] = value
		meta.inputField.SetText(selectLabel(field, meta.selectIdx))
	})
}

func (f *inlineForm) multiSelectKey(meta *formFieldMeta, event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyEnter {
		return event
	}
	field := meta.field
	f.s.showSubMultiSelect(field.Label, field.Options, f.multi[field.Key], f.form, func(selected []string) {
		f.multi[field.Key] = selected
		meta.inputField.SetText(multiSummary(selected, field.Options))
	})
	return nil
}

// ── Submit, cancel ───────────────────────────────────────────────────────────

// submit closes the form (and the window it was opened from) and hands the
// values over; a refused submission (required field, invalid value) shows a
// warning and keeps it open.
func (f *inlineForm) submit() {
	if !f.valid() {
		return
	}
	f.s.answerWindow(f.win, func() {
		if f.cfg.OnSubmit != nil {
			f.cfg.OnSubmit(f.values, f.multi)
		}
	})
}

// cancel closes the form and gives the window it was opened from back.
func (f *inlineForm) cancel() {
	f.s.closeWindow(f.win)
	if f.cfg.OnCancel != nil {
		f.cfg.OnCancel()
	}
}

// valid checks the required fields, the values and the form as a whole; it
// warns about the first one refused.
func (f *inlineForm) valid() bool {
	for _, field := range f.cfg.Fields {
		if field.Required && f.isEmpty(field) {
			f.s.ShowToast(i18n.Tf("tui.shell.field_required", field.Label), ToastWarning)
			return false
		}
		if field.Validate != nil && (field.Conditional == nil || field.Conditional(f.values)) {
			if msg, invalid := field.Validate(f.values[field.Key]); invalid {
				f.s.ShowToast(msg, ToastWarning)
				return false
			}
		}
	}
	if f.cfg.Check != nil {
		if msg := f.cfg.Check(f.values, f.multi); msg != "" {
			f.s.ShowToast(msg, ToastWarning)
			return false
		}
	}
	return true
}

// isEmpty reports whether a required field has no value. A bool is always
// set (true/false).
func (f *inlineForm) isEmpty(field views.FormField) bool {
	switch field.Type {
	case views.FieldBool:
		return false
	case views.FieldMultiSelect:
		return len(f.multi[field.Key]) == 0
	default:
		return f.values[field.Key] == ""
	}
}

// ── Layout ───────────────────────────────────────────────────────────────────

// buildFrame wraps the form in a bordered frame with the hints line, a
// separator and the button bar.
func (f *inlineForm) buildFrame() *tview.Flex {
	frame := tview.NewFlex().SetDirection(tview.FlexRow)
	frame.SetBackgroundColor(theme.BgModal)
	frame.SetBorder(true)
	frame.SetBorderColor(theme.ActiveMode.Accent)
	applyRoundedCorners(frame, theme.ActiveMode.Accent, theme.BgModal)

	pad := strings.Repeat(" ", theme.ModalTitlePad)
	frame.SetTitle(fmt.Sprintf("%s%s%s", pad, f.cfg.Title, pad))
	frame.SetTitleColor(theme.ActiveMode.Accent)

	frame.AddItem(f.form, 0, 1, true)
	frame.AddItem(formHints(), 1, 0, false)
	frame.AddItem(newSeparator(), 1, 0, false)

	// The callbacks dismiss the form themselves: a refused submission keeps
	// it open.
	buttonBar, buttons := buildButtonBar(
		[]views.ModalAction{
			{Label: i18n.T("tui.modal.btn.confirm"), Callback: f.submit},
			{Label: i18n.T("tui.modal.btn.cancel"), Callback: f.cancel},
		},
		func() {},
	)
	f.buttons = buttons
	frame.AddItem(buttonBar, 1, 0, false)
	frame.AddItem(tview.NewBox().SetBackgroundColor(theme.BgModal), 1, 0, false) // bottom padding
	return frame
}

// formHints is the line of keys above the separator.
func formHints() *tview.TextView {
	hints := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)
	hints.SetBackgroundColor(theme.BgModal)
	accent := theme.ColorTag(theme.ActiveMode.AccentHex)
	hints.SetText(fmt.Sprintf(
		"  "+i18n.T("tui.shell.form_hints"),
		accent, theme.TagColor,
		accent, theme.TagColor,
		accent, theme.TagColor,
		accent, theme.TagColor,
	))
	return hints
}

// show puts the frame over the content, sized for the visible fields.
func (f *inlineForm) show(frame *tview.Flex) {
	size := f.s.computeModalSize(modalSizeHint{FieldCount: f.visibleRows(), FixedWidth: theme.ModalMaxWidth})
	grid := f.s.overlayGrid(frame, size.cols, size.rows, f.win.backdrop(), nil)
	f.s.pages.AddPage(f.win.page, grid, true, true)
	f.s.app.SetFocus(f.form)
}

// visibleRows counts the rows of the fields shown, with the room for the
// message under the fields that have a check.
func (f *inlineForm) visibleRows() int {
	rows := 0
	for _, field := range f.cfg.Fields {
		if field.Conditional == nil || field.Conditional(f.values) {
			rows++
			if field.Validate != nil {
				rows++
			}
		}
	}
	return rows
}

// ── Focus cycling ────────────────────────────────────────────────────────────

// captureFocusCycle makes Tab / Shift+Tab go through every field then every
// button (the focused element is read from the app: mouse-safe), ←/→ move
// between the buttons, ↑ from a button goes to the last field and Esc cancels.
func (f *inlineForm) captureFocusCycle(event *tcell.EventKey) *tcell.EventKey {
	fields := f.focusStops()
	total := len(fields) + len(f.buttons)
	cur := f.currentStop(fields)
	onButton := cur >= len(fields)
	switch event.Key() {
	case tcell.KeyTab:
		f.focusStop(fields, (cur+1)%total)
		return nil
	case tcell.KeyBacktab:
		if cur < 0 {
			cur = 0
		}
		f.focusStop(fields, (cur-1+total)%total)
		return nil
	case tcell.KeyLeft, tcell.KeyRight:
		if onButton {
			step := 1
			if event.Key() == tcell.KeyLeft {
				step = -1
			}
			btn := (cur - len(fields) + step + len(f.buttons)) % len(f.buttons)
			f.focusStop(fields, len(fields)+btn)
			return nil
		}
	case tcell.KeyUp:
		if onButton && len(fields) > 0 {
			f.focusStop(fields, len(fields)-1)
			return nil
		}
	case tcell.KeyEscape:
		f.cancel()
		return nil
	}
	return event
}

// focusStops are the indices of the focusable form items (not the messages
// and hints), recomputed on each key: conditional fields come and go.
func (f *inlineForm) focusStops() (fields []int) {
	for i := 0; i < f.form.GetFormItemCount(); i++ {
		if _, msg := f.form.GetFormItem(i).(*tview.TextView); !msg {
			fields = append(fields, i)
		}
	}
	return fields
}

// focusStop focuses the k-th stop: the fields first, then the buttons.
func (f *inlineForm) focusStop(fields []int, k int) {
	if k < len(fields) {
		f.form.SetFocus(fields[k])
		f.s.app.SetFocus(f.form)
		return
	}
	f.s.app.SetFocus(f.buttons[k-len(fields)])
}

// currentStop is the stop that has the focus, -1 when none.
func (f *inlineForm) currentStop(fields []int) int {
	for k, i := range fields {
		if f.form.GetFormItem(i).HasFocus() {
			return k
		}
	}
	for j, b := range f.buttons {
		if b.HasFocus() {
			return len(fields) + j
		}
	}
	return -1
}

// selectLabel formats the display text for a FieldSelect input: "◂ Label ▸".
func selectLabel(field *views.FormField, idx int) string {
	if idx < 0 || idx >= len(field.Options) {
		return theme.IconChoicePrev + " — " + theme.IconChoiceNext
	}
	return theme.IconChoicePrev + " " + field.Options[idx].Label + " " + theme.IconChoiceNext
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
	const maxDisplay = 2
	var summary string
	if len(labels) <= maxDisplay {
		summary = strings.Join(labels, ", ")
	} else {
		summary = strings.Join(labels[:maxDisplay], ", ") + fmt.Sprintf(", +%d", len(labels)-maxDisplay)
	}
	return summary + " ⏎"
}
