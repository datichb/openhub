package cmd

import (
	"context"
	"fmt"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
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

// initWizardTeamState holds mutable state shared across the team wizard steps.
// It is allocated once by buildInitWizardTeamSteps and captured by closures.
type initWizardTeamState struct {
	Skipped       bool   // user clicked "Skip" on the intro step
	Repo          string // team-state repo URL
	MemberID      string // member identifier
	DisplayName   string // optional display name
	Configured    bool   // true after successful teamInitCore
	TeamID        string // derived team ID (after init)
	attachProject bool   // true if user wants to attach the project to this team
}

// buildInitWizardTeamSteps returns the two WizardSteps for the "Team" group
// used in the hub initialization wizards (first-run inline + oh init standalone).
//
// The returned steps are:
//  1. Form — repo URL, member ID, display name
//  2. Processing — clone/pull, init structure, register member, write hub.toml
//
// The caller is responsible for prepending an intro step (via buildIntroStep)
// that sets state.Skipped. Both steps use SkipIf → state.Skipped.
//
// After successful completion, state.Configured is true and the app should be
// reloaded (config.Reset + ReloadApp) so subsequent steps can see the team.
func buildInitWizardTeamSteps(a **app.App, state *initWizardTeamState) []views.WizardStep {
	return []views.WizardStep{
		// ── Team form: repo + identity ──
		{
			Label: i18n.T("cmd.init.wizard_step_team"),
			SkipIf: func() bool {
				return state.Skipped
			},
			Validate: func() string {
				if state.Repo == "" {
					return i18n.T("cmd.init.wizard_team_repo_required")
				}
				if state.MemberID == "" {
					return i18n.T("cmd.init.wizard_team_member_required")
				}
				return ""
			},
			Form: func(_ *tview.Application, onDone func()) *tview.Form {
				form := tview.NewForm()
				form.AddInputField(
					i18n.T("cmd.init.wizard_team_repo"), state.Repo, 60, nil,
					func(t string) { state.Repo = t },
				)
				form.AddInputField(
					i18n.T("cmd.init.wizard_team_member_id"), state.MemberID, 30, nil,
					func(t string) { state.MemberID = t },
				)
				form.AddInputField(
					i18n.T("cmd.init.wizard_team_display_name"), state.DisplayName, 40, nil,
					func(t string) { state.DisplayName = t },
				)
				form.AddButton(i18n.T("wizard.hint.submit"), func() { onDone() })
				return form
			},
			InfoFields: func() []views.InfoField {
				return []views.InfoField{
					{Label: "Repo", Value: state.Repo},
					{Label: "Member", Value: state.MemberID},
				}
			},
		},
		// ── Team processing: clone + init + register ──
		{
			Label: i18n.T("cmd.init.wizard_step_team"),
			SkipIf: func() bool {
				return state.Skipped
			},
			Processing: i18n.T("cmd.init.wizard_processing_team"),
			OnDone: func() error {
				displayName := state.DisplayName
				if displayName == "" {
					displayName = state.MemberID
				}
				err := teamInitCore(context.Background(), *a, teamInitParams{
					StateRepo:   state.Repo,
					MemberID:    state.MemberID,
					DisplayName: displayName,
					Role:        "dev",
				})
				if err != nil {
					return err
				}
				state.Configured = true
				state.TeamID = config.RepoNameFromRemote(state.Repo)

				// Reload app so subsequent steps see the team
				config.Reset()
				if newApp, reloadErr := ReloadApp(); reloadErr == nil {
					*a = newApp
				}
				return nil
			},
			InfoFields: func() []views.InfoField {
				if state.Configured {
					return []views.InfoField{
						{Label: i18n.T("cmd.init.wizard_step_team"), Value: i18n.T("cmd.init.wizard_team_connected")},
					}
				}
				return []views.InfoField{
					{Label: i18n.T("cmd.init.wizard_step_team"), Value: i18n.T("cmd.init.wizard_team_skipped")},
				}
			},
		},
	}
}

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
