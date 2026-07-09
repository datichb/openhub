package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/buildinfo"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/prompt"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/common"
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
	startCmd.Flags().StringP("prompt", "p", "", "Prompt initial")
	startCmd.Flags().StringP("provider", "P", "", "Provider LLM (bedrock, anthropic, openai)")
	startCmd.Flags().StringP("project", "j", "", "ID du projet (détection auto sinon)")
	startCmd.Flags().StringP("resume", "r", "", "Reprendre une session existante (ID)")
	startCmd.Flags().StringP("worktree", "w", "", "Branche pour lancer dans un git worktree")
	startCmd.Flags().Bool("dev", false, "Mode développement (orchestrator-dev + tickets)")
	startCmd.Flags().StringP("ticket", "t", "", "Ticket ID à travailler directement (skip le picker, requiert --dev)")
	startCmd.Flags().StringP("label", "l", "", "Filtrer tickets par label (requiert --dev)")
	startCmd.Flags().StringP("assignee", "A", "", "Filtrer tickets par assignee (requiert --dev)")
	startCmd.Flags().Bool("onboard", false, "Mode onboarding — crée/enrichit le wiki projet")
	startCmd.Flags().Bool("refresh", false, "Force la re-découverte du wiki (requiert --onboard)")
	startCmd.Flags().BoolP("yes", "y", false, "Skip confirmation and launch immediately")

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
				common.WarningStyle.Render(common.IconWarning),
				compat.Warning)
		}
	}

	// --- Resume mode ---
	resumeID, _ := cmd.Flags().GetString("resume")
	if resumeID != "" {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconArrow), i18n.Tf("cmd.start.resume", resumeID))
		return opencode.Exec(opencode.StartOpts{
			ResumeSessionID: resumeID,
		})
	}

	// --- Validate flag combinations ---
	devMode, _ := cmd.Flags().GetBool("dev")
	onboardMode, _ := cmd.Flags().GetBool("onboard")
	labelFlag, _ := cmd.Flags().GetString("label")
	assigneeFlag, _ := cmd.Flags().GetString("assignee")
	refreshFlag, _ := cmd.Flags().GetBool("refresh")

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
		provider = project.Provider // project-level override
	}
	if provider == "" {
		provider = a.Config.Opencode.DefaultProvider
	}
	if provider == "" {
		provider = "bedrock" // ultimate fallback
	}

	var bearerToken, apiKey, awsProfile, awsRegion string
	if a.Secrets != nil {
		switch provider {
		case "bedrock":
			// Resolve bearer token: project → hub
			bearerToken, _ = a.Secrets.Get(ctx, "bedrock-token-"+project.ID)
			if bearerToken == "" {
				bearerToken, _ = a.Secrets.Get(ctx, "bedrock-token-default")
			}
			// Resolve AWS config: project → hub
			if project.ProviderConfig != nil && project.ProviderConfig.AWSProfile != "" {
				awsProfile = project.ProviderConfig.AWSProfile
			} else if a.Config.Provider.Bedrock.AWSProfile != "" {
				awsProfile = a.Config.Provider.Bedrock.AWSProfile
			}
			if project.ProviderConfig != nil && project.ProviderConfig.AWSRegion != "" {
				awsRegion = project.ProviderConfig.AWSRegion
			} else if a.Config.Provider.Bedrock.AWSRegion != "" {
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
	}

	// --- Detect stack and build context ---
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
				common.SuccessStyle.Render(common.IconArrow), i18n.T("cmd.start.onboard_refresh"))
		} else {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				common.SuccessStyle.Render(common.IconArrow), i18n.T("cmd.start.onboard_launching"))
		}
	}

	// --- Display summary ---
	projCfg := opencode.ReadProjectConfig(launchPath)

	// Resolve display values
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

	// MCP servers enabled
	var mcpNames []string
	if a.Config.MCP.Figma.Enabled {
		mcpNames = append(mcpNames, "figma")
	}
	if a.Config.MCP.Gitlab.Enabled {
		mcpNames = append(mcpNames, "gitlab")
	}
	if a.Config.MCP.Gslides.Enabled {
		mcpNames = append(mcpNames, "gslides")
	}
	mcpDisplay := i18n.T("cmd.start.mcp_none")
	if len(mcpNames) > 0 {
		mcpDisplay = strings.Join(mcpNames, ", ")
	}

	// Plugins
	pluginsDisplay := i18n.T("cmd.start.mcp_none")
	if len(projCfg.Plugins) > 0 {
		pluginsDisplay = strings.Join(projCfg.Plugins, ", ")
	}

	// Provider status line
	providerStatus := provider
	if bearerToken != "" {
		providerStatus = common.SuccessStyle.Render(common.IconSuccess) + " " + provider + " — " + i18n.T("cmd.start.token_configured")
	}

	// --- Block 1: Project ---
	gutter := common.Subtitle.Render("│")
	header := common.Title.Render("◆")
	footer := common.Subtitle.Render("└")

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "%s  %s\n", header, common.Bold.Render(project.Name))
	fmt.Fprintf(a.IO.Out, "%s\n", gutter)
	fmt.Fprintf(a.IO.Out, "%s  %s%s\n", gutter, i18n.T("cmd.start.label_path"), launchPath)
	fmt.Fprintf(a.IO.Out, "%s  %s%s\n", gutter, i18n.T("cmd.start.label_branch"), branch)
	fmt.Fprintf(a.IO.Out, "%s  %s%s\n", gutter, i18n.T("cmd.start.label_provider"), providerStatus)
	fmt.Fprintf(a.IO.Out, "%s\n", gutter)
	fmt.Fprintln(a.IO.Out)

	// --- Block 2: Configuration ---
	fmt.Fprintf(a.IO.Out, "%s  %s\n", header, common.Bold.Render(i18n.T("cmd.start.section_config")))
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
	fmt.Fprintf(a.IO.Out, "%s  %s\n", footer, common.Subtitle.Render(i18n.Tf("cmd.start.summary_version", buildinfo.Version)))
	fmt.Fprintln(a.IO.Out)

	// --- Confirmation ---
	skipConfirm, _ := cmd.Flags().GetBool("yes")
	if !skipConfirm {
		var confirm bool
		err := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title(i18n.T("cmd.start.confirm_launch")).
					Affirmative("Launch").
					Negative("Cancel").
					Value(&confirm),
			),
		).Run()
		if err != nil || !confirm {
			fmt.Fprintf(a.IO.Out, "%s %s\n", common.Subtitle.Render(common.IconArrow), i18n.T("cmd.start.cancelled"))
			return err
		}
	}

	// --- Launch ---
	fmt.Fprintf(a.IO.Out, "%s %s\n\n",
		common.SuccessStyle.Render(common.IconArrow), i18n.T("cmd.start.launching"))

	// Create session in oh DB
	session := &domain.Session{
		ID:        uuid.New().String(),
		ProjectID: project.ID,
		Status:    domain.SessionStatusRunning,
		Provider:  provider,
	}
	if a.Sessions != nil {
		if err := a.Sessions.Create(ctx, session); err != nil {
			// Non-fatal — don't block the launch
			slog.Warn("session tracking failed", "error", err)
		}
	}

	// Run opencode as subprocess (not exec) to retain control for post-session updates
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

	// Update session status after opencode exits
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

