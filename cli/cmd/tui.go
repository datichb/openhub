package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/common"
	"github.com/datichb/openhub/cli/internal/tui/v2/menu"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// tuiShell holds a reference to the running shell for action callbacks.
var tuiShell *shell.Shell

// runTUI launches the unified TUI shell.
func runTUI() error {
	a := MustApp()

	cfg := shell.Config{
		ProjectName: a.Config.Name,
		MenuItems:   buildMenuItems(a),
		Views:       buildViews(a),
		HomeViewID:  "home",
	}

	tuiShell = shell.New(cfg)
	tuiShell.NavigateHome(cfg.HomeViewID)
	return tuiShell.Run()
}

// buildMenuItems constructs the menu tree for the shell sidebar.
func buildMenuItems(a *app.App) []*menu.MenuItem {
	// Team children — conditional based on team enabled state
	var teamChildren []*menu.MenuItem
	if a.Config.Team.Enabled {
		teamChildren = []*menu.MenuItem{
			{ID: "team.kanban", Label: "Kanban équipe", ViewID: "team.board"},
			{ID: "team.status", Label: "Status", ViewID: "team.status"},
			{ID: "team.activity", Label: "Activité", ViewID: "team.activity"},
			{ID: "team.briefs", Label: "Takeover Briefs", ViewID: "takeover-briefs"},
			{ID: "team.worktrees", Label: "Worktrees", ViewID: "worktrees"},
			{ID: "team.patterns", Label: "Patterns", ViewID: "patterns"},
			{ID: "team.policies", Label: "Policies", ViewID: "policies"},
		}
	} else {
		teamChildren = []*menu.MenuItem{
			{ID: "team.init", Label: "Initialiser", Action: actionTeamInit},
			{ID: "team.worktrees", Label: "Worktrees", ViewID: "worktrees"},
		}
	}

	return []*menu.MenuItem{
		{ID: "home", Label: "Home", ViewID: "home"},
		{
			ID: "sessions", Label: "Sessions", Expanded: true,
			Children: []*menu.MenuItem{
				{ID: "sessions.start", Label: "Start", Action: actionStartLauncher},
				{ID: "sessions.audit", Label: "Audit", Action: actionAuditLauncher},
				{ID: "sessions.review", Label: "Review", Action: actionReviewLauncher},
				{ID: "sessions.debug", Label: "Debug", Action: actionDebugLauncher},
				{ID: "sessions.quick", Label: "Quick", Action: actionOpencode("", "")},
				{ID: "sessions.parallel", Label: "Parallel", ViewID: "parallel"},
			},
		},
		{
			ID: "projects", Label: "Projets", Expanded: false,
			Children: []*menu.MenuItem{
				{ID: "projects.kanban", Label: "Kanban", ViewID: "board"},
				{ID: "projects.list", Label: "Liste", ViewID: "projects.list"},
				{ID: "projects.deploy", Label: "Deploy", Action: actionDeploy},
				{ID: "projects.sync", Label: "Sync", Action: actionSync},
			},
		},
		{
			ID: "team", Label: "Team", Expanded: false,
			Children: teamChildren,
		},
		{
			ID: "config", Label: "Configuration", Expanded: false,
			Children: []*menu.MenuItem{
				{ID: "config.hub", Label: "Hub", ViewID: "config"},
				{ID: "config.models", Label: "Models", ViewID: "models"},
				{ID: "config.provider", Label: "Provider", ViewID: "provider"},
				{ID: "config.mcp", Label: "MCP", ViewID: "mcp"},
			},
		},
		{
			ID: "system", Label: "Système", Expanded: false,
			Children: []*menu.MenuItem{
				{ID: "system.status", Label: "Status", ViewID: "status"},
				{ID: "system.doctor", Label: "Doctor", ViewID: "doctor"},
				{ID: "system.metrics", Label: "Métriques", ViewID: "metrics"},
				{ID: "system.plugins", Label: "Plugins", ViewID: "plugins"},
				{ID: "system.upgrade", Label: "Mise à jour", Action: actionUpgrade},
				{ID: "system.help", Label: "Aide", ViewID: "help"},
			},
		},
	}
}

