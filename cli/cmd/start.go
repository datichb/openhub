package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/prompt"
	"github.com/datichb/openhub/cli/internal/tui/progress"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/worktree"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Lance une session opencode",
	Long: `Prépare le contexte projet puis lance opencode.
Détecte automatiquement le projet si vous êtes dans un répertoire enregistré.`,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
	startCmd.Flags().StringP("agent", "a", "", "Agent à utiliser")
	startCmd.Flags().StringP("prompt", "m", "", "Prompt initial")
	startCmd.Flags().StringP("provider", "P", "", "Provider LLM (bedrock, anthropic, openai)")
	startCmd.Flags().StringP("project", "p", "", "Nom du projet (détection auto sinon)")
	startCmd.Flags().StringP("resume", "r", "", "Reprendre une session existante (ID)")
	startCmd.Flags().StringP("worktree", "w", "", "Branche pour lancer dans un git worktree")
	startCmd.Flags().Bool("dev", false, "Mode développement (orchestrator-dev + tickets)")
	startCmd.Flags().StringP("ticket", "t", "", "Ticket ID à travailler directement (skip le picker, requiert --dev)")
	startCmd.Flags().StringP("label", "l", "", "Filtrer tickets par label (requiert --dev)")
	startCmd.Flags().StringP("assignee", "A", "", "Filtrer tickets par assignee (requiert --dev)")
	startCmd.Flags().Bool("onboard", false, "Mode onboarding — crée/enrichit le wiki projet")
	startCmd.Flags().Bool("refresh", false, "Force la re-découverte du wiki (requiert --onboard)")
	startCmd.Flags().BoolP("yes", "y", false, "Skip confirmation and launch immediately")
	startCmd.Flags().Bool("parallel", false, "Lance N sessions en parallèle sur des tickets différents")
	startCmd.Flags().StringSlice("tickets", nil, "Liste des tickets à traiter en parallèle (séparés par des virgules)")
	startCmd.Flags().Int("max-sessions", 0, "Nombre max de sessions parallèles (0 = valeur config, default: 3)")
	startCmd.Flags().String("priority", "", "Ticket prioritaire (merge en premier)")

	_ = startCmd.RegisterFlagCompletionFunc("project", completeProjectIDs)
}

