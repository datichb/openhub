package shell

import (
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// ── Types ────────────────────────────────────────────────────────────────────

// CellPos is an (x, y) position in absolute screen coordinates.
type CellPos struct {
	X int
	Y int
}

// selectionMode controls what unit the selection extends over after a click.
type selectionMode int

const (
	selectionModeChar selectionMode = iota // drag extends character by character
	selectionModeWord                       // double-click: extend to word boundaries
	selectionModeLine                       // triple-click: extend to full line
)

// multiClickThreshold is the maximum duration between clicks for them to count
// as a multi-click sequence (double / triple).
const multiClickThreshold = 400 * time.Millisecond

// multiClickPosTolerance is the maximum cell distance (Chebyshev) between
// click positions that still counts as the same location.
const multiClickPosTolerance = 2

// ── SelectionState ────────────────────────────────────────────────────────────

// selectionState is the immutable snapshot of the current selection.
type selectionState struct {
	active bool
	anchor CellPos
	cursor CellPos
	mode   selectionMode
}

// normalized returns (start, end) with start <= end in reading order
// (top-to-bottom, left-to-right).
func (s selectionState) normalized() (start, end CellPos) {
	if s.anchor.Y < s.cursor.Y ||
		(s.anchor.Y == s.cursor.Y && s.anchor.X <= s.cursor.X) {
		return s.anchor, s.cursor
	}
	return s.cursor, s.anchor
}

// ── SelectionManager ─────────────────────────────────────────────────────────

// SelectionManager handles text selection across the entire TUI screen.
// It tracks mouse events, renders the selection highlight via SetAfterDrawFunc,
// and copies the selected text to the clipboard on mouse release.
//
// All methods that accept (x, y) expect absolute screen coordinates as
// delivered by tcell.EventMouse.Position().
type SelectionManager struct {
	mu    sync.Mutex
	state selectionState

	// Multi-click detection
	lastClickTime time.Time
	lastClickPos  CellPos
	clickCount    int

	// onCopy is called after text is extracted and copied. Intended to show a toast.
	onCopy func(text string)

	// screenWidth / screenHeight are updated before each highlight pass.
	screenWidth  int
	screenHeight int
}

// NewSelectionManager creates a SelectionManager.
// onCopy is called with the copied text after a successful mouse-up.
func NewSelectionManager(onCopy func(text string)) *SelectionManager {
	return &SelectionManager{onCopy: onCopy}
}

// IsActive reports whether a selection is currently in progress or visible.
func (sm *SelectionManager) IsActive() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.state.active
}

// Clear resets the selection state.
func (sm *SelectionManager) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.state = selectionState{}
}

// ── Mouse handling ────────────────────────────────────────────────────────────

// HandleMouseDown is called when the primary mouse button is pressed.
// It sets the selection anchor and determines the mode based on click count.
func (sm *SelectionManager) HandleMouseDown(x, y int, screen tcell.Screen) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	pos := CellPos{X: x, Y: y}

	if now.Sub(sm.lastClickTime) <= multiClickThreshold &&
		chebyshevDist(pos, sm.lastClickPos) <= multiClickPosTolerance {
		sm.clickCount++
	} else {
		sm.clickCount = 1
	}
	sm.lastClickTime = now
	sm.lastClickPos = pos

	switch sm.clickCount % 3 {
	case 0: // triple
		anchor, cursor := sm.expandToLine(x, y, screen)
		sm.state = selectionState{
			active: true,
			anchor: anchor,
			cursor: cursor,
			mode:   selectionModeLine,
		}
	case 2: // double
		anchor, cursor := sm.expandToWord(x, y, screen)
		sm.state = selectionState{
			active: true,
			anchor: anchor,
			cursor: cursor,
			mode:   selectionModeWord,
		}
	default: // single
		sm.state = selectionState{
			active: true,
			anchor: pos,
			cursor: pos,
			mode:   selectionModeChar,
		}
	}
}

// HandleMouseDrag is called when the mouse moves with the button held.
// It extends the selection cursor to the current position.
func (sm *SelectionManager) HandleMouseDrag(x, y int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if !sm.state.active {
		return
	}
	sm.state.cursor = CellPos{X: x, Y: y}
}

// HandleMouseUp is called when the primary mouse button is released.
// If a selection is active, it extracts the text, copies it to the clipboard,
// and fires the onCopy callback.
func (sm *SelectionManager) HandleMouseUp(x, y int, screen tcell.Screen) {
	sm.mu.Lock()
	sm.state.cursor = CellPos{X: x, Y: y}
	active := sm.state.active
	stateCopy := sm.state
	sm.mu.Unlock()

	if !active {
		return
	}

	text := sm.ExtractText(stateCopy, screen)
	text = strings.TrimRight(text, " \n")
	if text == "" {
		return
	}

	// Copy in a goroutine so we don't block the event loop
	cb := sm.onCopy
	go func() {
		_ = CopyToClipboard(text)
		if cb != nil {
			cb(text)
		}
	}()
}

