package widgets

import (
	"testing"
)

func TestNewUndoStack_DefaultCapacity(t *testing.T) {
	s := NewUndoStack[int](0)
	if s.cap != 10 {
		t.Errorf("expected default capacity 10, got %d", s.cap)
	}
}

func TestUndoStack_PushAndPop(t *testing.T) {
	s := NewUndoStack[string](5)

	s.Push("a")
	s.Push("b")
	s.Push("c")

	if s.Len() != 3 {
		t.Fatalf("expected len 3, got %d", s.Len())
	}

	val, ok := s.Pop()
	if !ok || val != "c" {
		t.Errorf("expected (c, true), got (%s, %v)", val, ok)
	}

	val, ok = s.Pop()
	if !ok || val != "b" {
		t.Errorf("expected (b, true), got (%s, %v)", val, ok)
	}

	val, ok = s.Pop()
	if !ok || val != "a" {
		t.Errorf("expected (a, true), got (%s, %v)", val, ok)
	}

	_, ok = s.Pop()
	if ok {
		t.Error("expected Pop on empty stack to return false")
	}
}

func TestUndoStack_CapacityEviction(t *testing.T) {
	s := NewUndoStack[int](3)

	s.Push(1)
	s.Push(2)
	s.Push(3)
	s.Push(4) // should evict 1

	if s.Len() != 3 {
		t.Fatalf("expected len 3 after eviction, got %d", s.Len())
	}

	val, _ := s.Pop()
	if val != 4 {
		t.Errorf("expected 4, got %d", val)
	}
	val, _ = s.Pop()
	if val != 3 {
		t.Errorf("expected 3, got %d", val)
	}
	val, _ = s.Pop()
	if val != 2 {
		t.Errorf("expected 2, got %d", val)
	}
	_, ok := s.Pop()
	if ok {
		t.Error("expected empty after popping all items")
	}
}

func TestUndoStack_Peek(t *testing.T) {
	s := NewUndoStack[string](5)

	_, ok := s.Peek()
	if ok {
		t.Error("Peek on empty stack should return false")
	}

	s.Push("x")
	val, ok := s.Peek()
	if !ok || val != "x" {
		t.Errorf("expected (x, true), got (%s, %v)", val, ok)
	}

	// Peek should not remove
	if s.Len() != 1 {
		t.Errorf("Peek should not change length, got %d", s.Len())
	}
}

func TestUndoStack_Clear(t *testing.T) {
	s := NewUndoStack[int](5)
	s.Push(1)
	s.Push(2)
	s.Clear()

	if !s.IsEmpty() {
		t.Error("expected stack to be empty after Clear")
	}
	if s.Len() != 0 {
		t.Errorf("expected len 0 after Clear, got %d", s.Len())
	}
}

func TestUndoStack_IsEmpty(t *testing.T) {
	s := NewUndoStack[int](3)
	if !s.IsEmpty() {
		t.Error("new stack should be empty")
	}
	s.Push(42)
	if s.IsEmpty() {
		t.Error("stack with one item should not be empty")
	}
	s.Pop()
	if !s.IsEmpty() {
		t.Error("stack should be empty after popping all items")
	}
}

// TestUndoStack_StructSnapshot verifies the stack works with struct values
// (copy semantics), which is the primary use case for config state snapshots.
type configSnapshot struct {
	Language string
	Enabled  bool
	Count    int
}

func TestUndoStack_StructSnapshot(t *testing.T) {
	s := NewUndoStack[configSnapshot](5)

	original := configSnapshot{Language: "fr", Enabled: true, Count: 1}
	s.Push(original)

	// Mutate the original — should not affect the stack
	original.Language = "en"
	original.Count = 99

	restored, ok := s.Pop()
	if !ok {
		t.Fatal("expected Pop to succeed")
	}
	if restored.Language != "fr" || restored.Count != 1 {
		t.Errorf("expected original snapshot, got %+v", restored)
	}
}
