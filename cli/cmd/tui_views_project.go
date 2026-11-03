package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/mcpresolve"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The views of the projects: the list, the project mode, and the configuration
// pages of the active project.

// buildProjectsView is the list of the projects; with a store it adds,
// removes, configures, renames and moves them, enters the project mode and
// initializes its tickets.
func buildProjectsView(a *app.App) *views.ProjectsView {
	projectsView := views.NewProjectsView(views.ProjectsViewConfig{
		Projects:         loadProjectItems(a.Projects, a.Config),
		KnownMCPServices: []string{"figma", "gitlab", "gslides"},
		RefreshFunc: func() []views.ProjectItem {
			return loadProjectItems(a.Projects, a.Config)
		},
	})
	if a.Projects != nil {
		projectsView.SetOnAdd(projectAdder(a))
		projectsView.SetOnRemove(projectRemover(a))
		projectsView.SetOnConfigure(projectConfigurer(a))
		projectsView.SetOnRename(projectRenamer(a))
		projectsView.SetOnMove(projectMover(a))
		// Enter project mode from the projects list view
		projectsView.SetOnEnterProject(func(p *views.ActiveProject) {
			if tuiShell != nil {
				tuiShell.SetProjectMode(p)
			}
		})
		projectsView.SetOnInitBeads(func(id, name, path string) {
			initBeadsForProject(a, id, name, path)
		})
	}
	return projectsView
}

// buildProjectModeView is the project mode, which resolves the team of the
// project through the same function as the team views.
func buildProjectModeView(a *app.App, sessionsSection views.SessionsSectionConfig, startSection views.StartSectionConfig) *views.ProjectModeView {
	projectModeView := views.NewProjectModeView(views.ProjectModeConfig{
		Sessions: sessionsSection,
		Start:    startSection,
		OnNavigate: func(viewID string) {
			if tuiShell != nil {
				tuiShell.NavigateTo(viewID)
			}
		},
		OnExitProjectMode: func() {
			if tuiShell != nil {
				tuiShell.SetProjectMode(nil)
			}
		},
		ToolLine: tuiToolLine,
		Runtimes: tuiProjectRuntimes(a),
	})
	projectModeView.SetResolveTeam(makeResolveTeamFunc(a))
	return projectModeView
}

// projectAdder adds the project of the projects view to the store, in the active team when the shell is in team mode.
func projectAdder(a *app.App) func(name, path string) {
	return func(name, path string) {
		p := &domain.Project{
			ID:     generateProjectID(name),
			Name:   name,
			Path:   path,
			Status: domain.ProjectStatusActive,
		}
		if tuiShell != nil && tuiShell.Mode() == views.ModeTeam && tuiShell.ActiveTeam() != nil {
			teamID := tuiShell.ActiveTeam().ID
			p.TeamID = &teamID
		}
		if _, _, err := upsertProject(context.Background(), a.Projects, p); err != nil {
			slog.Warn("failed to create/update project", "error", err)
			if tuiShell != nil {
				tuiShell.ShowToastMsg(i18n.Tf("tui.projects.save_failed", err.Error()), false)
			}
		} else {
			offerBeadsClean(a, p.Path)
		}
	}
}

// projectRemover deletes a project from the store.
func projectRemover(a *app.App) func(id string) {
	return func(id string) {
		if err := a.Projects.Delete(context.Background(), id); err != nil {
			slog.Warn("failed to remove project", "error", err)
			if tuiShell != nil {
				tuiShell.ShowToastMsg(i18n.Tf("tui.projects.delete_failed", err.Error()), false)
			}
		}
	}
}

