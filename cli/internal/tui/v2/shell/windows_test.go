package shell

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// twoPaneView has two focusable panes, the way the tabs of Sessions have a
// list and a reader: a window opened from the second one must give the focus
// back to it, not to the first.
type twoPaneView struct {
	id          string
	left, right *tview.TextView
}

func (v *twoPaneView) ID() string                                  { return v.id }
func (v *twoPaneView) Title() string                               { return v.id }
func (v *twoPaneView) StatusHints() string                         { return v.id + " hints" }
func (v *twoPaneView) Unmount()                                    {}
func (v *twoPaneView) HandleKey(e *tcell.EventKey) *tcell.EventKey { return e }
func (v *twoPaneView) Mount(content *tview.Flex, _ *tview.Application) {
	v.left = tview.NewTextView().SetText("left pane")
	v.right = tview.NewTextView().SetText("right pane")
	content.AddItem(v.left, 0, 1, true)
	content.AddItem(v.right, 0, 1, false)
}

// windowsSim is a shell on the two-pane view, the focus on the right pane.
func windowsSim(t *testing.T, cfg Config) (*panicSim, *twoPaneView) {
	t.Helper()
	v := &twoPaneView{id: "home"}
	cfg.ProjectName, cfg.Views, cfg.HomeViewID = "test", []views.View{v}, "home"
	p := runPanicSim(t, 120, 36, cfg)
	p.loopSafe(func() {
		p.NavigateHome("home", views.ModeHub)
		p.app.SetFocus(v.right)
	})
	return p, v
}

func (p *panicSim) focusIs(want tview.Primitive) bool {
	p.t.Helper()
	return p.focus() == want
}

func (p *panicSim) hasOverlayOnLoop() (has bool) {
	p.t.Helper()
	p.loopSafe(func() { has = p.hasOverlay() })
	return has
}

// A window gives the focus back to the widget that had it, not to the first
// one of the view: ↑↓ would change the session under the reader.
func TestWindowsGiveTheFocusBackToTheWidgetThatHadIt(t *testing.T) {
	open := map[string]func(p *panicSim){
		"reader":   func(p *panicSim) { p.ShowScrollableModal("reader", "text", nil) },
		"card":     func(p *panicSim) { p.ShowScrollableModal("card", "text", []views.ModalAction{{Label: "ok"}}) },
		"input":    func(p *panicSim) { p.ShowInputModal("input", "", nil) },
		"password": func(p *panicSim) { p.ShowPasswordModal("password", nil) },
		"confirm":  func(p *panicSim) { p.Confirm("confirm", "sure?", false, nil) },
		"select": func(p *panicSim) {
			p.ShowSelectModal("select", []views.SelectOption{{Label: "a", Value: "a"}}, "", nil)
		},
		"multiselect": func(p *panicSim) {
			p.ShowMultiSelectModal("multi", []views.SelectOption{{Label: "a", Value: "a"}}, nil, nil)
		},
		"form": func(p *panicSim) {
			p.ShowInlineForm(views.InlineFormConfig{Title: "form", Fields: []views.FormField{{Key: "k", Label: "k", Type: views.FieldText}}})
		},
	}
	for name, show := range open {
		t.Run(name, func(t *testing.T) {
			p, v := windowsSim(t, Config{})
			p.loopSafe(func() { show(p) })
			require.True(t, p.page("inline-overlay"))
			p.press(tcell.KeyEscape, 0)
			assert.False(t, p.hasOverlayOnLoop())
			assert.True(t, p.focusIs(v.right), "the right pane has the focus back")
		})
	}

	t.Run("answered", func(t *testing.T) {
		p, v := windowsSim(t, Config{})
		got := ""
		p.loopSafe(func() { p.ShowInputModal("input", "", func(s string) { got = s }) })
		p.press(tcell.KeyRune, 'o')
		p.press(tcell.KeyRune, 'k')
		p.press(tcell.KeyEnter, 0)
		assert.Equal(t, "ok", got)
		assert.True(t, p.focusIs(v.right))
	})

	t.Run("the view changed: no focus on a widget that is gone", func(t *testing.T) {
		p, v := windowsSim(t, Config{})
		other := &twoPaneView{id: "other"}
		p.loopSafe(func() { p.router.Register(other) })
		p.loopSafe(func() { p.ShowScrollableModal("reader", "text", nil) })
		p.loopSafe(func() { p.router.NavigateTo("other") })
		p.press(tcell.KeyEscape, 0)
		assert.False(t, p.focusIs(v.right), "not the widget of the view that is not shown")
		assert.True(t, p.loopContentFocused())
	})
}

