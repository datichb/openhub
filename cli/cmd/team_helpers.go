package cmd

import (
	"fmt"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Team resolution helpers
// ─────────────────────────────────────────────────────────────────────────────

// teamEnabledForProject reports whether team features are active for a specific project.
// Pass nil as project to check hub-level only.
func teamEnabledForProject(a *app.App, project *domain.Project) bool {
	return resolvedTeamConfig(a, project).Enabled
}

// resolvedTeamConfig returns the effective TeamConfig for a project.
// Pass nil as project to get hub-level only.
func resolvedTeamConfig(a *app.App, project *domain.Project) config.ResolvedTeamConfig {
	if project != nil {
		return config.ResolveTeamForProject(a.Config, project)
	}
	// Hub-level only: use ActiveTeam() directly
	hub := a.Config.ActiveTeam()
	return config.ResolvedTeamConfig{
		Enabled:   hub.Enabled,
		TeamID:    hub.ID,
		StateRepo: hub.StateRepo,
		StatePath: hub.StatePath,
		MemberID:  hub.MemberID,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Wizard step (used in oh project add)
// ─────────────────────────────────────────────────────────────────────────────

// buildProjectTeamStep returns a WizardStep that collects team configuration
// for a project during project creation or reconfiguration.
//
// The step sets out to:
//   - &activeTeamID → attach the project to the hub's active team
//   - nil           → no team for this project
func buildProjectTeamStep(a *app.App, out **string) views.WizardStep {
	hubTeam := a.Config.ActiveTeam()

	// Local state for the step
	var attachToTeam bool

	// If hub has a team, default to attaching
	if hubTeam.Enabled && hubTeam.ID != "" {
		attachToTeam = true
	}

	return views.WizardStep{
		Label: "Team",
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()

			if hubTeam.Enabled && hubTeam.ID != "" {
				hubSummary := fmt.Sprintf("hub : %s (member : %s)", hubTeam.StateRepo, hubTeam.MemberID)
				form.AddTextView("Équipe hub", hubSummary, 0, 1, false, false)
				modeOptions := []string{
					fmt.Sprintf("Utiliser l'équipe du hub (%s)", hubTeam.MemberID),
					"Pas d'équipe pour ce projet",
				}
				form.AddDropDown("Équipe", modeOptions, 0, func(_ string, idx int) {
					attachToTeam = idx == 0
				})
			} else {
				form.AddTextView("Équipe hub", "Aucune équipe configurée au niveau du hub.", 0, 1, false, false)
			}

			form.AddButton("Next", func() { onDone() })
			return form
		},
		OnDone: func() error {
			if attachToTeam && hubTeam.ID != "" {
				id := hubTeam.ID
				*out = &id
			} else {
				*out = nil
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			if attachToTeam && hubTeam.ID != "" {
				return []views.InfoField{{Label: "Team", Value: hubTeam.ID}}
			}
			return []views.InfoField{{Label: "Team", Value: "aucune"}}
		},
	}
}