// buildViews constructs all registered views for the shell.
func buildViews(a *app.App) []views.View {
	// Fetch project list for the ProjectsView
	var projectItems []views.ProjectItem
	if a.Projects != nil {
		projects, _ := a.Projects.List(context.Background(), "")
		for _, p := range projects {
			projectItems = append(projectItems, views.ProjectItem{
				ID:       p.ID,
				Name:     p.Name,
				Path:     p.Path,
				Language: p.Language,
				Provider: p.Provider,
				Model:    p.Model,
				Agents:   p.Agents,
				Status:   string(p.Status),
			})
		}
	}

	// Build ProjectsView with store callbacks
	projectsView := views.NewProjectsView(views.ProjectsViewConfig{
		Projects:        projectItems,
		AvailableAgents: discoverAgents(),
	})
	if a.Projects != nil {
		projectsView.SetOnAdd(func(name, path string) {
			p := &domain.Project{Name: name, Path: path}
			if err := a.Projects.Create(context.Background(), p); err != nil {
				slog.Warn("failed to add project", "error", err)
			}
		})
		projectsView.SetOnRemove(func(id string) {
			if err := a.Projects.Delete(context.Background(), id); err != nil {
				slog.Warn("failed to remove project", "error", err)
			}
		})
		projectsView.SetOnConfigure(func(id string, cfg views.ProjectConfigUpdate) {
			ctx := context.Background()
			project, err := a.Projects.Get(ctx, id)
			if err != nil {
				slog.Warn("failed to get project for configure", "id", id, "error", err)
				return
			}
			project.Language = cfg.Language
			project.Provider = cfg.Provider
			project.Model = cfg.Model
			project.Agents = cfg.Agents
			project.UpdatedAt = time.Now()
			if err := a.Projects.Update(ctx, project); err != nil {
				slog.Warn("failed to update project config", "id", id, "error", err)
			}
		})
		projectsView.SetOnRename(func(id, newName string) {
			ctx := context.Background()
			project, err := a.Projects.Get(ctx, id)
			if err != nil {
				slog.Warn("failed to get project for rename", "id", id, "error", err)
				return
			}
			project.Name = newName
			project.UpdatedAt = time.Now()
			if err := a.Projects.Update(ctx, project); err != nil {
				slog.Warn("failed to rename project", "id", id, "error", err)
			}
		})
		projectsView.SetOnMove(func(id, newPath string) {
			ctx := context.Background()
			project, err := a.Projects.Get(ctx, id)
			if err != nil {
				slog.Warn("failed to get project for move", "id", id, "error", err)
				return
			}
			project.Path = newPath
			project.UpdatedAt = time.Now()
			if err := a.Projects.Update(ctx, project); err != nil {
				slog.Warn("failed to move project", "id", id, "error", err)
			}
		})
	}

	allViews := []views.View{
		views.NewHomeView(),
		views.NewBoardView(views.BoardViewConfig{}),
		views.NewTeamBoardView(views.TeamBoardViewConfig{}),
		views.NewParallelView(views.ParallelViewConfig{}),
		projectsView,
		views.NewTeamStatusView(a),
		views.NewActivityView(a),
		views.NewWorktreeView(a),
		views.NewStatusView(a),
		views.NewMetricsView(),
		views.NewDoctorView(a),
		views.NewConfigView(),
		views.NewModelsView(a),
		views.NewProviderView(a),
		views.NewMCPView(a),
		views.NewPluginsView(),
		views.NewHelpView(),
	}

	// Conditionally add team-dependent views
	if a.Config.Team.Enabled {
		takeoverView := views.NewTakeoverView(a)
		takeoverView.SetOnEnrich(func(project, ticketID string) error {
			return runTakeoverEnrich(a, project, ticketID)
		})
		allViews = append(allViews,
			views.NewPatternsView(a),
			views.NewPoliciesView(a),
			takeoverView,
		)
	}

	return allViews
}

