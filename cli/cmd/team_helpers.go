package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
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
// Hub member info
// ─────────────────────────────────────────────────────────────────────────────

// hubMemberInfo holds the display name and role of the current user as stored
// in the hub's team-state repo. Fields are empty if unavailable.
type hubMemberInfo struct {
	MemberID    string
	DisplayName string
	Role        string
}

// getHubMemberInfo reads the current user's display name and role from the
// hub's team-state repo (members.toml). Returns zero-value struct when the
// hub has no team, the repo is not cloned, or the member is not found.
func getHubMemberInfo(a *app.App) hubMemberInfo {
	if !a.Config.ActiveTeam().Enabled || a.Config.ActiveTeam().MemberID == "" {
		return hubMemberInfo{}
	}
	statePath := a.Config.ActiveTeam().StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}
	repo := teamstate.NewRepo(a.Config.ActiveTeam().StateRepo, statePath)
	if !repo.IsCloned() {
		return hubMemberInfo{MemberID: a.Config.ActiveTeam().MemberID}
	}
	member, err := repo.GetMember(a.Config.ActiveTeam().MemberID)
	if err != nil {
		return hubMemberInfo{MemberID: a.Config.ActiveTeam().MemberID}
	}
	return hubMemberInfo{
		MemberID:    member.ID,
		DisplayName: member.DisplayName,
		Role:        member.Role,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Custom team setup (shared between wizard and TUI action)
// ─────────────────────────────────────────────────────────────────────────────

// runTeamCustomSetupResult holds the result of a team custom setup operation.
type runTeamCustomSetupResult struct {
	StatePath string
	// PullWarning is non-empty when the repo was already cloned but pull failed
	// (e.g. missing HTTPS credentials). The setup succeeded — this is a warning only.
	PullWarning string
}

// runTeamCustomSetup clones (or pulls) a team-state repo, initialises its
// structure when needed, and adds/updates the member entry. It returns the
// auto-derived statePath so callers can persist it.
//
// If memberID is empty it falls back to the hub's member_id. If both are empty
// an error is returned.
func runTeamCustomSetup(a *app.App, remote, memberID, displayName, role string) (runTeamCustomSetupResult, error) {
	// Resolve member ID
	effectiveMemberID := memberID
	if effectiveMemberID == "" {
		effectiveMemberID = a.Config.ActiveTeam().MemberID
	}
	if effectiveMemberID == "" {
		return runTeamCustomSetupResult{}, fmt.Errorf("member ID requis (ni fourni, ni disponible dans le hub)")
	}

	statePath := config.TeamStatePath(remote)

	// Use a timeout context so the clone/pull never hangs the TUI indefinitely.
	// GIT_TERMINAL_PROMPT=0 (set in repo.git) prevents blocking on credential prompts.
	cloneCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := teamInitCore(cloneCtx, a, teamInitParams{
		StateRepo:   remote,
		StatePath:   statePath,
		MemberID:    effectiveMemberID,
		DisplayName: displayName,
		Role:        role,
	})

	var pullWarning string
	if err != nil && teamstate.IsPullWarning(err) {
		pullWarning = "Synchronisation impossible (credentials manquants) — " +
			"contenu local utilisé. Configurez un credential helper git pour les pulls automatiques."
		err = nil
	}
	if err != nil {
		return runTeamCustomSetupResult{}, err
	}

	return runTeamCustomSetupResult{StatePath: statePath, PullWarning: pullWarning}, nil
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
