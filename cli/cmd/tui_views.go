package cmd

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// domainProjectToViewItem converts a domain.Project to a views.ProjectItem.
func domainProjectToViewItem(p domain.Project) views.ProjectItem {
	var mcpOverrides map[string]string
	if p.MCPConfig != nil && len(p.MCPConfig.Services) > 0 {
		mcpOverrides = make(map[string]string, len(p.MCPConfig.Services))
		for _, svc := range p.MCPConfig.Services {
			switch {
			case svc.Enabled == nil:
				mcpOverrides[svc.Name] = "inherit"
			case *svc.Enabled:
				mcpOverrides[svc.Name] = "enabled"
			default:
				mcpOverrides[svc.Name] = "disabled"
			}
		}
	}
	item := views.ProjectItem{
		ID:           p.ID,
		Name:         p.Name,
		Path:         p.Path,
		Language:     p.Language,
		Provider:     p.Provider,
		Model:        p.Model,
		Status:       string(p.Status),
		MCPOverrides: mcpOverrides,
	}
	if p.TeamID != nil {
		item.TeamID = *p.TeamID
	}
	return item
}

// teamProjects lists the projects attached to a team.
func teamProjects(a *app.App, teamID string) []views.ActiveProject {
	if a == nil || a.Projects == nil {
		return nil
	}
	projects, err := a.Projects.List(context.Background(), "")
	if err != nil {
		slog.Warn("failed to list team projects", "team", teamID, "error", err)
		return []views.ActiveProject{}
	}
	out := []views.ActiveProject{}
	for _, p := range projects {
		if p.TeamID != nil && *p.TeamID == teamID {
			out = append(out, views.ActiveProject{ID: p.ID, Name: p.Name, Path: p.Path})
		}
	}
	return out
}

// loadProjectItems fetches all projects from the store and converts them to view items.
func loadProjectItems(store domain.ProjectStore) []views.ProjectItem {
	if store == nil {
		return nil
	}
	projects, err := store.List(context.Background(), "")
	if err != nil {
		slog.Warn("failed to list projects for view", "error", err)
		return nil
	}
	items := make([]views.ProjectItem, 0, len(projects))
	for _, p := range projects {
		items = append(items, domainProjectToViewItem(p))
	}
	return items
}

// buildViews constructs all registered views for the shell, in the order the
// shell registers them: the home, the boards, the modes, the lists of the hub,
// then the team pages and the configuration views.
func buildViews(a *app.App) []views.View {
	tuiSess = newTUISessions(a)
	sessionsSection := tuiSess.sectionConfig()
	tuiStartWiring = newTUIStart(a)
	startSection := tuiStartWiring.sectionConfig()

	projectsView := buildProjectsView(a)
	projectModeView := buildProjectModeView(a, sessionsSection, startSection)

	allViews := []views.View{
		buildHomeView(a, sessionsSection, startSection),
		buildBoardView(a),
		views.NewTeamBoardView(buildTeamBoardViewConfig(a)),
		buildTeamModeView(a, sessionsSection, startSection),
		tuiSess.view,
		newWorkflowCatalogView(tuiStartWiring),
		projectsView,
		projectModeView,
		views.NewTeamStatusView(makeResolveTeamFunc(a)),
		views.NewActivityView(makeResolveTeamFunc(a)),
		views.NewWorktreeView(a, views.WorktreeViewConfig{
			OpenSession: func(path string) { openLaunchForm(a, tuiLaunchRequest{WorkflowID: "libre", Location: path}) },
		}),
		views.NewStatusView(a),
		views.NewMetricsView(views.MetricsViewConfig{
			AgentEvents:  a.AgentEvents,
			Stats:        a.Stats,
			TeamProjects: func(teamID string) []views.ActiveProject { return teamProjects(a, teamID) },
		}),
		views.NewDoctorView(a),
		buildTeamsView(a),
		newModelsView(a),
		buildProviderView(a),
		buildMCPView(a),
		views.NewHelpView(),
		views.NewNotificationsView(views.NotificationsViewConfig{
			FilePath:  shell.NotificationsFilePath(),
			ReadLastN: shell.ReadLastN,
		}),
	}
	return append(allViews, buildTeamAndConfigViews(a)...)
}

