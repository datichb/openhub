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
	var projectTeamCfg *domain.ProjectTeamConfig
	if project != nil {
		projectTeamCfg = project.TeamConfig
	}
	return config.ResolveTeamConfig(a.Config.Team, projectTeamCfg)
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
	if !a.Config.Team.Enabled || a.Config.Team.MemberID == "" {
		return hubMemberInfo{}
	}
	statePath := a.Config.Team.StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}
	repo := teamstate.NewRepo(a.Config.Team.StateRepo, statePath)
	if !repo.IsCloned() {
		return hubMemberInfo{MemberID: a.Config.Team.MemberID}
	}
	member, err := repo.GetMember(a.Config.Team.MemberID)
	if err != nil {
		return hubMemberInfo{MemberID: a.Config.Team.MemberID}
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
		effectiveMemberID = a.Config.Team.MemberID
	}
	if effectiveMemberID == "" {
		return runTeamCustomSetupResult{}, fmt.Errorf("member ID requis (ni fourni, ni disponible dans le hub)")
	}

	statePath := config.TeamStatePath(remote)
	repo := teamstate.NewRepo(remote, statePath)

	// Use a timeout context so the clone/pull never hangs the TUI indefinitely.
	// GIT_TERMINAL_PROMPT=0 (set in repo.git) prevents blocking on credential prompts.
	cloneCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var pullWarning string
	if err := repo.EnsureReady(cloneCtx); err != nil {
		if teamstate.IsPullWarning(err) {
			pullWarning = "Synchronisation impossible (credentials manquants) — " +
				"contenu local utilisé. Configurez un credential helper git pour les pulls automatiques."
		} else {
			return runTeamCustomSetupResult{}, fmt.Errorf("initialisation du repo team-state (timeout 30s) : %w", err)
		}
	}
	if err := repo.InitStructure(cloneCtx); err != nil {
		return runTeamCustomSetupResult{}, fmt.Errorf("init structure : %w", err)
	}

	member := teamstate.Member{
		ID:          effectiveMemberID,
		DisplayName: displayName,
		Role:        role,
		DefaultMode: "semi-auto",
	}
	if repo.HasMember(effectiveMemberID) {
		if err := repo.UpdateMember(member); err != nil {
			return runTeamCustomSetupResult{}, fmt.Errorf("mise à jour du membre : %w", err)
		}
	} else {
		if err := repo.AddMember(member); err != nil {
			return runTeamCustomSetupResult{}, fmt.Errorf("ajout du membre : %w", err)
		}
	}

	if err := repo.CommitAndPush(context.Background(), "team: init "+effectiveMemberID, "."); err != nil {
		return runTeamCustomSetupResult{}, fmt.Errorf("commit/push : %w", err)
	}

	return runTeamCustomSetupResult{StatePath: statePath, PullWarning: pullWarning}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Wizard step (used in oh project add)
// ─────────────────────────────────────────────────────────────────────────────

