// Package common provides shared TUI styles and constants for the oh CLI.
// Design System: Aurum — see docs/design/aurum.md for full reference.
package common

import (
	"github.com/charmbracelet/lipgloss"
)

// ─────────────────────────────────────────────────────────────────────────────
// Aurum Color Palette
// ─────────────────────────────────────────────────────────────────────────────

// Semantic colors.
var (
	Primary   = lipgloss.Color("178") // Gold — titles, active element, focus border
	Accent    = lipgloss.Color("99")  // Amethyst — highlights, selection, cursor
	Success   = lipgloss.Color("78")  // Jade — completed, confirmed
	Warning   = lipgloss.Color("214") // Amber — warnings, medium priority
	Error     = lipgloss.Color("196") // Ruby — errors, blocked, critical
	Info      = lipgloss.Color("33")  // Sapphire — in progress, running
	Subtle    = lipgloss.Color("244") // Slate — muted text, descriptions, footers
	Highlight = lipgloss.Color("99")  // Amethyst (alias for Accent)
)

// Structural colors.
var (
	TextLight    = lipgloss.Color("255") // Ivory — text on colored backgrounds
	Surface      = lipgloss.Color("235") // Obsidian — title bar backgrounds, panels
	Border       = lipgloss.Color("240") // Graphite — normal borders, separators
	BorderActive = lipgloss.Color("178") // Gold — active/focused element border
)

// ─────────────────────────────────────────────────────────────────────────────
// Aurum Styles
// ─────────────────────────────────────────────────────────────────────────────

// Common styles.
var (
	// Title renders a bold Gold title.
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary)

	// Subtitle renders a dimmed Slate subtitle.
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
		Foreground(Border).
		SetString("│ ")

	// Box renders a bordered box with Gold border.
	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Padding(0, 1)

	// BoxSubtle renders a bordered box with Graphite border.
	BoxSubtle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Border).
			Padding(0, 1)
)

// ─────────────────────────────────────────────────────────────────────────────
// Aurum Icons
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

// Separator characters.
const (
	IconSepBold   = "━" // Bold horizontal rule (U+2501)
	IconSepNormal = "─" // Normal horizontal rule (U+2500)
	IconSepConn   = "───" // Connector between steps
)
