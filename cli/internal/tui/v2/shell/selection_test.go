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

// ── chebyshevDist ─────────────────────────────────────────────────────────────

func TestChebyshevDist(t *testing.T) {
	assert.Equal(t, 0, chebyshevDist(CellPos{0, 0}, CellPos{0, 0}))
	assert.Equal(t, 3, chebyshevDist(CellPos{0, 0}, CellPos{3, 2}))
	assert.Equal(t, 3, chebyshevDist(CellPos{3, 2}, CellPos{0, 0}))
	assert.Equal(t, 5, chebyshevDist(CellPos{0, 0}, CellPos{5, 3}))
}

// ── selectionState.normalized ─────────────────────────────────────────────────

func TestSelectionState_Normalized_AnchorBeforeCursor(t *testing.T) {
	s := selectionState{
		active: true,
		anchor: CellPos{X: 2, Y: 1},
		cursor: CellPos{X: 8, Y: 3},
	}
	start, end := s.normalized()
	assert.Equal(t, CellPos{X: 2, Y: 1}, start)
	assert.Equal(t, CellPos{X: 8, Y: 3}, end)
}

func TestSelectionState_Normalized_CursorBeforeAnchor(t *testing.T) {
	s := selectionState{
		active: true,
		anchor: CellPos{X: 8, Y: 3},
		cursor: CellPos{X: 2, Y: 1},
	}
	start, end := s.normalized()
	assert.Equal(t, CellPos{X: 2, Y: 1}, start)
	assert.Equal(t, CellPos{X: 8, Y: 3}, end)
}

func TestSelectionState_Normalized_SameLine_AnchorRight(t *testing.T) {
	s := selectionState{
		active: true,
		anchor: CellPos{X: 10, Y: 2},
		cursor: CellPos{X: 3, Y: 2},
	}
	start, end := s.normalized()
	assert.Equal(t, CellPos{X: 3, Y: 2}, start)
	assert.Equal(t, CellPos{X: 10, Y: 2}, end)
}

// ── ExtractText ───────────────────────────────────────────────────────────────

func TestExtractText_SingleLine(t *testing.T) {
	sc := newSimScreen(t, []string{"Hello World"})
	defer sc.Fini()

	sm := NewSelectionManager(nil)
	s := selectionState{
		active: true,
		anchor: CellPos{X: 0, Y: 0},
		cursor: CellPos{X: 4, Y: 0},
	}
	text := sm.ExtractText(s, sc)
	assert.Equal(t, "Hello", text)
}

func TestExtractText_MultiLine(t *testing.T) {
	sc := newSimScreen(t, []string{
		"Hello",
		"World",
	})
	defer sc.Fini()

	sm := NewSelectionManager(nil)
	s := selectionState{
		active: true,
		anchor: CellPos{X: 2, Y: 0}, // "llo"
		cursor: CellPos{X: 2, Y: 1}, // "Wor"
	}
	text := sm.ExtractText(s, sc)
	assert.Contains(t, text, "llo")
	assert.Contains(t, text, "\n")
	assert.Contains(t, text, "Wor")
}

func TestExtractText_InactiveReturnsEmpty(t *testing.T) {
	sc := newSimScreen(t, []string{"Hello"})
	defer sc.Fini()

	sm := NewSelectionManager(nil)
	text := sm.ExtractText(selectionState{active: false}, sc)
	assert.Empty(t, text)
}

func TestExtractText_TrailingSpacesTrimmedPerLine(t *testing.T) {
	sc := newSimScreen(t, []string{"Hi   "})
	defer sc.Fini()

	sm := NewSelectionManager(nil)
	s := selectionState{
		active: true,
		anchor: CellPos{X: 0, Y: 0},
		cursor: CellPos{X: 4, Y: 0},
	}
	text := sm.ExtractText(s, sc)
	// Trailing spaces on the line should be trimmed
	assert.Equal(t, "Hi", text)
}

// ── Multi-click detection ─────────────────────────────────────────────────────

func TestHandleMouseDown_SingleClick_SetsAnchor(t *testing.T) {
	sc := newSimScreen(t, []string{"Hello World"})
	defer sc.Fini()

	sm := NewSelectionManager(nil)
	sm.HandleMouseDown(3, 0, sc)

	sm.mu.Lock()
	defer sm.mu.Unlock()
	assert.True(t, sm.state.active)
	assert.Equal(t, CellPos{X: 3, Y: 0}, sm.state.anchor)
	assert.Equal(t, selectionModeChar, sm.state.mode)
	assert.Equal(t, 1, sm.clickCount)
}

func TestHandleMouseDown_DoubleClick_ExpandsToWord(t *testing.T) {
	sc := newSimScreen(t, []string{"Hello World"})
	defer sc.Fini()

	sm := NewSelectionManager(nil)
	// First click
	sm.HandleMouseDown(2, 0, sc) // middle of "Hello"
	// Second click immediately after (same position)
	sm.HandleMouseDown(2, 0, sc)

	sm.mu.Lock()
	defer sm.mu.Unlock()
	assert.Equal(t, selectionModeWord, sm.state.mode, "double-click should select word")
	assert.Equal(t, 2, sm.clickCount)
}

