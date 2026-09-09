package shell

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// modalConfig describes a modal to build and display.
// ─────────────────────────────────────────────────────────────────────────────

type modalConfig struct {
	// Title displayed in the border frame — padded uniformly.
	Title string
	// Content is the main interactive primitive (TextView, List, Form, etc.).
	Content tview.Primitive
	// Actions are buttons at the bottom. nil = no button bar.
	Actions []views.ModalAction

	// Sizing
	Size modalSizeHint

	// Hints is a short help line displayed below the title inside the frame,
	// in muted color. e.g. "↑↓ naviguer · Enter confirmer · Esc fermer".
	// Empty string = no hints line.
	Hints string

	// Behavior
	LightDismiss bool            // click outside to close
	PageName     string          // "inline-overlay" or "sub-overlay"
	FocusReturn  tview.Primitive // where to return focus on dismiss
	IsSub        bool            // use darker backdrop for sub-overlay depth

	// FocusTarget is the primitive to focus initially. If nil, Content is focused.
	FocusTarget tview.Primitive
}

// ─────────────────────────────────────────────────────────────────────────────
// showModal builds and displays a modal using the unified layout.
// ─────────────────────────────────────────────────────────────────────────────

func (s *Shell) showModal(cfg modalConfig) {
	frame, focusables := s.buildModalFrame(cfg)

	size := s.computeModalSize(cfg.Size)

	bg := theme.BgDimOverlay
	if cfg.IsSub {
		bg = theme.BgDimSubOverlay
	}

	dismissPage := ""
	if cfg.LightDismiss {
		dismissPage = cfg.PageName
	}

	grid := s.overlayGrid(frame, size.cols, size.rows, bg, dismissPage, cfg.FocusReturn)

	s.pages.AddPage(cfg.PageName, grid, true, true)

	// Set initial focus
	if cfg.FocusTarget != nil {
		s.app.SetFocus(cfg.FocusTarget)
	} else if len(focusables) > 0 {
		s.app.SetFocus(focusables[0])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// buildModalFrame constructs the visual frame: hints + content + separator + buttons.
// Returns the assembled frame and the list of focusable primitives (for Tab cycling).
// ─────────────────────────────────────────────────────────────────────────────

func (s *Shell) buildModalFrame(cfg modalConfig) (tview.Primitive, []tview.Primitive) {
	frame := tview.NewFlex().SetDirection(tview.FlexRow)
	frame.SetBackgroundColor(theme.BgPanel)
	frame.SetBorder(true)
	frame.SetBorderColor(theme.Accent)

	// ── Title ──
	titleText := cfg.Title
	if titleText != "" {
		pad := strings.Repeat(" ", theme.ModalTitlePad)
		frame.SetTitle(fmt.Sprintf("%s%s%s", pad, titleText, pad))
		frame.SetTitleColor(theme.Accent)
	}

	// ── Hints line (below title, inside frame) ──
	if cfg.Hints != "" {
		hints := tview.NewTextView().
			SetText("  " + cfg.Hints).
			SetTextColor(theme.FgMuted).
			SetDynamicColors(false)
		hints.SetBackgroundColor(theme.BgPanel)
		frame.AddItem(hints, 1, 0, false)
	}

	// ── Content ──
	var focusables []tview.Primitive
	frame.AddItem(cfg.Content, 0, 1, true) // flex weight 1 — fills available space
	focusables = append(focusables, cfg.Content)

	// ── Separator + Button bar ──
	if len(cfg.Actions) > 0 {
		// Separator line
		sep := newSeparator()
		frame.AddItem(sep, 1, 0, false)

		// Button bar (centered)
		buttonBar, buttonFocusables := buildButtonBar(cfg.Actions, func() {
			s.pages.RemovePage(cfg.PageName)
			if cfg.FocusReturn != nil {
				s.app.SetFocus(cfg.FocusReturn)
			} else {
				s.app.SetFocus(s.content)
			}
		})
		frame.AddItem(buttonBar, 1, 0, false)
		focusables = append(focusables, buttonFocusables...)

		// Extra bottom padding
		pad := tview.NewBox().SetBackgroundColor(theme.BgPanel)
		frame.AddItem(pad, 1, 0, false)
	}

	// ── Tab cycling for focusables ──
	if len(focusables) > 1 {
		currentFocus := 0
		frame.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyTab || event.Key() == tcell.KeyBacktab {
				if event.Key() == tcell.KeyBacktab {
					currentFocus = (currentFocus - 1 + len(focusables)) % len(focusables)
				} else {
					currentFocus = (currentFocus + 1) % len(focusables)
				}
				s.app.SetFocus(focusables[currentFocus])
				return nil
			}
			if event.Key() == tcell.KeyEscape {
				s.pages.RemovePage(cfg.PageName)
				if cfg.FocusReturn != nil {
					s.app.SetFocus(cfg.FocusReturn)
				} else {
					s.app.SetFocus(s.content)
				}
				return nil
			}
			return event
		})
	}

	return frame, focusables
}

