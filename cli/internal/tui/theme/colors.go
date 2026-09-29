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
	// AccentHex is the structural accent for focus borders, navigation — Tangerine.
	// Aligned with the Hub (default) mode accent.
	AccentHex = TangerineHex
	// ActionHex is the CTA accent for active menu items, buttons, spinners — Honey.
	// Aligned with the Hub (default) mode primary.
	ActionHex = HoneyHex
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
// Cards — elevated elements (kanban tickets, dashboard cards)
// Aurum spec: SurfaceElem bg + BorderElem border
// ─────────────────────────────────────────────────────────────────────────────

const (
	// BgCardHex is the card background — same as BgElement (Surface0).
	BgCardHex = "#313244"
	// BorderCardHex is the subtle card border — Aurum "BorderElem".
	// Slightly brighter than BorderNormal to be visible against BgCard.
	BorderCardHex = "#45475a"
)

// ─────────────────────────────────────────────────────────────────────────────
// Modal — elevated overlay surface with its own 3-level depth hierarchy
// Catppuccin Mocha: Base (#1e1e2e) < Surface0 (#313244) < Surface1 (#45475a)
// ─────────────────────────────────────────────────────────────────────────────

const (
	// BgModalHex is the modal surface background — Surface0.
	// Brighter than BgPanel to create a "floating card" effect.
	BgModalHex = "#313244"
	// BgModalFieldHex is the background for input fields inside modals — Base.
	// Darker than the modal surface so fields appear inset/recessed.
	BgModalFieldHex = "#1e1e2e"
	// BgModalHighlightHex is for selected items, hover states, ghost buttons — Surface1.
	// Brighter than the modal surface to show active/selected state.
	BgModalHighlightHex = "#45475a"
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

// ─────────────────────────────────────────────────────────────────────────────
// Mode indicator colors — accent color per navigation mode
// ─────────────────────────────────────────────────────────────────────────────

const (
	// ModeHubHex is the accent color for hub mode — Tangerine.
	ModeHubHex = TangerineHex
	// ModeTeamHex is the accent color for team mode — Mauve Saturated.
	ModeTeamHex = MauveSaturatedHex
	// ModeProjectHex is the accent color for project mode — Azure.
	ModeProjectHex = AzureHex
)

// ─────────────────────────────────────────────────────────────────────────────
// Extended Catppuccin Mocha palette — used by ModeTheme 4-level system
// ─────────────────────────────────────────────────────────────────────────────

const (
	// ── Hub mode (Warm saturated) ──

	// TangerineHex — hub mode accent.
	TangerineHex = "#f28c50"
	// HoneyHex — hub mode primary.
	HoneyHex = "#f5c468"
	// CoralHex — hub mode secondary.
	CoralHex = "#e89078"

	// ── Team mode (Violet saturated) ──

	// MauveSaturatedHex — team mode accent.
	MauveSaturatedHex = "#c37ef5"
	// Violet350Hex — Violet 350, team mode primary.
	Violet350Hex = "#b197fc"
	// Violet300Hex — Violet 300, team mode secondary.
	Violet300Hex = "#d8b4fe"

	// ── Project mode (Blue saturated) ──

	// AzureHex — project mode accent.
	AzureHex = "#60b8f0"
	// CobaltHex — project mode primary.
	CobaltHex = "#78a8f5"
	// TealVifHex — project mode secondary.
	TealVifHex = "#68d8c0"

	// ── Catppuccin Mocha reference palette ──

	// TealHex — Catppuccin Teal.
	TealHex = "#94e2d5"
	// SkyHex — Catppuccin Sky.
	SkyHex = "#89dceb"
	// SapphireHex — Catppuccin Sapphire.
	SapphireHex = "#74c7ec"
	// BlueHex — Catppuccin Blue (used by board columns for data-driven "blue" color).
	BlueHex = "#89b4fa"
	// LavenderHex — Catppuccin Lavender (= InfoHex).
	LavenderHex = "#b4befe"
	// MauveHex — Catppuccin Mauve (base, unsaturated).
	MauveHex = "#cba6f7"
	// PinkHex — Catppuccin Pink.
	PinkHex = "#f5c2e7"
	// FlamingoHex — Catppuccin Flamingo.
	FlamingoHex = "#f2cdcd"
	// PeachHex — Catppuccin Peach.
	PeachHex = "#fab387"
	// YellowHex — Catppuccin Yellow (= WarningHex).
	YellowHex = WarningHex
)

// Muted mode tints — darkened versions of each mode's accent for subtle
// decorative elements (separators, background tints, discrete indicators).
const (
	// MutedHubHex is a dark tangerine for hub mode muted elements.
	MutedHubHex = "#4d3828"
	// MutedTeamHex is a dark violet for team mode muted elements.
	MutedTeamHex = "#4a3060"
	// MutedProjectHex is a dark azure for project mode muted elements.
	MutedProjectHex = "#283f50"
)

// ─────────────────────────────────────────────────────────────────────────────
// ModeTheme — per-mode color palette consumed by views and widgets
// ─────────────────────────────────────────────────────────────────────────────
// See tcell.go for the ModeTheme struct definition, instances (ThemeHub,
// ThemeTeam, ThemeProject), ActiveMode variable, and SetActiveMode/ThemeForMode
// functions. The struct is defined in tcell.go because it contains tcell.Color
// fields alongside hex string fields.
//
// Hub     = Warm family   (Tangerine / Honey / Coral)
// Team    = Violet family (Mauve Saturated / Violet 350 / Violet 300)
// Project = Blue family   (Azure / Cobalt / Teal Vif)
