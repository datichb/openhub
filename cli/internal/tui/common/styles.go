// Package common provides shared TUI styles and constants for the oh CLI.
package common

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette.
var (
	Primary   = lipgloss.Color("99")  // Purple
	Success   = lipgloss.Color("82")  // Green
	Warning   = lipgloss.Color("214") // Orange
	Error     = lipgloss.Color("196") // Red
	Subtle    = lipgloss.Color("241") // Gray
	Highlight = lipgloss.Color("212") // Pink
)

// Common styles.
var (
	// Title renders a bold colored title.
	Title = lipgloss.NewStyle().
		Bold(true).
		Foreground(Primary)

	// Subtitle renders a dimmed subtitle.
	Subtitle = lipgloss.NewStyle().
			Foreground(Subtle)

	// SuccessStyle for success messages.
	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success)

	// WarningStyle for warning messages.
	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning)

	// ErrorStyle for error messages.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error)

	// Bold text.
	Bold = lipgloss.NewStyle().Bold(true)

	// Gutter renders the left gutter (clack-style).
	Gutter = lipgloss.NewStyle().
		Foreground(Subtle).
		SetString("│ ")

	// Box renders a bordered box.
	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Primary).
		Padding(0, 1)
)

// Icons used in the TUI.
const (
	IconSuccess = "✓"
	IconError   = "✗"
	IconWarning = "!"
	IconInfo    = "●"
	IconArrow   = "→"
	IconDot     = "·"
)
