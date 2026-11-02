package shell

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// These tests pin the key contract of globalKeyHandler (N19) before its code
// is split (A10); the Ctrl+T cycle, Esc and the toasts, ? and the palette
// have their own tests (mode_cycle_test.go, keys_test.go).

func (s *simShell) depth() (d int) {
	s.loop(func() { d = s.router.StackDepth() })
	return d
}

func (s *simShell) overlayShown() (shown bool) {
	s.loop(func() { shown = s.pages.HasPage("inline-overlay") })
	return shown
}

func (s *simShell) listIndex(t *testing.T) int {
	t.Helper()
	idx := -1
	s.loop(func() {
		if l, ok := s.app.GetFocus().(*widgets.SectionedList); ok {
			idx, _, _ = l.CurrentItem()
		}
	})
	return idx
}

// Ctrl+Q and Ctrl+C go through BeforeQuit; a second one while it is pending
// quits at once.
func TestKeysCtrlQAndCtrlCQuitThroughBeforeQuit(t *testing.T) {
	for _, k := range []tcell.Key{tcell.KeyCtrlQ, tcell.KeyCtrlC} {
		var asked atomic.Int32
		home := &keyView{id: "home", focusList: true}
		s := runSimShell(t, 120, 40, Config{ProjectName: "test", Views: []views.View{home}, HomeViewID: "home",
			BeforeQuit: func(func()) { asked.Add(1) }})
		s.loop(func() { s.NavigateHome("home", views.ModeHub) })
		s.key(k, 0, tcell.ModNone)
		assert.EqualValues(t, 1, asked.Load(), "asked first")
		assert.True(t, s.IsRunning())
		s.sim.InjectKey(k, 0, tcell.ModNone) // the loop stops: no fence
		require.Eventually(t, func() bool { return !s.IsRunning() }, 2*time.Second, 10*time.Millisecond, "second request quits")
	}
}

// Ctrl+Q works over a window and with the palette open.
func TestKeysCtrlQWorksEverywhere(t *testing.T) {
	var asked atomic.Int32
	s := runSimShell(t, 120, 40, Config{ProjectName: "test", Views: []views.View{&keyView{id: "home", focusList: true}}, HomeViewID: "home",
		BeforeQuit: func(func()) { asked.Add(1) }})
	s.loop(func() { s.NavigateHome("home", views.ModeHub) })
	s.key(tcell.KeyCtrlP, 0, tcell.ModNone)
	require.True(t, s.omnibarActive())
	s.key(tcell.KeyCtrlQ, 0, tcell.ModNone)
	assert.EqualValues(t, 1, asked.Load(), "palette open")
	s.loop(func() { s.CancelQuit(); s.omnibar.Deactivate() })

	s.loop(func() { s.ShowScrollableModal("t", "body", []views.ModalAction{{Label: "ok", Callback: func() {}}}) })
	require.True(t, s.overlayShown())
	s.key(tcell.KeyCtrlQ, 0, tcell.ModNone)
	assert.EqualValues(t, 2, asked.Load(), "window open")
}

// Esc with a text selection clears it and goes no further.
func TestKeysEscClearsTheSelectionFirst(t *testing.T) {
	home := &keyView{id: "home", focusList: true}
	s := newKeyShell(t, home)
	s.loop(func() { s.PushView(&keyView{id: "pushed", focusList: true}) })
	s.loop(func() { s.selection.HandleMouseDown(5, 3, s.Shell.screen) })
	require.True(t, s.selection.IsActive())

	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	assert.False(t, s.selection.IsActive(), "selection cleared")
	assert.Equal(t, 2, s.depth(), "the view stays")
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	assert.Equal(t, 1, s.depth(), "second Esc goes back")
}