// actionOpencode returns a menu action callback that suspends the TUI and launches opencode.
func actionOpencode(agent, extraArg string) func() {
	return func() {
		if tuiShell == nil {
			return
		}

		// Pre-flight: check binary exists
		if _, err := opencode.FindBinary(); err != nil {
			tuiShell.ShowToast("opencode non trouvé", shell.ToastError)
			return
		}

		a := MustApp()
		project, err := resolveActiveProject(a)
		if err != nil {
			tuiShell.ShowToast("Aucun projet actif", shell.ToastWarning)
			return
		}

		opts := opencode.StartOpts{
			ProjectPath: project.Path,
			ProjectID:   project.ID,
			Agent:       agent,
		}
		if extraArg != "" {
			opts.ExtraArgs = []string{extraArg}
		}

		// Resolve provider credentials
		resolveProviderCreds(a, project, &opts)

		err = tuiShell.SuspendAndExec(func() error {
			return opencode.Run(opts)
		})
		if err != nil {
			slog.Warn("opencode session ended with error", "error", err)
			tuiShell.ShowToast(fmt.Sprintf("Session: %s", err), shell.ToastWarning)
		} else {
			tuiShell.ShowToast("Session terminée", shell.ToastSuccess)
		}
	}
}

// resolveActiveProject returns the currently selected project or the first one.
func resolveActiveProject(a *app.App) (*domain.Project, error) {
	ctx := context.Background()
	// Try resolveProject with empty ID (picks first available)
	return resolveProject(ctx, a, "")
}

