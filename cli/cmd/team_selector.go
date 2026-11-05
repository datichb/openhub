package cmd

import (
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// teamSelectorOptions are the teams of the selector of Ctrl+T: the real teams
// that are not archived, the default team first.
func teamSelectorOptions(cfg *config.Config) []views.SelectOption {
	var out []views.SelectOption
	for _, t := range attachableTeams(cfg) {
		// The label becomes the name of the active team: no mark in it.
		out = append(out, views.SelectOption{Label: t.DisplayName(), Value: t.ID})
	}
	return out
}