// Esc goes back one view; at the root of the hub it does nothing; at the
// root of another mode it returns to the hub.
func TestKeysEscNavigation(t *testing.T) {
	views3 := func() []views.View {
		return []views.View{&keyView{id: "home", focusList: true}, &keyView{id: "team.mode", focusList: true}, &keyView{id: "team.detail", focusList: true}}
	}
	s := runSimShell(t, 120, 40, Config{ProjectName: "test", HomeViewID: "home", Views: views3()})
	s.loop(func() { s.NavigateHome("home", views.ModeHub) })
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	assert.Equal(t, 1, s.depth())
	assert.Equal(t, views.ModeHub, s.mode())

	s.loop(func() { s.SetMode(views.ModeTeam) })
	s.loop(func() { s.NavigateTo("team.detail") })
	assert.Equal(t, 3, s.depth())
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	assert.Equal(t, 2, s.depth())
	assert.Equal(t, views.ModeTeam, s.mode(), "back on the landing of the team")
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	assert.Equal(t, 1, s.depth())
	s.awaitMode(views.ModeHub)

	// Started in the team mode: the landing is the root, Esc goes to the hub.
	team := runSimShell(t, 120, 40, Config{ProjectName: "test", HomeViewID: "team.mode", Views: views3()})
	team.loop(func() { team.NavigateHome("team.mode", views.ModeTeam) })
	require.Equal(t, 1, team.depth())
	team.key(tcell.KeyEscape, 0, tcell.ModNone)
	team.awaitMode(views.ModeHub)
}

// F1 opens the help everywhere a window is not shown, forms included; the
// help is not stacked over a window.
func TestKeysF1OpensTheHelp(t *testing.T) {
	home := &keyView{id: "home", focusList: true, consume: "x"}
	s := newKeyShell(t, home)
	s.key(tcell.KeyF1, 0, tcell.ModNone)
	assert.True(t, s.overlayShown())
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	assert.False(t, s.overlayShown())

	form := &capturingView{keyView: keyView{id: "form", focusList: true}}
	s.loop(func() { s.PushView(form) })
	s.key(tcell.KeyF1, 0, tcell.ModNone)
	assert.True(t, s.overlayShown(), "a form that keeps the keys still gets the help")
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	assert.Equal(t, 2, s.depth(), "Esc closes the help only")

	s.loop(func() {
		s.ShowScrollableModal("window", "body", []views.ModalAction{{Label: "ok", Callback: func() {}}})
	})
	s.key(tcell.KeyF1, 0, tcell.ModNone)
	s.key(tcell.KeyEscape, 0, tcell.ModNone)
	assert.False(t, s.overlayShown(), "one Esc closes the window: the help was not stacked")
}

// Ctrl+P, Ctrl+T and ? are reserved: a view never gets them; Ctrl+T does
// nothing while the palette is open nor in a view that keeps the keys.
func TestKeysReservedKeysAreNotForTheViews(t *testing.T) {
	home := &recordingView{keyView: keyView{id: "home", focusList: true}}
	s := runSimShell(t, 120, 40, Config{ProjectName: "test", Views: []views.View{home}, HomeViewID: "home",
		TeamsProvider: func() []views.SelectOption { return nil }})
	s.loop(func() { s.NavigateHome("home", views.ModeHub) })
	s.key(tcell.KeyCtrlT, 0, tcell.ModNone)
	s.awaitMode(views.ModeHub)
	s.key(tcell.KeyCtrlP, 0, tcell.ModNone)
	s.key(tcell.KeyRune, '?', tcell.ModNone)
	assert.Empty(t, home.keysSeen(), "the view never gets the reserved keys")
	assert.True(t, s.omnibarActive())

	s.key(tcell.KeyCtrlT, 0, tcell.ModNone)
	assert.Equal(t, views.ModeHub, s.mode(), "palette open: Ctrl+T is for the palette")
	s.loop(func() { s.omnibar.Deactivate() })

	tab := &capturingView{keyView: keyView{id: "tab", focusList: true}}
	s.loop(func() { s.PushView(tab) })
	s.key(tcell.KeyCtrlT, 0, tcell.ModNone)
	assert.Equal(t, views.ModeHub, s.mode())
	assert.Equal(t, 2, s.depth())
}

