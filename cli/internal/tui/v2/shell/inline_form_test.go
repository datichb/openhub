package shell

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// These tests pin the behaviour of ShowInlineForm (typing, fields of every
// type, validation, Esc, buttons) before its code is split.

type formResult struct {
	values map[string]string
	multi  map[string][]string
}

// formShell shows an inline form over a plain home view. submitted receives
// the values of the form, cancelled counts the cancellations.
type formShell struct {
	*simShell
	submitted chan formResult
	cancelled chan struct{}
}

func openForm(t *testing.T, title string, fields ...views.FormField) *formShell {
	t.Helper()
	s := newKeyShell(t, &keyView{id: "home", focusList: true})
	f := &formShell{simShell: s, submitted: make(chan formResult, 4), cancelled: make(chan struct{}, 4)}
	s.loop(func() {
		s.ShowInlineForm(views.InlineFormConfig{Title: title, Fields: fields,
			OnSubmit: func(v map[string]string, m map[string][]string) { f.submitted <- formResult{v, m} },
			OnCancel: func() { f.cancelled <- struct{}{} }})
	})
	return f
}

func (f *formShell) press(k tcell.Key) { f.t.Helper(); f.key(k, 0, tcell.ModNone) }

func (f *formShell) typeText(text string) {
	f.t.Helper()
	for _, r := range text {
		f.key(tcell.KeyRune, r, tcell.ModNone)
	}
}

func (f *formShell) formShown() (shown bool) {
	f.loop(func() { shown = f.pages.HasPage("inline-overlay") })
	return shown
}

func (f *formShell) subShown() (shown bool) {
	f.loop(func() { shown = f.pages.HasPage("sub-overlay") })
	return shown
}

// confirm tabs from the last of n fields to « Confirm » and presses it.
func (f *formShell) confirm() {
	f.t.Helper()
	f.press(tcell.KeyTab)
	f.press(tcell.KeyEnter)
}

func (f *formShell) result() formResult {
	f.t.Helper()
	select {
	case r := <-f.submitted:
		return r
	case <-time.After(2 * time.Second):
		f.t.Fatal("the form was not submitted")
	}
	return formResult{}
}

func (f *formShell) notSubmitted() {
	f.t.Helper()
	select {
	case r := <-f.submitted:
		f.t.Fatalf("form submitted: %v", r.values)
	case <-time.After(30 * time.Millisecond):
	}
}

func options(values ...string) []views.SelectOption {
	out := make([]views.SelectOption, len(values))
	for i, v := range values {
		out[i] = views.SelectOption{Value: v, Label: "opt-" + v}
	}
	return out
}

// Text, password and bool fields: what is typed, and the defaults, reach
// OnSubmit; the form closes and the focus goes back to the content.
func TestInlineFormTypedValuesAreSubmitted(t *testing.T) {
	f := openForm(t, "Account",
		views.FormField{Key: "name", Label: "Name", Type: views.FieldText, Default: "x"},
		views.FormField{Key: "secret", Label: "Secret", Type: views.FieldPassword},
		views.FormField{Key: "flag", Label: "Flag", Type: views.FieldBool},
		views.FormField{Key: "kept", Label: "Kept", Type: views.FieldBool, Default: "true"})
	assert.Contains(t, f.screenText(), "Account")

	f.typeText("yz") // appended to the default
	f.press(tcell.KeyDown)
	f.typeText("pw")
	f.press(tcell.KeyDown)
	f.key(tcell.KeyRune, ' ', tcell.ModNone) // checks « Flag »
	f.press(tcell.KeyDown)
	f.press(tcell.KeyUp) // back on « Flag »
	f.press(tcell.KeyDown)
	// on « Kept »; the Tab of confirm() goes to the button
	f.confirm()

	r := f.result()
	assert.Equal(t, map[string]string{"name": "xyz", "secret": "pw", "flag": "true", "kept": "true"}, r.values)
	assert.Empty(t, r.multi)
	assert.False(t, f.formShown(), "the form closes")
	var focus tview.Primitive
	f.loop(func() { focus = f.app.GetFocus() })
	_, onList := focus.(*widgets.SectionedList)
	assert.True(t, onList, "focus back on the view")
}

