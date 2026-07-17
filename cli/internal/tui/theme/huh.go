package theme

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// AurumTheme returns the unified huh theme using the canonical palette.
func AurumTheme() *huh.Theme {
	t := huh.ThemeBase()

	// Focused field styles
	t.Focused.Base = t.Focused.Base.BorderForeground(LipAccent)
	t.Focused.Title = t.Focused.Title.Foreground(LipTextPrimary).Bold(true)
	t.Focused.NoteTitle = t.Focused.NoteTitle.Foreground(LipTextPrimary).Bold(true).MarginBottom(1)
	t.Focused.Description = t.Focused.Description.Foreground(LipTextSecondary)
	t.Focused.Directory = t.Focused.Directory.Foreground(LipInfo)
	t.Focused.File = t.Focused.File.Foreground(LipTextPrimary)
	t.Focused.ErrorIndicator = t.Focused.ErrorIndicator.Foreground(LipError)
	t.Focused.ErrorMessage = t.Focused.ErrorMessage.Foreground(LipError)
	t.Focused.SelectSelector = t.Focused.SelectSelector.Foreground(LipAction)
	t.Focused.NextIndicator = t.Focused.NextIndicator.Foreground(LipAction)
	t.Focused.PrevIndicator = t.Focused.PrevIndicator.Foreground(LipAction)
	t.Focused.Option = t.Focused.Option.Foreground(LipTextSecondary)
	t.Focused.MultiSelectSelector = t.Focused.MultiSelectSelector.Foreground(LipAction)
	t.Focused.SelectedOption = t.Focused.SelectedOption.Foreground(LipSuccess)
	t.Focused.SelectedPrefix = lipgloss.NewStyle().Foreground(LipSuccess).SetString(IconSuccess + " ")
	t.Focused.UnselectedPrefix = lipgloss.NewStyle().Foreground(LipTextMuted).SetString(IconPending + " ")
	t.Focused.UnselectedOption = t.Focused.UnselectedOption.Foreground(LipTextSecondary)
	t.Focused.FocusedButton = t.Focused.FocusedButton.Foreground(LipAction).Background(LipSurfaceElem).Bold(true)
	t.Focused.BlurredButton = t.Focused.BlurredButton.Foreground(LipTextMuted)
	t.Focused.TextInput.Placeholder = t.Focused.TextInput.Placeholder.Foreground(LipTextMuted)
	t.Focused.TextInput.Prompt = t.Focused.TextInput.Prompt.Foreground(LipAction)
	t.Focused.TextInput.Text = t.Focused.TextInput.Text.Foreground(LipTextPrimary)

	// Blurred field styles
	t.Blurred.Base = t.Blurred.Base.BorderStyle(lipgloss.HiddenBorder())
	t.Blurred.Title = t.Blurred.Title.Foreground(LipTextMuted)
	t.Blurred.NoteTitle = t.Blurred.NoteTitle.Foreground(LipTextMuted)
	t.Blurred.TextInput.Placeholder = t.Blurred.TextInput.Placeholder.Foreground(LipTextMuted)
	t.Blurred.TextInput.Text = t.Blurred.TextInput.Text.Foreground(LipTextMuted)
	t.Blurred.Option = t.Blurred.Option.Foreground(LipTextMuted)
	t.Blurred.NextIndicator = lipgloss.NewStyle()
	t.Blurred.PrevIndicator = lipgloss.NewStyle()

	// Help styles
	t.Help.ShortKey = t.Help.ShortKey.Foreground(LipTextSecondary)
	t.Help.ShortDesc = t.Help.ShortDesc.Foreground(LipTextSecondary)
	t.Help.ShortSeparator = t.Help.ShortSeparator.Foreground(LipTextMuted)

	return t
}

// NewForm creates a new huh form with the Aurum theme applied.
func NewForm(groups ...*huh.Group) *huh.Form {
	return huh.NewForm(groups...).WithTheme(AurumTheme())
}
