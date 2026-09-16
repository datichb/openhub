package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/prompt"
	"github.com/datichb/openhub/cli/internal/provider"
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
	startCmd.Flags().Bool("recap", false, "Afficher le récap et demander confirmation avant le lancement")
	startCmd.Flags().BoolP("yes", "y", false, "Deprecated: le lancement rapide est le défaut. Utilisez --recap pour forcer le récap.")
	startCmd.Flags().Bool("parallel", false, "Lance N sessions en parallèle sur des tickets différents")
	startCmd.Flags().StringSlice("tickets", nil, "Liste des tickets à traiter en parallèle (séparés par des virgules)")
	startCmd.Flags().Int("max-sessions", 0, "Nombre max de sessions parallèles (0 = valeur config, default: 3)")
	startCmd.Flags().String("priority", "", "Ticket prioritaire (merge en premier)")

	// Mark --yes as deprecated (no-op with warning)
	_ = startCmd.Flags().MarkDeprecated("yes", "le lancement rapide est le défaut. Utilisez --recap pour forcer le récap.")

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

	// --- Resume mode (special: uses Exec, not the launcher) ---
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
	recapMode, _ := cmd.Flags().GetBool("recap")

	// --- Parallel mode (delegates entirely) ---
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

	// --- Resolve agent + prompt (pre-launch, mode-specific) ---
	providerFlag, _ := cmd.Flags().GetString("provider")
	agent, _ := cmd.Flags().GetString("agent")
	userPrompt, _ := cmd.Flags().GetString("prompt")

	// --- Auto-deploy if needed ---
	if !onboardMode {
		autoDeployIfNeeded(a, project, findHubDir(), providerFlag, "", !recapMode)
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

	// --- Default agent fallback ---
	if agent == "" {
		agent = "orchestrator"
	}

	// --- Delegate to launcher ---
	l := launcher.New(a, launcher.NewCLIUI(a.IO.Out))

	// Summary + confirmation only if --recap is explicitly set
	skipSummary := !recapMode
	skipConfirm := !recapMode

	// Print summary inline if recap mode (the launcher doesn't own summary rendering)
	if recapMode {
		stack := prompt.DetectStack(launchPath)
		var bearerToken string
		if a.Secrets != nil {
			bearerToken, _, _, _ = resolveCredentials(ctx, a, project, resolveProviderForDisplay(providerFlag, project, a))
		}
		printStartSummary(a, project, launchPath, resolveProviderForDisplay(providerFlag, project, a), stack, agent, bearerToken)
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n\n",
		theme.SuccessStyle.Render(theme.IconArrow), i18n.T("cmd.start.launching"))

	return l.Launch(ctx, launcher.LaunchOpts{
		ProjectID:   project.ID,
		ProjectPath: launchPath,
		Agent:       agent,
		Prompt:      userPrompt,
		Provider:    providerFlag,
		SkipSummary: skipSummary,
		SkipConfirm: skipConfirm,
		SkipDeploy:  true, // already handled above
	})
}

// resolveProviderForDisplay returns the effective provider name for display purposes.
func resolveProviderForDisplay(providerFlag string, project *domain.Project, a *app.App) string {
	prov := providerFlag
	if prov == "" {
		prov = project.Provider
	}
	if prov == "" {
		prov = a.Config.Opencode.DefaultProvider
	}
	if prov == "" {
		prov = "bedrock"
	}
	return prov
}

// resolveCredentials extracts provider-specific credentials from secrets.
// Uses provider.KeychainKey() for canonical key naming (openhub.provider.<name>.token[.<projectID>]).
func resolveCredentials(ctx context.Context, a *app.App, project *domain.Project, prov string) (bearerToken, apiKey, awsProfile, awsRegion string) {
	provName := provider.Name(prov)
	switch provName {
	case provider.Bedrock:
		bearerToken, _ = a.Secrets.Get(ctx, provider.KeychainKey(provName, project.ID))
		if bearerToken == "" {
			bearerToken, _ = a.Secrets.Get(ctx, provider.KeychainKey(provName, ""))
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
	case provider.Anthropic:
		apiKey, _ = a.Secrets.Get(ctx, provider.KeychainKey(provName, project.ID))
		if apiKey == "" {
			apiKey, _ = a.Secrets.Get(ctx, provider.KeychainKey(provName, ""))
		}
	case provider.OpenRouter:
		apiKey, _ = a.Secrets.Get(ctx, provider.KeychainKey(provName, project.ID))
		if apiKey == "" {
			apiKey, _ = a.Secrets.Get(ctx, provider.KeychainKey(provName, ""))
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
