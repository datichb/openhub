package views

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ─────────────────────────────────────────────────────────────────────────────
// Wizard Navigation — unified keyboard navigation for wizard step pages.
//
// Two levels of navigation:
//   - SetupFormNavigation: internal to a form (arrow remap, DropDown awareness)
//   - SetupCrossSectionNav: between content and button bar (Tab/Backtab/Enter)
//
// These install SetInputCapture on the primitives. Callers that need to
// compose additional captures (e.g., the wizard engine for escPending, Ctrl+S,
// updateDropdownHints) should save the capture via GetInputCapture() after
// calling these functions, then install their own capture that delegates to
// the saved one.
// ─────────────────────────────────────────────────────────────────────────────

// SetupFormNavigation installs arrow-key remapping (Up/Down → Backtab/Tab)
// on a form. DropDown-focused items are excluded (they use Up/Down natively
// to change selection).
//
// This handles INTERNAL navigation only (between form items). Cross-section
// navigation (form ↔ buttonForm) is handled by SetupCrossSectionNav.
func SetupFormNavigation(form *tview.Form) {
	prev := form.GetInputCapture()
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if remapped := remapArrowToTab(form, event); remapped != nil {
			return remapped
		}
		if prev != nil {
			return prev(event)
		}
		return event
	})
}

// CrossSectionNavConfig configures cross-section navigation between
// a content area and a button bar.
type CrossSectionNavConfig struct {
	// App is the tview application (for SetFocus calls).
	App *tview.Application

	// Content is the main interactive area (form, custom widget, or nil).
	// When nil, only button-internal navigation is set up (Left/Right remap).
	Content tview.Primitive

	// Buttons is the button bar form (typically from NewStyledButtonForm).
	Buttons *tview.Form

	// IsContentAtEnd returns true when the content's focus is on its last
	// interactive item. Used to decide when Tab/Down/Enter should jump to
	// the button bar.
	//
	// For *tview.Form content, this is auto-detected (uses isLastFocusableFormItem).
	// For custom widgets (e.g., InlineSelect), the caller provides this.
	// Ignored when Content is nil.
	IsContentAtEnd func() bool
}

// SetupCrossSectionNav wires keyboard navigation between a content area
// and a button bar, providing a consistent UX across all wizard steps:
//
//   - Tab/Down/Enter on the last content item → focus buttons
//   - Backtab/Up from buttons → focus content
//   - Left/Right on buttons → navigate between buttons
//
// For button-only pages (Content=nil), only Left/Right remap is installed.
func SetupCrossSectionNav(cfg CrossSectionNavConfig) {
	if cfg.Buttons == nil {
		return
	}

	// ── Button bar navigation ──
	// Always installed: Left/Right to navigate between buttons.
	// When Content is present: Backtab/Up to return to content.
	prevBtnCapture := cfg.Buttons.GetInputCapture()
	cfg.Buttons.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyLeft:
			return tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone)
		case tcell.KeyRight:
			return tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)
		case tcell.KeyBacktab, tcell.KeyUp:
			if cfg.Content != nil {
				cfg.App.SetFocus(cfg.Content)
				return nil
			}
		}
		if prevBtnCapture != nil {
			return prevBtnCapture(event)
		}
		return event
	})

	// ── Content → button bar navigation ──
	if cfg.Content == nil {
		return
	}

	// Determine the "at end" check.
	isAtEnd := cfg.IsContentAtEnd
	if isAtEnd == nil {
		// Auto-detect for *tview.Form.
		if form, ok := cfg.Content.(*tview.Form); ok {
			isAtEnd = func() bool {
				itemIdx, _ := form.GetFocusedItemIndex()
				return isLastFocusableFormItem(form, itemIdx)
			}
		}
	}

	// If we can't determine "at end" (custom widget without callback), we
	// only support Tab → buttons (no smart last-item detection).
	if isAtEnd == nil {
		isAtEnd = func() bool { return false }
	}

	// Determine if the focused item is a DropDown (Enter should open the
	// popup, not jump to buttons).
	isDropDownFocused := func() bool {
		if form, ok := cfg.Content.(*tview.Form); ok {
			itemIdx, _ := form.GetFocusedItemIndex()
			if itemIdx >= 0 && itemIdx < form.GetFormItemCount() {
				_, isDD := form.GetFormItem(itemIdx).(*tview.DropDown)
				return isDD
			}
		}
		return false
	}

	// inputCapturable is a subset of tview.Box methods for input capture.
	type inputCapturable interface {
		GetInputCapture() func(event *tcell.EventKey) *tcell.EventKey
		SetInputCapture(capture func(event *tcell.EventKey) *tcell.EventKey) *tview.Box
	}

	capturable, ok := cfg.Content.(inputCapturable)
	if !ok {
		return // Content doesn't support SetInputCapture — skip
	}

	prevContentCapture := capturable.GetInputCapture()
	capturable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			if isAtEnd() {
				cfg.App.SetFocus(cfg.Buttons)
				return nil
			}
		case tcell.KeyEnter:
			if isAtEnd() && !isDropDownFocused() {
				cfg.App.SetFocus(cfg.Buttons)
				return nil
			}
		}
		if prevContentCapture != nil {
			return prevContentCapture(event)
		}
		return event
	})
}
