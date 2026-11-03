package shell

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Global key handler — the key contract
// ─────────────────────────────────────────────────────────────────────────────
//
// Layers, in order:
//  1. urgency: Ctrl+Q / Ctrl+C (quit), Esc clearing a text selection;
//  2. the palette, the overlays (modals, forms) and the views that capture
//     input (forms, wizards) receive every other key, Esc included (it
//     does not close the toasts there);
//  3. reserved keys, never redefined by a view: Esc (closes the toasts
//     shown, then goes back), Ctrl+P, Ctrl+T, ? and F1;
//  4. the active view (HandleKey);
//  5. the focused widget: Enter and the letters are never taken from it;
//  6. fallbacks: j/k/g/G become arrows/Home/End, / opens the palette, Enter
//     opens the palette only when the focused widget does not use it.
//
// Each layer is a method returning the event to hand back to tview and
// whether it settled the key.

func (s *Shell) globalKeyHandler(event *tcell.EventKey) *tcell.EventKey {
	if ev, done := s.urgentKey(event); done {
		return ev
	}
	// The palette and the windows take the keys they are given.
	if s.omnibar.IsActive() || s.hasOverlay() {
		return event
	}
	// F1 opens the help in every view, forms and editors included.
	if event.Key() == tcell.KeyF1 {
		s.showHelpOverlay()
		return nil
	}
	cur := s.router.Current()
	if cur != nil {
		if ic, ok := cur.(views.InputCapturing); ok && ic.CapturesInput() {
			return s.capturingViewKey(cur, event)
		}
	}
	// Esc closes the toasts shown before going back.
	if event.Key() == tcell.KeyEscape && s.DismissToasts() {
		return nil
	}
	if ev, done := s.reservedKey(event); done {
		return ev
	}
	focus := s.app.GetFocus()
	textInput := focusIsTextInput(focus)
	if event.Key() == tcell.KeyRune && keys.Help.Matches(event) && !textInput {
		s.showHelpOverlay()
		return nil
	}

	// Delegate to current view's key handler.
	// Note: Tab is intentionally NOT intercepted here — views that want to
	// use Tab for panel switching (e.g. MetricsView) capture it in HandleKey.
	// Views that don't handle Tab return the event, and tview's native focus
	// cycling takes over (Phase 2).
	if cur != nil && cur.HandleKey(event) == nil {
		return nil // view consumed the event
	}

	// A text field keeps every key the view did not take.
	if textInput {
		return event
	}
	return s.fallbackKey(event, focus)
}

// urgentKey is the first layer: quitting, the « terminal too small » screen
// and the Esc that clears a text selection.
func (s *Shell) urgentKey(event *tcell.EventKey) (*tcell.EventKey, bool) {
	// Ctrl+Q / Ctrl+C: quit (always available)
	if event.Key() == tcell.KeyCtrlQ || event.Key() == tcell.KeyCtrlC {
		s.RequestQuit()
		return nil, true
	}
	// « Terminal too small »: only quitting (and the quit window) work.
	if s.tooSmall.Load() && !s.hasOverlay() {
		return nil, true
	}
	// Esc: if a text selection is active, clear it first.
	// This avoids accidentally triggering navigation or modal-close while
	// the user is dismissing a selection.
	if event.Key() == tcell.KeyEscape && s.selection.IsActive() {
		s.selection.Clear()
		s.app.ForceDraw()
		return nil, true // consume the Esc — do not propagate
	}
	return event, false
}

// capturingViewKey handles the keys of a view that captures input (inline
// wizards with forms). Only Ctrl+Q/C and the selection-clear Esc remain
// global. Everything else is delegated to HandleKey first; if the view
// doesn't consume the event, it falls through to tview's normal focus-based
// widget delivery (forms, etc.). Esc keeps its meaning there (close the
// form, « Quit the wizard? »): it does not close the toasts, which go away
// on their own.
func (s *Shell) capturingViewKey(cur views.View, event *tcell.EventKey) *tcell.EventKey {
	if cur.HandleKey(event) == nil {
		return nil // view consumed the event
	}
	// ? and Ctrl+P the view did not use (tabs, batch).
	if p, ok := cur.(views.ShellKeysPassing); ok && p.PassesShellKeys() {
		switch {
		case event.Key() == tcell.KeyCtrlP:
			s.omnibar.Activate()
			return nil
		case event.Key() == tcell.KeyRune && keys.Help.Matches(event) && !focusIsTextInput(s.app.GetFocus()):
			s.showHelpOverlay()
			return nil
		}
	}
	// View didn't consume → let tview deliver to the focused widget
	return event
}

