package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type configSnapshot struct {
	Language string
	Enabled  bool
	Count    int
}

func TestUndoStack(t *testing.T) {
	t.Run("push, peek and pop in LIFO order", func(t *testing.T) {
		s := NewUndoStack[string](5)
		assert.True(t, s.IsEmpty(), "new stack is empty")
		_, ok := s.Peek()
		assert.False(t, ok, "Peek on an empty stack")
		_, ok = s.Pop()
		assert.False(t, ok, "Pop on an empty stack")

		s.Push("a")
		s.Push("b")
		s.Push("c")
		require.Equal(t, 3, s.Len(), "length after three pushes")
		assert.False(t, s.IsEmpty(), "stack with items is not empty")

		top, ok := s.Peek()
		assert.True(t, ok, "Peek finds the top")
		assert.Equal(t, "c", top, "Peek returns the last push")
		assert.Equal(t, 3, s.Len(), "Peek does not remove")

		for _, want := range []string{"c", "b", "a"} {
			got, ok := s.Pop()
			assert.True(t, ok, "Pop %q", want)
			assert.Equal(t, want, got, "pop order")
		}
		assert.True(t, s.IsEmpty(), "empty after popping everything")
	})

	t.Run("capacity evicts the oldest", func(t *testing.T) {
		s := NewUndoStack[int](3)
		for i := 1; i <= 4; i++ {
			s.Push(i)
		}
		require.Equal(t, 3, s.Len(), "length is capped")
		for _, want := range []int{4, 3, 2} {
			got, ok := s.Pop()
			assert.True(t, ok, "Pop %d", want)
			assert.Equal(t, want, got, "1 was evicted")
		}
		assert.True(t, s.IsEmpty(), "nothing left after the three entries")
	})

	t.Run("capacity 0 means 10", func(t *testing.T) {
		s := NewUndoStack[int](0)
		for i := 0; i < 12; i++ {
			s.Push(i)
		}
		assert.Equal(t, 10, s.Len(), "default capacity")
	})

	t.Run("clear", func(t *testing.T) {
		s := NewUndoStack[int](5)
		s.Push(1)
		s.Push(2)
		s.Clear()
		assert.True(t, s.IsEmpty(), "empty after Clear")
		assert.Equal(t, 0, s.Len(), "length after Clear")
	})

	t.Run("struct values are copied", func(t *testing.T) {
		s := NewUndoStack[configSnapshot](5)
		original := configSnapshot{Language: "fr", Enabled: true, Count: 1}
		s.Push(original)
		original.Language, original.Count = "en", 99

		restored, ok := s.Pop()
		require.True(t, ok, "Pop succeeds")
		assert.Equal(t, configSnapshot{Language: "fr", Enabled: true, Count: 1}, restored, "the snapshot is not affected by later changes")
	})
}
