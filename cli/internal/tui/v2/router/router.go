// Package router manages view navigation with a stack-based history.
// It provides push/pop/replace semantics for moving between views
// in the unified TUI shell.
package router

import (
	"sync"

	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/rivo/tview"
)

// OnNavigate is called by the router after a view switch completes.
// Shell uses this to update breadcrumb, status bar, and menu highlight.
type OnNavigate func(v views.View)

// BeforeMount is called with the navigation stack (the view about to be
// mounted last) before the view is mounted. It must not call the router.
type BeforeMount func(stack []views.View)

// Router maintains a stack of views for back-navigation.
//
// The stack changes under the lock, the views are called outside it: a view
// whose Unmount, Mount or OnNavigate callback navigates (or only asks for
// Current) would else wait for a lock its own caller holds. A navigation
// asked for while another one is making its calls (a view navigating from
// its Unmount, say) is queued and runs, on the stack the first one left, once
// the first one is done: the calls of two navigations never interleave, and
// the order of a navigation stays unmount, before-mount, mount, navigate.
type Router struct {
	stack       []views.View
	registry    map[string]views.View
	content     *tview.Flex
	app         *tview.Application
	onNavigate  OnNavigate
	beforeMount BeforeMount
	mu          sync.Mutex
	busy        bool              // a navigation is making its calls
	queue       []func() []func() // navigations asked for meanwhile
}

// New creates a router bound to the given content panel and app.
func New(content *tview.Flex, app *tview.Application, onNav OnNavigate) *Router {
	return &Router{
		stack:      make([]views.View, 0, 8),
		registry:   make(map[string]views.View),
		content:    content,
		app:        app,
		onNavigate: onNav,
	}
}

// Register adds a view to the registry for lookup by ID.
func (r *Router) Register(v views.View) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registry[v.ID()] = v
}

// SetBeforeMount installs the hook run before every mount.
func (r *Router) SetBeforeMount(fn BeforeMount) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.beforeMount = fn
}

// navigate runs a navigation. plan changes the stack, under the lock, and
// returns the calls to make on the views, which run outside it. When a
// navigation is already making its calls, plan is queued for the end of that
// one and navigate returns at once.
func (r *Router) navigate(plan func() []func()) {
	r.mu.Lock()
	if r.busy {
		r.queue = append(r.queue, plan)
		r.mu.Unlock()
		return
	}
	r.busy = true
	calls := plan()
	r.mu.Unlock()

	// A view that panics must not leave the router busy for ever.
	clean := false
	defer func() {
		if !clean {
			r.mu.Lock()
			r.busy, r.queue = false, nil
			r.mu.Unlock()
		}
	}()
	for {
		for _, call := range calls {
			call()
		}
		r.mu.Lock()
		if len(r.queue) == 0 {
			r.busy = false
			r.mu.Unlock()
			clean = true
			return
		}
		next := r.queue[0]
		r.queue = r.queue[1:]
		calls = next()
		r.mu.Unlock()
	}
}

// planPush puts v on the stack; the calls unmount the view it covers and
// mount v. The caller holds the lock.
func (r *Router) planPush(v views.View) []func() {
	cur := r.currentLocked()
	if cur != nil && cur.ID() == v.ID() {
		return nil // already on this view — no-op
	}
	r.stack = append(r.stack, v)
	return r.planMount(cur, v)
}

// planMount returns the calls that unmount old (when not nil) and mount v on
// the stack as it is now. The caller holds the lock.
func (r *Router) planMount(old, v views.View) []func() {
	snapshot, before := append([]views.View(nil), r.stack...), r.beforeMount
	var calls []func()
	if old != nil {
		calls = append(calls, old.Unmount)
	}
	return append(calls, func() { r.mount(v, snapshot, before) })
}

// Push navigates to a new view, unmounting the current one.
// The previous view remains on the stack for Pop().
func (r *Router) Push(v views.View) {
	r.navigate(func() []func() { return r.planPush(v) })
}

// Pop returns to the previous view. Returns false if stack has 1 or fewer
// items. Asked for while a navigation is making its calls, it answers from
// the stack as it is then, before the navigations queued.
func (r *Router) Pop() bool {
	r.mu.Lock()
	ok := len(r.stack) > 1
	r.mu.Unlock()
	if !ok {
		return false
	}
	r.navigate(func() []func() {
		if len(r.stack) <= 1 {
			return nil
		}
		cur := r.stack[len(r.stack)-1]
		r.stack = r.stack[:len(r.stack)-1]
		return r.planMount(cur, r.stack[len(r.stack)-1])
	})
	return true
}

// Replace swaps the current view without adding to history.
func (r *Router) Replace(v views.View) {
	r.navigate(func() []func() {
		cur := r.currentLocked()
		if cur != nil {
			r.stack = r.stack[:len(r.stack)-1]
		}
		r.stack = append(r.stack, v)
		return r.planMount(cur, v)
	})
}

// Remount unmounts and mounts the current view again (fresh data).
func (r *Router) Remount() {
	r.navigate(func() []func() {
		cur := r.currentLocked()
		if cur == nil {
			return nil
		}
		return r.planMount(cur, cur)
	})
}

// NavigateTo looks up a view by ID in the registry and navigates to it.
// If the view is already in the stack, pops to it instead of pushing a duplicate.
// Returns false if the view ID is not registered.
func (r *Router) NavigateTo(viewID string) bool {
	r.mu.Lock()
	v, ok := r.registry[viewID]
	r.mu.Unlock()
	if !ok {
		return false
	}
	r.navigate(func() []func() {
		// Check if this view is already in the stack — pop to it instead of pushing a duplicate
		for i := len(r.stack) - 1; i >= 0; i-- {
			if r.stack[i].ID() != viewID {
				continue
			}
			if i == len(r.stack)-1 {
				return nil // already the active view — no-op
			}
			// Unmount all views above the target, from top down
			var calls []func()
			for j := len(r.stack) - 1; j > i; j-- {
				calls = append(calls, r.stack[j].Unmount)
			}
			// Truncate stack to this point + remount
			r.stack = r.stack[:i+1]
			return append(calls, r.planMount(nil, r.stack[i])...)
		}
		return r.planPush(v) // not in stack — push normally
	})
	return true
}

// Current returns the active view, or nil if the stack is empty.
func (r *Router) Current() views.View {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.currentLocked()
}

// StackDepth returns the current navigation depth.
func (r *Router) StackDepth() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.stack)
}

func (r *Router) currentLocked() views.View {
	if len(r.stack) == 0 {
		return nil
	}
	return r.stack[len(r.stack)-1]
}

// mount mounts v, with the stack as it was when the navigation was planned.
func (r *Router) mount(v views.View, stack []views.View, before BeforeMount) {
	if before != nil {
		before(stack)
	}
	r.content.SetDrawFunc(nil) // clear stale resize handler from previous view
	r.content.Clear()
	v.Mount(r.content, r.app)
	r.app.SetFocus(r.content)
	if r.onNavigate != nil {
		r.onNavigate(v)
	}
}
