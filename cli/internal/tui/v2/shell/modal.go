package shell

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
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
	// FocusAction, when > 0, focuses the button of Actions[FocusAction-1]
	// initially (it wins over FocusTarget).
	FocusAction int
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
	switch {
	case cfg.FocusAction > 0 && cfg.FocusAction < len(focusables):
		s.app.SetFocus(focusables[cfg.FocusAction])
	case cfg.FocusTarget != nil:
		s.app.SetFocus(cfg.FocusTarget)
	case len(focusables) > 0:
		s.app.SetFocus(focusables[0])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// buildModalFrame constructs the visual frame: hints + content + separator + buttons.
// Returns the assembled frame and the list of focusable primitives (for Tab cycling).
// ─────────────────────────────────────────────────────────────────────────────

func (s *Shell) buildModalFrame(cfg modalConfig) (result tview.Primitive, resultFocusables []tview.Primitive) {
	frame := tview.NewFlex().SetDirection(tview.FlexRow)
	frame.SetBackgroundColor(theme.BgModal)
	frame.SetBorder(true)
	frame.SetBorderColor(theme.ActiveMode.Accent)

	// ── Rounded corners via DrawFunc ──
	applyRoundedCorners(frame, theme.ActiveMode.Accent, theme.BgModal)

	// ── Title ──
	titleText := cfg.Title
	if titleText != "" {
		pad := strings.Repeat(" ", theme.ModalTitlePad)
		frame.SetTitle(fmt.Sprintf("%s%s%s", pad, titleText, pad))
		frame.SetTitleColor(theme.ActiveMode.Accent)
	}

	// ── Content ──
	var focusables []tview.Primitive
	frame.AddItem(cfg.Content, 0, 1, true) // flex weight 1 — fills available space
	focusables = append(focusables, cfg.Content)

	// ── Hints line (above separator, inside frame) ──
	if cfg.Hints != "" {
		hints := tview.NewTextView().
			SetText("  " + cfg.Hints).
			SetTextColor(theme.FgMuted).
			SetDynamicColors(false)
		hints.SetBackgroundColor(theme.BgModal)
		frame.AddItem(hints, 1, 0, false)
	}

	// ── Separator + Button bar ──
	if len(cfg.Actions) > 0 {
		// Separator line (centered 60% rule)
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
		pad := tview.NewBox().SetBackgroundColor(theme.BgModal)
		frame.AddItem(pad, 1, 0, false)
	}

	// ── Focus cycling (the focused element is read from the app, so a mouse
	// click keeps it in sync) ──
	if len(focusables) > 1 {
		frame.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			cur := focusIndex(s.app, focusables)
			switch event.Key() {
			case tcell.KeyTab, tcell.KeyBacktab:
				// Cycle through the content and every button.
				next := cur + 1
				if event.Key() == tcell.KeyBacktab {
					next = cur - 1
					if cur < 0 {
						next = 0
					}
				}
				s.app.SetFocus(focusables[(next+len(focusables))%len(focusables)])
				return nil
			case tcell.KeyDown:
				// ↓ scrolls a text or moves in a list; from other content
				// (an input…) it reaches the buttons.
				if cur == 0 && !scrollsVertically(cfg.Content) {
					s.app.SetFocus(focusables[1])
					return nil
				}
			case tcell.KeyUp:
				// ↑ from a button → back to the content
				if cur > 0 {
					s.app.SetFocus(focusables[0])
					return nil
				}
			case tcell.KeyLeft, tcell.KeyRight:
				// ←/→ navigate between buttons when focus is on a button
				if cur > 0 {
					btnCount := len(focusables) - 1 // exclude content (index 0)
					btnIdx := cur - 1
					if event.Key() == tcell.KeyLeft {
						btnIdx = (btnIdx - 1 + btnCount) % btnCount
					} else {
						btnIdx = (btnIdx + 1) % btnCount
					}
					s.app.SetFocus(focusables[btnIdx+1])
					return nil
				}
			case tcell.KeyEscape:
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

// focusIndex is the index of the focusable that has the focus (or contains
// it), -1 when none.
func focusIndex(app *tview.Application, focusables []tview.Primitive) int {
	f := app.GetFocus()
	for i, p := range focusables {
		if p == f {
			return i
		}
	}
	for i, p := range focusables {
		if p.HasFocus() {
			return i
		}
	}
	return -1
}

// scrollsVertically reports whether ↑/↓ belong to the content (text to
// scroll, list to move in) rather than to the modal navigation.
func scrollsVertically(p tview.Primitive) bool {
	switch p.(type) {
	case *tview.TextView, *tview.List, *tview.TreeView, *tview.Table:
		return true
	}
	return false
}

// ─────────────────────────────────────────────────────────────────────────────
// Separator — centered short rule (60% width) for a lighter visual weight
// ─────────────────────────────────────────────────────────────────────────────

func newSeparator() *tview.Box {
	sep := tview.NewBox().SetBackgroundColor(theme.BgModal)
	sep.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		bgStyle := tcell.StyleDefault.Background(theme.BgModal).Foreground(theme.BgModal)
		fgStyle := tcell.StyleDefault.Background(theme.BgModal).Foreground(theme.FgMuted)

		lineW := width * 60 / 100
		if lineW < 3 {
			lineW = width
		}
		pad := (width - lineW) / 2

		for dx := 0; dx < width; dx++ {
			if dx >= pad && dx < pad+lineW {
				screen.SetContent(x+dx, y, theme.ModalSeparatorRune, nil, fgStyle)
			} else {
				screen.SetContent(x+dx, y, ' ', nil, bgStyle)
			}
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
	bar.SetBackgroundColor(theme.BgModal)

	var focusables []tview.Primitive

	// Left spacer for centering (weight 1)
	bar.AddItem(tview.NewBox().SetBackgroundColor(theme.BgModal), 0, 1, false)

	for i, act := range actions {
		a := act
		isPrimary := i == 0

		// Shared look (theme.ButtonStyle / ButtonFocusedStyle): the first
		// action is the primary one, a dangerous one is drawn in red.
		role := widgets.ButtonNormal
		switch {
		case a.Danger:
			role = widgets.ButtonDanger
		case isPrimary:
			role = widgets.ButtonPrimary
		}
		btn := widgets.DecorateButton(tview.NewButton(a.Label), role)
		btn.SetSelectedFunc(func() {
			dismiss()
			if a.Callback != nil {
				a.Callback()
			}
		})

		btnWidth := runewidth.StringWidth(a.Label) + theme.ModalButtonPadX*2
		if i > 0 {
			// Gap between buttons — wider when Separator is set to group actions visually
			gap := theme.ModalButtonGap
			if a.Separator {
				gap = theme.ModalButtonGap * 3
			}
			bar.AddItem(tview.NewBox().SetBackgroundColor(theme.BgModal), gap, 0, false)
		}
		bar.AddItem(btn, btnWidth, 0, isPrimary)
		focusables = append(focusables, btn)
	}

	// Right spacer for centering (weight 1)
	bar.AddItem(tview.NewBox().SetBackgroundColor(theme.BgModal), 0, 1, false)

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
	list.SetSelectedBackgroundColor(theme.BgModalHighlight)
	list.SetSelectedTextColor(theme.FgPrimary)
	list.SetBackgroundColor(theme.BgModal)
}

// styleScrollableText applies the standard modal style to a tview.TextView used in scrollable modals.
func styleScrollableText(tv *tview.TextView) {
	tv.SetDynamicColors(true)
	tv.SetScrollable(true)
	tv.SetWrap(true)
	tv.SetBackgroundColor(theme.BgModal)
	tv.SetTextColor(theme.FgPrimary)
	tv.SetBorderPadding(0, 0, theme.ModalContentPadX, theme.ModalContentPadX)
}

// styleInputField applies the standard modal style to a tview.InputField.
func styleInputField(input *tview.InputField) {
	input.SetLabelColor(theme.FgPrimary)
	input.SetFieldBackgroundColor(theme.BgModalField)
	input.SetFieldTextColor(theme.FgPrimary)
	input.SetBackgroundColor(theme.BgModal)
}

// ─────────────────────────────────────────────────────────────────────────────
// Rounded corners helper
// ─────────────────────────────────────────────────────────────────────────────

// applyRoundedCorners overdraws the four corners of a bordered primitive with
// rounded arc glyphs (╭╮╰╯). This is safe because tview calls DrawFunc after
// rendering the standard border, so we simply replace the corner runes.
// This avoids mutating the global tview.Borders which would affect all views.
func applyRoundedCorners(p *tview.Flex, borderColor, bgColor tcell.Color) {
	original := p.GetDrawFunc()
	p.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		// Call the original DrawFunc if any
		ix, iy, iw, ih := x+1, y+1, width-2, height-2
		if original != nil {
			ix, iy, iw, ih = original(screen, x, y, width, height)
		}
		// Overdraw corners with rounded glyphs
		if width >= 2 && height >= 2 {
			style := tcell.StyleDefault.Foreground(borderColor).Background(bgColor)
			screen.SetContent(x, y, '╭', nil, style)
			screen.SetContent(x+width-1, y, '╮', nil, style)
			screen.SetContent(x, y+height-1, '╰', nil, style)
			screen.SetContent(x+width-1, y+height-1, '╯', nil, style)
		}
		return ix, iy, iw, ih
	})
}

// applyRoundedCornersBox overdraws rounded corners on a tview.Box (for lists with borders).
func applyRoundedCornersBox(p *tview.Box, borderColor, bgColor tcell.Color) {
	original := p.GetDrawFunc()
	p.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		ix, iy, iw, ih := x+1, y+1, width-2, height-2
		if original != nil {
			ix, iy, iw, ih = original(screen, x, y, width, height)
		}
		if width >= 2 && height >= 2 {
			style := tcell.StyleDefault.Foreground(borderColor).Background(bgColor)
			screen.SetContent(x, y, '╭', nil, style)
			screen.SetContent(x+width-1, y, '╮', nil, style)
			screen.SetContent(x, y+height-1, '╰', nil, style)
			screen.SetContent(x+width-1, y+height-1, '╯', nil, style)
		}
		return ix, iy, iw, ih
	})
}