// resolveProviderCreds populates provider credentials into opts.
func resolveProviderCreds(a *app.App, project *domain.Project, opts *opencode.StartOpts) {
	provider := project.Provider
	if provider == "" {
		provider = a.Config.Opencode.DefaultProvider
	}
	opts.Provider = provider

	if a.Secrets == nil {
		return
	}

	ctx := context.Background()
	switch provider {
	case "bedrock":
		token, _ := a.Secrets.Get(ctx, fmt.Sprintf("bedrock-token-%s", project.ID))
		if token == "" {
			token, _ = a.Secrets.Get(ctx, "bedrock-token-default")
		}
		opts.BearerToken = token
		opts.AWSProfile = a.Config.Provider.Bedrock.AWSProfile
		opts.AWSRegion = a.Config.Provider.Bedrock.AWSRegion
	case "anthropic":
		key, _ := a.Secrets.Get(ctx, fmt.Sprintf("anthropic-api-key-%s", project.ID))
		if key == "" {
			key, _ = a.Secrets.Get(ctx, "anthropic-api-key-default")
		}
		opts.APIKey = key
	case "openrouter":
		key, _ := a.Secrets.Get(ctx, fmt.Sprintf("openrouter-api-key-%s", project.ID))
		if key == "" {
			key, _ = a.Secrets.Get(ctx, "openrouter-api-key-default")
		}
		opts.APIKey = key
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Session launchers with option selection
// ─────────────────────────────────────────────────────────────────────────────

func actionStartLauncher() {
	if tuiShell == nil {
		return
	}
	tuiShell.ShowSessionLauncher(shell.SessionLaunchConfig{
		Title: "Lancer une session",
		Options: []shell.SessionOption{
			{Label: "Standard", Description: "Session interactive classique", Agent: ""},
			{Label: "Dev (ticket)", Description: "Session orientée développement", Agent: "", ExtraArgs: []string{"--dev"}},
			{Label: "Onboard", Description: "Session d'onboarding projet", Agent: "", ExtraArgs: []string{"--onboard"}},
		},
		OnLaunch: func(opt shell.SessionOption) {
			launchOpencode(opt.Agent, opt.ExtraArgs...)
		},
	})
}

func actionAuditLauncher() {
	if tuiShell == nil {
		return
	}
	tuiShell.ShowSessionLauncher(shell.SessionLaunchConfig{
		Title: "Lancer un audit",
		Options: []shell.SessionOption{
			{Label: "Sécurité", Description: "Audit de sécurité (OWASP, injections, auth)", Agent: "auditor", ExtraArgs: []string{"--type", "security"}},
			{Label: "Performance", Description: "Audit de performance (N+1, mémoire, CPU)", Agent: "auditor", ExtraArgs: []string{"--type", "performance"}},
			{Label: "Architecture", Description: "Audit d'architecture (couplage, patterns)", Agent: "auditor", ExtraArgs: []string{"--type", "architecture"}},
			{Label: "Accessibilité", Description: "Audit a11y (WCAG, ARIA, contraste)", Agent: "auditor", ExtraArgs: []string{"--type", "accessibility"}},
			{Label: "Éco-conception", Description: "Audit impact environnemental", Agent: "auditor", ExtraArgs: []string{"--type", "ecodesign"}},
			{Label: "Observabilité", Description: "Audit logs, traces, métriques", Agent: "auditor", ExtraArgs: []string{"--type", "observability"}},
		},
		OnLaunch: func(opt shell.SessionOption) {
			launchOpencode(opt.Agent, opt.ExtraArgs...)
		},
	})
}

func actionReviewLauncher() {
	if tuiShell == nil {
		return
	}

	options := []shell.SessionOption{
		{Label: "Standard", Description: "Code review classique", Agent: "reviewer"},
		{Label: "Adversarial", Description: "Review adversariale (trouver les failles)", Agent: "reviewer", ExtraArgs: []string{"--mode", "adversarial"}},
		{Label: "Edge cases", Description: "Review orientée cas limites", Agent: "reviewer", ExtraArgs: []string{"--mode", "edge-case"}},
		{Label: "Complète", Description: "Review complète (tous les modes)", Agent: "reviewer", ExtraArgs: []string{"--mode", "all"}},
	}

	// Add publish option if GitLab write is enabled
	a := MustApp()
	if a.Config.MCP.Gitlab.WriteEnabled {
		options = append(options, shell.SessionOption{
			Label: "Publish", Description: "Publier pour review (crée MR + notifie l'équipe)", Agent: "reviewer", ExtraArgs: []string{"--publish"},
		})
	}

	tuiShell.ShowSessionLauncher(shell.SessionLaunchConfig{
		Title:   "Lancer une review",
		Options: options,
		OnLaunch: func(opt shell.SessionOption) {
			launchOpencode(opt.Agent, opt.ExtraArgs...)
		},
	})
}

func actionDebugLauncher() {
	if tuiShell == nil {
		return
	}
	tuiShell.ShowInputModal("Description du problème", "", func(issue string) {
		args := []string{}
		if issue != "" {
			args = append(args, "--issue", issue)
		}
		launchOpencode("debugger", args...)
	})
}

// launchOpencode is the common session launch logic with SuspendAndExec.
func launchOpencode(agent string, extraArgs ...string) {
	if tuiShell == nil {
		return
	}

	if _, err := opencode.FindBinary(); err != nil {
		tuiShell.ShowToast("opencode non trouvé", shell.ToastError)
		return
	}

	a := MustApp()
	project, err := resolveActiveProject(a)
	if err != nil {
		tuiShell.ShowToast("Aucun projet actif", shell.ToastWarning)
		return
	}

	opts := opencode.StartOpts{
		ProjectPath: project.Path,
		ProjectID:   project.ID,
		Agent:       agent,
		ExtraArgs:   extraArgs,
	}
	resolveProviderCreds(a, project, &opts)

	err = tuiShell.SuspendAndExec(func() error {
		return opencode.Run(opts)
	})
	if err != nil {
		tuiShell.ShowToast("Session terminée avec erreur", shell.ToastWarning)
	} else {
		tuiShell.ShowToast("Session terminée", shell.ToastSuccess)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Deploy / Sync / Upgrade actions
// ─────────────────────────────────────────────────────────────────────────────

func actionDeploy() {
	if tuiShell == nil {
		return
	}
	a := MustApp()
	if a.Projects == nil {
		tuiShell.ShowToast("Hub non initialisé", shell.ToastError)
		return
	}
	project, err := resolveActiveProject(a)
	if err != nil {
		tuiShell.ShowToast("Aucun projet actif", shell.ToastWarning)
		return
	}

	hubDir := findHubDir()
	if hubDir == "" {
		tuiShell.ShowToast("Hub content non trouvé", shell.ToastError)
		return
	}

	// Compute diff to show preview before deploying
	tuiShell.ShowToast("Analyse des changements...", shell.ToastInfo)

	go func() {
		report, err := deploy.ComputeDiff(hubDir, project.Path, project.Agents)
		tuiShell.App().QueueUpdateDraw(func() {
			if err != nil {
				tuiShell.ShowToast("Erreur diff: "+truncateErr(err), shell.ToastError)
				return
			}

			if !report.HasChanges() {
				tuiShell.ShowToast("Déjà à jour — rien à déployer", shell.ToastSuccess)
				return
			}

			// Show diff preview with apply/cancel actions
			diffContent := deploy.FormatDiffReport(report, false)
			tuiShell.ShowScrollableModal(
				"Deploy Preview: "+project.Name,
				diffContent,
				[]views.ModalAction{
					{Label: "Appliquer", Callback: func() {
						tuiShell.ShowToast("Deploy en cours...", shell.ToastInfo)
						go func() {
							err := runDeployForProject(a, project)
							tuiShell.App().QueueUpdateDraw(func() {
								if err != nil {
									tuiShell.ShowToast("Deploy échoué: "+truncateErr(err), shell.ToastError)
								} else {
									tuiShell.ShowToast("Deploy réussi", shell.ToastSuccess)
								}
							})
						}()
					}},
					{Label: "Annuler", Callback: func() {}},
				},
			)
		})
	}()
}

func actionSync() {
	if tuiShell == nil {
		return
	}

	a := MustApp()
	if a.Projects == nil {
		tuiShell.ShowToast("Hub non initialisé", shell.ToastError)
		return
	}

	tuiShell.ShowToast("Sync en cours...", shell.ToastInfo)

	go func() {
		err := runSyncAll(a)
		tuiShell.App().QueueUpdateDraw(func() {
			if err != nil {
				tuiShell.ShowToast("Sync échoué: "+truncateErr(err), shell.ToastError)
			} else {
				tuiShell.ShowToast("Sync réussi", shell.ToastSuccess)
			}
		})
	}()
}

func actionUpgrade() {
	if tuiShell == nil {
		return
	}

	tuiShell.ShowToast("Mise à jour opencode...", shell.ToastInfo)

	go func() {
		err := runUpgradeOpencode()
		tuiShell.App().QueueUpdateDraw(func() {
			if err != nil {
				tuiShell.ShowToast("Mise à jour échouée: "+truncateErr(err), shell.ToastError)
			} else {
				tuiShell.ShowToast("opencode mis à jour", shell.ToastSuccess)
			}
		})
	}()
}

// truncateErr returns a short error message suitable for a toast.
func truncateErr(err error) string {
	msg := err.Error()
	if len(msg) > 40 {
		return msg[:37] + "..."
	}
	return msg
}

// canLaunchTUI returns true if the environment supports launching the TUI shell.
func canLaunchTUI() bool {
	return common.UseRichTUI()
}

// ─────────────────────────────────────────────────────────────────────────────
// Team Init action
// ─────────────────────────────────────────────────────────────────────────────

func actionTeamInit() {
	if tuiShell == nil {
		return
	}

	a := MustApp()

	// Step 1: Git remote URL
	tuiShell.ShowInputModal("Git remote URL du team-state", "", func(remote string) {
		if remote == "" {
			return
		}
		// Step 2: Member ID
		tuiShell.ShowInputModal("Votre member ID", "", func(memberID string) {
			if memberID == "" {
				return
			}
			// Step 3: Display name
			tuiShell.ShowInputModal("Nom d'affichage", "", func(displayName string) {
				if displayName == "" {
					return
				}
				// Step 4: Role
				roleOpts := []shell.SessionOption{
					{Label: "Lead", Description: "Tech lead"},
					{Label: "Dev", Description: "Développeur"},
					{Label: "Reviewer", Description: "Revieweur"},
				}
				_ = roleOpts // use ShowSelectModal instead

				roleOptions := []views.SelectOption{
					{Label: "Lead", Value: "lead"},
					{Label: "Développeur", Value: "dev"},
					{Label: "Reviewer", Value: "reviewer"},
				}
				tuiShell.ShowSelectModal("Rôle", roleOptions, "dev", func(role string) {
					// Execute team init async
					tuiShell.ShowToast("Initialisation team...", shell.ToastInfo)

					go func() {
						err := runTeamInitFromTUI(a, remote, memberID, displayName, role)
						tuiShell.App().QueueUpdateDraw(func() {
							if err != nil {
								tuiShell.ShowToast("Team init échoué: "+truncateErr(err), shell.ToastError)
							} else {
								tuiShell.ShowToast("Team initialisé ! Redémarrez le TUI pour les nouvelles options.", shell.ToastSuccess)
							}
						})
					}()
				})
			})
		})
	})
}

func runTeamInitFromTUI(a *app.App, remote, memberID, displayName, role string) error {
	statePath := a.Config.Team.StatePath
	if statePath == "" {
		statePath = config.DefaultTeamStatePath()
	}

	repo := teamstate.NewRepo(remote, statePath)

	// Clone or pull
	ctx := context.Background()
	if err := repo.EnsureReady(ctx); err != nil {
		return fmt.Errorf("cloning team-state: %w", err)
	}

	// Init structure if needed
	if err := repo.InitStructure(ctx); err != nil {
		return fmt.Errorf("init structure: %w", err)
	}

	// Register member
	member := teamstate.Member{
		ID:          memberID,
		DisplayName: displayName,
		Role:        role,
		DefaultMode: "semi-auto",
	}
	if repo.HasMember(memberID) {
		if err := repo.UpdateMember(member); err != nil {
			return fmt.Errorf("update member: %w", err)
		}
	} else {
		if err := repo.AddMember(member); err != nil {
			return fmt.Errorf("add member: %w", err)
		}
	}

	// Commit and push
	if err := repo.CommitAndPush(ctx, "team: init "+memberID, "members.toml"); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// Write team section to hub.toml
	vip := configViper()
	vip.Set("team.enabled", true)
	vip.Set("team.state_repo", remote)
	vip.Set("team.state_path", statePath)
	vip.Set("team.member_id", memberID)
	if err := vip.WriteConfigAs(config.ConfigPath()); err != nil {
		return fmt.Errorf("writing hub.toml: %w", err)
	}

	// Update in-memory config
	a.Config.Team.Enabled = true
	a.Config.Team.StateRepo = remote
	a.Config.Team.StatePath = statePath
	a.Config.Team.MemberID = memberID

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Takeover brief enrichment
// ─────────────────────────────────────────────────────────────────────────────

func runTakeoverEnrich(a *app.App, project, ticketID string) error {
	repo := teamstate.NewRepo(a.Config.Team.StateRepo, a.Config.Team.StatePath)

	// Read existing brief content
	content, err := repo.ReadBrief(project, ticketID)
	if err != nil {
		return fmt.Errorf("reading brief: %w", err)
	}

	// Run headless enrichment via opencode
	enriched, err := opencode.RunHeadless(opencode.HeadlessOpts{
		Agent:  "brief-enricher",
		Prompt: content,
	})
	if err != nil {
		return fmt.Errorf("enrichment: %w", err)
	}

	// Find latest brief to derive enriched filename
	briefsDir := repo.Path() + "/projects/" + project + "/takeover-briefs/"
	entries, _ := os.ReadDir(briefsDir)
	var latestBase string
	for _, e := range entries {
		name := e.Name()
		if len(name) > len(ticketID)+1 && name[:len(ticketID)] == ticketID &&
			strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".enriched.md") {
			latestBase = strings.TrimSuffix(name, ".md")
		}
	}
	if latestBase == "" {
		latestBase = ticketID
	}

	// Write enriched file
	enrichedContent := fmt.Sprintf("# Takeover Brief (enrichi): %s\n\n%s", ticketID, enriched)
	enrichedFile := briefsDir + latestBase + ".enriched.md"
	if err := os.WriteFile(enrichedFile, []byte(enrichedContent), 0o644); err != nil {
		return fmt.Errorf("writing enriched brief: %w", err)
	}

	// Commit and push
	relPath := "projects/" + project + "/takeover-briefs/" + latestBase + ".enriched.md"
	_ = repo.CommitAndPush(context.Background(), fmt.Sprintf("takeover: enriched brief for %s/%s", project, ticketID), relPath)

	return nil
}