// ─────────────────────────────────────────────────────────────────────────────
// Separator — thin horizontal rule between content and buttons
// ─────────────────────────────────────────────────────────────────────────────

func newSeparator() *tview.Box {
	sep := tview.NewBox().SetBackgroundColor(theme.BgPanel)
	sep.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		style := tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.BorderCard)
		for dx := 0; dx < width; dx++ {
			screen.SetContent(x+dx, y, theme.ModalSeparatorRune, nil, style)
		}
		return x, y, width, height
	})
	return sep
}

// ─────────────────────────────────────────────────────────────────────────────
// Button bar — horizontally centered buttons with primary accent on first
// ─────────────────────────────────────────────────────────────────────────────

func buildButtonBar(actions []views.ModalAction, dismiss func()) (*tview.Flex, []tview.Primitive) {
	bar := tview.NewFlex()
	bar.SetBackgroundColor(theme.BgPanel)

	var focusables []tview.Primitive

	// Calculate total width needed for centering
	totalWidth := 0
	for i, act := range actions {
		totalWidth += len(act.Label) + theme.ModalButtonPadX*2
		if i > 0 {
			totalWidth += theme.ModalButtonGap
		}
	}

	// Left spacer for centering (weight 1)
	bar.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)

	for i, act := range actions {
		a := act
		isPrimary := i == 0

		btn := tview.NewButton(a.Label)
		if isPrimary {
			// Primary button: accent background at rest
			btn.SetBackgroundColor(theme.Accent)
			btn.SetLabelColor(theme.BgPanel)
			btn.SetBackgroundColorActivated(theme.Action)
			btn.SetLabelColorActivated(theme.BgPanel)
		} else {
			// Secondary button: subtle background
			btn.SetBackgroundColor(theme.BgElement)
			btn.SetLabelColor(theme.FgPrimary)
			btn.SetBackgroundColorActivated(theme.Accent)
			btn.SetLabelColorActivated(theme.BgPanel)
		}
		btn.SetSelectedFunc(func() {
			dismiss()
			if a.Callback != nil {
				a.Callback()
			}
		})

		btnWidth := len(a.Label) + theme.ModalButtonPadX*2
		if i > 0 {
			// Gap between buttons
			bar.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), theme.ModalButtonGap, 0, false)
		}
		bar.AddItem(btn, btnWidth, 0, isPrimary)
		focusables = append(focusables, btn)
	}

	// Right spacer for centering (weight 1)
	bar.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)

	return bar, focusables
}

// ─────────────────────────────────────────────────────────────────────────────
// Modal styling helpers — apply uniform style to content primitives
// ─────────────────────────────────────────────────────────────────────────────

// styleSelectList applies the standard modal style to a tview.List used in select modals.
func styleSelectList(list *tview.List) {
	list.ShowSecondaryText(false)
	list.SetHighlightFullLine(true)
	list.SetMainTextColor(theme.FgPrimary)
	list.SetSelectedBackgroundColor(theme.BgElement)
	list.SetSelectedTextColor(theme.FgPrimary)
	list.SetBackgroundColor(theme.BgPanel)
}

// styleScrollableText applies the standard modal style to a tview.TextView used in scrollable modals.
func styleScrollableText(tv *tview.TextView) {
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	tv.SetWrap(true)
	tv.SetBackgroundColor(theme.BgPanel)
	tv.SetTextColor(theme.FgPrimary)
	tv.SetBorderPadding(0, 0, theme.ModalContentPadX, theme.ModalContentPadX)
}

// styleInputField applies the standard modal style to a tview.InputField.
func styleInputField(input *tview.InputField) {
	input.SetLabelColor(theme.Accent)
	input.SetFieldBackgroundColor(theme.BgElement)
	input.SetFieldTextColor(theme.FgPrimary)
	input.SetBackgroundColor(theme.BgPanel)
}