func TestHandleMouseDown_TripleClick_ExpandsToLine(t *testing.T) {
	sc := newSimScreen(t, []string{"Hello World"})
	defer sc.Fini()

	sm := NewSelectionManager(nil)
	sm.HandleMouseDown(2, 0, sc)
	sm.HandleMouseDown(2, 0, sc)
	sm.HandleMouseDown(2, 0, sc)

	sm.mu.Lock()
	defer sm.mu.Unlock()
	assert.Equal(t, selectionModeLine, sm.state.mode, "triple-click should select line")
	assert.Equal(t, 3, sm.clickCount)
}

func TestHandleMouseDown_SlowDoubleClick_ResetsToSingle(t *testing.T) {
	sc := newSimScreen(t, []string{"Hello World"})
	defer sc.Fini()

	sm := NewSelectionManager(nil)
	sm.HandleMouseDown(2, 0, sc)

	// Manually set last click time to be beyond the threshold
	sm.mu.Lock()
	sm.lastClickTime = time.Now().Add(-(multiClickThreshold + 100*time.Millisecond))
	sm.mu.Unlock()

	sm.HandleMouseDown(2, 0, sc)

	sm.mu.Lock()
	defer sm.mu.Unlock()
	assert.Equal(t, 1, sm.clickCount, "slow second click should reset to single click")
	assert.Equal(t, selectionModeChar, sm.state.mode)
}

// ── IsActive / Clear ─────────────────────────────────────────────────────────

func TestSelectionManager_IsActive(t *testing.T) {
	sm := NewSelectionManager(nil)
	assert.False(t, sm.IsActive())

	sm.mu.Lock()
	sm.state.active = true
	sm.mu.Unlock()
	assert.True(t, sm.IsActive())
}

func TestSelectionManager_Clear(t *testing.T) {
	sm := NewSelectionManager(nil)
	sm.mu.Lock()
	sm.state = selectionState{active: true, anchor: CellPos{1, 1}, cursor: CellPos{5, 3}}
	sm.mu.Unlock()

	sm.Clear()
	assert.False(t, sm.IsActive())
}

// ── HandleMouseDrag ───────────────────────────────────────────────────────────

func TestHandleMouseDrag_UpdatesCursor(t *testing.T) {
	sc := newSimScreen(t, []string{"Hello World"})
	defer sc.Fini()

	sm := NewSelectionManager(nil)
	sm.HandleMouseDown(0, 0, sc)
	sm.HandleMouseDrag(5, 0)

	sm.mu.Lock()
	defer sm.mu.Unlock()
	assert.Equal(t, CellPos{X: 5, Y: 0}, sm.state.cursor)
}

func TestHandleMouseDrag_NoEffectWhenNotActive(t *testing.T) {
	sm := NewSelectionManager(nil)
	sm.HandleMouseDrag(5, 0) // should not panic or activate

	assert.False(t, sm.IsActive())
}

// ── ApplyHighlight ────────────────────────────────────────────────────────────

func TestApplyHighlight_InactiveLeavesScreenUnchanged(t *testing.T) {
	sc := newSimScreen(t, []string{"Hello"})
	defer sc.Fini()

	// Capture original style of first cell
	_, _, styleBefore, _ := sc.GetContent(0, 0)

	sm := NewSelectionManager(nil)
	// selection is inactive — ApplyHighlight must be a no-op
	sm.ApplyHighlight(sc)

	_, _, styleAfter, _ := sc.GetContent(0, 0)
	assert.Equal(t, styleBefore, styleAfter, "inactive selection must not modify any cell")
}

func TestApplyHighlight_ActiveReversesSelectedCells(t *testing.T) {
	sc := newSimScreen(t, []string{"Hello World"})
	defer sc.Fini()

	sm := NewSelectionManager(nil)
	sm.mu.Lock()
	sm.state = selectionState{
		active: true,
		anchor: CellPos{X: 0, Y: 0},
		cursor: CellPos{X: 4, Y: 0},
	}
	sm.mu.Unlock()

	_, _, styleBefore, _ := sc.GetContent(0, 0)
	sm.ApplyHighlight(sc)
	_, _, styleAfter, _ := sc.GetContent(0, 0)

	assert.NotEqual(t, styleBefore, styleAfter, "selected cell style should change (reverse video)")
	// Cell 5 (outside selection) must be unchanged
	_, _, styleOutside, _ := sc.GetContent(5, 0)
	_, _, styleOriginal, _ := sc.GetContent(5, 0)
	assert.Equal(t, styleOriginal, styleOutside, "non-selected cell must be unchanged")
}