// buildTeamAndConfigViews are the team pages (always registered, they handle
// « not configured » themselves) and the views of the configuration of the
// hub, the team and the project.
func buildTeamAndConfigViews(a *app.App) []views.View {
	return []views.View{
		views.NewPatternsView(makeResolveTeamFunc(a)),
		views.NewPoliciesView(makeResolveTeamFunc(a), a),
		views.NewWikiView(makeResolveTeamFunc(a)),
		buildTakeoverView(a),
		buildTeamDetailView(a),
		buildTeamMCPView(a),
		buildTeamModelsView(a),
		buildSettingsView(a),
		buildProjectConfigView(a),
		buildProjectMCPView(a),
		newBricksView(a),
		newProjectWikiView(a),
		buildProjectModelsView(a),
		buildSecretsView(a),
	}
}

// hubMCPServerConfig maps a service name to the corresponding hub MCPServerConfig.
func hubMCPServerConfig(cfg *config.Config, service string) config.MCPServerConfig {
	switch service {
	case "gitlab":
		return cfg.MCP.Gitlab
	case "jira":
		return cfg.MCP.Jira
	case "figma":
		return cfg.MCP.Figma
	case "gslides":
		return cfg.MCP.Gslides
	default:
		return config.MCPServerConfig{}
	}
}

// resolveGitBranch returns the current git branch for a project path.
// Returns "" on any error (not a git repo, git not found, etc.).
func resolveGitBranch(projectPath string) string {
	if projectPath == "" {
		return ""
	}
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// tuiToolLine is the tool line of the hub and project headers (« opencode
// v2.0.20 · adaptateur opencode-v2 ✔ »; cached version, detected when the TUI
// starts: no process on the event loop).
func tuiToolLine() string {
	if v, _ := v5Ver.Load().(string); v != "" {
		return i18n.Tf("tui.views.tool_line", v5Tool.DisplayName, v, v5Tool.Name) + " " + theme.SymSuccess
	}
	if v5Err != nil {
		return i18n.Tf("cmd.v1.unsupported.doctor_name", toolName()) + " " + theme.SymFailure
	}
	return ""
}

// teamTrackerType is the type of the tracker of a team (gitlab, jira) as its
// team-state declares it; "" when it is not cloned or says nothing. It reads
// files: call it off the event loop.
func teamTrackerType(team config.TeamConfig) string {
	if team.StateRepo == "" {
		return ""
	}
	repo := teamstate.NewRepo(team.StateRepo, team.StatePath)
	if !repo.IsCloned() {
		return ""
	}
	cfg, err := repo.LoadConfig()
	if err != nil || cfg == nil {
		return ""
	}
	return cfg.Tracker.Type
}

// secretChecker tells whether a secret is stored, and shows its last four
// characters; the views that edit a token share it.
func secretChecker(a *app.App) func(ctx context.Context, key string) (bool, string) {
	return func(ctx context.Context, key string) (bool, string) {
		if a.Secrets == nil {
			return false, ""
		}
		val, err := a.Secrets.Get(ctx, key)
		if err != nil || val == "" {
			return false, ""
		}
		masked := "****"
		if len(val) > 4 {
			masked = "****" + val[len(val)-4:]
		}
		return true, masked
	}
}

// secretSetter stores a secret in the keychain of the app.
func secretSetter(a *app.App) func(ctx context.Context, key, value string) error {
	return func(ctx context.Context, key, value string) error {
		if a.Secrets == nil {
			return errors.New(i18n.T("tui.views.keychain_unavailable"))
		}
		return a.Secrets.Set(ctx, key, value)
	}
}

// activeProjectCopy gives a copy of the active project (nil without one), so
// that a view edits it without touching the store.
func activeProjectCopy(a *app.App) func() *domain.Project {
	return func() *domain.Project {
		p, _ := resolveActiveProject(a)
		if p == nil {
			return nil
		}
		cp := *p
		return &cp
	}
}

// saveProject writes a project to the store.
func saveProject(a *app.App) func(ctx context.Context, p *domain.Project) error {
	return func(ctx context.Context, p *domain.Project) error {
		return a.Projects.Update(ctx, p)
	}
}
