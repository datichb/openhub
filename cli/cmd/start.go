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

	effectiveServers := buildMCPServersForProject(a, project.MCPConfig)
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
		removed, _ := worktree.CleanupMerged(project.Path, baseBranch)
		if len(removed) > 0 {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess), i18n.Tf("cmd.start.worktree_cleanup", len(removed)))
			for _, b := range removed {
				fmt.Fprintf(a.IO.Out, "    %s %s\n", theme.Subtitle.Render("·"), b)
			}
		}
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconArrow), i18n.Tf("cmd.start.worktree_prep", theme.Bold.Render(branch)))

	wtPath, err := worktree.ResolveOrCreate(project.Path, branch)
	if err != nil {
		return "", fmt.Errorf("worktree: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.start.worktree_path", wtPath))

	hubDir := findHubDir()
	if hubDir == "" {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.WarningStyle.Render(theme.IconWarning), i18n.T("cmd.start.hub_not_found_warning"))
		return wtPath, nil
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconArrow), i18n.T("cmd.start.worktree_deploy"))

	plan := buildDeployPlan(a, wtPath, project.ID, hubDir, "", "", project.Agents, project.ModelOverrides, project.MCPConfig)
	results, err := deploy.Execute(plan)
	if err != nil {
		return "", fmt.Errorf("%s", i18n.Tf("cmd.start.worktree_deploy_failed", err))
	}

	for _, r := range results {
		if r.Success {
			fmt.Fprintf(a.IO.Out, "    %s %s\n",
				theme.SuccessStyle.Render(theme.IconSuccess), r.Name)
		} else {
			fmt.Fprintf(a.IO.Out, "    %s %s: %s\n",
				theme.ErrorStyle.Render(theme.IconError), r.Name, r.Message)
		}
	}
	fmt.Fprintln(a.IO.Out)

	return wtPath, nil
}
