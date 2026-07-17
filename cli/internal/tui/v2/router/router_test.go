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
func (v *mockView) Unmount()           { v.unmounted = true; v.mounted = false }
func (v *mockView) StatusHints() string { return v.id + " hints" }
func (v *mockView) HandleKey(_ *tcell.EventKey) *tcell.EventKey { return nil }

func TestRouter_Push(t *testing.T) {
	content := tview.NewFlex()
	app := tview.NewApplication()
	var navigated views.View
	r := New(content, app, func(v views.View) { navigated = v })

	home := &mockView{id: "home", title: "Home"}
	r.Push(home)

	assert.Equal(t, "home", r.Current().ID())
	assert.True(t, home.mounted)
	assert.Equal(t, home, navigated)
	assert.Equal(t, 1, r.StackDepth())
}

func TestRouter_PushPop(t *testing.T) {
	content := tview.NewFlex()
	app := tview.NewApplication()
	r := New(content, app, nil)

	home := &mockView{id: "home", title: "Home"}
	board := &mockView{id: "board", title: "Board"}

	r.Push(home)
	r.Push(board)

	assert.Equal(t, "board", r.Current().ID())
	assert.True(t, home.unmounted)
	assert.True(t, board.mounted)
	assert.Equal(t, 2, r.StackDepth())

	ok := r.Pop()
	require.True(t, ok)
	assert.Equal(t, "home", r.Current().ID())
	assert.True(t, board.unmounted)
	assert.True(t, home.mounted)
	assert.Equal(t, 1, r.StackDepth())
}

func TestRouter_PopEmpty(t *testing.T) {
	content := tview.NewFlex()
	app := tview.NewApplication()
	r := New(content, app, nil)

	ok := r.Pop()
	assert.False(t, ok)

	home := &mockView{id: "home", title: "Home"}
	r.Push(home)
	ok = r.Pop()
	assert.False(t, ok, "should not pop last item")
}

func TestRouter_Replace(t *testing.T) {
	content := tview.NewFlex()
	app := tview.NewApplication()
	r := New(content, app, nil)

	home := &mockView{id: "home", title: "Home"}
	board := &mockView{id: "board", title: "Board"}

	r.Push(home)
	r.Replace(board)

	assert.Equal(t, "board", r.Current().ID())
	assert.True(t, home.unmounted)
	assert.Equal(t, 1, r.StackDepth())
}

func TestRouter_NavigateTo(t *testing.T) {
	content := tview.NewFlex()
	app := tview.NewApplication()
	r := New(content, app, nil)

	board := &mockView{id: "board", title: "Board"}
	r.Register(board)

	home := &mockView{id: "home", title: "Home"}
	r.Push(home)

	ok := r.NavigateTo("board")
	assert.True(t, ok)
	assert.Equal(t, "board", r.Current().ID())

	ok = r.NavigateTo("nonexistent")
	assert.False(t, ok)
}

func TestRouter_CurrentNil(t *testing.T) {
	content := tview.NewFlex()
	app := tview.NewApplication()
	r := New(content, app, nil)

	assert.Nil(t, r.Current())
}
