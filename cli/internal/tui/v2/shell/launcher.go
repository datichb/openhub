package shell

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// SessionLaunchConfig holds the options for a session launch dialog.
type SessionLaunchConfig struct {
	// Title is the dialog title (e.g., "Lancer Start", "Lancer Audit").
	Title string
	// Options is the list of selectable options.
	Options []SessionOption
	// OnLaunch is called with the selected option when the user confirms.
	OnLaunch func(selected SessionOption)
}

// SessionOption represents a single launchable session variant.
type SessionOption struct {
	Label       string // Display label
	Description string // One-line description
	Agent       string // opencode --agent value
	ExtraArgs   []string
}

// ShowSessionLauncher displays a modal with session options to choose from.
func (s *Shell) ShowSessionLauncher(cfg SessionLaunchConfig) {
	s.overlayActive = true
	s.app.EnableMouse(false)

	list := tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary).
		SetSelectedBackgroundColor(theme.BgElement).
		SetSelectedTextColor(theme.FgPrimary)
	list.SetBackgroundColor(theme.BgPanel)

	for _, opt := range cfg.Options {
		list.AddItem(opt.Label, "  "+opt.Description, 0, nil)
	}

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			s.pages.RemovePage("session-launcher")
			s.overlayActive = false
			s.app.EnableMouse(true)
			s.app.SetFocus(s.content)
			return nil
		case tcell.KeyEnter:
			idx := list.GetCurrentItem()
			s.pages.RemovePage("session-launcher")
			s.overlayActive = false
			s.app.EnableMouse(true)
			s.app.SetFocus(s.content)
			if idx >= 0 && idx < len(cfg.Options) && cfg.OnLaunch != nil {
				cfg.OnLaunch(cfg.Options[idx])
			}
			return nil
		}
		return event
	})

	// Frame
	frame := tview.NewFlex().SetDirection(tview.FlexRow)
	frame.AddItem(list, 0, 1, true)
	frame.SetBorder(true)
	frame.SetBorderColor(theme.Accent)
	frame.SetTitle(" " + cfg.Title + " · Esc annuler ")
	frame.SetTitleColor(theme.Accent)
	frame.SetBackgroundColor(theme.BgPanel)

	// Center (50% width, 60% height)
	grid := tview.NewGrid().
		SetColumns(0, -3, 0).
		SetRows(2, -3, 2)
	grid.AddItem(frame, 1, 1, 1, 1, 0, 0, true)

	s.pages.AddPage("session-launcher", grid, true, true)
	s.app.SetFocus(list)
}