func runStart(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	// --- Ensure opencode is installed ---
	if err := ensureOpencode(a); err != nil {
		return err
	}

	// --- Compatibility warning ---
	if ocVersion, err := opencode.Version(); err == nil {
		compat := opencode.CheckCompatibility(buildinfo.Version, ocVersion)
		if !compat.Compatible {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.WarningStyle.Render(theme.IconWarning),
				compat.Warning)
		}
	}

	// --- Resume mode ---
	resumeID, _ := cmd.Flags().GetString("resume")
	if resumeID != "" {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.SuccessStyle.Render(theme.IconArrow), i18n.Tf("cmd.start.resume", resumeID))
		return opencode.Exec(opencode.StartOpts{
			ResumeSessionID: resumeID,
		})
	}

	// --- Validate flag combinations ---
	devMode, _ := cmd.Flags().GetBool("dev")
	onboardMode, _ := cmd.Flags().GetBool("onboard")
	parallelMode, _ := cmd.Flags().GetBool("parallel")
	labelFlag, _ := cmd.Flags().GetString("label")
	assigneeFlag, _ := cmd.Flags().GetString("assignee")
	refreshFlag, _ := cmd.Flags().GetBool("refresh")

	// --- Parallel mode ---
	if parallelMode {
		return runParallelMode(cmd, a, ctx)
	}

	if labelFlag != "" && !devMode {
		return fmt.Errorf("%s", i18n.Tf("cmd.start.flag_requires_dev", "label"))
	}
	if assigneeFlag != "" && !devMode {
		return fmt.Errorf("%s", i18n.Tf("cmd.start.flag_requires_dev", "assignee"))
	}
	if labelFlag != "" && assigneeFlag != "" {
		return fmt.Errorf("%s", i18n.T("cmd.start.label_assignee_exclusive"))
	}
	if refreshFlag && !onboardMode {
		return fmt.Errorf("%s", i18n.T("cmd.start.refresh_requires_onboard"))
	}
	if devMode && onboardMode {
		return fmt.Errorf("%s", i18n.T("cmd.start.dev_onboard_exclusive"))
	}

	// --- Resolve project ---
	projectID, _ := cmd.Flags().GetString("project")
	project, err := resolveProject(ctx, a, projectID)
	if err != nil {
		return err
	}

	// --- Worktree mode ---
	wtBranch, _ := cmd.Flags().GetString("worktree")
	var launchPath string

	if wtBranch != "" || cmd.Flags().Changed("worktree") {
		launchPath, err = handleWorktreeMode(a, project, wtBranch)
		if err != nil {
			return err
		}
	} else {
		launchPath = project.Path
	}

	// --- Resolve provider + credentials ---
	provider, _ := cmd.Flags().GetString("provider")
	if provider == "" {
		provider = project.Provider
	}
	if provider == "" {
		provider = a.Config.Opencode.DefaultProvider
	}
	if provider == "" {
		provider = "bedrock"
	}

	var bearerToken, apiKey, awsProfile, awsRegion string
	if a.Secrets != nil {
		bearerToken, apiKey, awsProfile, awsRegion = resolveCredentials(ctx, a, project, provider)
	}

	// --- Detect stack ---
	stack := prompt.DetectStack(launchPath)
	agent, _ := cmd.Flags().GetString("agent")
	userPrompt, _ := cmd.Flags().GetString("prompt")

	// --- Auto-deploy if needed ---
	// Ensures the project's deployed config (.opencode/, opencode.json) is up
	// to date before launching. Non-blocking on success; warns and pauses on error.
	if !onboardMode {
		skipConfirmEarly, _ := cmd.Flags().GetBool("yes")
		autoDeployIfNeeded(a, project, findHubDir(), provider, "", skipConfirmEarly)
	}

	// --- Dev mode ---
	if devMode {
		devAgent, devPrompt, err := handleDevMode(cmd, a, project, launchPath)
		if err != nil {
			return err
		}
		agent = devAgent
		userPrompt = devPrompt
	}

	// --- Onboard mode ---
	if onboardMode {
		agent = "onboarder"
		hubDir := findHubDir()
		userPrompt = prompt.BuildOnboardPrompt(project, hubDir, refreshFlag || prompt.WikiExists(launchPath))
		if refreshFlag {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconArrow), i18n.T("cmd.start.onboard_refresh"))
		} else {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconArrow), i18n.T("cmd.start.onboard_launching"))
		}
	}

	// --- Display summary ---
	printStartSummary(a, project, launchPath, provider, stack, agent, bearerToken)

	// --- Confirmation ---
	skipConfirm, _ := cmd.Flags().GetBool("yes")
	if !skipConfirm {
		var confirm bool
		err := theme.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(i18n.T("cmd.start.confirm_launch")).
					Affirmative("Launch").
					Negative("Cancel").
					Value(&confirm),
			),
		).Run()
		if err != nil || !confirm {
			fmt.Fprintf(a.IO.Out, "%s %s\n", theme.Subtitle.Render(theme.IconArrow), i18n.T("cmd.start.cancelled"))
			return err
		}
	}

	// --- Launch ---
	fmt.Fprintf(a.IO.Out, "%s %s\n\n",
		theme.SuccessStyle.Render(theme.IconArrow), i18n.T("cmd.start.launching"))

	session := &domain.Session{
		ID:        uuid.New().String(),
		ProjectID: project.ID,
		Status:    domain.SessionStatusRunning,
		Provider:  provider,
	}
	if a.Sessions != nil {
		if err := a.Sessions.Create(ctx, session); err != nil {
			slog.Warn("session tracking failed", "error", err)
		}
	}

	runErr := opencode.Run(opencode.StartOpts{
		ProjectPath: launchPath,
		ProjectID:   project.ID,
		Agent:       agent,
		Prompt:      userPrompt,
		Provider:    provider,
		BearerToken: bearerToken,
		APIKey:      apiKey,
		AWSProfile:  awsProfile,
		AWSRegion:   awsRegion,
	})

	if a.Sessions != nil && session.ID != "" {
		if runErr != nil {
			session.Status = domain.SessionStatusFailed
		} else {
			session.Status = domain.SessionStatusCompleted
		}
		now := time.Now()
		session.EndedAt = &now
		_ = a.Sessions.Update(ctx, session)
	}

	return runErr
}

