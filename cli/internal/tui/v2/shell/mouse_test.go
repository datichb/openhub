package shell

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

func newMouseShell(t *testing.T) *Shell {
	t.Helper()
	home := &testView{id: "home", title: "Home"}
	s := New(Config{Views: []views.View{home}, HomeViewID: "home"})
	s.selection.copy = func(string) error { return nil }
	s.selection.onCopy = nil
	s.NavigateHome("home", views.ModeHub)
	s.omnibar.container.SetRect(0, 20, 80, 6) // below the cells of the test
	s.screen = newSimScreen(t, []string{"Hello World, hello", "second line here", "third line"})
	t.Cleanup(func() { s.screen.Fini() })
	return s
}

func mouseAt(x, y int, buttons tcell.ButtonMask) *tcell.EventMouse {
	return tcell.NewEventMouse(x, y, buttons, tcell.ModNone)
}

// tview calls the capture again for the next action of the same terminal
// event with the event the previous call returned: nil after a call that
// consumed it.
func TestMouseCaptureAfterAConsumedEvent(t *testing.T) {
	s := newMouseShell(t)
	for _, action := range []tview.MouseAction{tview.MouseMove, tview.MouseLeftDown, tview.MouseLeftUp, tview.MouseLeftClick,
		tview.MouseLeftDoubleClick, tview.MouseScrollUp, tview.MouseRightClick} {
		assert.NotPanics(t, func() {
			ev, _ := s.mouseCapture(nil, action)
			assert.Nil(t, ev, "action %d", action)
		}, "action %d", action)
	}
}

// A click while an earlier selection is still shown: the terminal reports
// the press at its new position as a move with the button down, then the
// press. The move is no drag, the press starts a new selection.
func TestMouseCaptureClickAfterASelection(t *testing.T) {
	s := newMouseShell(t)

	ev, _ := s.mouseCapture(mouseAt(1, 0, tcell.Button1), tview.MouseLeftDown)
	assert.Nil(t, ev, "the press is taken by the selection")
	ev, _ = s.mouseCapture(mouseAt(1, 0, tcell.ButtonNone), tview.MouseLeftUp)
	assert.Nil(t, ev, "so is the release")
	require.True(t, s.selection.IsActive(), "the selection stays shown")
	require.False(t, s.selection.IsDragging(), "but nothing is held")

	move := mouseAt(8, 1, tcell.Button1)
	ev, action := s.mouseCapture(move, tview.MouseMove)
	assert.Same(t, move, ev, "the move of a new press is not a drag: it goes on")
	assert.Equal(t, tview.MouseMove, action)
	assert.Equal(t, CellPos{X: 1, Y: 0}, s.selection.state.cursor, "the old selection did not follow")

	ev, _ = s.mouseCapture(mouseAt(8, 1, tcell.Button1), tview.MouseLeftDown)
	assert.Nil(t, ev, "the press starts a new selection")
	assert.Equal(t, CellPos{X: 8, Y: 1}, s.selection.state.anchor)
	assert.True(t, s.selection.IsDragging())
}

func TestMouseCaptureDragSelects(t *testing.T) {
	s := newMouseShell(t)
	s.mouseCapture(mouseAt(0, 0, tcell.Button1), tview.MouseLeftDown)

	ev, _ := s.mouseCapture(mouseAt(4, 0, tcell.Button1), tview.MouseMove)
	assert.Nil(t, ev, "the move with the button held extends the selection")
	assert.Equal(t, CellPos{X: 4, Y: 0}, s.selection.state.cursor)

	ev, _ = s.mouseCapture(mouseAt(4, 0, tcell.ButtonNone), tview.MouseLeftUp)
	assert.Nil(t, ev, "the release ends it")
	assert.False(t, s.selection.IsDragging())
	assert.True(t, s.selection.IsActive(), "the selection stays shown")

	move := mouseAt(6, 0, tcell.ButtonNone)
	ev, _ = s.mouseCapture(move, tview.MouseMove)
	assert.Same(t, move, ev, "a move without a drag goes on")
}

// A release after a press in an interactive zone (no selection started)
// goes on to the widget.
func TestMouseCaptureReleaseWithoutSelection(t *testing.T) {
	s := newMouseShell(t)
	up := mouseAt(3, 1, tcell.ButtonNone)
	ev, _ := s.mouseCapture(up, tview.MouseLeftUp)
	assert.Same(t, up, ev)
}