// A question asked from a window opens over it: cancelled, the window is
// back with the button it had; answered, both go.
func TestAQuestionAskedFromAWindowStacksOverIt(t *testing.T) {
	asked := map[string]func(p *panicSim, answered *int){
		"confirm": func(p *panicSim, answered *int) {
			p.Confirm("sure?", "text", false, func() { *answered++ })
		},
		"input": func(p *panicSim, answered *int) {
			p.ShowInputModal("input", "x", func(string) { *answered++ })
		},
		"form": func(p *panicSim, answered *int) {
			p.ShowInlineForm(views.InlineFormConfig{Title: "form",
				Fields:   []views.FormField{{Key: "k", Label: "k", Type: views.FieldText, Default: "v"}},
				OnSubmit: func(map[string]string, map[string][]string) { *answered++ }})
		},
		"select": func(p *panicSim, answered *int) {
			p.ShowSelectModal("select", []views.SelectOption{{Label: "a", Value: "a"}}, "", func(string) { *answered++ })
		},
		"multiselect": func(p *panicSim, answered *int) {
			p.ShowMultiSelectModal("multi", []views.SelectOption{{Label: "a", Value: "a"}}, nil, func([]string) { *answered++ })
		},
	}
	setup := func(t *testing.T, ask func(p *panicSim, answered *int)) (*panicSim, *twoPaneView, *int, tview.Primitive) {
		p, v := windowsSim(t, Config{})
		answered := new(int)
		p.loopSafe(func() {
			p.ShowScrollableModal("the card", "text of the card", []views.ModalAction{
				{Label: "other"},
				{Label: "ask", KeepOpen: true, Callback: func() { ask(p, answered) }},
			})
		})
		p.press(tcell.KeyRight, 0) // « ask »
		button := p.focus()
		p.press(tcell.KeyEnter, 0)
		require.True(t, p.page("sub-overlay"), "the question opens over the card:\n%s", p.text())
		return p, v, answered, button
	}
	for name, ask := range asked {
		t.Run(name+" cancelled", func(t *testing.T) {
			p, _, answered, button := setup(t, ask)
			p.press(tcell.KeyEscape, 0)
			assert.False(t, p.page("sub-overlay"))
			require.True(t, p.page("inline-overlay"), "the card is still there")
			assert.Contains(t, p.text(), "text of the card")
			assert.Same(t, button, p.focus(), "with the button it had")
			assert.Zero(t, *answered)

			p.press(tcell.KeyEscape, 0)
			assert.False(t, p.hasOverlayOnLoop())
		})
		t.Run(name+" answered", func(t *testing.T) {
			p, v, answered, _ := setup(t, ask)
			switch name {
			case "confirm":
				p.press(tcell.KeyEnter, 0) // « Oui »
			case "form":
				p.press(tcell.KeyTab, 0)
				p.press(tcell.KeyEnter, 0) // « Confirmer »
			default:
				p.press(tcell.KeyEnter, 0)
			}
			assert.Equal(t, 1, *answered)
			assert.False(t, p.hasOverlayOnLoop(), "the card closes with the answer:\n%s", p.text())
			assert.True(t, p.focusIs(v.right))
		})
	}
}

// A form asked from a card refuses an incomplete submission: it stays open,
// the card under it too, nothing is sent.
func TestAFormCheckRefusesWithoutClosing(t *testing.T) {
	p, v := windowsSim(t, Config{})
	sent := 0
	p.loopSafe(func() {
		p.ShowScrollableModal("the card", "text of the card", []views.ModalAction{
			{Label: "ask", KeepOpen: true, Callback: func() {
				p.ShowInlineForm(views.InlineFormConfig{Title: "form",
					Fields: []views.FormField{{Key: "message", Label: "message", Type: views.FieldText}},
					Check: func(values map[string]string, _ map[string][]string) string {
						if values["message"] == "" {
							return "an instruction is needed"
						}
						return ""
					},
					OnSubmit: func(map[string]string, map[string][]string) { sent++ },
				})
			}},
		})
	})
	p.press(tcell.KeyEnter, 0) // « ask »
	require.True(t, p.page("sub-overlay"))

	p.press(tcell.KeyTab, 0)   // « Confirmer »
	p.press(tcell.KeyEnter, 0) // refused
	assert.True(t, p.page("sub-overlay"), "the form stays open")
	assert.True(t, p.page("inline-overlay"), "and the card under it")
	assert.Zero(t, sent)
	awaitText(t, p, "an instruction is needed")

	p.press(tcell.KeyBacktab, 0) // the field
	p.press(tcell.KeyRune, 'g')
	p.press(tcell.KeyTab, 0)
	p.press(tcell.KeyEnter, 0)
	assert.Equal(t, 1, sent)
	assert.False(t, p.hasOverlayOnLoop())
	assert.True(t, p.focusIs(v.right))
}