// buildProjectTeamStep returns a WizardStep that collects team configuration
// for a project during project creation or reconfiguration.
//
// Modes:
//   - "inherit"  → use hub team as-is (default when hub has a team)
//   - "custom"   → configure a project-specific team-state repo; full setup
//     (clone, init structure, add/update member) is run in OnDone
//   - "disabled" → explicitly opt out of team for this project
//
// The custom team setup is optional: if the git remote URL is left empty the
// mode is treated as disabled instead of returning an error.
func buildProjectTeamStep(a *app.App, out **domain.ProjectTeamConfig) views.WizardStep {
	hubTeam := a.Config.Team
	hubMember := getHubMemberInfo(a)

	// Local state for the step
	var (
		teamMode    = domain.ProjectTeamModeInherit
		customRepo  string
		customMember = hubMember.MemberID
		displayName  = hubMember.DisplayName
		role         = hubMember.Role
	)
	if role == "" {
		role = "dev"
	}

	// If hub has no team, default to disabled (user must explicitly opt in)
	if !hubTeam.Enabled {
		teamMode = domain.ProjectTeamModeDisabled
	}

	roleOptions := []string{"Lead", "Développeur", "Reviewer"}
	roleValues := []string{"lead", "dev", "reviewer"}
	defaultRoleIdx := 1 // Développeur
	for i, v := range roleValues {
		if v == role {
			defaultRoleIdx = i
			break
		}
	}

	return views.WizardStep{
		Label:      "Team",
		Processing: "Initialisation du repo team-state...",
		Form: func(_ *tview.Application, onDone func()) *tview.Form {
			form := tview.NewForm()

			if hubTeam.Enabled {
				hubSummary := fmt.Sprintf("hub : %s (member : %s)", hubTeam.StateRepo, hubTeam.MemberID)
				form.AddTextView("Team hub", hubSummary, 0, 1, false, false)
				modeOptions := []string{
					fmt.Sprintf("Utiliser la team du hub (%s)", hubTeam.MemberID),
					"Configuration spécifique à ce projet",
					"Pas de team pour ce projet",
				}
				modeValues := []string{
					domain.ProjectTeamModeInherit,
					domain.ProjectTeamModeCustom,
					domain.ProjectTeamModeDisabled,
				}
				form.AddDropDown("Mode team", modeOptions, 0, func(_ string, idx int) {
					if idx >= 0 && idx < len(modeValues) {
						teamMode = modeValues[idx]
					}
				})
			} else {
				form.AddTextView("Team hub", "Aucune team configurée au niveau du hub.", 0, 1, false, false)
				modeOptions := []string{
					"Pas de team pour ce projet",
					"Configurer une team spécifique à ce projet",
				}
				modeValues := []string{
					domain.ProjectTeamModeDisabled,
					domain.ProjectTeamModeCustom,
				}
				form.AddDropDown("Mode team", modeOptions, 0, func(_ string, idx int) {
					if idx >= 0 && idx < len(modeValues) {
						teamMode = modeValues[idx]
					}
				})
			}

			// ── Custom fields ────────────────────────────────────────────────
			form.AddInputField("Git remote URL (vide = pas de custom)", "", 0, nil,
				func(text string) { customRepo = text })
			form.AddTextView("", "  Laisser vide pour désactiver la team", 0, 1, false, false)

			// ── Identity fields (regroupés) ──────────────────────────────────
			form.AddInputField("Member ID", customMember, 0, nil,
				func(text string) { customMember = text })
			form.AddTextView("", "  Identifiant unique dans l'équipe (ex: benjamin, alice)", 0, 1, false, false)
			form.AddInputField("Nom d'affichage", displayName, 0, nil,
				func(text string) { displayName = text })
			form.AddTextView("", "  Votre nom tel qu'il apparaîtra dans les events team", 0, 1, false, false)
			form.AddDropDown("Rôle", roleOptions, defaultRoleIdx, func(_ string, idx int) {
				if idx >= 0 && idx < len(roleValues) {
					role = roleValues[idx]
				}
			})

			form.AddButton("Next", func() { onDone() })
			return form
		},
		OnDone: func() error {
			switch teamMode {
			case domain.ProjectTeamModeDisabled:
				*out = &domain.ProjectTeamConfig{Mode: domain.ProjectTeamModeDisabled}
				return nil

			case domain.ProjectTeamModeInherit:
				*out = nil
				return nil

			case domain.ProjectTeamModeCustom:
				// Remote vide → traité comme disabled (optionnel)
				if customRepo == "" {
					*out = &domain.ProjectTeamConfig{Mode: domain.ProjectTeamModeDisabled}
					return nil
				}

				result, err := runTeamCustomSetup(a, customRepo, customMember, displayName, role)
				if err != nil {
					return err
				}

				effectiveMemberID := customMember
				if effectiveMemberID == "" {
					effectiveMemberID = a.Config.Team.MemberID
				}

				*out = &domain.ProjectTeamConfig{
					Mode:      domain.ProjectTeamModeCustom,
					StateRepo: customRepo,
					StatePath: result.StatePath,
					MemberID:  effectiveMemberID,
				}
				return nil
			}
			return nil
		},
		InfoFields: func() []views.InfoField {
			switch teamMode {
			case domain.ProjectTeamModeDisabled:
				return []views.InfoField{{Label: "Team", Value: "disabled"}}
			case domain.ProjectTeamModeCustom:
				if customRepo != "" {
					return []views.InfoField{{Label: "Team", Value: "custom : " + customRepo}}
				}
				return []views.InfoField{{Label: "Team", Value: "disabled (URL vide)"}}
			default:
				return []views.InfoField{{Label: "Team", Value: "inherit (hub)"}}
			}
		},
	}
}
