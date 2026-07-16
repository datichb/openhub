// Package common provides the Aurum theme for huh forms.
// Design System: Aurum v2 "Floating Panels" — see docs/design/aurum.md
//
// This theme ensures visual consistency between the BubbleTea alt-screen wizards
// and the inline huh forms used across all commands.
package common

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// AurumTheme returns a huh Theme styled with the Aurum design system palette.
// It provides a premium, floating feel consistent with the alt-screen wizards.
//
// Color hierarchy (distinct roles):
//   - Accent (Amethyst)  → left border of focused field, select cursor, navigation
//   - TextLight (Ivory)  → field titles (bold), input text, option text
//   - Subtle (Lavender)  → descriptions, help text
//   - Primary (Copper)   → selected/active item highlight (used sparingly)
//   - Success (Jade)     → checkmarks, confirmed selections
//   - Error (Ruby)       → validation errors
//   - Muted (Ash)        → placeholders, blurred fields, separators
func AurumTheme() *huh.Theme {
	t := huh.ThemeBase()

	// ── Focused field styles ──
	// Left border uses Accent (Amethyst) — subtle but clearly distinct from text.
	t.Focused.Base = t.Focused.Base.BorderForeground(Accent)
	t.Focused.Card = t.Focused.Base

	// Titles are bold Ivory — readable without competing with the border color.
	t.Focused.Title = t.Focused.Title.Foreground(TextLight).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(TextLight).Bold(true).MarginBottom(1)
	t.Focused.Description = t.Focused.Description.Foreground(Subtle)
	t.Focused.Directory = t.Focused.Directory.Foreground(Info)
	t.Focused.File = t.Focused.File.Foreground(TextLight)

	// Error indicators — Ruby.
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(Error)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(Error)

	// Select / navigation — Copper for the selector arrow (the ONE active element).
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(Primary)
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(Primary)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(Primary)
	t.Focused.Option = t.Focused.Option.Foreground(Subtle)

	// Multi-select — Copper selector, Jade for selected, Lavender for unselected.
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(Primary)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(Success)
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(Success).SetString("✓ ")
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(Muted).SetString("○ ")
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(Subtle)

	// Buttons — subtle styling to avoid "floating patches" effect.
	// Focused button: Copper text on no background (just bold+colored).
	// Blurred button: Muted text.
	t.Focused.FocusedButton = t.Focused.FocusedButton.
		Foreground(Primary).
		Background(SurfaceElem).
		Bold(true)
	t.Focused.Next = t.Focused.FocusedButton
	t.Focused.BlurredButton = t.Focused.BlurredButton.
		Foreground(Muted).
		Background(lipgloss.NoColor{})

	// Text input — NO cursor color override (let terminal handle blinking natively).
	// This prevents the Reverse(true) color-flash bug in huh's cursor rendering.
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(Muted)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(Primary)
	t.Focused.TextInput.Text = t.Focused.TextInput.Text.Foreground(TextLight)

	// ── Blurred field styles ──
	// Inherit from Focused with a hidden border (maintains layout alignment).
	t.Blurred = t.Focused
	t.Blurred.Base = t.Focused.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Card = t.Blurred.Base
	t.Blurred.Title = t.Blurred.Title.Foreground(Muted).Bold(false)
	t.Blurred.NoteTitle = t.Blurred.NoteTitle.Foreground(Muted).Bold(false)
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()
	t.Blurred.TextInput.Text = t.Blurred.TextInput.Text.Foreground(Muted)
	t.Blurred.Option = t.Blurred.Option.Foreground(Muted)

	// ── Group styles ──
	t.Group.Title = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	t.Group.Description = lipgloss.NewStyle().Foreground(Subtle)

	// ── Help styles (keybind hints below the form) ──
	// Use Subtle for all — Muted is too low-contrast on dark terminals.
	t.Help.ShortKey = t.Help.ShortKey.Foreground(Subtle)
	t.Help.ShortDesc = t.Help.ShortDesc.Foreground(Subtle)
	t.Help.ShortSeparator = t.Help.ShortSeparator.Foreground(Muted)
	t.Help.FullKey = t.Help.FullKey.Foreground(Subtle)
	t.Help.FullDesc = t.Help.FullDesc.Foreground(Subtle)
	t.Help.FullSeparator = t.Help.FullSeparator.Foreground(Muted)
	t.Help.Ellipsis = t.Help.Ellipsis.Foreground(Subtle)

	return t
}

// NewForm creates a huh.Form pre-configured with the Aurum theme.
// Use this instead of huh.NewForm() to ensure consistent styling across all commands.
func NewForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithTheme(AurumTheme())
}

// RunConfirm runs a single confirm prompt with the Aurum theme applied.
// It wraps the confirm in a themed form for consistent styling.
//
//	var ok bool
//	err := common.RunConfirm(huh.NewConfirm().Title("Continue?").Value(&ok))
func RunConfirm(confirm *huh.Confirm) error {
	return huh.NewForm(huh.NewGroup(confirm)).WithTheme(AurumTheme()).Run()
}