// resolveCredentials extracts provider-specific credentials from secrets.
func resolveCredentials(ctx context.Context, a *app.App, project *domain.Project, provider string) (bearerToken, apiKey, awsProfile, awsRegion string) {
	switch provider {
	case "bedrock":
		bearerToken, _ = a.Secrets.Get(ctx, "bedrock-token-"+project.ID)
		if bearerToken == "" {
			bearerToken, _ = a.Secrets.Get(ctx, "bedrock-token-default")
		}
		if project.ProviderConfig != nil && project.ProviderConfig.AWSProfile != "" {
			awsProfile = project.ProviderConfig.AWSProfile
		} else {
			awsProfile = a.Config.Provider.Bedrock.AWSProfile
		}
		if project.ProviderConfig != nil && project.ProviderConfig.AWSRegion != "" {
			awsRegion = project.ProviderConfig.AWSRegion
		} else {
			awsRegion = a.Config.Provider.Bedrock.AWSRegion
		}
	case "anthropic":
		apiKey, _ = a.Secrets.Get(ctx, "anthropic-api-key-"+project.ID)
		if apiKey == "" {
			apiKey, _ = a.Secrets.Get(ctx, "anthropic-api-key-default")
		}
	case "openrouter":
		apiKey, _ = a.Secrets.Get(ctx, "openrouter-api-key-"+project.ID)
		if apiKey == "" {
			apiKey, _ = a.Secrets.Get(ctx, "openrouter-api-key-default")
		}
	}
	return
}

// printStartSummary prints the pre-launch info blocks to stdout.
func printStartSummary(a *app.App, project *domain.Project, launchPath, provider string, stack prompt.StackInfo, agent, bearerToken string) {
	projCfg := opencode.ReadProjectConfig(launchPath)

	branch := "—"
	if b, err := worktree.CurrentBranch(launchPath); err == nil {
		branch = b
	}
	model := projCfg.Model
	if model == "" {
		model = "—"
	}
	compactionStatus := i18n.T("cmd.start.compaction_disabled")
	if projCfg.Compaction != nil && projCfg.Compaction.Auto {
		compactionStatus = i18n.T("cmd.start.compaction_auto")
	}

	effectiveServers := buildMCPServersForProject(a, project.MCPConfig, resolvedTeamConfig(a, project))
	var mcpNames []string
	for _, srv := range effectiveServers {
		if srv.Enabled && srv.Name != "team" {
			mcpNames = append(mcpNames, srv.Name)
		}
	}
	mcpDisplay := i18n.T("cmd.start.mcp_none")
	if len(mcpNames) > 0 {
		mcpDisplay = strings.Join(mcpNames, ", ")
	}

	pluginsDisplay := i18n.T("cmd.start.mcp_none")
	if len(projCfg.Plugins) > 0 {
		pluginsDisplay = strings.Join(projCfg.Plugins, ", ")
	}

	providerStatus := provider
	if bearerToken != "" {
		providerStatus = theme.SuccessStyle.Render(theme.IconSuccess) + " " + provider + " — " + i18n.T("cmd.start.token_configured")
	}

	gutter := theme.Subtitle.Render("│")
	header := theme.Title.Render("◆")
	footer := theme.Subtitle.Render("└")

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "%s  %s\n", header, theme.Bold.Render(project.Name))
	fmt.Fprintf(a.IO.Out, "%s\n", gutter)
	fmt.Fprintf(a.IO.Out, "%s  %s%s\n", gutter, i18n.T("cmd.start.label_path"), launchPath)
	fmt.Fprintf(a.IO.Out, "%s  %s%s\n", gutter, i18n.T("cmd.start.label_branch"), branch)
	fmt.Fprintf(a.IO.Out, "%s  %s%s\n", gutter, i18n.T("cmd.start.label_provider"), providerStatus)
	fmt.Fprintf(a.IO.Out, "%s\n", gutter)
	fmt.Fprintln(a.IO.Out)

	fmt.Fprintf(a.IO.Out, "%s  %s\n", header, theme.Bold.Render(i18n.T("cmd.start.section_config")))
	fmt.Fprintf(a.IO.Out, "%s\n", gutter)
	fmt.Fprintf(a.IO.Out, "%s  %s%s\n", gutter, i18n.T("cmd.start.label_provider_short"), provider)
	fmt.Fprintf(a.IO.Out, "%s  %s%s\n", gutter, i18n.T("cmd.start.label_model"), model)
	fmt.Fprintf(a.IO.Out, "%s  %s%s\n", gutter, i18n.T("cmd.start.label_language"), displayOrDefault(stack.Language, project.Language))
	fmt.Fprintf(a.IO.Out, "%s  %s%s\n", gutter, i18n.T("cmd.start.label_compaction"), compactionStatus)
	fmt.Fprintf(a.IO.Out, "%s  %s%s\n", gutter, i18n.T("cmd.start.label_mcp"), mcpDisplay)
	fmt.Fprintf(a.IO.Out, "%s  %s%s\n", gutter, i18n.T("cmd.start.label_plugins"), pluginsDisplay)
	if agent != "" {
		fmt.Fprintf(a.IO.Out, "%s  %s%s\n", gutter, i18n.T("cmd.start.label_agent"), agent)
	}
	fmt.Fprintf(a.IO.Out, "%s  %s\n", footer, theme.Subtitle.Render(i18n.Tf("cmd.start.summary_version", buildinfo.Version)))
	fmt.Fprintln(a.IO.Out)
}