// Ctrl+Q while a reader is shown over a card: the dialog opens over them;
// cancelled, the reader is back with its focus and nothing is orphaned.
func TestQuitDialogOverOpenWindows(t *testing.T) {
	var sh *Shell
	cancelled := 0
	cfg := Config{BeforeQuit: func(quit func()) {
		sh.ShowInlineForm(views.InlineFormConfig{Title: "quit",
			Fields:   []views.FormField{{Key: "k", Label: "choice", Type: views.FieldBool, Default: "true"}},
			OnCancel: func() { cancelled++; sh.CancelQuit() }})
	}}
	p, v := windowsSim(t, cfg)
	sh = p.Shell
	p.loopSafe(func() {
		p.ShowScrollableModal("the card", "text of the card", []views.ModalAction{
			{Label: "read", KeepOpen: true, Callback: func() { p.ShowScrollableModal("the reader", "text of the reader", nil) }},
		})
	})
	p.press(tcell.KeyEnter, 0)
	require.True(t, p.page("sub-overlay"))
	reader := p.focus()

	p.press(tcell.KeyCtrlQ, 0)
	require.True(t, p.page("sub-overlay-2"), "the dialog opens over the reader:\n%s", p.text())
	assert.Contains(t, p.text(), "quit")

	p.press(tcell.KeyEscape, 0)
	assert.Equal(t, 1, cancelled)
	assert.False(t, p.page("sub-overlay-2"))
	assert.True(t, p.page("sub-overlay"), "the reader is back")
	assert.Same(t, reader, p.focus())

	p.press(tcell.KeyEscape, 0)
	p.press(tcell.KeyEscape, 0)
	assert.False(t, p.hasOverlayOnLoop(), "nothing is left on the screen")
	assert.True(t, p.focusIs(v.right))

	p.press(tcell.KeyCtrlQ, 0)
	require.True(t, p.page("inline-overlay"), "alone, the dialog is the first window")
	assert.False(t, p.page("sub-overlay"))
}

// Closing the first window removes the ones over it: none stays on the
// screen without the focus.
func TestClosingAWindowClosesTheOnesOverIt(t *testing.T) {
	p, v := windowsSim(t, Config{})
	p.loopSafe(func() {
		p.ShowScrollableModal("one", "text of one", nil)
		p.ShowScrollableModal("two", "text of two", nil)
		p.Confirm("three", "text of three", false, nil)
	})
	require.True(t, p.page("sub-overlay-2"))
	p.loopSafe(func() { p.closeWindows() })
	assert.False(t, p.hasOverlayOnLoop())
	assert.True(t, p.focusIs(v.right))

	p.loopSafe(func() {
		p.ShowScrollableModal("one", "text of one", nil)
		p.ShowScrollableModal("two", "text of two", nil)
		p.ShowScrollableModal("three", "text of three", nil)
		p.ShowScrollableModal("four", "text of four", nil) // full: replaces the last
	})
	assert.Contains(t, p.text(), "text of four")
	p.press(tcell.KeyEscape, 0)
	assert.Contains(t, p.text(), "text of two", "the second is under the replaced third")
	assert.False(t, p.page("sub-overlay-2"))
}

// A list opened from a form that is itself over a card: the form gets the
// focus back and its arrows keep moving between the fields.
func TestListOverAFormOverACard(t *testing.T) {
	p, _ := windowsSim(t, Config{})
	options := []views.SelectOption{{Label: "o1", Value: "1"}, {Label: "o2", Value: "2"}, {Label: "o3", Value: "3"},
		{Label: "o4", Value: "4"}, {Label: "o5", Value: "5"}, {Label: "o6", Value: "6"}}
	var got map[string]string
	p.loopSafe(func() {
		p.ShowScrollableModal("the card", "text of the card", []views.ModalAction{
			{Label: "ask", KeepOpen: true, Callback: func() {
				p.ShowInlineForm(views.InlineFormConfig{Title: "form",
					Fields: []views.FormField{
						{Key: "pick", Label: "pick", Type: views.FieldSelect, Options: options},
						{Key: "text", Label: "text", Type: views.FieldText},
					},
					OnSubmit: func(v map[string]string, _ map[string][]string) { got = v }})
			}},
		})
	})
	p.press(tcell.KeyEnter, 0) // the form, over the card
	require.True(t, p.page("sub-overlay"))
	p.press(tcell.KeyEnter, 0) // Enter on the select: the list
	require.True(t, p.page("sub-overlay-2"), "the list opens over the form:\n%s", p.text())
	assert.Contains(t, p.text(), "Esc")

	p.press(tcell.KeyEscape, 0)
	assert.False(t, p.page("sub-overlay-2"))
	require.True(t, p.page("sub-overlay"), "the form is back")

	p.press(tcell.KeyEnter, 0)
	p.press(tcell.KeyDown, 0)
	p.press(tcell.KeyEnter, 0) // o2
	assert.False(t, p.page("sub-overlay-2"))
	assert.Contains(t, p.text(), "o2")

	p.press(tcell.KeyDown, 0) // the arrows still move in the form
	p.press(tcell.KeyRune, 'x')
	p.press(tcell.KeyTab, 0)
	p.press(tcell.KeyEnter, 0)
	assert.Equal(t, map[string]string{"pick": "2", "text": "x"}, got)
	assert.False(t, p.hasOverlayOnLoop())
}

