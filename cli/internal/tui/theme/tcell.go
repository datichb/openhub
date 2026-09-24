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

// Overlay backdrops — near-black to signal that background content is inert.
var (
	// BgDimOverlay is the backdrop for inline-overlays (modals, selects, inputs).
	BgDimOverlay = tcell.NewRGBColor(8, 8, 16)
	// BgDimSubOverlay is the backdrop for sub-overlays layered on top of a form.
	// Slightly darker to reinforce the extra depth level.
	BgDimSubOverlay = tcell.NewRGBColor(4, 4, 12)
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

// Cards
var (
	// BgCard is the card background (= BgElement / Surface0).
	BgCard = tcell.GetColor(BgCardHex)
	// BorderCard is the subtle card border (Aurum BorderElem).
	BorderCard = tcell.GetColor(BorderCardHex)
)

// Modal surfaces — 3-level depth hierarchy within modals.
var (
	// BgModal is the modal frame background (Surface0 — brighter than BgPanel).
	BgModal = tcell.GetColor(BgModalHex)
	// BgModalField is the recessed input field background within modals (Base).
	BgModalField = tcell.GetColor(BgModalFieldHex)
	// BgModalHighlight is for selected items, ghost buttons within modals (Surface1).
	BgModalHighlight = tcell.GetColor(BgModalHighlightHex)
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

// Mode indicators — named aliases for clarity in the omnibar gutter / mode bar.
var (
	// ModeHubColor is the primary color for hub mode (= Accent / Blue).
	ModeHubColor = tcell.GetColor(ModeHubHex)
	// ModeTeamColor is the primary color for team mode (= Info / Lavender).
	ModeTeamColor = tcell.GetColor(ModeTeamHex)
	// ModeProjectColor is the primary color for project mode (= Action / Peach).
	ModeProjectColor = tcell.GetColor(ModeProjectHex)
)

// ─────────────────────────────────────────────────────────────────────────────
// ModeTheme instances — one per navigation mode
// ─────────────────────────────────────────────────────────────────────────────

// ModeTheme holds the color palette for a navigation mode (Hub, Team, Project).
// Views and widgets read theme.ActiveMode instead of hardcoding Accent/Action
// tokens, so the entire UI adapts when the user switches context.
//
// Usage in views:
//
//	theme.ColorTag(theme.ActiveMode.PrimaryHex)  // banners, headers, borders
//	theme.ActiveMode.Primary                      // tcell button bg, focus border
//	theme.ColorTag(theme.ActiveMode.SecondaryHex) // activated buttons, CTA highlights
type ModeTheme struct {
	// PrimaryHex is the dominant mode color used for banners, section headers,
	// focus borders, modal borders, button backgrounds, step indicators, and spinners.
	PrimaryHex string
	// Primary is the tcell.Color equivalent of PrimaryHex.
	Primary tcell.Color

	// SecondaryHex is a softer accent used for activated button backgrounds,
	// CTA key highlights, filter labels, and active-item arrows.
	SecondaryHex string
	// Secondary is the tcell.Color equivalent of SecondaryHex.
	Secondary tcell.Color

	// SeparatorHex is the color for the horizontal mode separator in the omnibar.
	SeparatorHex string
	// Separator is the tcell.Color equivalent of SeparatorHex.
	Separator tcell.Color
}

var (
	// ThemeHub is the mode theme for hub navigation (Blue / Sapphire).
	ThemeHub = ModeTheme{
		PrimaryHex:   ModeHubHex,
		Primary:      ModeHubColor,
		SecondaryHex: SapphireHex,
		Secondary:    tcell.GetColor(SapphireHex),
		SeparatorHex: ModeHubHex,
		Separator:    ModeHubColor,
	}
	// ThemeTeam is the mode theme for team navigation (Lavender / Mauve).
	ThemeTeam = ModeTheme{
		PrimaryHex:   ModeTeamHex,
		Primary:      ModeTeamColor,
		SecondaryHex: MauveHex,
		Secondary:    tcell.GetColor(MauveHex),
		SeparatorHex: ModeTeamHex,
		Separator:    ModeTeamColor,
	}
	// ThemeProject is the mode theme for project navigation (Peach / Yellow).
	ThemeProject = ModeTheme{
		PrimaryHex:   ModeProjectHex,
		Primary:      ModeProjectColor,
		SecondaryHex: YellowHex,
		Secondary:    tcell.GetColor(YellowHex),
		SeparatorHex: ModeProjectHex,
		Separator:    ModeProjectColor,
	}

	// ActiveMode is the current mode theme, read by all views and widgets.
	// Updated by the shell via SetActiveMode when the user switches context.
	ActiveMode = ThemeHub
)

// SetActiveMode updates the global ActiveMode theme to match the given mode.
// The mode parameter is a string ("hub", "team", "project") to avoid a
// circular dependency between the theme and views packages.
func SetActiveMode(mode string) {
	ActiveMode = ThemeForMode(mode)
}

// ThemeForMode returns the ModeTheme for the given mode string.
func ThemeForMode(mode string) ModeTheme {
	switch mode {
	case "team":
		return ThemeTeam
	case "project":
		return ThemeProject
	default:
		return ThemeHub
	}
}

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