// handleWorktreeMode manages the worktree workflow.
func handleWorktreeMode(a *app.App, project *domain.Project, branch string) (string, error) {
	if !worktree.IsGitRepo(project.Path) {
		return "", fmt.Errorf("%s", i18n.Tf("cmd.start.worktree_not_git", project.Name))
	}

	if branch == "" {
		form := theme.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title(i18n.T("cmd.worktree.branch_name")).
					Description(i18n.T("cmd.worktree.branch_desc")).
					Value(&branch),
			),
		)
		if err := form.Run(); err != nil {
			return "", err
		}
		if branch == "" {
			return "", fmt.Errorf("%s", i18n.T("cmd.start.worktree_branch_required"))
		}
	}

	if a.Config.Worktree.AutoCleanup {
		baseBranch := a.Config.Worktree.BaseBranch
		if baseBranch == "" {
			baseBranch = worktree.DetectBaseBranch(project.Path)
		}
		var cleanupResult worktree.CleanupResult
		_ = progress.Run(
			i18n.T("cmd.start.worktree_autocleanup"),
			func() error {
				cleanupResult, _ = worktree.CleanupMerged(project.Path, baseBranch, false)
				return nil
			},
		)
		if len(cleanupResult.Removed) > 0 {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.start.worktree_cleanup", len(cleanupResult.Removed)))
			for _, b := range cleanupResult.Removed {
				fmt.Fprintf(a.IO.Out, "    %s %s\n", theme.Subtitle.Render("·"), b)
			}
		}
		if len(cleanupResult.Skipped) > 0 {
			fmt.Fprintf(a.IO.Out, "  %s %s\n",
				theme.Subtitle.Render(theme.IconWarning), i18n.Tf("cmd.start.worktree_cleanup_skipped", len(cleanupResult.Skipped)))
		}
	}

	var wtPath string
	if err := progress.Run(
		i18n.Tf("cmd.start.worktree_prep", theme.Bold.Render(branch)),
		func() error {
			var e error
			wtPath, e = worktree.ResolveOrCreate(project.Path, branch)
			return e
		},
	); err != nil {
		return "", fmt.Errorf("worktree: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.start.worktree_path", wtPath))

	// Link worktree to the main project's deployed config via relative symlinks.
	// The main project is guaranteed to be deployed by autoDeployIfNeeded()
	// which runs before handleWorktreeMode in the start flow.
	if err := worktree.EnsureWorktreeConfig(wtPath, project.Path); err != nil {
		return "", fmt.Errorf("worktree config: %w", err)
	}
	fmt.Fprintf(a.IO.Out, "  %s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.T("cmd.start.worktree_linked"))

	return wtPath, nil
}

