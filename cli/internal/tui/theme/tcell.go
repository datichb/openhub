package theme

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ─────────────────────────────────────────────────────────────────────────────
// tcell.Color vars derived from hex constants
// ─────────────────────────────────────────────────────────────────────────────

// Backgrounds
var (
	// BgApp is the deepest background (terminal level).
	BgApp = tcell.GetColor(BgAppHex)
	// BgPanel is the surface background (sidebar, cards).
	BgPanel = tcell.GetColor(BgPanelHex)
	// BgElement is the elevated background (selection, header).
	BgElement = tcell.GetColor(BgElementHex)
)

// Text
var (
	// FgPrimary is for titles and primary content.
	FgPrimary = tcell.GetColor(TextPrimaryHex)
	// FgSecondary is for labels and descriptions.
	FgSecondary = tcell.GetColor(TextSecondaryHex)
	// FgMuted is for placeholders and disabled state.
	FgMuted = tcell.GetColor(TextMutedHex)
)

// Semantic
var (
	// Accent is the structural focus color (Azure).
	Accent = tcell.GetColor(AccentHex)
	// Action is the CTA color (Gold).
	Action = tcell.GetColor(ActionHex)
	// Success is for confirmations (Jade).
	Success = tcell.GetColor(SuccessHex)
	// Warning is for non-blocking alerts (Amber).
	Warning = tcell.GetColor(WarningHex)
	// Error is for failures and blocked states (Ruby).
	Error = tcell.GetColor(ErrorHex)
	// Info is for in-progress states (Sapphire).
	Info = tcell.GetColor(InfoHex)
)

// Borders
var (
	// BorderNormal is for discreet panel delimiters.
	BorderNormal = tcell.GetColor(BorderNormalHex)
	// BorderFocus is for the focused panel (= Accent).
	BorderFocus = tcell.GetColor(BorderFocusHex)
	// BorderActive is for the active menu item (= Action).
	BorderActive = tcell.GetColor(BorderActiveHex)
)

// ─────────────────────────────────────────────────────────────────────────────
// Pre-built tcell.Style values for convenience
// ─────────────────────────────────────────────────────────────────────────────

var (
	// StyleDefault is the base style (panel background + primary text).
	StyleDefault = tcell.StyleDefault.Background(BgPanel).Foreground(FgPrimary)
	// StylePanel is for elevated surfaces (panel background + primary text).
	StylePanel = tcell.StyleDefault.Background(BgPanel).Foreground(FgPrimary)
	// StyleElement is for interactive elements (element background + primary text).
	StyleElement = tcell.StyleDefault.Background(BgElement).Foreground(FgPrimary)
	// StyleMuted is for disabled/placeholder content.
	StyleMuted = tcell.StyleDefault.Background(BgPanel).Foreground(FgMuted)
	// StyleAccent is for accent-highlighted content.
	StyleAccent = tcell.StyleDefault.Background(BgPanel).Foreground(Accent).Bold(true)
)

// ─────────────────────────────────────────────────────────────────────────────
// tview global theme override
// ─────────────────────────────────────────────────────────────────────────────

// TviewTheme returns the tview.Theme that should be applied globally
// via tview.Styles = TviewTheme().
func TviewTheme() tview.Theme {
	return tview.Theme{
		PrimitiveBackgroundColor:    BgPanel,
		ContrastBackgroundColor:     BgElement,
		MoreContrastBackgroundColor: BgPanel,
		BorderColor:                 BorderNormal,
		TitleColor:                  FgPrimary,
		GraphicsColor:               BorderNormal,
		PrimaryTextColor:            FgPrimary,
		SecondaryTextColor:          FgSecondary,
		TertiaryTextColor:           FgMuted,
		InverseTextColor:            BgPanel,
		ContrastSecondaryTextColor:  FgSecondary,
	}
}
