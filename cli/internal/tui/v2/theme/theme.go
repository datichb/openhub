// Package theme defines the design system for the oh TUI.
//
// Design direction: sober + modern.
// - High contrast text, clean borders, single accent color.
// - Two background levels only (app → panel). No "whisper" borders.
// - Inspired by lazygit (sobriety) + OpenCode (modern interactions).
package theme

import "github.com/gdamore/tcell/v2"

// ─────────────────────────────────────────────────────────────────────────────
// Background colors (2 levels: app → panel/element)
// ─────────────────────────────────────────────────────────────────────────────

var (
	// BgApp is the application-wide background. Fills every cell by default.
	BgApp = tcell.NewRGBColor(22, 22, 30) // #16161e

	// BgPanel is for elevated surfaces (forms, cards, modals).
	BgPanel = tcell.NewRGBColor(30, 30, 42) // #1e1e2a

	// BgElement is for active/focused elements within a panel (selected card, active input).
	BgElement = tcell.NewRGBColor(42, 42, 58) // #2a2a3a
)

// ─────────────────────────────────────────────────────────────────────────────
// Text colors (3 levels: primary → secondary → muted)
// ─────────────────────────────────────────────────────────────────────────────

var (
	// FgPrimary is the default text color. High contrast on dark backgrounds.
	FgPrimary = tcell.NewRGBColor(220, 220, 230) // #dcdce6

	// FgSecondary is for labels, descriptions, non-critical info.
	FgSecondary = tcell.NewRGBColor(140, 140, 160) // #8c8ca0

	// FgMuted is for placeholders, disabled items, hints.
	FgMuted = tcell.NewRGBColor(90, 90, 110) // #5a5a6e
)

// ─────────────────────────────────────────────────────────────────────────────
// Semantic colors (accent + status)
// ─────────────────────────────────────────────────────────────────────────────

var (
	// Accent is the single focus/active color. Used for focused borders,
	// active step indicator, selected items. One accent = clear focus.
	Accent = tcell.NewRGBColor(100, 160, 255) // #64a0ff — calm blue

	// Success for confirmations, completed steps, positive feedback.
	Success = tcell.NewRGBColor(80, 200, 120) // #50c878

	// Warning for non-blocking alerts, medium-priority items.
	Warning = tcell.NewRGBColor(230, 180, 60) // #e6b43c

	// Error for failures, blocked items, validation errors.
	Error = tcell.NewRGBColor(230, 80, 80) // #e65050
)

// ─────────────────────────────────────────────────────────────────────────────
// Border colors
// ─────────────────────────────────────────────────────────────────────────────

var (
	// BorderNormal is clearly visible (not "quasi-invisible"). Panels should be
	// obviously delimited — no guessing where one ends and another begins.
	BorderNormal = tcell.NewRGBColor(55, 55, 75) // #37374b

	// BorderFocus is the focused element border — matches Accent.
	BorderFocus = Accent
)

// ─────────────────────────────────────────────────────────────────────────────
// Tcell styles (pre-built for convenience)
// ─────────────────────────────────────────────────────────────────────────────

var (
	// StyleDefault is the base style applied to the entire screen.
	StyleDefault = tcell.StyleDefault.
			Background(BgApp).
			Foreground(FgPrimary)

	// StylePanel is the style for elevated panels/cards.
	StylePanel = tcell.StyleDefault.
			Background(BgPanel).
			Foreground(FgPrimary)

	// StyleElement is the style for active elements within panels.
	StyleElement = tcell.StyleDefault.
			Background(BgElement).
			Foreground(FgPrimary)

	// StyleMuted renders secondary/muted text on app background.
	StyleMuted = tcell.StyleDefault.
			Background(BgApp).
			Foreground(FgMuted)

	// StyleAccent renders accent-colored text (bold).
	StyleAccent = tcell.StyleDefault.
			Background(BgApp).
			Foreground(Accent).
			Bold(true)
)

// ─────────────────────────────────────────────────────────────────────────────
// Icons
// ─────────────────────────────────────────────────────────────────────────────

const (
	IconDone    = "●"
	IconActive  = "◆"
	IconPending = "○"
	IconSkipped = "○"

	IconSuccess = "✓"
	IconError   = "✗"
	IconWarning = "!"
	IconArrow   = "▸"
	IconDot     = "·"

	// Connector between steps in horizontal step bar.
	IconConnector = "───"
)
