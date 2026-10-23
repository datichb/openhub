package cmd

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/hubcontent"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// needsFirstRunWizard returns true if the setup wizard has never been completed.
func needsFirstRunWizard(a *app.App) bool {
	return !a.Config.CLI.SetupDone
}

// buildFirstRunInlineWizard creates an InlineWizardView for the first-run setup.
// It assembles steps from individual builders in init_steps.go and the team
// builders in team_helpers.go.
//
// This function is used by both the TUI shell (via PushView) and the CLI
// `oh init` command (via RunInlineWizardStandalone).
func buildFirstRunInlineWizard(a *app.App) *views.InlineWizardView {
	cfg, _ := buildInitWizardConfig(a)
	return views.NewInlineWizardView(*cfg)
}

// buildInitWizardConfig assembles the configuration of the init wizard and
// the shared state its steps read and write.
func buildInitWizardConfig(a *app.App) (*views.InlineWizardConfig, *initStepState) {
	// ── Shared mutable state ─────────────────────────────────────────────
	appPtr := &a
	teamState := &initWizardTeamState{Ctx: context.Background()}

	s := &initStepState{
		ProviderOptions: []string{"bedrock", "anthropic", "openrouter", "github-copilot"},
		TeamState:       teamState,
		AppPtr:          appPtr,
	}

	// ── Pre-fill from existing config (re-init scenario) ────────────────
	// When SetupDone was previously true, provider/region values exist in
	// the config. Pre-populate the state so the user sees their previous
	// choices and can confirm or adjust them.
	if a.Config.CLI.SetupDone {
		if p := a.Config.LLM.DefaultProvider; p != "" {
			s.SelectedProvider = p
			for idx, opt := range s.ProviderOptions {
				if opt == p {
					s.ProviderIdx = idx
					break
				}
			}
		}
		if a.Config.LLM.DefaultProvider == "bedrock" {
			if am := a.Config.Provider.Bedrock.AuthMode; am != "" {
				s.AuthMode = am
				switch am {
				case "bearer":
					s.AuthIdx = 0
				case "profile":
					s.AuthIdx = 1
					if p := a.Config.Provider.Bedrock.AWSProfile; p != "" {
						s.ProfileName = p
					}
				case "env":
					s.AuthIdx = 2
				}
			}
			if r := a.Config.Provider.Bedrock.AWSRegion; r != "" {
				s.Region = r
			}
		}
	}

	// ── Pre-fill project from existing DB entry at CWD ──────────────────
	if a.Projects != nil {
		if cwd, err := filepath.Abs("."); err == nil {
			if existing, err := a.Projects.GetByPath(context.Background(), cwd); err == nil &&
				existing != nil && existing.Status == domain.ProjectStatusActive {
				s.ExistingProject = existing
				s.ProjectName = existing.Name
				s.ProjectPath = existing.Path
				s.ProjectID = existing.ID
			}
		}
	}

	// ── Pre-fill team from existing hub config ──────────────────────────
	if activeTeam := a.Config.ActiveTeam(); activeTeam.Enabled && activeTeam.StateRepo != "" {
		tc := activeTeam // copy to avoid pointer aliasing
		teamState.ExistingTeam = &tc
		teamState.Repo = activeTeam.StateRepo
		teamState.MemberID = activeTeam.MemberID
	}

	// ── Detect system locale early ───────────────────────────────────────
	sysLang := os.Getenv("LANG")
	if sysLang == "" {
		sysLang = os.Getenv("LC_ALL")
	}
	if sysLang != "" && strings.Contains(strings.ToLower(sysLang), "fr") {
		s.SelectedLang = "fr"
		s.LangIdx = 0
	} else {
		s.SelectedLang = "en"
		s.LangIdx = 1
	}
	i18n.SetLocale(s.SelectedLang)

	// The setup type is preselected here (not only when the welcome screen
	// draws) so the sidebar already shows the right groups.
	s.SetupMode = s.defaultSetupMode()

	// ── Assemble steps ───────────────────────────────────────────────────
	// Screens: (1) welcome — language and setup type, (2) provider,
	// (3) project; between 2 and 3 the team group, for the "team" and "full"
	// setup types only. MCP integrations are not part of the wizard: the
	// summary suggests `oh mcp setup`.
	var steps []views.WizardStep
	s.Steps = &steps

	// Group: Welcome (language + setup type)
	steps = append(steps, buildWelcomeStep(s))

	// Group: Provider (intro as header + form).
	// In "team" mode, provider is inherited from team config → skip.
	providerStep := buildProviderStep(s)
	origProviderSkipIf := providerStep.SkipIf
	providerStep.SkipIf = func() bool {
		if s.SetupMode == "team" {
			return true
		}
		if origProviderSkipIf != nil {
			return origProviderSkipIf()
		}
		return false
	}
	steps = append(steps, providerStep)

	// Group: Team (intro + init steps + rejoin steps), "team" and "full"
	// setup types only. In "solo" mode the whole group is skipped.
	teamModeIntro := buildTeamModeIntroStep(teamState)
	teamModeIntro.SkipIf = func() bool {
		if s.SetupMode == "solo" {
			teamState.Skipped = true
			teamState.SoloSpace = true // working alone: a solo workflow space (P2-T16)
			return true
		}
		return false
	}
	// The intro is Required (no skipping with Esc) but must not block the
	// automatic skip of the solo setup type.
	teamModeIntro.Required = false
	steps = append(steps, teamModeIntro)
	steps = append(steps, buildInitWizardTeamSteps(appPtr, teamState)...)
	steps = append(steps, buildInitWizardRejoinSteps(appPtr, teamState)...)

	// Group: Project (optional, filled from the current folder)
	steps = append(steps, buildProjectStep(s))

	// ── Groups: each starts at the step it is named after ────────────────
	groupStarts := []struct {
		labelKey string
		stepID   string
		hidden   func() bool
	}{
		{"cmd.init.wizard_group_welcome", "welcome", nil},
		{"cmd.init.wizard_group_provider", "provider", func() bool { return s.SetupMode == "team" }},
		{"cmd.init.wizard_group_team", "team_mode", func() bool { return s.SetupMode == "solo" }},
		{"cmd.init.wizard_group_project", "project", nil},
	}
	groups := make([]views.StepGroup, len(groupStarts))
	groupLabelKeys := make([]string, len(groupStarts))
	for i, g := range groupStarts {
		groups[i] = views.StepGroup{
			Label:    i18n.T(g.labelKey),
			StartIdx: bedrockStepIndex(&steps, g.stepID),
			Hidden:   g.hidden,
		}
		groupLabelKeys[i] = g.labelKey
	}

	// ── Build config ─────────────────────────────────────────────────────
	cfg := &views.InlineWizardConfig{
		ID:     "wizard.init",
		Title:  i18n.T("cmd.init.wizard_title"),
		Steps:  steps,
		Groups: groups,
		SummaryTargetViewFunc: func() string {
			if s.ProjectCreated {
				return "project.mode"
			}
			return "home"
		},
		SummaryTargetLabelFunc: func() string {
			if s.ProjectCreated {
				return i18n.T("wizard.summary.goto_projects")
			}
			return i18n.T("wizard.summary.go_home")
		},
		SummaryOnAction: func(shell views.ShellAccess) {
			if s.ProjectCreated {
				shell.SetActiveProject(&views.ActiveProject{
					ID:   s.ProjectID,
					Name: s.ProjectName,
					Path: s.ProjectPath,
				})
				shell.SetMode(views.ModeProject)
			} else {
				shell.NavigateTo("home")
			}
		},
		SummaryNote: func() string { return i18n.T("cmd.init.wizard_summary_mcp") },
		OnComplete:  buildInitOnComplete(s),
	}

	// RefreshLabels re-evaluates all construction-time i18n labels.
	cfg.RefreshLabels = buildInitRefreshLabels(cfg, steps, groupLabelKeys)

	return cfg, s
}

