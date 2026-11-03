package shell

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// mouseCapture is the global mouse capture, for the text selection (drag →
// clipboard copy). Events in interactive zones (omnibar, modals) are passed
// through unchanged.
//
// tview calls it again for the follow-up actions of one terminal event
// (move, press, release, click) with the event the previous call returned:
// after a call that consumed one (nil), the event is nil.
func (s *Shell) mouseCapture(event *tcell.EventMouse, action tview.MouseAction) (*tcell.EventMouse, tview.MouseAction) {
	if event == nil {
		return nil, action
	}
	x, y := event.Position()

	switch action {
	case tview.MouseLeftDown:
		if s.isInteractiveZone(x, y) {
			s.selection.Clear()
			return event, action // pass through to tview widgets
		}
		if screen := s.screen; screen != nil {
			s.selection.HandleMouseDown(x, y, screen)
		}
		return nil, tview.MouseConsumed

	case tview.MouseMove:
		// Only intercept a drag: the button was pressed on the selection and
		// is still held. A press at a new position also reports a move with
		// the button down, which is no drag: it must reach the press that
		// follows (a selection left by an earlier click is active).
		if event.Buttons()&tcell.Button1 != 0 && s.selection.IsDragging() {
			s.selection.HandleMouseDrag(x, y)
			s.app.ForceDraw()
			return nil, tview.MouseConsumed
		}

	case tview.MouseLeftUp:
		if s.selection.IsDragging() {
			if screen := s.screen; screen != nil {
				s.selection.HandleMouseUp(x, y, screen)
			}
			return nil, tview.MouseConsumed
		}
	}

	return event, action
}

// isInteractiveZone reports whether the cell at (x, y) belongs to a widget
// that handles its own mouse events. Mouse events in interactive zones are
// passed through to tview unmodified; events outside are routed to the
// SelectionManager for text selection.
//
// Interactive zones:
//   - Omnibar container (always, whether active or not)
//   - Suggestions overlay (when visible)
//   - Any inline-overlay page (modals, prompts) — the entire screen is treated
//     as interactive when a modal is open so that the user cannot accidentally
//     start a selection while interacting with the modal.
func (s *Shell) isInteractiveZone(x, y int) bool {
	// If any overlay modal is active, the whole screen is "interactive" —
	// pass all events to tview so the modal works normally.
	if s.pages.HasPage("inline-overlay") || s.pages.HasPage("sub-overlay") {
		return true
	}

	// Omnibar container
	ox, oy, ow, oh := s.omnibar.container.GetRect()
	if x >= ox && x < ox+ow && y >= oy && y < oy+oh {
		return true
	}

	// Suggestions list (when visible)
	if s.suggestionsShown {
		sx, sy, sw, sh := s.omnibar.SuggestionsList().GetRect()
		if x >= sx && x < sx+sw && y >= sy && y < sy+sh {
			return true
		}
	}

	return false
}
