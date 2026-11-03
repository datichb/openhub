package shell

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// The windows: one rule for opening, closing and giving the focus back
// ─────────────────────────────────────────────────────────────────────────────
//
// A window opened while others are shown opens over them, on the next page of
// windowPages; closing a window removes the ones over it and gives the focus
// back to the widget that had it when the window was opened (the button of
// the window under it, the widget of the view for the first one). Nothing is
// left on the screen without the focus, and the window under comes back as
// it was.

// windowPages are the pages of the windows, from the first to the one over
// all the others. A window opened with the pages full replaces the last.
var windowPages = [...]string{"inline-overlay", "sub-overlay", "sub-overlay-2"}

// window is one window of the stack.
type window struct {
	page  string
	depth int
	// back is where the focus goes back when the window closes.
	back tview.Primitive
	// viewID is the view that was shown when the window opened: the widget
	// of another view is no place to give the focus back to.
	viewID string
	// prompt marks a question (confirmation, input, form, choice). Once it
	// is answered, the windows it was asked from close with it: their action
	// is done. Cancelled, it gives them back.
	prompt bool
}

// sub reports whether the window is over another one.
func (w *window) sub() bool { return w.depth > 0 }

// backdrop is the color of what the window dims: darker over another window.
func (w *window) backdrop() tcell.Color {
	if w.sub() {
		return theme.BgDimSubOverlay
	}
	return theme.BgDimOverlay
}

// openWindow reserves the place of a window that shows a content: the first
// when none is open, else over the open ones. back is where the focus goes
// back when it closes (nil: where it is now). It is called right before
// the page is added, with the focus as it is when the user asked for the
// window.
func (s *Shell) openWindow(back tview.Primitive) *window {
	depth := 0
	for depth < len(windowPages) && s.pages.HasPage(windowPages[depth]) {
		depth++
	}
	if depth == len(windowPages) { // full: the last one is replaced
		depth--
		if back == nil && s.windows[depth] != nil {
			back = s.windows[depth].back
		}
	}
	if back == nil {
		back = s.app.GetFocus()
	}
	w := &window{page: windowPages[depth], depth: depth, back: back, viewID: s.currentViewID()}
	s.windows[depth] = w
	return w
}

// openPrompt is openWindow for a question.
func (s *Shell) openPrompt() *window {
	w := s.openWindow(nil)
	w.prompt = true
	return w
}

// windowOpen reports whether w is still shown (not closed, not replaced).
func (s *Shell) windowOpen(w *window) bool {
	return w != nil && s.windows[w.depth] == w && s.pages.HasPage(w.page)
}

// closeWindow closes w and the windows over it, and gives the focus back. It
// does nothing when w is already closed.
func (s *Shell) closeWindow(w *window) {
	if !s.windowOpen(w) {
		return
	}
	for d := len(windowPages) - 1; d >= w.depth; d-- {
		s.pages.RemovePage(windowPages[d])
		s.windows[d] = nil
	}
	s.app.SetFocus(s.focusAfter(w))
}

// closeWindows closes every window.
func (s *Shell) closeWindows() {
	if first := s.windows[0]; s.windowOpen(first) {
		s.closeWindow(first)
	}
}

// answerWindow closes w because it was answered, and runs then (what the
// answer does). A question closes the windows it was asked from with it
// (the action they offered is done), unless then asks another question over
// them: « Passer outre » asks a reason, then a confirmation, and the card
// stays under both. Any other window just closes.
func (s *Shell) answerWindow(w *window, then func()) {
	var below *window
	if w != nil && w.prompt && w.depth > 0 {
		below = s.windows[w.depth-1]
	}
	s.closeWindow(w)
	if then != nil {
		then()
	}
	if below == nil || !s.windowOpen(below) {
		return
	}
	if over := s.windows[below.depth+1]; over != nil && over.prompt && s.windowOpen(over) {
		return
	}
	s.closeWindows()
}

// hasWindowOver reports whether a window is shown over w (a list opened from
// a form, a quit dialog).
func (s *Shell) hasWindowOver(w *window) bool {
	return w.depth+1 < len(windowPages) && s.pages.HasPage(windowPages[w.depth+1])
}

// focusAfter is where the focus goes back when w closes.
func (s *Shell) focusAfter(w *window) tview.Primitive {
	if w.back == nil {
		return s.content
	}
	if w.depth == 0 {
		// The widget belongs to the view that was shown; the palette input
		// only takes the focus while the palette is open.
		if s.currentViewID() != w.viewID {
			return s.content
		}
		if w.back == s.omnibar.input && !s.omnibar.IsActive() {
			return s.content
		}
	}
	return w.back
}

// currentViewID is the ID of the view shown ("" when none).
func (s *Shell) currentViewID() string {
	if cur := s.router.Current(); cur != nil {
		return cur.ID()
	}
	return ""
}