// Tab and Shift+Tab in an input do not end it: the text is kept.
func TestInputKeepsItsTextOnTab(t *testing.T) {
	p, v := windowsSim(t, Config{})
	got, calls := "", 0
	p.loopSafe(func() { p.ShowInputModal("input", "", func(s string) { got = s; calls++ }) })
	p.press(tcell.KeyRune, 'a')
	p.press(tcell.KeyRune, 'b')
	p.press(tcell.KeyTab, 0)
	p.press(tcell.KeyBacktab, 0)
	require.True(t, p.page("inline-overlay"), "the input is still open")
	assert.Zero(t, calls)
	p.press(tcell.KeyRune, 'c')
	p.press(tcell.KeyEnter, 0)
	assert.Equal(t, "abc", got)
	assert.Equal(t, 1, calls)
	assert.True(t, p.focusIs(v.right))
}

// The lists of choices say which keys they take.
func TestListsShowTheirHints(t *testing.T) {
	p, _ := windowsSim(t, Config{})
	p.loopSafe(func() {
		p.ShowMultiSelectModal("pick many", []views.SelectOption{{Label: "a", Value: "a"}, {Label: "b", Value: "b"}}, nil, nil)
	})
	assert.Contains(t, p.text(), i18n.T("tui.modal.hints.multiselect"))
	p.press(tcell.KeyEscape, 0)

	p.loopSafe(func() { p.ShowSelectModal("pick one", []views.SelectOption{{Label: "a", Value: "a"}}, "", nil) })
	assert.Contains(t, p.text(), i18n.T("tui.modal.hints.select"))
}

// A toast shown and dismissed does not take the focus from the widget of
// the view that had it.
func TestToastKeepsTheFocusedWidget(t *testing.T) {
	p, v := windowsSim(t, Config{})
	p.loopSafe(func() { p.ShowToast("hello", ToastError) })
	require.Eventually(t, func() bool {
		n := 0
		p.loopSafe(func() { n = len(p.activeToastIDs) })
		return n == 1
	}, 2*time.Second, 5*time.Millisecond)
	assert.True(t, p.focusIs(v.right), "toast shown")
	p.press(tcell.KeyEscape, 0) // closes the toast
	assert.True(t, p.focusIs(v.right), "toast dismissed")
}

// A reason asked, then a confirmation (« Passer outre ») : the card stays
// under both, cancelling the confirmation gives it back, confirming closes it.
func TestAChainOfQuestionsKeepsTheCard(t *testing.T) {
	setup := func(t *testing.T) (*panicSim, *twoPaneView, *[]string, tview.Primitive) {
		p, v := windowsSim(t, Config{})
		done := &[]string{}
		p.loopSafe(func() {
			p.ShowScrollableModal("the card", "text of the card", []views.ModalAction{
				{Label: "override", KeepOpen: true, Callback: func() {
					p.ShowInputModal("reason", "", func(reason string) {
						p.Confirm("really?", "detail", true, func() { *done = append(*done, reason) })
					})
				}},
			})
		})
		button := p.focus()
		p.press(tcell.KeyEnter, 0)
		p.press(tcell.KeyRune, 'r')
		p.press(tcell.KeyEnter, 0)
		require.True(t, p.page("inline-overlay"), "the card is still there")
		require.True(t, p.page("sub-overlay"), "the confirmation opens over it:\n%s", p.text())
		assert.Contains(t, p.text(), "really?")
		return p, v, done, button
	}

	t.Run("cancelled", func(t *testing.T) {
		p, _, done, button := setup(t)
		p.press(tcell.KeyEscape, 0)
		assert.False(t, p.page("sub-overlay"))
		assert.Contains(t, p.text(), "text of the card")
		assert.Same(t, button, p.focus())
		assert.Empty(t, *done)
	})
	t.Run("confirmed", func(t *testing.T) {
		p, v, done, _ := setup(t)
		p.press(tcell.KeyRight, 0) // « Annuler » has the focus on a danger: move to « Oui »
		p.press(tcell.KeyEnter, 0)
		assert.Equal(t, []string{"r"}, *done)
		assert.False(t, p.hasOverlayOnLoop())
		assert.True(t, p.focusIs(v.right))
	})
}
