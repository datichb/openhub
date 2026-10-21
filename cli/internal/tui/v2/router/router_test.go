package router

import (
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/gdamore/tcell/v2"
)

// mockView implements views.View for testing.
type mockView struct {
	id        string
	title     string
	mounted   bool
	unmounted bool
}

var _ views.View = (*mockView)(nil)

func (v *mockView) ID() string    { return v.id }
func (v *mockView) Title() string { return v.title }
func (v *mockView) Mount(content *tview.Flex, _ *tview.Application) {
	v.mounted = true
	v.unmounted = false
	content.AddItem(tview.NewBox(), 0, 1, false)
}
func (v *mockView) Unmount()                                    { v.unmounted = true; v.mounted = false }
func (v *mockView) StatusHints() string                         { return v.id + " hints" }
func (v *mockView) HandleKey(_ *tcell.EventKey) *tcell.EventKey { return nil }

func newTestRouter(onNavigate func(views.View)) *Router {
	return New(tview.NewFlex(), tview.NewApplication(), onNavigate)
}

func TestRouter(t *testing.T) {
	t.Run("push mounts the view and notifies", func(t *testing.T) {
		var navigated views.View
		r := newTestRouter(func(v views.View) { navigated = v })
		assert.Nil(t, r.Current(), "no current view before the first push")

		home := &mockView{id: "home", title: "Home"}
		r.Push(home)

		require.NotNil(t, r.Current(), "current view after a push")
		assert.Equal(t, "home", r.Current().ID(), "current view")
		assert.True(t, home.mounted, "pushed view mounted")
		assert.Equal(t, views.View(home), navigated, "navigation callback")
		assert.Equal(t, 1, r.StackDepth(), "stack depth")
	})

	t.Run("pop goes back to the previous view", func(t *testing.T) {
		r := newTestRouter(nil)
		home := &mockView{id: "home", title: "Home"}
		board := &mockView{id: "board", title: "Board"}
		r.Push(home)
		r.Push(board)

		assert.Equal(t, "board", r.Current().ID(), "current view after the second push")
		assert.True(t, home.unmounted, "previous view unmounted")
		assert.True(t, board.mounted, "new view mounted")
		assert.Equal(t, 2, r.StackDepth(), "stack depth")

		require.True(t, r.Pop(), "pop succeeds")
		assert.Equal(t, "home", r.Current().ID(), "current view after the pop")
		assert.True(t, board.unmounted, "popped view unmounted")
		assert.True(t, home.mounted, "previous view mounted again")
		assert.Equal(t, 1, r.StackDepth(), "stack depth after the pop")

		assert.False(t, r.Pop(), "the last view is never popped")
	})

	t.Run("pop on an empty router", func(t *testing.T) {
		assert.False(t, newTestRouter(nil).Pop(), "nothing to pop")
	})

	t.Run("replace swaps the current view", func(t *testing.T) {
		r := newTestRouter(nil)
		home := &mockView{id: "home", title: "Home"}
		r.Push(home)
		r.Replace(&mockView{id: "board", title: "Board"})

		assert.Equal(t, "board", r.Current().ID(), "current view")
		assert.True(t, home.unmounted, "replaced view unmounted")
		assert.Equal(t, 1, r.StackDepth(), "stack depth")
	})

	t.Run("navigate to a registered view", func(t *testing.T) {
		r := newTestRouter(nil)
		r.Register(&mockView{id: "board", title: "Board"})
		r.Push(&mockView{id: "home", title: "Home"})

		assert.True(t, r.NavigateTo("board"), "registered view")
		assert.Equal(t, "board", r.Current().ID(), "current view")
		assert.False(t, r.NavigateTo("nonexistent"), "unknown view")
		assert.Equal(t, "board", r.Current().ID(), "unknown view leaves the current view")
	})
}