// autoDeployIfNeeded checks whether the project's deployed configuration is
// absent or stale, and auto-deploys if necessary. It always prints a status
// line so the user knows what happened. On error it prints a warning and waits
// for the user to press Enter (unless skipPrompt is true).
func autoDeployIfNeeded(a *app.App, project *domain.Project, hubDir, provider, model string, skipPrompt bool) {
	if hubDir == "" {
		return // hub content not available — nothing to deploy
	}

	out := a.IO.Out

	// CASE 1: Never deployed (no .deploy-state)
	if deploy.ReadDeployState(project.Path) == nil {
		var results []deploy.PhaseResult
		err := progress.Run(
			i18n.T("cmd.start.autodeploy_first"),
			func() error {
				plan := buildDeployPlan(a, project.Path, project.ID, hubDir, provider, model,
					project.Agents, project.ModelOverrides, project.MCPConfig, project.TeamConfig)
				var e error
				results, e = deploy.Execute(plan)
				return e
			},
		)
		if err != nil {
			fmt.Fprintf(out, "  %s %s\n",
				theme.WarningStyle.Render(theme.IconWarning),
				i18n.Tf("cmd.start.autodeploy_failed", err))
			if !skipPrompt {
				fmt.Fprintf(out, "  %s", i18n.T("cmd.start.autodeploy_continue"))
				fmt.Scanln()
			}
			return
		}
		agentCount, skillCount, mcpCount := countDeployResults(results)
		fmt.Fprintf(out, "  %s %s\n",
			theme.SuccessStyle.Render(theme.IconSuccess),
			i18n.Tf("cmd.start.autodeploy_done_first", agentCount, skillCount, mcpCount))
		return
	}

	// CASE 2: Already deployed — check staleness
	report, err := deploy.ComputeDiff(hubDir, project.Path, project.Agents)
	if err != nil || !report.HasChanges() {
		// CASE 3: Up to date (or diff error — conservative, no redeploy)
		fmt.Fprintf(out, "  %s %s\n",
			theme.SuccessStyle.Render(theme.IconSuccess),
			i18n.T("cmd.start.autodeploy_uptodate"))
		return
	}

	// CASE 4: Stale — incremental deploy
	added, modified, removed, _ := report.Summary()
	err = progress.Run(
		i18n.Tf("cmd.start.autodeploy_updating", added, modified, removed),
		func() error {
			plan := buildDeployPlan(a, project.Path, project.ID, hubDir, provider, model,
				project.Agents, project.ModelOverrides, project.MCPConfig, project.TeamConfig)
			_, e := deploy.Execute(plan)
			return e
		},
	)
	if err != nil {
		fmt.Fprintf(out, "  %s %s\n",
			theme.WarningStyle.Render(theme.IconWarning),
			i18n.Tf("cmd.start.autodeploy_failed", err))
		if !skipPrompt {
			fmt.Fprintf(out, "  %s", i18n.T("cmd.start.autodeploy_continue"))
			fmt.Scanln()
		}
		return
	}
	fmt.Fprintf(out, "  %s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.T("cmd.start.autodeploy_done_update"))
}

// countDeployResults extracts agent, skill and MCP counts from phase results.
func countDeployResults(results []deploy.PhaseResult) (agents, skills, mcp int) {
	for _, r := range results {
		if !r.Success {
			continue
		}
		switch r.Name {
		case "agents":
			agents = parseCount(r.Message)
		case "skills":
			skills = parseCount(r.Message)
		case "mcp":
			mcp = parseCount(r.Message)
		}
	}
	return
}

// parseCount extracts the first integer from a string like "3 deployed".
func parseCount(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else if n > 0 {
			break
		}
	}
	return n
}
