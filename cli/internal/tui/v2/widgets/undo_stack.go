// Package widgets provides reusable tview primitives for the TUI shell.
package widgets

// UndoStack is a bounded, generic undo history for configuration views.
// It stores snapshots of arbitrary state, allowing any view to offer
// undo (u key) after auto-save mutations.
//
// Usage:
//
//	stack := NewUndoStack[MyState](10)
//	stack.Push(currentState)  // before mutation
//	applyMutation()
//	// on undo:
//	if prev, ok := stack.Pop(); ok {
//	    restoreState(prev)
//	}
type UndoStack[T any] struct {
	items []T
	cap   int
}

// NewUndoStack creates an undo stack with the given maximum depth.
// When capacity is exceeded, the oldest entry is discarded.
func NewUndoStack[T any](capacity int) *UndoStack[T] {
	if capacity <= 0 {
		capacity = 10
	}
	return &UndoStack[T]{
		items: make([]T, 0, capacity),
		cap:   capacity,
	}
}

// Push stores a snapshot. If the stack is at capacity, the oldest entry
// is evicted to make room.
func (s *UndoStack[T]) Push(state T) {
	if len(s.items) >= s.cap {
		// Shift left, dropping the oldest entry.
		copy(s.items, s.items[1:])
		s.items = s.items[:len(s.items)-1]
	}
	s.items = append(s.items, state)
}

// Pop removes and returns the most recent snapshot.
// Returns false if the stack is empty.
func (s *UndoStack[T]) Pop() (T, bool) {
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}
	last := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return last, true
}

// Peek returns the most recent snapshot without removing it.
// Returns false if the stack is empty.
func (s *UndoStack[T]) Peek() (T, bool) {
	if len(s.items) == 0 {
		var zero T
		return zero, false
	}
	return s.items[len(s.items)-1], true
}

// Len returns the current number of stored snapshots.
func (s *UndoStack[T]) Len() int {
	return len(s.items)
}

// Clear removes all entries from the stack.
func (s *UndoStack[T]) Clear() {
	s.items = s.items[:0]
}

// IsEmpty reports whether the stack has no entries.
func (s *UndoStack[T]) IsEmpty() bool {
	return len(s.items) == 0
}