// projectConfigurer saves the language, provider, model and MCP overrides of a project.
func projectConfigurer(a *app.App) func(id string, cfg views.ProjectConfigUpdate) {
	return func(id string, cfg views.ProjectConfigUpdate) {
		ctx := context.Background()
		project, err := a.Projects.Get(ctx, id)
		if err != nil {
			slog.Warn("failed to get project for configure", "id", id, "error", err)
			return
		}
		project.Language = cfg.Language
		project.Provider = cfg.Provider
		project.Model = cfg.Model
		project.UpdatedAt = time.Now()

		// Persist per-project MCP overrides
		if len(cfg.MCPOverrides) > 0 {
			services := make([]domain.ProjectMCPService, 0, len(cfg.MCPOverrides))
			for svcName, state := range cfg.MCPOverrides {
				svc := domain.ProjectMCPService{Name: svcName}
				switch state {
				case "enabled":
					t := true
					svc.Enabled = &t
				case "disabled":
					f := false
					svc.Enabled = &f
					// "inherit" → nil (omit, let hub config win)
				}
				services = append(services, svc)
			}
			project.MCPConfig = &domain.ProjectMCPConfig{Services: services}
		} else {
			project.MCPConfig = nil
		}

		if err := a.Projects.Update(ctx, project); err != nil {
			slog.Warn("failed to update project config", "id", id, "error", err)
			if tuiShell != nil {
				tuiShell.ShowToastMsg(i18n.Tf("tui.projects.config_save_failed", err.Error()), false)
			}
		}
	}
}

// projectRenamer renames a project.
func projectRenamer(a *app.App) func(id, newName string) {
	return func(id, newName string) {
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
			if tuiShell != nil {
				tuiShell.ShowToastMsg(i18n.Tf("tui.projects.rename_failed", err.Error()), false)
			}
		}
	}
}

// projectMover changes the path of a project.
func projectMover(a *app.App) func(id, newPath string) {
	return func(id, newPath string) {
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
			if tuiShell != nil {
				tuiShell.ShowToastMsg(i18n.Tf("tui.projects.move_failed", err.Error()), false)
			}
		}
	}
}

// buildProjectConfigView is the configuration of the active project.
func buildProjectConfigView(a *app.App) views.View {
	return views.NewProjectConfigView(views.ProjectConfigViewConfig{
		GetProject:  activeProjectCopy(a),
		SaveProject: saveProject(a),
		ExecHints:   projectExecHints,
		WorkflowIDs: tuiWorkflowIDs,
	})
}

// buildProjectMCPView is the MCP services of the active project.
func buildProjectMCPView(a *app.App) views.View {
	return views.NewProjectMCPView(views.ProjectMCPViewConfig{
		GetProject:       activeProjectCopy(a),
		SaveProject:      saveProject(a),
		ResolveMCPSource: projectMCPSource(a),
	})
}

// buildProjectModelsView is the models of the active project.
func buildProjectModelsView(a *app.App) views.View {
	return views.NewProjectModelsView(views.ProjectModelsViewConfig{
		GetProject:  activeProjectCopy(a),
		SaveProject: saveProject(a),
	})
}

// projectMCPSource says what the MCP setting of a service is for the active project, and why: the team enforces it, the hub or the project sets it.
func projectMCPSource(a *app.App) func(service, field string) (effective string, source string, locked bool) {
	return func(service, field string) (effective string, source string, locked bool) {
		project, _ := resolveActiveProject(a)
		tc := resolvedTeamConfig(a, project)

		// Load team shared MCP config
		var shared *teamstate.SharedMCPConfig
		var teamID string
		if tc.Enabled && tc.StatePath != "" {
			teamID = tc.TeamID
			repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
			if repo.IsCloned() {
				teamCfg, _ := repo.LoadConfig(tuiContext())
				if teamCfg != nil {
					if s, ok := teamCfg.MCP[service]; ok {
						shared = &s
					}
				}
			}
		}

		// Hub MCP config for this service
		hub := hubMCPServerConfig(a.Config, service)

		// Project MCP service
		var projectSvc *domain.ProjectMCPService
		if project != nil && project.MCPConfig != nil {
			for i, svc := range project.MCPConfig.Services {
				if svc.Name == service {
					projectSvc = &project.MCPConfig.Services[i]
					break
				}
			}
		}

		eff := mcpresolve.ResolveFull(shared, hub, projectSvc, teamID)

		switch field {
		case "enabled":
			src := ""
			if eff.EnabledEnforced {
				src = fmt.Sprintf("[%s: enforced]", teamID)
			}
			return strconv.FormatBool(eff.Enabled), src, eff.EnabledEnforced
		case "url":
			src := ""
			switch {
			case eff.URLEnforced:
				src = fmt.Sprintf("[%s: enforced]", teamID)
			case eff.EnforcedURLPending != "":
				src = i18n.Tf("tui.config.url_pending", teamID, service)
			}
			return eff.URL, src, eff.URLEnforced
		default:
			return "", "", false
		}
	}
}
