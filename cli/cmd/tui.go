package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/opencode"
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
		MenuItems:   buildMenuItems(),
		Views:       buildViews(a),
		HomeViewID:  "home",
	}

	tuiShell = shell.New(cfg)
	tuiShell.NavigateHome(cfg.HomeViewID)
	return tuiShell.Run()
}

// buildMenuItems constructs the menu tree for the shell sidebar.
func buildMenuItems() []*menu.MenuItem {
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
			Children: []*menu.MenuItem{
				{ID: "team.kanban", Label: "Kanban équipe", ViewID: "team.board"},
				{ID: "team.status", Label: "Status", ViewID: "team.status"},
				{ID: "team.worktrees", Label: "Worktrees", ViewID: "worktrees"},
			},
		},
		{
			ID: "config", Label: "Configuration", Expanded: false,
			Children: []*menu.MenuItem{
				{ID: "config.hub", Label: "Hub", ViewID: "config"},
				{ID: "config.mcp", Label: "MCP", ViewID: "mcp"},
			},
		},
		{
			ID: "system", Label: "Système", Expanded: false,
			Children: []*menu.MenuItem{
				{ID: "system.status", Label: "Status", ViewID: "status"},
				{ID: "system.doctor", Label: "Doctor", ViewID: "doctor"},
				{ID: "system.metrics", Label: "Métriques", ViewID: "metrics"},
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

	return []views.View{
		views.NewHomeView(),
		views.NewBoardView(views.BoardViewConfig{}),
		views.NewTeamBoardView(views.TeamBoardViewConfig{}),
		views.NewParallelView(views.ParallelViewConfig{}),
		projectsView,
		views.NewTeamStatusView(a),
		views.NewWorktreeView(a),
		views.NewStatusView(a),
		views.NewMetricsView(),
		views.NewDoctorView(a),
		views.NewConfigView(),
		views.NewMCPView(a),
		views.NewHelpView(),
	}
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
	tuiShell.ShowSessionLauncher(shell.SessionLaunchConfig{
		Title: "Lancer une review",
		Options: []shell.SessionOption{
			{Label: "Standard", Description: "Code review classique", Agent: "reviewer"},
			{Label: "Adversarial", Description: "Review adversariale (trouver les failles)", Agent: "reviewer", ExtraArgs: []string{"--mode", "adversarial"}},
			{Label: "Edge cases", Description: "Review orientée cas limites", Agent: "reviewer", ExtraArgs: []string{"--mode", "edge-case"}},
			{Label: "Complète", Description: "Review complète (tous les modes)", Agent: "reviewer", ExtraArgs: []string{"--mode", "all"}},
		},
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

	tuiShell.ShowToast("Deploy: "+project.Name+"...", shell.ToastInfo)

	// Run deploy in background then show result
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
