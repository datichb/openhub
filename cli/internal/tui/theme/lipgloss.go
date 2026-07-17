package theme

import "github.com/charmbracelet/lipgloss"

// ─────────────────────────────────────────────────────────────────────────────
// lipgloss.Color vars derived from hex constants
// ─────────────────────────────────────────────────────────────────────────────

// Semantic colors
var (
	// LipAccent is the structural focus color (Azure).
	LipAccent = lipgloss.Color(AccentHex)
	// LipAction is the CTA color (Gold).
	LipAction = lipgloss.Color(ActionHex)
	// LipSuccess is for confirmations (Jade).
	LipSuccess = lipgloss.Color(SuccessHex)
	// LipWarning is for non-blocking alerts (Amber).
	LipWarning = lipgloss.Color(WarningHex)
	// LipError is for failures (Ruby).
	LipError = lipgloss.Color(ErrorHex)
	// LipInfo is for in-progress states (Sapphire).
	LipInfo = lipgloss.Color(InfoHex)
)

// Text colors
var (
	// LipTextPrimary is for titles and primary content (Snow).
	LipTextPrimary = lipgloss.Color(TextPrimaryHex)
	// LipTextSecondary is for labels and descriptions (Lavender).
	LipTextSecondary = lipgloss.Color(TextSecondaryHex)
	// LipTextMuted is for placeholders and disabled state (Ash).
	LipTextMuted = lipgloss.Color(TextMutedHex)
)

// Depth backgrounds
var (
	// LipSurface is the panel background.
	LipSurface = lipgloss.Color(BgPanelHex)
	// LipSurfaceElem is the elevated element background.
	LipSurfaceElem = lipgloss.Color(BgElementHex)
)

// Borders
var (
	// LipBorder is for panel delimiters.
	LipBorder = lipgloss.Color(BorderNormalHex)
	// LipBorderElem is an alias for the slightly brighter element border.
	LipBorderElem = lipgloss.Color(BorderNormalHex)
	// LipBorderActive is for focused/active elements (= Action/Gold).
	LipBorderActive = lipgloss.Color(BorderActiveHex)
)

// ─────────────────────────────────────────────────────────────────────────────
// Backward-compatible aliases for migration from common package
// ─────────────────────────────────────────────────────────────────────────────

// ─────────────────────────────────────────────────────────────────────────────
// Pre-built lipgloss.Style presets
// ─────────────────────────────────────────────────────────────────────────────

var (
	// Title is for bold primary headings.
	Title = lipgloss.NewStyle().Bold(true).Foreground(LipTextPrimary)
	// Subtitle is for secondary labels.
	Subtitle = lipgloss.NewStyle().Foreground(LipTextSecondary)
	// SuccessStyle is for success messages.
	SuccessStyle = lipgloss.NewStyle().Foreground(LipSuccess)
	// WarningStyle is for warning messages.
	WarningStyle = lipgloss.NewStyle().Foreground(LipWarning)
	// ErrorStyle is for error messages.
	ErrorStyle = lipgloss.NewStyle().Foreground(LipError)
	// InfoStyle is for informational messages.
	InfoStyle = lipgloss.NewStyle().Foreground(LipInfo)
	// AccentStyle is for accent-highlighted labels.
	AccentStyle = lipgloss.NewStyle().Foreground(LipAccent)
	// ActionStyle is for CTA-highlighted labels.
	ActionStyle = lipgloss.NewStyle().Foreground(LipAction)
	// Bold is a simple bold modifier.
	Bold = lipgloss.NewStyle().Bold(true)
	// Gutter is the left-margin indicator.
	Gutter = lipgloss.NewStyle().Foreground(LipBorder).SetString("│ ")
	// Box is a panel with rounded border.
	Box = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(LipBorder).
		Padding(0, 1)
	// BoxElem is a panel with element-level border.
	BoxElem = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(LipBorderElem).
		Padding(0, 1)
	// BoxActive is a panel with active/focused border.
	BoxActive = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(LipBorderActive).
		Padding(0, 1)
)

// ─────────────────────────────────────────────────────────────────────────────
// Backward-compatible aliases for migration from common package.
// These allow cmd/ files to use theme.Primary, theme.TextLight, etc.
// ─────────────────────────────────────────────────────────────────────────────

var (
	// Primary is the primary brand color (= Action/Gold). Used by cmd/ for lipgloss rendering.
	Primary = LipAction
	// TextLight is an alias for LipTextPrimary.
	TextLight = LipTextPrimary
	// Subtle is an alias for LipTextSecondary.
	Subtle = LipTextSecondary
	// Muted is an alias for LipTextMuted.
	Muted = LipTextMuted
)