// buildInitOnComplete creates the OnComplete callback for the init wizard.
func buildInitOnComplete(s *initStepState) func(bool, error) {
	return func(completed bool, err error) {
		if !completed || err != nil {
			return
		}

		// ── Extract hub content (agents, skills, permissions) ────────
		hubContentDir := hubcontent.HubContentDir()
		if extractErr := hubcontent.Extract(hubContentDir); extractErr != nil {
			slog.Error("failed to extract hub content", "error", extractErr)
		}

		// ── Detect branch pattern heuristic ─────────────────────────
		if pattern := detectBranchPatternHeuristic("."); pattern != "" {
			_ = updateHubConfig(*s.AppPtr, func(c *config.Config) error {
				if c.Worktree.BranchPattern == "" {
					c.Worktree.BranchPattern = pattern
				}
				return nil
			})
		}

		// ── Mark setup as done ──────────────────────────────────────
		for attempt := 0; attempt < 2; attempt++ {
			if err := updateHubConfig(*s.AppPtr, func(c *config.Config) error {
				c.CLI.SetupDone = true
				return nil
			}); err == nil {
				break
			} else if attempt == 1 {
				slog.Error("failed to persist setup_done flag after retry — wizard may re-launch", "error", err)
			} else {
				slog.Warn("retrying setup_done persistence", "error", err)
			}
		}

		// The Config the views hold takes what the wizard wrote (setup_done,
		// provider, region, branch_pattern…): the App they captured is not
		// replaced, so none of them keeps or saves an older copy.
		if reloadErr := refreshSharedConfig(*s.AppPtr); reloadErr != nil {
			slog.Warn("config refresh failed in init completion", "err", reloadErr)
		}

		// Launch the Tracker Discovery Wizard if requested.
		if s.TeamState.LaunchTrackerDiscovery {
			go func() {
				time.Sleep(200 * time.Millisecond)
				if tuiShell != nil {
					tuiShell.App().QueueUpdateDraw(func() {
						actionTrackerDiscovery()
					})
				}
			}()
		}
	}
}

