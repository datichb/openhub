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

// Router maintains a stack of views for back-navigation.
type Router struct {
	stack      []views.View
	registry   map[string]views.View
	content    *tview.Flex
	app        *tview.Application
	onNavigate OnNavigate
	mu         sync.Mutex
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

// Push navigates to a new view, unmounting the current one.
// The previous view remains on the stack for Pop().
func (r *Router) Push(v views.View) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cur := r.currentLocked(); cur != nil && cur.ID() == v.ID() {
		return // already on this view — no-op
	}

	if cur := r.currentLocked(); cur != nil {
		cur.Unmount()
	}

	r.stack = append(r.stack, v)
	r.mountLocked(v)
}

// Pop returns to the previous view. Returns false if stack has 1 or fewer items.
func (r *Router) Pop() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.stack) <= 1 {
		return false
	}

	cur := r.stack[len(r.stack)-1]
	cur.Unmount()
	r.stack = r.stack[:len(r.stack)-1]

	prev := r.stack[len(r.stack)-1]
	r.mountLocked(prev)
	return true
}

// Replace swaps the current view without adding to history.
func (r *Router) Replace(v views.View) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if cur := r.currentLocked(); cur != nil {
		cur.Unmount()
		r.stack = r.stack[:len(r.stack)-1]
	}

	r.stack = append(r.stack, v)
	r.mountLocked(v)
}

// NavigateTo looks up a view by ID in the registry and navigates to it.
// If the view is already in the stack, pops to it instead of pushing a duplicate.
// Returns false if the view ID is not registered.
func (r *Router) NavigateTo(viewID string) bool {
	r.mu.Lock()
	v, ok := r.registry[viewID]
	if !ok {
		r.mu.Unlock()
		return false
	}

	// Check if this view is already in the stack — pop to it instead of pushing a duplicate
	for i := len(r.stack) - 1; i >= 0; i-- {
		if r.stack[i].ID() == viewID {
			// Unmount everything above it
			if cur := r.currentLocked(); cur != nil && cur.ID() != viewID {
				cur.Unmount()
			}
			// Truncate stack to this point + remount
			r.stack = r.stack[:i+1]
			r.mountLocked(r.stack[i])
			r.mu.Unlock()
			return true
		}
	}
	r.mu.Unlock()

	// Not in stack — push normally
	r.Push(v)
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

func (r *Router) mountLocked(v views.View) {
	r.content.Clear()
	v.Mount(r.content, r.app)
	if r.onNavigate != nil {
		r.onNavigate(v)
	}
}