// handleWorktreeMode manages the worktree workflow:
// 1. Prompt for branch if empty
// 2. Auto-cleanup merged worktrees (if configured)
// 3. Create or reuse worktree
// 4. Deploy hub config into worktree
// Returns the launch path (worktree directory).
func handleWorktreeMode(a *app.App, project *domain.Project, branch string) (string, error) {
	// Verify git repo
	if !worktree.IsGitRepo(project.Path) {
		return "", fmt.Errorf("%s", i18n.Tf("cmd.start.worktree_not_git", project.Name))
	}

	// Prompt for branch name if not provided
	if branch == "" {
		form := huh.NewForm(
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

	// Auto-cleanup merged worktrees if configured
	if a.Config.Worktree.AutoCleanup {
		baseBranch := a.Config.Worktree.BaseBranch
		if baseBranch == "" {
			baseBranch = worktree.DetectBaseBranch(project.Path)
		}

		removed, _ := worktree.CleanupMerged(project.Path, baseBranch)
		if len(removed) > 0 {
			fmt.Fprintf(a.IO.Out, "%s %s\n",
				common.SuccessStyle.Render(common.IconSuccess), i18n.Tf("cmd.start.worktree_cleanup", len(removed)))
			for _, b := range removed {
				fmt.Fprintf(a.IO.Out, "    %s %s\n", common.Subtitle.Render("·"), b)
			}
		}
	}

	// Create or reuse worktree
	fmt.Fprintf(a.IO.Out, "%s %s\n",
		common.SuccessStyle.Render(common.IconArrow), i18n.Tf("cmd.start.worktree_prep", common.Bold.Render(branch)))

	wtPath, err := worktree.ResolveOrCreate(project.Path, branch)
	if err != nil {
		return "", fmt.Errorf("worktree: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.start.worktree_path", wtPath))

	// Deploy hub config into worktree
	hubDir := findHubDir()
	if hubDir == "" {
		// Not a fatal error — worktree can work without hub deploy
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.WarningStyle.Render(common.IconWarning), i18n.T("cmd.start.hub_not_found_warning"))
		return wtPath, nil
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		common.SuccessStyle.Render(common.IconArrow), i18n.T("cmd.start.worktree_deploy"))

	plan := buildDeployPlan(a, wtPath, project.ID, hubDir, "", "", project.Agents, project.ModelOverrides, project.MCPConfig)

	results, err := deploy.Execute(plan)
	if err != nil {
		return "", fmt.Errorf("%s", i18n.Tf("cmd.start.worktree_deploy_failed", err))
	}

	// Show deployment results (compact)
	for _, r := range results {
		if r.Success {
			fmt.Fprintf(a.IO.Out, "    %s %s\n",
				common.SuccessStyle.Render(common.IconSuccess), r.Name)
		} else {
			fmt.Fprintf(a.IO.Out, "    %s %s: %s\n",
				common.ErrorStyle.Render(common.IconError), r.Name, r.Message)
		}
	}
	fmt.Fprintln(a.IO.Out)

	return wtPath, nil
}

// ensureOpencode checks that the opencode binary is available.
// If not found, prompts the user to install it via Homebrew or auto-download.
func ensureOpencode(a *app.App) error {
	_, err := opencode.FindBinary()
	if err == nil {
		return nil // already installed
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n\n",
		common.WarningStyle.Render(common.IconWarning), i18n.T("cmd.start.opencode_not_found"))

	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(i18n.T("cmd.start.install_choice")).
				Options(
					huh.NewOption(i18n.T("cmd.start.install_brew"), "brew"),
					huh.NewOption(i18n.T("cmd.start.install_download"), "download"),
					huh.NewOption(i18n.T("cmd.start.install_cancel"), "cancel"),
				).
				Value(&choice),
		),
	)
	if err := form.Run(); err != nil {
		return fmt.Errorf("selection cancelled")
	}

	switch choice {
	case "brew":
		fmt.Fprintf(a.IO.Out, "\n  %s\n\n",
			i18n.Tf("cmd.start.install_run_brew", common.Bold.Render("brew install anomalyco/tap/opencode")))
		return fmt.Errorf("%s", i18n.T("cmd.start.install_required"))
	case "download":
		return downloadOpencode(a)
	default:
		return fmt.Errorf("%s", i18n.T("cmd.start.install_required_generic"))
	}
}

