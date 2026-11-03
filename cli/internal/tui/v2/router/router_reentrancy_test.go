package router

import (
	"testing"
	"time"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// hookView is a view whose callbacks run code of the test, as a view that
// navigates from Mount or Unmount would.
type hookView struct {
	mockView
	log       *[]string
	onMount   func()
	onUnmount func()
}

func (v *hookView) Mount(content *tview.Flex, app *tview.Application) {
	*v.log = append(*v.log, "mount "+v.id)
	v.mockView.Mount(content, app)
	if v.onMount != nil {
		v.onMount()
	}
}

func (v *hookView) Unmount() {
	*v.log = append(*v.log, "unmount "+v.id)
	v.mockView.Unmount()
	if v.onUnmount != nil {
		v.onUnmount()
	}
}

// within fails the test when fn does not return: a router that holds its lock
// during the calls on the views waits for itself.
func within(t *testing.T, fn func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the router is blocked: a callback of a view navigated while the router held its lock")
	}
}

// A view that goes back from its Unmount does not block the router, and the
// navigation it asked for runs once the first one is over.
func TestRouterPopFromUnmountDoesNotBlock(t *testing.T) {
	var log []string
	r := newTestRouter(nil)
	home := &hookView{mockView: mockView{id: "home"}, log: &log}
	form := &hookView{mockView: mockView{id: "form"}, log: &log}
	picker := &hookView{mockView: mockView{id: "picker"}, log: &log}
	form.onUnmount = func() { r.Pop() } // e.g. a form that closes itself when it is covered

	within(t, func() {
		r.Push(home)
		r.Push(form)
		r.Push(picker) // unmounts the form, whose Unmount pops
	})

	// The pop that was asked for during the push ran after it, on the stack
	// the push left: it removed the picker.
	assert.Equal(t, []string{
		"mount home",
		"unmount home", "mount form",
		"unmount form", "mount picker",
		"unmount picker", "mount form",
	}, log)
	require.NotNil(t, r.Current())
	assert.Equal(t, "form", r.Current().ID())
	assert.Equal(t, 2, r.StackDepth())
}

// The router answers from the callbacks of the views it calls.
func TestRouterAnswersFromTheCallbacks(t *testing.T) {
	var log []string
	var depthSeen, currentSeen []string
	var r *Router
	r = newTestRouter(func(v views.View) {
		currentSeen = append(currentSeen, r.Current().ID()+"/"+v.ID())
		depthSeen = append(depthSeen, string(rune('0'+r.StackDepth())))
	})
	home := &hookView{mockView: mockView{id: "home"}, log: &log}
	board := &hookView{mockView: mockView{id: "board"}, log: &log}
	board.onMount = func() { assert.Equal(t, "board", r.Current().ID()) }

	within(t, func() {
		r.Push(home)
		r.Push(board)
	})
	assert.Equal(t, []string{"home/home", "board/board"}, currentSeen)
	assert.Equal(t, []string{"1", "2"}, depthSeen)
}

// A view that navigates when it is mounted (a redirection) lands on its
// target, and the order of the calls of each navigation is kept.
func TestRouterNavigateFromMount(t *testing.T) {
	var log []string
	r := newTestRouter(nil)
	home := &hookView{mockView: mockView{id: "home"}, log: &log}
	gate := &hookView{mockView: mockView{id: "gate"}, log: &log}
	target := &hookView{mockView: mockView{id: "target"}, log: &log}
	gate.onMount = func() { r.Replace(target) }

	within(t, func() {
		r.Push(home)
		r.Push(gate)
	})
	assert.Equal(t, []string{
		"mount home",
		"unmount home", "mount gate",
		"unmount gate", "mount target",
	}, log)
	assert.Equal(t, "target", r.Current().ID())
	assert.Equal(t, 2, r.StackDepth(), "home, then the target in place of the gate")
}

// NavigateTo and Remount from a callback are queued the same way.
func TestRouterNavigateToAndRemountFromCallbacks(t *testing.T) {
	var log []string
	r := newTestRouter(nil)
	home := &hookView{mockView: mockView{id: "home"}, log: &log}
	other := &hookView{mockView: mockView{id: "other"}, log: &log}
	r.Register(home)
	r.Register(other)
	remounted := false
	other.onMount = func() {
		if !remounted {
			remounted = true
			r.Remount()
		}
	}
	within(t, func() {
		r.Push(home)
		assert.True(t, r.NavigateTo("other"))
	})
	assert.Equal(t, "other", r.Current().ID())
	assert.Equal(t, []string{"mount home", "unmount home", "mount other", "unmount other", "mount other"}, log)

	within(t, func() { assert.True(t, r.NavigateTo("home")) })
	assert.Equal(t, "home", r.Current().ID())
	assert.Equal(t, 1, r.StackDepth())
}

// A view that panics in a call does not leave the router busy.
func TestRouterSurvivesAPanickingView(t *testing.T) {
	var log []string
	r := newTestRouter(nil)
	home := &hookView{mockView: mockView{id: "home"}, log: &log}
	bad := &hookView{mockView: mockView{id: "bad"}, log: &log}
	bad.onMount = func() { panic("view exploded") }
	r.Push(home)
	assert.PanicsWithValue(t, "view exploded", func() { r.Push(bad) })

	other := &hookView{mockView: mockView{id: "other"}, log: &log}
	within(t, func() { r.Push(other) })
	assert.Equal(t, "other", r.Current().ID())
}
