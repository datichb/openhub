package cmd

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// resolveActiveProject returns the currently selected project or the first one.
//
// In the TUI the shell's active project is the single source of truth (B4):
// no interactive picker is ever started from here (it would fight with tview);
// when nothing is selected and several projects exist, an error is returned
// and the caller shows its own selector.
func resolveActiveProject(a *app.App) (*domain.Project, error) {
	ctx := context.Background()
	if a == nil || a.Projects == nil {
		return nil, errors.New(i18n.T("cmd.tui_helpers.no_project_store"))
	}
	if tuiShell != nil {
		if ap := tuiShell.ActiveProject(); ap != nil && ap.ID != "" {
			return a.Projects.Get(ctx, ap.ID)
		}
	}
	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		return nil, err
	}
	if tuiShell == nil {
		// Views are being built: the project of the current directory only.
		if p := projectAtCwd(projects); p != nil {
			return p, nil
		}
	}
	if len(projects) == 1 {
		return &projects[0], nil
	}
	return nil, errors.New(i18n.T("cmd.tui_helpers.no_active_project_selected"))
}

// projectAtCwd returns the project containing the current directory, if any.
func projectAtCwd(projects []domain.Project) *domain.Project {
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}
	for i, p := range projects {
		absPath, _ := filepath.Abs(p.Path)
		if absPath == cwd || isSubPath(cwd, absPath) {
			return &projects[i]
		}
	}
	return nil
}

// makeResolveTeamFunc returns a ResolveTeamFunc for use in team views.
// It resolves the effective team config for the currently active project
// (project-level override → hub fallback) each time it is called.
func makeResolveTeamFunc(a *app.App) views.ResolveTeamFunc {
	return func() views.TeamResolution {
		// Team mode: the team chosen in the TUI wins (B5).
		if tuiShell != nil && tuiShell.Mode() == views.ModeTeam && tuiShell.ActiveTeam() != nil {
			if t := a.Config.FindTeam(tuiShell.ActiveTeam().ID); t != nil {
				return views.TeamResolution{Enabled: t.Enabled && !t.Solo, StateRepo: t.StateRepo, StatePath: t.StatePath, MemberID: t.MemberID,
					Solo: t.Solo, TeamID: t.ID}
			}
		}
		project, _ := resolveActiveProject(a)
		tc := resolvedTeamConfig(a, project)
		return views.TeamResolution{
			Enabled:   tc.Enabled,
			StateRepo: tc.StateRepo,
			StatePath: tc.StatePath,
			MemberID:  tc.MemberID,
			Solo:      tc.Solo,
			TeamID:    tc.TeamID,
		}
	}
}

// initBeadsForActiveProject initialises beads for the currently active project.
// Shows a confirmation modal before proceeding.
func initBeadsForActiveProject(a *app.App) {
	if tuiShell == nil {
		return
	}
	path := resolveActiveProjectPath(a)
	if path == "" {
		tuiShell.ShowToastMsg(i18n.T("tui.helpers.no_active_project"), false)
		return
	}
	// Resolve project ID for the prefix
	var id, name string
	if ap := tuiShell.ActiveProject(); ap != nil {
		id = ap.ID
		name = ap.Name
	} else if p, err := resolveActiveProject(a); err == nil && p != nil {
		id = p.ID
		name = p.Name
	}
	initBeadsForProject(a, id, name, path)
}

// initBeadsForProject shows a confirmation modal then initialises beads for the
// given project (path, id/name as prefix).
func initBeadsForProject(a *app.App, id, name, path string) {
	if tuiShell == nil {
		return
	}
	if beads.IsInitialized(path) {
		tuiShell.ShowToastMsg(i18n.Tf("tui.helpers.board_already_init", name), true)
		tuiShell.NavigateTo("board")
		return
	}

	label := name
	if label == "" {
		label = path
	}

	tuiShell.ShowSelectModal(
		i18n.Tf("tui.helpers.init_board_confirm", label),
		[]views.SelectOption{
			{Label: i18n.T("tui.helpers.yes_init_beads"), Value: "yes"},
			{Label: i18n.T("tui.helpers.cancel"), Value: "no"},
		}, "",
		func(value string) {
			if value != "yes" {
				return
			}
			prefix := id
			if prefix == "" {
				prefix = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
			}
			if err := beads.Init(path, prefix); err != nil {
				tuiShell.ShowToastMsg(i18n.T("tui.helpers.init_beads_error")+err.Error(), false)
				return
			}
			tuiShell.ShowToastMsg(i18n.Tf("tui.helpers.board_initialized", label), true)
			offerBeadsClean(a, path)
			tuiShell.NavigateTo("board")
		},
	)
}

// fetchBoardTicketsForPath loads and normalises tickets from the beads system
// for the given project path. Returns nil (and no error) when bd is not installed,
// when the project has no tickets yet, or when beads is not initialized.
func fetchBoardTicketsForPath(projectPath string) []views.BoardTicket {
	if projectPath == "" {
		return nil
	}
	if err := beads.Available(); err != nil {
		return nil // bd not installed — board shows empty, no crash
	}
	if !beads.IsInitialized(projectPath) {
		return nil // no .beads/ — board will show the init invite screen
	}
	raw, err := beads.ListAll(projectPath)
	if err != nil {
		slog.Warn("board: failed to list beads tickets", "path", projectPath, "error", err)
		return nil
	}
	out := make([]views.BoardTicket, 0, len(raw))
	for _, t := range raw {
		// Epics are containers, not actionable items — exclude from the board.
		if strings.EqualFold(t.Type, "epic") {
			continue
		}
		out = append(out, views.BoardTicket{
			ID:          t.ID,
			Title:       t.Title,
			Status:      boardNormalizeStatus(t.Status),
			Priority:    t.Priority,
			Type:        t.Type,
			ExternalRef: beads.ExternalRefForTicket(t),
		})
	}
	return out
}

// boardNormalizeStatus maps beads status values to the board column statuses.
func boardNormalizeStatus(s string) string {
	return normalizeStatus(s)
}

// resolveActiveProjectPath returns the path of the active project.
// Prefers the shell's active project; falls back to the first registered project.
// Returns "" if no project is available (safe to call with a minimal App in tests).
func resolveActiveProjectPath(a *app.App) string {
	if tuiShell != nil {
		if ap := tuiShell.ActiveProject(); ap != nil {
			return ap.Path
		}
	}
	if a == nil || a.Projects == nil {
		return ""
	}
	// Non-interactive fallback: pick the first active project without prompting.
	ctx := context.Background()
	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil || len(projects) == 0 {
		return ""
	}
	return projects[0].Path
}