// The view sees every key the shell does not reserve, and keeps what it
// consumes: j/k/g/G only move the list when the view leaves them.
func TestKeysListFallbacks(t *testing.T) {
	home := &keyView{id: "home", focusList: true}
	s := newKeyShell(t, home)
	s.key(tcell.KeyRune, 'j', tcell.ModNone)
	assert.Equal(t, 1, s.listIndex(t), "j: down")
	s.key(tcell.KeyRune, 'k', tcell.ModNone)
	assert.Equal(t, 0, s.listIndex(t), "k: up")
	s.key(tcell.KeyRune, 'G', tcell.ModNone)
	assert.Equal(t, 1, s.listIndex(t), "G: last")
	s.key(tcell.KeyRune, 'g', tcell.ModNone)
	assert.Equal(t, 0, s.listIndex(t), "g: first")

	eater := &keyView{id: "eater", focusList: true, consume: "jg"}
	s.loop(func() { s.PushView(eater) })
	s.key(tcell.KeyRune, 'j', tcell.ModNone)
	s.key(tcell.KeyRune, 'g', tcell.ModNone)
	assert.Equal(t, 0, s.listIndex(t), "the view consumed j and g")
	keys, _ := eater.seen()
	assert.Equal(t, []rune{'j', 'g'}, keys)
}

// recordingView records the key of every event its HandleKey gets.
type recordingView struct {
	keyView
	all []tcell.Key
}

func (v *recordingView) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	v.mu.Lock()
	v.all = append(v.all, event.Key())
	v.mu.Unlock()
	return event
}

func (v *recordingView) keysSeen() []tcell.Key {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]tcell.Key(nil), v.all...)
}

// inputView has a text field in focus.
type inputView struct {
	keyView
	input *tview.InputField
}

func (v *inputView) Mount(content *tview.Flex, _ *tview.Application) {
	v.input = tview.NewInputField()
	content.AddItem(v.input, 1, 0, true)
}

// A text field keeps every key: letters, ? and /, neither help nor palette.
func TestKeysTextFieldKeepsTheLetters(t *testing.T) {
	home := &keyView{id: "home", focusList: true}
	field := &inputView{keyView: keyView{id: "field"}}
	s := newKeyShell(t, home)
	s.loop(func() { s.PushView(field) })
	for _, r := range "a?/jg" {
		s.key(tcell.KeyRune, r, tcell.ModNone)
	}
	assert.False(t, s.omnibarActive())
	assert.False(t, s.overlayShown())
	var text string
	s.loop(func() { text = field.input.GetText() })
	assert.Equal(t, "a?/jg", text)
	s.key(tcell.KeyEnter, 0, tcell.ModNone)
	assert.False(t, s.omnibarActive(), "Enter belongs to the field")
}

// While a window is open the keys go to the window: Ctrl+T, ? and / do not
// reach the shell.
func TestKeysWindowKeepsTheKeys(t *testing.T) {
	home := &keyView{id: "home", focusList: true}
	s := newKeyShell(t, home)
	s.loop(func() {
		s.ShowScrollableModal("window", "body", []views.ModalAction{{Label: "ok", Callback: func() {}}})
	})
	for _, k := range []struct {
		key tcell.Key
		r   rune
	}{{tcell.KeyCtrlT, 0}, {tcell.KeyCtrlP, 0}, {tcell.KeyRune, '/'}, {tcell.KeyRune, '?'}} {
		s.key(k.key, k.r, tcell.ModNone)
	}
	assert.False(t, s.omnibarActive())
	assert.Equal(t, views.ModeHub, s.mode())
	assert.True(t, s.overlayShown())
}

// A view that keeps the keys gets them first through HandleKey; what it
// consumes goes no further.
func TestKeysCapturingViewGetsTheKeysFirst(t *testing.T) {
	home := &keyView{id: "home", focusList: true}
	tab := &capturingView{keyView: keyView{id: "tab", focusList: true, consume: "a/"}}
	s := newKeyShell(t, home)
	s.loop(func() { s.PushView(tab) })
	s.key(tcell.KeyRune, 'a', tcell.ModNone)
	s.key(tcell.KeyRune, '/', tcell.ModNone)
	keys, _ := tab.seen()
	assert.Equal(t, []rune{'a', '/'}, keys)
	assert.False(t, s.omnibarActive())
}