// Esc cancels: OnCancel runs once, nothing is submitted.
func TestInlineFormEscCancels(t *testing.T) {
	f := openForm(t, "Cancel me", views.FormField{Key: "name", Label: "Name", Type: views.FieldText})
	f.typeText("abc")
	f.press(tcell.KeyEscape)
	select {
	case <-f.cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("OnCancel not called")
	}
	assert.False(t, f.formShown())
	f.notSubmitted()
	assert.Empty(t, f.cancelled, "cancelled once")
}

// The buttons: Tab goes field → « Confirm » → « Cancel » → field, ←/→ move
// between the buttons, and « Cancel » closes the form.
func TestInlineFormButtons(t *testing.T) {
	f := openForm(t, "Buttons", views.FormField{Key: "name", Label: "Name", Type: views.FieldText})
	focusedButton := func() (label string) {
		f.loop(func() { label = buttonLabelOf(f.app.GetFocus()) })
		return label
	}
	confirm, cancel := i18n.T("tui.modal.btn.confirm"), i18n.T("tui.modal.btn.cancel")
	assert.Equal(t, "", focusedButton(), "starts on the field")
	f.press(tcell.KeyTab)
	assert.Equal(t, confirm, focusedButton())
	f.press(tcell.KeyRight)
	assert.Equal(t, cancel, focusedButton())
	f.press(tcell.KeyRight)
	assert.Equal(t, confirm, focusedButton(), "wraps")
	f.press(tcell.KeyLeft)
	assert.Equal(t, cancel, focusedButton())
	f.press(tcell.KeyTab)
	assert.Equal(t, "", focusedButton(), "Tab wraps to the field")
	f.press(tcell.KeyBacktab)
	assert.Equal(t, cancel, focusedButton(), "Shift+Tab wraps to the last button")
	f.press(tcell.KeyUp)
	assert.Equal(t, "", focusedButton(), "↑ from a button goes to the last field")
	f.press(tcell.KeyTab)
	f.press(tcell.KeyTab)
	f.press(tcell.KeyEnter) // « Cancel »
	select {
	case <-f.cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("OnCancel not called")
	}
	assert.False(t, f.formShown())
}

// A required field left empty is refused with a warning, the form stays;
// once filled it goes through. A bool never counts as empty.
func TestInlineFormRequiredField(t *testing.T) {
	f := openForm(t, "Required",
		views.FormField{Key: "name", Label: "Name", Type: views.FieldText, Required: true},
		views.FormField{Key: "flag", Label: "Flag", Type: views.FieldBool, Required: true})
	f.press(tcell.KeyDown)
	f.confirm()
	f.notSubmitted()
	assert.True(t, f.formShown(), "the form stays")
	want := i18n.Tf("tui.shell.field_required", "Name")
	require.Eventually(t, func() bool { return strings.Contains(f.screenText(), want) }, 2*time.Second, 10*time.Millisecond, "warning toast")

	f.press(tcell.KeyUp) // from « Confirm » to the last field
	f.press(tcell.KeyUp)
	f.typeText("ok")
	f.press(tcell.KeyDown)
	f.confirm()
	assert.Equal(t, "ok", f.result().values["name"])
	assert.False(t, f.formShown())
}

// A refused value (Validate) is not submitted and its message is shown.
func TestInlineFormInvalidValueIsRefused(t *testing.T) {
	f := openForm(t, "Check", views.FormField{Key: "n", Label: "N", Type: views.FieldText, Default: "no",
		Validate: func(v string) (string, bool) { return "must not be " + v, v == "no" }})
	f.confirm()
	f.notSubmitted()
	assert.True(t, f.formShown())
	require.Eventually(t, func() bool { return strings.Contains(f.screenText(), "must not be no") }, 2*time.Second, 10*time.Millisecond)
}

// Ctrl+S is not a key of the inline form: it neither submits nor closes it.
func TestInlineFormCtrlSDoesNothing(t *testing.T) {
	f := openForm(t, "Save", views.FormField{Key: "name", Label: "Name", Type: views.FieldText, Default: "v"})
	f.press(tcell.KeyCtrlS)
	f.notSubmitted()
	assert.True(t, f.formShown())
	assert.Empty(t, f.cancelled)
}