// ── Highlight ─────────────────────────────────────────────────────────────────

// ApplyHighlight overrides the style of all cells within the current selection
// with reverse video. Called from SetAfterDrawFunc — screen cells have already
// been written by tview primitives and screen.Show() has not yet been called.
func (sm *SelectionManager) ApplyHighlight(screen tcell.Screen) {
	sm.mu.Lock()
	s := sm.state
	sm.mu.Unlock()

	if !s.active {
		return
	}

	w, h := screen.Size()
	sm.mu.Lock()
	sm.screenWidth = w
	sm.screenHeight = h
	sm.mu.Unlock()

	start, end := s.normalized()

	for row := start.Y; row <= end.Y; row++ {
		if row < 0 || row >= h {
			continue
		}
		colStart := 0
		colEnd := w - 1
		if row == start.Y {
			colStart = start.X
		}
		if row == end.Y {
			colEnd = end.X
		}

		for col := colStart; col <= colEnd; col++ {
			if col < 0 || col >= w {
				continue
			}
			mainc, combc, style, _ := screen.GetContent(col, row)
			screen.SetContent(col, row, mainc, combc, style.Reverse(true))
		}
	}
}

// ── Text extraction ───────────────────────────────────────────────────────────

// ExtractText reads the plain text content of the selected region from the
// screen buffer. Each row is terminated by a newline; trailing spaces per row
// are trimmed. The caller should trim the final result as needed.
func (sm *SelectionManager) ExtractText(s selectionState, screen tcell.Screen) string {
	if !s.active {
		return ""
	}

	w, h := screen.Size()
	start, end := s.normalized()

	var sb strings.Builder
	for row := start.Y; row <= end.Y; row++ {
		if row < 0 || row >= h {
			continue
		}
		colStart := 0
		colEnd := w - 1
		if row == start.Y {
			colStart = start.X
		}
		if row == end.Y {
			colEnd = end.X
		}

		var line strings.Builder
		for col := colStart; col <= colEnd; col++ {
			if col < 0 || col >= w {
				continue
			}
			mainc, _, _, _ := screen.GetContent(col, row)
			if mainc == 0 {
				mainc = ' '
			}
			line.WriteRune(mainc)
		}
		// Trim trailing spaces on each line
		trimmed := strings.TrimRight(line.String(), " ")
		sb.WriteString(trimmed)
		if row < end.Y {
			sb.WriteRune('\n')
		}
	}
	return sb.String()
}

// ── Multi-click expansion ─────────────────────────────────────────────────────

// expandToWord expands the selection from (x, y) to cover the full word at
// that cell. A "word" is a maximal run of non-space, non-punctuation runes.
// Returns (anchor, cursor) — anchor <= cursor in reading order.
func (sm *SelectionManager) expandToWord(x, y int, screen tcell.Screen) (anchor, cursor CellPos) {
	w, h := screen.Size()
	if y < 0 || y >= h {
		p := CellPos{X: x, Y: y}
		return p, p
	}

	isWordRune := func(col int) bool {
		if col < 0 || col >= w {
			return false
		}
		r, _, _, _ := screen.GetContent(col, y)
		return r != 0 && !unicode.IsSpace(r) && !unicode.IsPunct(r)
	}

	left := x
	for left > 0 && isWordRune(left-1) {
		left--
	}
	right := x
	for right < w-1 && isWordRune(right+1) {
		right++
	}
	return CellPos{X: left, Y: y}, CellPos{X: right, Y: y}
}

// expandToLine expands the selection to the full visible line at row y.
// Returns (anchor, cursor) with anchor at col 0 and cursor at last non-space col.
func (sm *SelectionManager) expandToLine(x, y int, screen tcell.Screen) (anchor, cursor CellPos) {
	w, _ := screen.Size()
	_ = x // unused — line selection ignores the click column

	// Find last non-space character on the line
	lastCol := 0
	for col := w - 1; col >= 0; col-- {
		r, _, _, _ := screen.GetContent(col, y)
		if r != 0 && r != ' ' {
			lastCol = col
			break
		}
	}
	return CellPos{X: 0, Y: y}, CellPos{X: lastCol, Y: y}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// chebyshevDist returns the Chebyshev (chessboard) distance between two cells.
// This is used for multi-click position tolerance.
func chebyshevDist(a, b CellPos) int {
	dx := a.X - b.X
	if dx < 0 {
		dx = -dx
	}
	dy := a.Y - b.Y
	if dy < 0 {
		dy = -dy
	}
	if dx > dy {
		return dx
	}
	return dy
}