// buildInitRefreshLabels creates the RefreshLabels callback for live locale switching.
// groupLabelKeys are the i18n keys of the group labels, in the order of cfg.Groups.
func buildInitRefreshLabels(cfg *views.InlineWizardConfig, steps []views.WizardStep, groupLabelKeys []string) func() {
	return func() {
		cfg.Title = i18n.T("cmd.init.wizard_title")

		// Group labels
		for i, key := range groupLabelKeys {
			cfg.Groups[i].Label = i18n.T(key)
		}

		// Step labels — resolved by ID (no fragile positional indices)
		refresh := func(id, labelKey string) {
			if step := views.StepByID(steps, id); step != nil {
				step.Label = i18n.T(labelKey)
			}
		}
		refreshP := func(id, labelKey, procKey string) {
			if step := views.StepByID(steps, id); step != nil {
				step.Label = i18n.T(labelKey)
				step.Processing = i18n.T(procKey)
			}
		}

		refresh("welcome", "cmd.init.wizard_step_welcome")
		refreshP("provider", "cmd.init.wizard_step_provider_label", "cmd.init.wizard_processing_credentials")
		refresh("team_mode", "cmd.init.wizard_step_team")

		// Team init steps
		refresh("team_form", "cmd.init.wizard_step_team_form")
		refresh("team_sync", "cmd.init.wizard_step_team_sync")

		// Team rejoin steps
		refreshP("rejoin_repo", "cmd.init.wizard_step_rejoin_repo", "cmd.init.wizard_processing_rejoin_clone")
		refresh("team_cred_mode", "cmd.init.wizard_step_team_cred_mode")
		refresh("team_creds", "cmd.init.wizard_step_team_creds")
		refresh("rejoin_member", "cmd.init.wizard_step_rejoin_member")
		refresh("rejoin_tracker_username", "tui.team.rejoin.tracker_step")
		refreshP("rejoin_validate", "cmd.init.wizard_step_rejoin_validate", "cmd.init.wizard_processing_rejoin_validate")

		// Project steps
		refreshP("project", "cmd.init.wizard_step_project", "cmd.init.wizard_processing_project")
	}
}