// downloadOpencode downloads and installs the opencode binary.
func downloadOpencode(a *app.App) error {
	installDir := a.Config.Opencode.InstallDir
	version := a.Config.Opencode.Version

	if version == "" {
		version = "latest"
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		common.SuccessStyle.Render(common.IconArrow), i18n.T("cmd.start.downloading"))

	var lastPercent int
	_, err := opencode.Download(version, installDir, func(downloaded, total int64) {
		if total > 0 {
			percent := int(downloaded * 100 / total)
			if percent != lastPercent && percent%5 == 0 {
				lastPercent = percent
				fmt.Fprintf(a.IO.Out, "\r  %s",
					i18n.Tf("cmd.start.download_progress", percent, downloaded/1024/1024, total/1024/1024))
			}
		}
	})
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	fmt.Fprintln(a.IO.Out) // newline after progress
	fmt.Fprintf(a.IO.Out, "%s %s\n\n",
		common.SuccessStyle.Render(common.IconSuccess), i18n.T("cmd.start.installed"))
	return nil
}

// resolveProject finds the project to use. Priority:
// 1. --project flag (explicit ID)
// 2. Current directory detection
// 3. Interactive selection if multiple projects exist
func resolveProject(ctx context.Context, a *app.App, projectID string) (*domain.Project, error) {
	// Explicit ID
	if projectID != "" {
		p, err := a.Projects.Get(ctx, projectID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, fmt.Errorf("projet %q introuvable", projectID)
			}
			return nil, err
		}
		return p, nil
	}

	// Auto-detect from cwd
	cwd, _ := os.Getwd()
	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		return nil, err
	}

	if len(projects) == 0 {
		return nil, fmt.Errorf("%s", i18n.T("cmd.project.no_projects"))
	}

	// Check if cwd matches a project
	for i, p := range projects {
		absPath, _ := filepath.Abs(p.Path)
		if absPath == cwd || isSubPath(cwd, absPath) {
			return &projects[i], nil
		}
	}

	// If only one project, use it
	if len(projects) == 1 {
		return &projects[0], nil
	}

	// Interactive selection
	var selectedID string
	options := make([]huh.Option[string], len(projects))
	for i, p := range projects {
		label := fmt.Sprintf("%s (%s)", p.Name, p.Language)
		options[i] = huh.NewOption(label, p.ID)
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choisir un projet").
				Options(options...).
				Value(&selectedID),
		),
	)
	if err := form.Run(); err != nil {
		return nil, err
	}

	for i, p := range projects {
		if p.ID == selectedID {
			return &projects[i], nil
		}
	}
	return nil, fmt.Errorf("%s", i18n.T("cmd.quick.not_found"))
}

