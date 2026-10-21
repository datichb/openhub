package shell

import (
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

// newSimScreen builds a tcell SimScreen pre-filled with ASCII content.
// Each string in rows becomes one screen row (padded or truncated to width).
func newSimScreen(t *testing.T, rows []string) tcell.SimulationScreen {
	t.Helper()
	sc := tcell.NewSimulationScreen("")
	require.NoError(t, sc.Init())
	w := 0
	for _, r := range rows {
		if len(r) > w {
			w = len(r)
		}
	}
	if w == 0 {
		w = 10
	}
	sc.SetSize(w, len(rows))

	st := tcell.StyleDefault
	for y, row := range rows {
		for x, ch := range row {
			sc.SetContent(x, y, ch, nil, st)
		}
		// Pad remaining cells with spaces
		for x := len(row); x < w; x++ {
			sc.SetContent(x, y, ' ', nil, st)
		}
	}
	return sc
}

func TestSelectionGeometry(t *testing.T) {
	t.Run("chebyshev distance", func(t *testing.T) {
		tests := []struct {
			a, b CellPos
			want int
		}{
			{CellPos{0, 0}, CellPos{0, 0}, 0},
			{CellPos{0, 0}, CellPos{3, 2}, 3},
			{CellPos{3, 2}, CellPos{0, 0}, 3},
			{CellPos{0, 0}, CellPos{5, 3}, 5},
		}
		for _, tc := range tests {
			assert.Equal(t, tc.want, chebyshevDist(tc.a, tc.b), "distance %v-%v", tc.a, tc.b)
		}
	})

	t.Run("normalized puts the start first", func(t *testing.T) {
		tests := []struct {
			name           string
			anchor, cursor CellPos
			start, end     CellPos
		}{
			{"anchor before cursor", CellPos{X: 2, Y: 1}, CellPos{X: 8, Y: 3}, CellPos{X: 2, Y: 1}, CellPos{X: 8, Y: 3}},
			{"cursor before anchor", CellPos{X: 8, Y: 3}, CellPos{X: 2, Y: 1}, CellPos{X: 2, Y: 1}, CellPos{X: 8, Y: 3}},
			{"same line, anchor on the right", CellPos{X: 10, Y: 2}, CellPos{X: 3, Y: 2}, CellPos{X: 3, Y: 2}, CellPos{X: 10, Y: 2}},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				start, end := selectionState{active: true, anchor: tc.anchor, cursor: tc.cursor}.normalized()
				assert.Equal(t, tc.start, start, "start")
				assert.Equal(t, tc.end, end, "end")
			})
		}
	})
}

func TestSelectionExtractText(t *testing.T) {
	tests := []struct {
		name           string
		rows           []string
		active         bool
		anchor, cursor CellPos
		want           string
	}{
		{"single line", []string{"Hello World"}, true, CellPos{X: 0, Y: 0}, CellPos{X: 4, Y: 0}, "Hello"},
		{"several lines", []string{"Hello", "World"}, true, CellPos{X: 2, Y: 0}, CellPos{X: 2, Y: 1}, "llo\nWor"},
		{"inactive selection", []string{"Hello"}, false, CellPos{}, CellPos{X: 4}, ""},
		{"trailing spaces trimmed per line", []string{"Hi   "}, true, CellPos{X: 0, Y: 0}, CellPos{X: 4, Y: 0}, "Hi"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sc := newSimScreen(t, tc.rows)
			defer sc.Fini()
			st := selectionState{active: tc.active, anchor: tc.anchor, cursor: tc.cursor}
			assert.Equal(t, tc.want, NewSelectionManager(nil).ExtractText(st, sc), "extracted text")
		})
	}
}

