package theme

import "github.com/gdamore/tcell/v2"

// ─────────────────────────────────────────────────────────────────────────────
// Buttons — one shared look for every button of the TUI (modals, inline
// forms, wizards, launch recap): a neutral pill at rest (the primary action
// in the mode accent, bold) and a strongly contrasted pill when focused.
// ─────────────────────────────────────────────────────────────────────────────

// Focus markers drawn around the label of the focused button.
const (
	ButtonFocusLeft  = "▸ "
	ButtonFocusRight = " ◂"
)

// ButtonStyle is the style of a button at rest: neutral background, the
// primary button's text in the active mode accent and bold.
func ButtonStyle(primary bool) tcell.Style {
	st := tcell.StyleDefault.Background(BgModalHighlight).Foreground(FgSecondary)
	if primary {
		st = st.Foreground(ActiveMode.Accent).Bold(true)
	}
	return st
}

// ButtonFocusedStyle is the style of the focused button (all buttons): light
// background, dark text, bold.
func ButtonFocusedStyle() tcell.Style {
	return tcell.StyleDefault.Background(FgPrimary).Foreground(BgModal).Bold(true)
}

// ButtonDisabledStyle is the style of a disabled button.
func ButtonDisabledStyle() tcell.Style {
	return tcell.StyleDefault.Background(BgModalHighlight).Foreground(FgMuted)
}
