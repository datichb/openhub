package shell

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// slowView finishes loading in the background: until gate is closed it has
// no widget with the focus; then it takes the focus unconditionally (the
// pattern of the views before focusIfOwned).
type slowView struct {
	id   string
	gate chan struct{}
}

func (v *slowView) ID() string          { return v.id }
func (v *slowView) Title() string       { return v.id }
func (v *slowView) StatusHints() string { return v.id + " hints" }
func (v *slowView) Unmount()            {}
func (v *slowView) Mount(content *tview.Flex, app *tview.Application) {
	list := widgets.NewSectionedList()
	list.SetItems([]widgets.SectionItem{{MainText: "one"}, {MainText: "two"}})
	content.AddItem(list, 0, 1, false)
	go func() {
		<-v.gate
		app.QueueUpdateDraw(func() { app.SetFocus(list) })
	}()
}
func (v *slowView) HandleKey(event *tcell.EventKey) *tcell.EventKey { return event }

// A palette or a help opened while a view is still loading must close on
// Esc even if the load took the focus meanwhile; a second Esc goes back.
func TestPaletteOpenedDuringALoadStillClosesOnEsc(t *testing.T) {
	for _, tc := range []struct {
		name  string
		open  func(s *simShell)
		open2 func(s *simShell) bool
	}{
		{"palette via Enter", func(s *simShell) { s.key(tcell.KeyEnter, 0, tcell.ModNone) },
			func(s *simShell) bool { return s.omnibarActive() }},
		{"palette via /", func(s *simShell) { s.key(tcell.KeyRune, '/', tcell.ModNone) },
			func(s *simShell) bool { return s.omnibarActive() }},
		{"help via ?", func(s *simShell) { s.key(tcell.KeyRune, '?', tcell.ModNone) },
			func(s *simShell) bool { return s.overlayShown() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gate := make(chan struct{})
			slow := &slowView{id: "slow", gate: gate}
			s := newKeyShell(t, &keyView{id: "home", focusList: true})
			s.loop(func() { s.PushView(slow) })
			require.Equal(t, 2, s.depth())

			tc.open(s)
			require.True(t, tc.open2(s), "the layer opened during the load")
			close(gate)
			s.loop(func() {}) // the load took the focus
			s.loop(func() {})

			s.key(tcell.KeyEscape, 0, tcell.ModNone)
			assert.False(t, tc.open2(s), "Esc closes the palette / the help")
			assert.Equal(t, 2, s.depth(), "the view stays")

			s.key(tcell.KeyEscape, 0, tcell.ModNone)
			assert.Equal(t, 1, s.depth(), "the second Esc goes back")
		})
	}
}

// The keys of a palette that lost the focus reach its input, not the view.
func TestPaletteThatLostTheFocusKeepsTheTyping(t *testing.T) {
	s := newKeyShell(t, &keyView{id: "home", focusList: true})
	s.key(tcell.KeyCtrlP, 0, tcell.ModNone)
	require.True(t, s.omnibarActive())
	s.loop(func() { s.app.SetFocus(s.content) }) // a load stole the focus
	s.key(tcell.KeyRune, 'z', tcell.ModNone)
	var text string
	s.loop(func() { text = s.omnibar.input.GetText() })
	assert.Equal(t, "z", text)
	s.loop(func() { assert.Equal(t, tview.Primitive(s.omnibar.input), s.app.GetFocus()) })
}

// A window that lost the focus to the view behind gets it back; Esc closes it.
func TestWindowThatLostTheFocusClosesOnEsc(t *testing.T) {
	s := newKeyShell(t, &keyView{id: "home", focusList: true})
	s.loop(func() { s.ShowScrollableModal("t", "body", []views.ModalAction{{Label: "ok", Callback: func() {}}}) })
	require.True(t, s.overlayShown())
	s.loop(func() { s.app.SetFocus(s.content) })

	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	assert.False(t, s.overlayShown(), "Esc closes the window")
	assert.Equal(t, 1, s.depth())
}

// With the focus in place, nothing changes: Esc of the palette closes the
// palette only.
func TestPaletteWithFocusEscClosesItOnly(t *testing.T) {
	s := newKeyShell(t, &keyView{id: "home", focusList: true})
	s.loop(func() { s.PushView(&keyView{id: "second", focusList: true}) })
	s.key(tcell.KeyCtrlP, 0, tcell.ModNone)
	require.True(t, s.omnibarActive())
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	assert.False(t, s.omnibarActive())
	assert.Equal(t, 2, s.depth())
}

// hintView is a view with the hints given.
type hintView struct {
	keyView
	hints string
}

func (v *hintView) StatusHints() string { return v.hints }

// The shell adds the way back to the hints of a view above the root, once.
func TestHintsOfAViewAboveTheRootShowTheWayBack(t *testing.T) {
	s := newKeyShell(t, &keyView{id: "home", focusList: true})
	var plain, named, root string
	s.loop(func() {
		root = s.hintsOf(&hintView{hints: "a b"})
		s.PushView(&hintView{keyView: keyView{id: "p1"}, hints: "a b"})
		plain = s.hintsOf(&hintView{hints: "a b"})
		named = s.hintsOf(&hintView{hints: "a b · Esc retour"})
	})
	assert.Equal(t, "a b", root, "the root is left as it is")
	assert.Contains(t, plain, "a b")
	assert.Contains(t, plain, keys.Back.Hint(), "the way back is added")
	assert.Equal(t, "a b · Esc retour", named, "a view that names it keeps its text")
}
