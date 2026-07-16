// Package common provides shared TUI styles and constants for the oh CLI.
// Design System: Aurum v2 "Floating Panels" — see docs/design/aurum.md
package common

import (
	"github.com/charmbracelet/lipgloss"
)

// ─────────────────────────────────────────────────────────────────────────────
// Aurum v2 — Color Palette (True Color hex, lipgloss auto-fallback to 256/16)
// ─────────────────────────────────────────────────────────────────────────────

// Semantic colors.
var (
	Primary   = lipgloss.Color("#e8a838") // Copper — active element, focus border, step label
	Accent    = lipgloss.Color("#a78bfa") // Amethyst — highlights, selection, cursor
	Success   = lipgloss.Color("#5fd787") // Jade — completed, confirmed
	Warning   = lipgloss.Color("#ffaf5f") // Amber — warnings, medium priority
	Error     = lipgloss.Color("#ff5f5f") // Ruby — errors, blocked, critical
	Info      = lipgloss.Color("#5fafff") // Sapphire — in progress, running
	Highlight = lipgloss.Color("#a78bfa") // Amethyst (alias for Accent)
)

// Text colors.
var (
	TextLight = lipgloss.Color("#e8e8e8") // Ivory — titles, primary text, text on colored bg
	Subtle    = lipgloss.Color("#8585a0") // Lavender — footer, help text, keybinds
	Muted     = lipgloss.Color("#6e6e82") // Ash — descriptions, metadata, timestamps
)

// Depth colors (3 levels: terminal → panel → element).
// Theme: Mocha (active). See docs/design/aurum.md for alternatives.
var (
	Surface     = lipgloss.Color("#1e1e2e") // Panel bg — floats above terminal
	SurfaceElem = lipgloss.Color("#323248") // Element bg — raised zone (step bar, form, cards)
)

// Border colors (quasi-invisible — same color family as backgrounds).
var (
	Border       = lipgloss.Color("#262636") // Panel border (nearly invisible on Surface)
	BorderElem   = lipgloss.Color("#3c3c52") // Element border (very subtle on Surface)
	BorderActive = lipgloss.Color("#e8a838") // = Primary (focused element border)
)

// ─────────────────────────────────────────────────────────────────────────────
// Aurum v2 — Styles
// ─────────────────────────────────────────────────────────────────────────────

// Common styles.
var (
	// Title renders a bold Ivory title.
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(TextLight)

	// Subtitle renders a Lavender subtitle.
	Subtitle = lipgloss.NewStyle().
			Foreground(Subtle)

	// SuccessStyle for success messages (Jade).
	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success)

	// WarningStyle for warning messages (Amber).
	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning)

	// ErrorStyle for error messages (Ruby).
	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error)

	// InfoStyle for informational messages (Sapphire).
	InfoStyle = lipgloss.NewStyle().
			Foreground(Info)

	// AccentStyle for highlighted interactive elements (Amethyst).
	AccentStyle = lipgloss.NewStyle().
			Foreground(Accent)

	// Bold text.
	Bold = lipgloss.NewStyle().Bold(true)

	// Gutter renders the left gutter (pipeline-style).
	Gutter = lipgloss.NewStyle().
		Foreground(BorderElem).
		SetString("│ ")

	// Box renders a bordered box with quasi-invisible border (floating panel).
	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Border).
		Padding(0, 1)

	// BoxElem renders a bordered box for raised elements (inner panel).
	BoxElem = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderElem).
		Padding(0, 1)

	// BoxActive renders a bordered box with focus border (Copper).
	BoxActive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BorderActive).
			Padding(0, 1)
)

// ─────────────────────────────────────────────────────────────────────────────
// Aurum v2 — Icons
// ─────────────────────────────────────────────────────────────────────────────

// Action and status icons.
const (
	IconSuccess = "✓"
	IconError   = "✗"
	IconWarning = "!"
	IconInfo    = "▸"
	IconArrow   = "▸"
	IconDot     = "·"
)

// Step progression icons.
const (
	IconStepDone    = "●"
	IconStepActive  = "◔"
	IconStepPending = "○"
	// Fallback for terminals that don't render ◔ properly.
	IconStepActiveFallback = "►"
)

// Separator characters (used sparingly — prefer background changes).
const (
	IconSepBold   = "━" // Bold horizontal rule (U+2501)
	IconSepNormal = "─" // Normal horizontal rule (U+2500)
	IconSepConn   = "───" // Connector between steps in step bar
)