func displayOrDefault(detected, fallback string) string {
	if detected != "" {
		return detected
	}
	if fallback != "" {
		return fallback
	}
	return "—"
}

// handleDevMode orchestrates the --dev workflow:
// 1. Verify bd is available
// 2. Sync tracker
// 3. Query epics and orphan tickets
// 4. Present picker (3 sections: epics, labeled tickets, other tickets)
// 5. Resolve selected tickets
// 6. Build prompt for orchestrator-dev
// Returns the agent name and constructed prompt.
func handleDevMode(cmd *cobra.Command, a *app.App, project *domain.Project, launchPath string) (agentName, devPrompt string, err error) {
	// 1. Verify bd is available
	if err := beads.Available(); err != nil {
		return "", "", fmt.Errorf("%s", i18n.T("cmd.start.dev_no_bd"))
	}

	labelFilter, _ := cmd.Flags().GetString("label")
	assigneeFilter, _ := cmd.Flags().GetString("assignee")
	ticketFlag, _ := cmd.Flags().GetString("ticket")

	// Team: pull team-state for claim awareness
	var teamRepo *teamstate.Repo
	if a.Config.Team.Enabled {
		statePath := a.Config.Team.StatePath
		if statePath == "" {
			statePath = config.DefaultTeamStatePath()
		}
		teamRepo = teamstate.NewRepo(a.Config.Team.StateRepo, statePath)
		if teamRepo.IsCloned() {
			_ = teamRepo.Pull(cmd.Context())
		}
	}

	// If --ticket is specified, skip the picker and work on that ticket directly
	if ticketFlag != "" {
		// Auto-claim if team is enabled
		if teamRepo != nil && teamRepo.IsCloned() {
			existing, claimErr := teamRepo.CreateClaim(cmd.Context(), teamstate.Claim{
				TicketID:  ticketFlag,
				Project:   project.ID,
				ClaimedBy: a.Config.Team.MemberID,
				Status:    "in_progress",
			})
			if claimErr == teamstate.ErrClaimExists && existing != nil {
				fmt.Fprintf(a.IO.Out, "  %s %s déjà pris par %s\n",
					common.WarningStyle.Render(common.IconWarning), ticketFlag, existing.ClaimedBy)
			} else if claimErr == nil {
				fmt.Fprintf(a.IO.Out, "  %s Claim %s/%s\n",
					common.SuccessStyle.Render(common.IconSuccess), project.ID, ticketFlag)
			}
		}

		// Store GitLab context in beads memory
		_ = beads.RememberGitLabContext(launchPath, ticketFlag, ticketFlag, "")

		// Build prompt with the ticket reference
		directPrompt := fmt.Sprintf("Travaille sur le ticket %s. Utilise `bd prime` pour le contexte et `bd ready` pour les tâches disponibles.", ticketFlag)
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconArrow), i18n.T("cmd.start.dev_launching"))
		return "orchestrator-dev", directPrompt, nil
	}

	// 2. Query tickets
	// Get epics with ready children
	epics, err := beads.ListEpicsWithReadyChildren(launchPath)
	if err != nil {
		slog.Warn("failed to list epics", "error", err)
	}

	// Get orphan tickets (no parent)
	withLabel, withoutLabel, err := beads.OrphanTickets(launchPath, labelFilter)
	if err != nil {
		return "", "", fmt.Errorf("querying tickets: %w", err)
	}

	// Apply assignee filter if set (re-query with assignee)
	if assigneeFilter != "" {
		readyOpts := beads.ReadyOpts{Assignee: assigneeFilter}
		filtered, err := beads.ListReady(launchPath, readyOpts)
		if err != nil {
			return "", "", fmt.Errorf("querying tickets by assignee: %w", err)
		}
		// Partition filtered tickets into labeled/unlabeled orphans
		withLabel = nil
		withoutLabel = nil
		for _, t := range filtered {
			if t.Parent != "" || t.Type == "epic" {
				continue
			}
			if beads.HasLabelExported(t, "ai-delegated") {
				withLabel = append(withLabel, t)
			} else {
				withoutLabel = append(withoutLabel, t)
			}
		}
	}

	// 4. Check we have something to show
	totalOptions := len(epics) + len(withLabel) + len(withoutLabel)
	if totalOptions == 0 {
		label := "ai-delegated"
		if labelFilter != "" {
			label = labelFilter
		}
		return "", "", fmt.Errorf("%s", i18n.Tf("cmd.start.dev_no_tickets", label))
	}

	// 5. Build picker options
	type pickerItem struct {
		label  string
		isEpic bool
		epicID string
		ticket beads.Ticket
	}

	var items []pickerItem

	// Section: Epics
	for _, e := range epics {
		items = append(items, pickerItem{
			label:  fmt.Sprintf("[Epic] %s (%d tickets)", e.Ticket.Title, e.ReadyCount),
			isEpic: true,
			epicID: e.Ticket.ID,
			ticket: e.Ticket,
		})
	}

	// Section: Tickets with ai-delegated label
	for _, t := range withLabel {
		items = append(items, pickerItem{
			label:  fmt.Sprintf("[ai-delegated] %s — %s", t.ID, t.Title),
			isEpic: false,
			ticket: t,
		})
	}

	// Section: Other ready tickets
	for _, t := range withoutLabel {
		items = append(items, pickerItem{
			label:  fmt.Sprintf("%s — %s", t.ID, t.Title),
			isEpic: false,
			ticket: t,
		})
	}

	// 6. Present picker
	options := make([]huh.Option[int], len(items))
	for i, item := range items {
		options[i] = huh.NewOption(item.label, i)
	}

	var selectedIdx int
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title(i18n.T("cmd.start.dev_picker_title")).
				Options(options...).
				Value(&selectedIdx),
		),
	)
	if err := form.Run(); err != nil {
		return "", "", err
	}

	selected := items[selectedIdx]

	// 7. Resolve tickets for the selected item
	var tickets []beads.Ticket
	if selected.isEpic {
		children, err := beads.ReadyChildren(launchPath, selected.epicID)
		if err != nil {
			return "", "", fmt.Errorf("querying epic children: %w", err)
		}
		tickets = children
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.start.dev_selected_epic", selected.ticket.Title, len(tickets)))
	} else {
		tickets = []beads.Ticket{selected.ticket}
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			common.SuccessStyle.Render(common.IconSuccess),
			i18n.Tf("cmd.start.dev_selected_ticket", selected.ticket.ID, selected.ticket.Title))
	}

	// 7b. Auto-claim the selected ticket in team-state
	if teamRepo != nil && teamRepo.IsCloned() && a.Config.Team.MemberID != "" {
		claimTicketID := selected.ticket.ID
		if selected.isEpic {
			claimTicketID = selected.epicID
		}
		existing, claimErr := teamRepo.CreateClaim(cmd.Context(), teamstate.Claim{
			TicketID:  claimTicketID,
			Project:   project.ID,
			ClaimedBy: a.Config.Team.MemberID,
			Status:    "in_progress",
		})
		if claimErr == teamstate.ErrClaimExists && existing != nil {
			if existing.ClaimedBy != a.Config.Team.MemberID {
				fmt.Fprintf(a.IO.Out, "  %s %s déjà pris par %s\n",
					common.WarningStyle.Render(common.IconWarning), claimTicketID, existing.ClaimedBy)
			}
		} else if claimErr == nil {
			fmt.Fprintf(a.IO.Out, "  %s Claim %s\n",
				common.SuccessStyle.Render(common.IconSuccess), claimTicketID)
		}
	}

	// 8. Build prompt
	fmt.Fprintf(a.IO.Out, "%s %s\n",
		common.SuccessStyle.Render(common.IconArrow), i18n.T("cmd.start.dev_launching"))

	devPrompt = prompt.BuildDevPrompt(tickets)
	return "orchestrator-dev", devPrompt, nil
}