func TestSelectionMouse(t *testing.T) {
	clicks := func(t *testing.T, n int) *SelectionManager {
		t.Helper()
		sc := newSimScreen(t, []string{"Hello World"})
		t.Cleanup(sc.Fini)
		sm := NewSelectionManager(nil)
		for range n {
			sm.HandleMouseDown(2, 0, sc)
		}
		return sm
	}

	t.Run("click count selects a character, a word, then a line", func(t *testing.T) {
		tests := []struct {
			name   string
			clicks int
			mode   selectionMode
		}{
			{"single click", 1, selectionModeChar},
			{"double click", 2, selectionModeWord},
			{"triple click", 3, selectionModeLine},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				sm := clicks(t, tc.clicks)
				sm.mu.Lock()
				defer sm.mu.Unlock()
				assert.True(t, sm.state.active, "selection active")
				assert.Equal(t, tc.mode, sm.state.mode, "selection mode")
				assert.Equal(t, tc.clicks, sm.clickCount, "click count")
			})
		}
	})

	t.Run("single click sets the anchor", func(t *testing.T) {
		sm := clicks(t, 1)
		sm.mu.Lock()
		defer sm.mu.Unlock()
		assert.Equal(t, CellPos{X: 2, Y: 0}, sm.state.anchor, "anchor")
	})

	t.Run("a slow second click starts again", func(t *testing.T) {
		sc := newSimScreen(t, []string{"Hello World"})
		defer sc.Fini()
		sm := NewSelectionManager(nil)
		sm.HandleMouseDown(2, 0, sc)
		sm.mu.Lock()
		sm.lastClickTime = time.Now().Add(-(multiClickThreshold + 100*time.Millisecond))
		sm.mu.Unlock()
		sm.HandleMouseDown(2, 0, sc)

		sm.mu.Lock()
		defer sm.mu.Unlock()
		assert.Equal(t, 1, sm.clickCount, "slow second click resets to a single click")
		assert.Equal(t, selectionModeChar, sm.state.mode, "mode")
	})

	t.Run("drag moves the cursor", func(t *testing.T) {
		sc := newSimScreen(t, []string{"Hello World"})
		defer sc.Fini()
		sm := NewSelectionManager(nil)
		sm.HandleMouseDown(0, 0, sc)
		sm.HandleMouseDrag(5, 0)

		sm.mu.Lock()
		defer sm.mu.Unlock()
		assert.Equal(t, CellPos{X: 5, Y: 0}, sm.state.cursor, "cursor")
	})

	t.Run("drag without a selection does nothing", func(t *testing.T) {
		sm := NewSelectionManager(nil)
		assert.False(t, sm.IsActive(), "inactive at the start")
		sm.HandleMouseDrag(5, 0)
		assert.False(t, sm.IsActive(), "still inactive")
	})

	t.Run("clear ends the selection", func(t *testing.T) {
		sm := clicks(t, 1)
		require.True(t, sm.IsActive(), "active after a click")
		sm.Clear()
		assert.False(t, sm.IsActive(), "inactive after Clear")
	})
}

func TestSelectionHighlight(t *testing.T) {
	t.Run("inactive selection leaves the screen unchanged", func(t *testing.T) {
		sc := newSimScreen(t, []string{"Hello"})
		defer sc.Fini()
		_, before, _ := sc.Get(0, 0)
		NewSelectionManager(nil).ApplyHighlight(sc)
		_, after, _ := sc.Get(0, 0)
		assert.Equal(t, before, after, "inactive selection must not modify any cell")
	})

	t.Run("active selection reverses the selected cells only", func(t *testing.T) {
		sc := newSimScreen(t, []string{"Hello World"})
		defer sc.Fini()
		sm := NewSelectionManager(nil)
		sm.mu.Lock()
		sm.state = selectionState{active: true, anchor: CellPos{X: 0, Y: 0}, cursor: CellPos{X: 4, Y: 0}}
		sm.mu.Unlock()

		_, selectedBefore, _ := sc.Get(0, 0)
		_, outsideBefore, _ := sc.Get(5, 0)
		sm.ApplyHighlight(sc)
		_, selectedAfter, _ := sc.Get(0, 0)
		_, outsideAfter, _ := sc.Get(5, 0)

		assert.NotEqual(t, selectedBefore, selectedAfter, "selected cell style changes (reverse video)")
		assert.Equal(t, outsideBefore, outsideAfter, "non-selected cell is unchanged")
	})
}
