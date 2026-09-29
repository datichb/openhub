package cmd

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/hubcontent"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// needsFirstRunWizard returns true if the setup wizard has never been completed.
func needsFirstRunWizard(a *app.App) bool {
	return !a.Config.CLI.SetupDone
}

// buildFirstRunInlineWizard creates an InlineWizardView for the first-run setup.
// It assembles steps from individual builders in init_steps.go and shared
// builders in team_helpers.go / mcp_steps.go.
//
// This function is used by both the TUI shell (via PushView) and the CLI
// `oh init` command (via RunInlineWizardStandalone).
func buildFirstRunInlineWizard(a *app.App) *views.InlineWizardView {
	// ── Shared mutable state ─────────────────────────────────────────────
	appPtr := &a
	focusBtn := false
	teamState := &initWizardTeamState{Ctx: context.Background()}

	s := &initStepState{
		ProviderOptions: []string{"bedrock", "anthropic", "openrouter", "github-copilot"},
		TeamState:       teamState,
		AppPtr:          appPtr,
		FocusBtn:        &focusBtn,
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

	// ── Assemble steps ───────────────────────────────────────────────────
	var steps []views.WizardStep
	s.Steps = &steps

	// Group: Langue (welcome + lang)
	steps = append(steps,
		buildWelcomeStep(s),
		buildLangStep(s),
	)

	// Group: Provider (single hybrid step with embedded intro)
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

	// Group: Equipe (intro + init steps + rejoin steps)
	// In "solo" mode, the entire team group is skipped.
	teamModeIntro := buildTeamModeIntroStep(teamState)
	teamModeIntro.SkipIf = func() bool {
		if s.SetupMode == "solo" {
			teamState.Skipped = true
			return true
		}
		return false
	}
	// When the team intro is skipped (solo mode), its Required flag would
	// block the skip. Override to non-required when solo.
	origRequired := teamModeIntro.Required
	_ = origRequired // keep the default for full/team modes
	teamModeIntro.Required = false
	steps = append(steps, teamModeIntro)
	steps = append(steps, buildInitWizardTeamSteps(appPtr, teamState)...)
	steps = append(steps, buildInitWizardRejoinSteps(appPtr, teamState)...)

	// Group: Projet (single step with embedded intro)
	projectGroupStart := len(steps)
	steps = append(steps, buildProjectStep(s))

	// Group: MCP (single consolidated step with intro + checkboxes + tokens)
	mcpGroupStart := len(steps)
	steps = append(steps, buildMCPConsolidatedStep(s, a))

	// Group: Déploiement (agent selection with embedded intro + deploy confirmation)
	deployGroupStart := len(steps)
	steps = append(steps, buildAgentSelectionStep(s))
	steps = append(steps, buildDeployStep(s))

	// ── Resolve step indices ─────────────────────────────────────────────
	s.LangStepIdx = 1 // lang is always at index 1 (after welcome)
	for i, step := range steps {
		if step.ID == "provider" {
			s.ProviderStepIdx = i
			break
		}
	}

	// ── Build config ─────────────────────────────────────────────────────
	// Provider group starts right after lang (welcome=0, lang=1, provider=2).
	providerGroupStart := 2
	// Team group starts right after provider step.
	teamGroupStart := providerGroupStart + 1

	cfg := views.InlineWizardConfig{
		ID:    "wizard.init",
		Title: i18n.T("cmd.init.wizard_title"),
		Steps: steps,
		Groups: []views.StepGroup{
			{Label: i18n.T("cmd.init.wizard_group_lang"), StartIdx: 0},
			{Label: i18n.T("cmd.init.wizard_group_provider"), StartIdx: providerGroupStart},
			{Label: i18n.T("cmd.init.wizard_group_team"), StartIdx: teamGroupStart},
			{Label: i18n.T("cmd.init.wizard_group_project"), StartIdx: projectGroupStart},
			{Label: i18n.T("cmd.init.wizard_group_mcp"), StartIdx: mcpGroupStart},
			{Label: i18n.T("cmd.init.wizard_group_deploy"), StartIdx: deployGroupStart},
		},
		FocusButtonAfterRender: &focusBtn,
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
				shell.SetProjectMode(&views.ActiveProject{
					ID:   s.ProjectID,
					Name: s.ProjectName,
					Path: s.ProjectPath,
				})
			} else {
				shell.NavigateTo("home")
			}
		},
		OnComplete: buildInitOnComplete(s),
	}

	// RefreshLabels re-evaluates all construction-time i18n labels.
	cfg.RefreshLabels = buildInitRefreshLabels(&cfg, steps)

	return views.NewInlineWizardView(cfg)
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
			_ = config.Update(func(c *config.Config) error {
				if c.Worktree.BranchPattern == "" {
					c.Worktree.BranchPattern = pattern
				}
				return nil
			})
		}

		// ── Mark setup as done ──────────────────────────────────────
		for attempt := 0; attempt < 2; attempt++ {
			if err := config.Update(func(c *config.Config) error {
				c.CLI.SetupDone = true
				return nil
			}); err == nil {
				break
			} else if attempt == 1 {
				slog.Error("failed to persist setup_done flag after retry — wizard may re-launch", "error", err)
			} else {
				slog.Warn("retrying setup_done persistence", "error", err)
				config.Reset()
			}
		}

		config.Reset()
		if newApp, reloadErr := ReloadApp(); reloadErr != nil {
			slog.Warn("ReloadApp failed in init completion", "err", reloadErr)
		} else {
			*s.AppPtr = newApp
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
func buildInitRefreshLabels(cfg *views.InlineWizardConfig, steps []views.WizardStep) func() {
	return func() {
		cfg.Title = i18n.T("cmd.init.wizard_title")

		// Group labels
		cfg.Groups[0].Label = i18n.T("cmd.init.wizard_group_lang")
		cfg.Groups[1].Label = i18n.T("cmd.init.wizard_group_provider")
		cfg.Groups[2].Label = i18n.T("cmd.init.wizard_group_team")
		cfg.Groups[3].Label = i18n.T("cmd.init.wizard_group_project")
		cfg.Groups[4].Label = i18n.T("cmd.init.wizard_group_mcp")
		cfg.Groups[5].Label = i18n.T("cmd.init.wizard_group_deploy")

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
		refresh("lang", "cmd.init.wizard_step_lang")
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
		refreshP("rejoin_validate", "cmd.init.wizard_step_rejoin_validate", "cmd.init.wizard_processing_rejoin_validate")

		// Project steps
		refreshP("project", "cmd.init.wizard_step_project", "cmd.init.wizard_processing_project")

		// Deploy steps
		refresh("agents", "cmd.init.wizard_step_agents")
		refreshP("deploy", "cmd.init.wizard_step_deploy_confirm", "cmd.init.wizard_deploy_processing")

		// MCP steps
		refresh("mcp_figma", "cmd.init.wizard_step_mcp_figma")
		refresh("mcp_gitlab", "cmd.init.wizard_step_mcp_gitlab")
		refresh("mcp_gslides", "cmd.init.wizard_step_mcp_gslides")
	}
}