// Select field: ←/→ cycle (wrapping), Enter cycles when there are few
// options, and the hint under a field does not shift the keys of the next one.
func TestInlineFormSelectCycles(t *testing.T) {
	f := openForm(t, "Select",
		views.FormField{Key: "note", Label: "Note", Type: views.FieldText, Hint: "a hint under the field"},
		views.FormField{Key: "mode", Label: "Mode", Type: views.FieldSelect, Options: options("a", "b", "c"), Default: "b", Hint: "another hint"})
	f.press(tcell.KeyDown)
	assert.Contains(t, f.screenText(), "opt-b")
	f.press(tcell.KeyRight) // c
	f.press(tcell.KeyRight) // a (wraps)
	f.press(tcell.KeyLeft)  // c
	f.press(tcell.KeyLeft)  // b
	f.press(tcell.KeyEnter) // c (few options: cycles)
	f.press(tcell.KeyEnter) // a
	f.press(tcell.KeyLeft)  // c
	f.confirm()
	r := f.result()
	assert.Equal(t, "c", r.values["mode"])
	assert.Equal(t, "", r.values["note"])
}

// Select field with many options: Enter opens a list; Enter in it picks the
// highlighted option; Esc closes it and keeps the value.
func TestInlineFormSelectSubList(t *testing.T) {
	f := openForm(t, "Many", views.FormField{Key: "mode", Label: "Mode", Type: views.FieldSelect,
		Options: options("a", "b", "c", "d", "e", "f", "g"), Default: "c"})
	f.press(tcell.KeyEnter)
	assert.True(t, f.subShown(), "the list opens")
	f.press(tcell.KeyEscape)
	assert.False(t, f.subShown())
	assert.True(t, f.formShown(), "Esc closes the list only")
	f.notSubmitted()

	f.press(tcell.KeyEnter)
	f.press(tcell.KeyDown) // d
	f.press(tcell.KeyDown) // e
	f.press(tcell.KeyEnter)
	assert.False(t, f.subShown())
	assert.True(t, f.formShown())
	assert.Contains(t, f.screenText(), "opt-e")
	f.confirm()
	assert.Equal(t, "e", f.result().values["mode"])
}

// Multi-select field: Enter opens the list, space toggles (x does not: it is
// the Archive of a list of objects), Enter validates; the summary shows the
// choice; a required one cannot stay empty.
func TestInlineFormMultiSelect(t *testing.T) {
	f := openForm(t, "Multi", views.FormField{Key: "tags", Label: "Tags", Type: views.FieldMultiSelect,
		Options: options("a", "b", "c"), DefaultMulti: []string{"c"}, Required: true})
	assert.Contains(t, f.screenText(), "opt-c")
	f.press(tcell.KeyEnter)
	assert.True(t, f.subShown())
	f.key(tcell.KeyRune, ' ', tcell.ModNone) // a
	f.press(tcell.KeyDown)
	f.key(tcell.KeyRune, 'x', tcell.ModNone) // nothing: x no longer ticks
	f.key(tcell.KeyRune, ' ', tcell.ModNone) // b
	f.press(tcell.KeyEnter)
	assert.False(t, f.subShown())
	require.Eventually(t, func() bool { return strings.Contains(f.screenText(), "opt-a, opt-b, +1") }, 2*time.Second, 10*time.Millisecond, "summary")
	f.confirm()
	r := f.result()
	assert.Equal(t, []string{"a", "b", "c"}, r.multi["tags"])
}

func TestInlineFormMultiSelectRequired(t *testing.T) {
	f := openForm(t, "Multi", views.FormField{Key: "tags", Label: "Tags", Type: views.FieldMultiSelect,
		Options: options("a", "b"), Required: true})
	f.confirm()
	f.notSubmitted()
	assert.True(t, f.formShown())
	require.Eventually(t, func() bool { return strings.Contains(f.screenText(), i18n.Tf("tui.shell.field_required", "Tags")) }, 2*time.Second, 10*time.Millisecond)
}

// A field whose condition is not met when the form opens is not shown and
// not submitted.
func TestInlineFormConditionalField(t *testing.T) {
	cond := func(want string) func(map[string]string) bool {
		return func(v map[string]string) bool { return v["kind"] == want }
	}
	f := openForm(t, "Cond",
		views.FormField{Key: "kind", Label: "Kind", Type: views.FieldText, Default: "basic"},
		views.FormField{Key: "extra", Label: "Extra", Type: views.FieldText, Default: "e", Conditional: cond("advanced")},
		views.FormField{Key: "plain", Label: "Plain", Type: views.FieldText, Default: "p", Conditional: cond("basic")})
	screen := f.screenText()
	assert.NotContains(t, screen, "Extra")
	assert.Contains(t, screen, "Plain")
	f.press(tcell.KeyDown) // Plain
	f.confirm()
	assert.Equal(t, map[string]string{"kind": "basic", "plain": "p"}, f.result().values)
}

