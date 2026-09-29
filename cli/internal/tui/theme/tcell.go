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

// Mode indicators — named aliases for the accent color of each mode.
var (
	// ModeHubColor is the accent color for hub mode (Peach).
	ModeHubColor = tcell.GetColor(ModeHubHex)
	// ModeTeamColor is the accent color for team mode (Mauve Saturated).
	ModeTeamColor = tcell.GetColor(ModeTeamHex)
	// ModeProjectColor is the accent color for project mode (Sapphire).
	ModeProjectColor = tcell.GetColor(ModeProjectHex)
)

// ─────────────────────────────────────────────────────────────────────────────
// ModeTheme instances — one per navigation mode (4-level palette)
// ─────────────────────────────────────────────────────────────────────────────

// ModeTheme holds a 4-level color palette for a navigation mode.
// Views and widgets read theme.ActiveMode instead of hardcoding color tokens,
// so the entire UI adapts when the user switches context.
//
// Usage:
//
//	theme.ActiveMode.AccentHex   // bannières ASCII, bordures modals, boutons primaires
//	theme.ActiveMode.PrimaryHex  // titres sections, badges, key highlights, mode label
//	theme.ActiveMode.SecondaryHex // boutons activés, CTA, cursors, arrows
//	theme.ActiveMode.MutedHex    // séparateur omnibar, indicateurs discrets
type ModeTheme struct {
	// AccentHex is the most vivid color, used sparingly for high-impact elements:
	// ASCII banners, modal borders/titles/corners, primary button backgrounds.
	AccentHex string
	Accent    tcell.Color

	// PrimaryHex is the main mode color, used frequently:
	// section headers, mode badges, key highlights, step indicators,
	// mode label in omnibar, spinners, focus borders.
	PrimaryHex string
	Primary    tcell.Color

	// SecondaryHex is a softer accent for interactive feedback:
	// activated button backgrounds, CTA highlights, active-item arrows,
	// filter labels, widget cursors.
	SecondaryHex string
	Secondary    tcell.Color

	// MutedHex is the most subtle color for decorative/structural elements:
	// omnibar mode separator, discrete indicators.
	MutedHex string
	Muted    tcell.Color
}

var (
	// ThemeHub is the mode theme for hub navigation (Warm family).
	ThemeHub = ModeTheme{
		AccentHex:    ActionHex,        // #fab387 Peach
		Accent:       tcell.GetColor(ActionHex),
		PrimaryHex:   YellowHex,        // #f9e2af Yellow
		Primary:      tcell.GetColor(YellowHex),
		SecondaryHex: FlamingoHex,      // #f2cdcd Flamingo
		Secondary:    tcell.GetColor(FlamingoHex),
		MutedHex:     MutedHubHex,      // #5a4838
		Muted:        tcell.GetColor(MutedHubHex),
	}
	// ThemeTeam is the mode theme for team navigation (Saturated Violet family).
	ThemeTeam = ModeTheme{
		AccentHex:    MauveSaturatedHex, // #c37ef5 Mauve Saturated
		Accent:       tcell.GetColor(MauveSaturatedHex),
		PrimaryHex:   Violet350Hex,      // #b197fc Violet 350
		Primary:      tcell.GetColor(Violet350Hex),
		SecondaryHex: Violet300Hex,      // #d8b4fe Violet 300
		Secondary:    tcell.GetColor(Violet300Hex),
		MutedHex:     MutedTeamHex,      // #4a3060
		Muted:        tcell.GetColor(MutedTeamHex),
	}
	// ThemeProject is the mode theme for project navigation (Sapphire/Blue family).
	ThemeProject = ModeTheme{
		AccentHex:    SapphireHex,       // #74c7ec Sapphire
		Accent:       tcell.GetColor(SapphireHex),
		PrimaryHex:   BlueHex,           // #89b4fa Blue
		Primary:      tcell.GetColor(BlueHex),
		SecondaryHex: TealHex,           // #94e2d5 Teal
		Secondary:    tcell.GetColor(TealHex),
		MutedHex:     MutedProjectHex,   // #2d4050
		Muted:        tcell.GetColor(MutedProjectHex),
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
