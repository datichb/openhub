package widgets

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ButtonRole is the visual role of a button.
type ButtonRole int

const (
	// ButtonNormal is a secondary action (neutral grey).
	ButtonNormal ButtonRole = iota
	// ButtonPrimary is the main action: mode accent text, bold.
	ButtonPrimary
	// ButtonDanger is a destructive action: error-coloured text, bold.
	ButtonDanger
)

// DecorateButton applies the shared button look (theme.ButtonStyle,
// theme.ButtonFocusedStyle) to btn and draws the « ▸ label ◂ » markers while
// it has the focus. The role colour is carried by style tags in the label so
// that it survives tview.Form, which resets its buttons' styles on every
// draw. The decorated label always has the width of the label plus 4, focused
// or not, so the layout does not move. Later SetLabel calls are picked up.
func DecorateButton(btn *tview.Button, role ButtonRole) *tview.Button {
	btn.SetStyle(theme.ButtonStyle(role == ButtonPrimary)).
		SetActivatedStyle(theme.ButtonFocusedStyle()).
		SetDisabledStyle(theme.ButtonDisabledStyle())
	base := btn.GetLabel()
	last := ""
	apply := func(focused bool) {
		if cur := btn.GetLabel(); cur != last {
			base = cur // changed by the caller since the last decoration
		}
		last = decorateLabel(base, role, focused)
		btn.SetLabel(last)
	}
	apply(false)
	prev := btn.GetDrawFunc()
	btn.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		apply(btn.HasFocus() && !btn.IsDisabled())
		if prev != nil {
			return prev(screen, x, y, width, height)
		}
		return btn.GetInnerRect()
	})
	return btn
}

// ButtonLabel returns the undecorated label of a button set up by
// DecorateButton (or its label as is).
func ButtonLabel(btn *tview.Button) string {
	return undecorateLabel(btn.GetLabel())
}

func roleTag(role ButtonRole) string {
	switch role {
	case ButtonPrimary:
		return "[" + theme.ActiveMode.AccentHex + "::b]"
	case ButtonDanger:
		return "[" + theme.ErrorHex + "::b]"
	}
	return ""
}

func decorateLabel(label string, role ButtonRole, focused bool) string {
	if focused {
		return theme.ButtonFocusLeft + label + theme.ButtonFocusRight
	}
	if tag := roleTag(role); tag != "" {
		return "  " + tag + label + theme.TagReset + "  "
	}
	return "  " + label + "  "
}

func undecorateLabel(s string) string {
	l, r := theme.ButtonFocusLeft, theme.ButtonFocusRight
	if len(s) >= len(l)+len(r) && s[:len(l)] == l && s[len(s)-len(r):] == r {
		return s[len(l) : len(s)-len(r)]
	}
	if len(s) >= 4 && s[:2] == "  " && s[len(s)-2:] == "  " {
		s = s[2 : len(s)-2]
		for _, role := range []ButtonRole{ButtonPrimary, ButtonDanger} {
			tag := roleTag(role)
			if len(s) >= len(tag)+len(theme.TagReset) && s[:len(tag)] == tag && s[len(s)-len(theme.TagReset):] == theme.TagReset {
				return s[len(tag) : len(s)-len(theme.TagReset)]
			}
		}
	}
	return s
}

// StyleFormButtons gives the buttons of f the shared look; the button at
// index primary (−1: none) is the primary action. Buttons added later are
// styled too (on the next draw, before the form lays them out).
func StyleFormButtons(f *tview.Form, primary int) {
	f.SetButtonStyle(theme.ButtonStyle(false)).
		SetButtonActivatedStyle(theme.ButtonFocusedStyle()).
		SetButtonDisabledStyle(theme.ButtonDisabledStyle())
	done := map[*tview.Button]bool{}
	decorate := func() {
		for i := 0; i < f.GetButtonCount(); i++ {
			b := f.GetButton(i)
			if done[b] {
				continue
			}
			done[b] = true
			role := ButtonNormal
			if i == primary {
				role = ButtonPrimary
			}
			DecorateButton(b, role)
		}
	}
	decorate()
	prev := f.GetDrawFunc()
	f.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		decorate()
		if prev != nil {
			return prev(screen, x, y, width, height)
		}
		return f.GetInnerRect()
	})
}