// reservedKey handles Ctrl+P, Ctrl+T and Esc, which no view redefines.
func (s *Shell) reservedKey(event *tcell.EventKey) (*tcell.EventKey, bool) {
	switch event.Key() {
	case tcell.KeyCtrlP:
		s.omnibar.Activate()
		return nil, true
	case tcell.KeyCtrlT:
		s.cycleMode()
		return nil, true
	case tcell.KeyEsc:
		s.escapeKey()
		return nil, true
	}
	return event, false
}

// escapeKey goes back one view, or exits to the hub at the root of a
// non-hub mode (Ctrl+T to come back).
func (s *Shell) escapeKey() {
	if s.router.StackDepth() > 1 {
		s.router.Pop()
		return
	}
	if s.activeMode != views.ModeHub {
		s.SetMode(views.ModeHub)
	}
}

// fallbackKey is the last layer: j/k/g/G are the universal list keys
// (arrows, Home, End); / opens the palette when the view has no filter; every
// other letter goes to the focused widget (key convention: package keys).
// Enter belongs to the focused widget; the palette opens only when the widget
// does not use it.
func (s *Shell) fallbackKey(event *tcell.EventKey, focus tview.Primitive) *tcell.EventKey {
	if event.Key() == tcell.KeyRune {
		switch {
		case keys.Down.Matches(event):
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case keys.Up.Matches(event):
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case keys.First.Matches(event):
			return tcell.NewEventKey(tcell.KeyHome, 0, tcell.ModNone)
		case keys.Last.Matches(event):
			return tcell.NewEventKey(tcell.KeyEnd, 0, tcell.ModNone)
		case keys.Filter.Matches(event):
			s.omnibar.Activate()
			return nil
		}
		return event
	}
	if event.Key() == tcell.KeyEnter && !focusAcceptsEnter(focus) {
		s.omnibar.Activate()
		return nil
	}
	return event
}

// capturesKeys reports whether the current view keeps every key (forms,
// wizards); a view that passes ? and Ctrl+P to the shell does not block
// the palette.
func (s *Shell) capturesKeys() bool {
	cur := s.router.Current()
	if cur == nil {
		return false
	}
	ic, ok := cur.(views.InputCapturing)
	if !ok || !ic.CapturesInput() {
		return false
	}
	p, ok := cur.(views.ShellKeysPassing)
	return !ok || !p.PassesShellKeys()
}

// focusReturn is where the focus goes back after the palette or the help:
// the focused widget in a view that keeps the keys (a tab), the content
// otherwise.
func (s *Shell) focusReturn() tview.Primitive {
	if cur := s.router.Current(); cur != nil {
		if ic, ok := cur.(views.InputCapturing); ok && ic.CapturesInput() {
			if f := s.app.GetFocus(); f != nil {
				return f
			}
		}
	}
	return s.content
}

// enterAcceptor is implemented by custom widgets that use Enter.
type enterAcceptor interface{ AcceptsEnter() bool }

// listLike matches tview.List and the widgets embedding it (SectionedList).
type listLike interface {
	SetSelectedFunc(func(int, string, string, rune)) *tview.List
}

// focusAcceptsEnter reports whether the focused widget uses Enter (lists,
// tables, buttons, forms and their fields, trees).
func focusAcceptsEnter(p tview.Primitive) bool {
	switch w := p.(type) {
	case nil:
		return false
	case enterAcceptor:
		return w.AcceptsEnter()
	case *tview.List, *tview.Table, *tview.Button, *tview.Form, *tview.InputField,
		*tview.DropDown, *tview.TreeView, *tview.Checkbox, *tview.TextArea:
		return true
	case listLike:
		return true
	}
	return false
}

// focusIsTextInput reports whether the focused widget is a text field: it
// keeps the letters, ? and / included.
func focusIsTextInput(p tview.Primitive) bool {
	switch p.(type) {
	case *tview.InputField, *tview.TextArea:
		return true
	}
	return false
}