func buttonLabelOf(p tview.Primitive) string {
	if b, ok := p.(*tview.Button); ok {
		return widgets.ButtonLabel(b)
	}
	return ""
}

// A click on an item of a select list confirms it; a click on an item of a
// multi-select list toggles it without closing the list (these handlers run
// their update off the event loop).
func TestInlineListsMouseClick(t *testing.T) {
	s := newKeyShell(t, &keyView{id: "home", focusList: true})
	click := func(row int) {
		time.Sleep(tview.DoubleClickInterval + 50*time.Millisecond) // two clicks in a row make a double click
		var x, y int
		s.loop(func() {
			l, ok := s.app.GetFocus().(*tview.List)
			require.True(t, ok, "a list has the focus")
			ix, iy, _, _ := l.GetInnerRect()
			x, y = ix+2, iy+row
		})
		s.sim.InjectMouse(x, y, tcell.Button1, tcell.ModNone)
		s.sim.InjectMouse(x, y, tcell.ButtonNone, tcell.ModNone)
	}

	picked := make(chan string, 1)
	s.loop(func() { s.ShowSelectModal("Pick", options("a", "b", "c"), "a", func(v string) { picked <- v }) })
	click(1)
	select {
	case v := <-picked:
		assert.Equal(t, "b", v)
	case <-time.After(2 * time.Second):
		t.Fatal("the click did not confirm the item")
	}
	require.Eventually(t, func() bool { return !s.overlayShown() }, 2*time.Second, 10*time.Millisecond, "the list closes")

	multi := make(chan []string, 1)
	s.loop(func() {
		s.ShowMultiSelectModal("Pick", options("a", "b", "c"), nil, func(v []string) { multi <- v })
	})
	checked := func(n int) {
		t.Helper()
		require.Eventually(t, func() bool {
			return strings.Count(s.screenText(), theme.SymSelectedMulti) == n
		}, 2*time.Second, 10*time.Millisecond, "%d item(s) checked", n)
	}
	click(0)
	checked(1)
	click(2)
	checked(2)
	s.key(tcell.KeyEnter, 0, tcell.ModNone)
	select {
	case v := <-multi:
		assert.Equal(t, []string{"a", "c"}, v)
	case <-time.After(2 * time.Second):
		t.Fatal("not confirmed")
	}
}

// In a multi-select list Space ticks the item under the cursor (x does not:
// it is the Archive of a list of objects), the arrows move, Enter validates
// and Esc closes without validating.
func TestInlineMultiSelectKeys(t *testing.T) {
	s := newKeyShell(t, &keyView{id: "home", focusList: true})
	multi := make(chan []string, 1)
	s.loop(func() {
		s.ShowMultiSelectModal("Pick", options("a", "b", "c"), nil, func(v []string) { multi <- v })
	})
	checked := func(n int) {
		t.Helper()
		require.Eventually(t, func() bool {
			return strings.Count(s.screenText(), theme.SymSelectedMulti) == n
		}, 2*time.Second, 10*time.Millisecond, "%d item(s) checked", n)
	}
	checked(0)
	s.key(tcell.KeyRune, ' ', tcell.ModNone)
	checked(1)
	s.key(tcell.KeyDown, 0, tcell.ModNone)
	s.key(tcell.KeyRune, 'x', tcell.ModNone)
	checked(1)
	s.key(tcell.KeyRune, ' ', tcell.ModNone)
	checked(2)
	s.key(tcell.KeyRune, ' ', tcell.ModNone)
	checked(1)
	s.key(tcell.KeyRune, 'z', tcell.ModNone)
	checked(1)
	s.key(tcell.KeyEnter, 0, tcell.ModNone)
	select {
	case v := <-multi:
		assert.Equal(t, []string{"a"}, v)
	case <-time.After(2 * time.Second):
		t.Fatal("not confirmed")
	}
}
