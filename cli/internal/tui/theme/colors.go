// Package theme provides a unified design token system for the OpenHub TUI.
// It defines colors, styles, and icons as hex string constants, then derives
// both tcell (tview) and lipgloss (huh/bubbletea) values from the same source.
//
// This package is the SINGLE SOURCE OF TRUTH for all visual tokens.
// No other package should define colors or icons — import this package instead.
package theme

// ─────────────────────────────────────────────────────────────────────────────
// Backgrounds — 3 depth levels creating visual hierarchy
// Palette: Catppuccin Mocha (proven on #1e1e2e base)
// ─────────────────────────────────────────────────────────────────────────────

const (
	// BgAppHex is the deepest background level — Mantle.
	BgAppHex = "#181825"
	// BgPanelHex is the surface level (sidebar, cards) — Base.
	BgPanelHex = "#1e1e2e"
	// BgElementHex is the elevated level (hover, selection, header) — Surface0.
	BgElementHex = "#313244"
)

// ─────────────────────────────────────────────────────────────────────────────
// Text — 3 contrast levels (blue-lavender tint for readability on dark)
// ─────────────────────────────────────────────────────────────────────────────

const (
	// TextPrimaryHex is for titles, menu items, primary content — Text.
	TextPrimaryHex = "#cdd6f4"
	// TextSecondaryHex is for descriptions, secondary labels — Subtext0.
	TextSecondaryHex = "#a6adc8"
	// TextMutedHex is for placeholders, disabled state, timestamps — Overlay1.
	TextMutedHex = "#7f849c"
)

// ─────────────────────────────────────────────────────────────────────────────
// Semantic colors — dual-accent system (Catppuccin Mocha accents)
// ─────────────────────────────────────────────────────────────────────────────

const (
	// AccentHex is the structural accent for focus borders, navigation — Blue.
	AccentHex = "#89b4fa"
	// ActionHex is the CTA accent for active menu items, buttons, spinners — Peach.
	ActionHex = "#fab387"
	// SuccessHex is for confirmations and completed states — Green.
	SuccessHex = "#a6e3a1"
	// WarningHex is for non-blocking alerts — Yellow.
	WarningHex = "#f9e2af"
	// ErrorHex is for errors, blocked, and critical states — Red.
	ErrorHex = "#f38ba8"
	// InfoHex is for in-progress and running states — Lavender.
	InfoHex = "#b4befe"
)

// ─────────────────────────────────────────────────────────────────────────────
// Borders
// ─────────────────────────────────────────────────────────────────────────────

const (
	// BorderNormalHex is for panel delimiters — Surface0.
	BorderNormalHex = "#313244"
	// BorderFocusHex is for the focused panel border (= Accent/Blue).
	BorderFocusHex = AccentHex
	// BorderActiveHex is for the active menu item border (= Action/Peach).
	BorderActiveHex = ActionHex
)
