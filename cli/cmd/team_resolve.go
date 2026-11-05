package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	teamsvc "github.com/datichb/openhub/cli/internal/services/team"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The one resolution of "which team?". Every command and every screen that
// acts on a team goes through resolveTeam (or teamOfProject for the work that
// belongs to one project) and takes the repository AND the member from the
// same team: the team of hub.toml that comes first (config.ActiveTeam) is not
// read directly any more.

type teamCtxKey struct{}

// teamAsk is what a command line says about the team: --team and the project
// (-p) the command works on.
type teamAsk struct {
	Team    string
	Project string
}

// withTeamAsk records --team and --project in ctx.
func withTeamAsk(ctx context.Context, ask teamAsk) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, teamCtxKey{}, ask)
}

func teamAskOf(ctx context.Context) teamAsk {
	if ctx == nil {
		return teamAsk{}
	}
	ask, _ := ctx.Value(teamCtxKey{}).(teamAsk)
	return ask
}

type resolvedTeamKey struct{}

// withResolvedTeam records the team a command resolved, so that every later
// ResolveTeam of the command answers the same.
func withResolvedTeam(ctx context.Context, t resolvedTeam) context.Context {
	return context.WithValue(ctx, resolvedTeamKey{}, t)
}

func resolvedTeamOf(ctx context.Context) (resolvedTeam, bool) {
	if ctx == nil {
		return resolvedTeam{}, false
	}
	t, ok := ctx.Value(resolvedTeamKey{}).(resolvedTeam)
	return t, ok
}

// teamFlagMiddleware reads --team (and --project when the command has it) and
// records them in the context of the command.
func teamFlagMiddleware(cmd *cobra.Command) {
	ask := teamAsk{}
	ask.Team, _ = cmd.Flags().GetString("team")
	if f := cmd.Flags().Lookup("project"); f != nil {
		ask.Project = f.Value.String()
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	cmd.SetContext(withTeamAsk(ctx, ask))
}

// addTeamFlag registers the persistent --team flag on c.
func addTeamFlag(c *cobra.Command) {
	c.PersistentFlags().String("team", "", i18n.T("v54r.team.flag"))
}

// tuiTeamID is the team chosen in the TUI: the team of the team mode, else the
// one opened from the list of teams ("" outside of the TUI, or when none).
func tuiTeamID() string {
	if tuiShell != nil {
		switch tuiShell.Mode() {
		case views.ModeTeam:
			if t := tuiShell.ActiveTeam(); t != nil {
				return t.ID
			}
		case views.ModeProject:
			return "" // the team of the project
		}
	}
	return pickedTeamID()
}

// ambientProject is the project the work is about when nothing names one: the
// active project of the TUI, else (command line) the project of the current
// folder.
func ambientProject(ctx context.Context, a *app.App) *domain.Project {
	if a == nil || a.Projects == nil {
		return nil
	}
	if tuiShell != nil {
		p, _ := resolveActiveProject(a)
		return p
	}
	p, _ := bundleProject(ctx, a, "")
	return p
}

// resolvedTeam is the team a command acts on: where its repository is, the
// member of the user in it, and why it was chosen.
type resolvedTeam struct {
	config.ResolvedTeamConfig
	Source config.TeamSource
	// Archived: the team is known but archived (--team named it).
	Archived bool
}

// ResolveTeam is the one rule that says which team a command acts on:
//
//	--team  >  the team chosen in the TUI  >  the team of the project
//	(-p, else the active project / the current folder)  >  the default team
//	(`default = true`)  >  the first enabled team.
//
// An archived team is never taken but when --team names it (Archived is then
// set and Enabled is false). The error is a *config.UnknownTeamError for an
// unknown --team, or says that the project belongs to another team than the
// one --team names.
func ResolveTeam(ctx context.Context, a *app.App) (resolvedTeam, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if t, ok := resolvedTeamOf(ctx); ok {
		return t, nil
	}
	var project *domain.Project
	if ref := teamAskOf(ctx).Project; ref != "" && a.Projects != nil {
		project, _ = bundleProject(ctx, a, ref) // unknown project: not a reason to refuse
	} else {
		project = ambientProject(ctx, a)
	}
	return pickResolvedTeam(ctx, a, project, tuiTeamID())
}

func pickResolvedTeam(ctx context.Context, a *app.App, project *domain.Project, tui string) (resolvedTeam, error) {
	ask := teamAskOf(ctx)
	t, src, err := a.Config.PickTeam(config.TeamSelection{Flag: ask.Team, TUI: tui, Project: project})
	if err != nil {
		var unknown *config.UnknownTeamError
		if errors.As(err, &unknown) {
			return resolvedTeam{}, errors.New(i18n.Tf("cmd.teams.not_found", unknown.ID, theme.Bold.Render("oh teams list")))
		}
		return resolvedTeam{}, err
	}
	if src == config.TeamFromFlag && project != nil && ask.Project != "" && project.TeamID != nil && *project.TeamID != "" && *project.TeamID != t.ID {
		return resolvedTeam{}, errors.New(i18n.Tf("v54r.team.project_elsewhere", project.Name, *project.TeamID, t.ID))
	}
	return newResolvedTeam(a.Config, t, src), nil
}

// teamOfProject is the team the work of project belongs to: the team of the
// project when it has one (even when the TUI shows another team: a session
// of a project of B is B's, whatever team is open on screen), else the team
// the default rule gives (--team still comes first).
func teamOfProject(ctx context.Context, a *app.App, project *domain.Project) (resolvedTeam, error) {
	if project == nil {
		return ResolveTeam(ctx, a)
	}
	if teamAskOf(ctx).Team == "" {
		if tc := config.ResolveTeamForProject(a.Config, project); tc.Enabled {
			return resolvedTeam{ResolvedTeamConfig: tc, Source: config.TeamFromProject}, nil
		}
	}
	return pickResolvedTeam(ctx, a, project, "")
}

func newResolvedTeam(cfg *config.Config, t config.TeamConfig, src config.TeamSource) resolvedTeam {
	rt := resolvedTeam{ResolvedTeamConfig: t.Resolved(), Source: src, Archived: !t.Enabled && !t.Solo && t.ID != ""}
	if rt.TeamID == "" && rt.StateRepo != "" {
		rt.TeamID = cfg.TeamIDFor(rt.StateRepo)
	}
	return rt
}

// notEnabledError says why the team has no team features.
func (t resolvedTeam) notEnabledError() error {
	switch {
	case t.Archived:
		return errors.New(i18n.Tf("v54r.team.archived", t.TeamID, theme.Bold.Render("oh teams restore "+t.TeamID)))
	case t.Solo:
		return errors.New(i18n.Tf("v54r.team.solo", t.TeamID))
	}
	return fmt.Errorf(i18n.T("cmd.team.team_features_not_enabled_run_first"), theme.Bold.Render("oh team init"))
}

// teamSelectionFor is what the team service reads of the user's choice (--team
// of the command, team chosen in the TUI, ambient project).
func teamSelectionFor(ctx context.Context, a *app.App) config.TeamSelection {
	sel := config.TeamSelection{Flag: teamAskOf(ctx).Team, TUI: tuiTeamID()}
	if sel.Flag == "" && sel.TUI == "" {
		if ref := teamAskOf(ctx).Project; ref != "" && a.Projects != nil {
			sel.Project, _ = bundleProject(ctx, a, ref)
		} else {
			sel.Project = ambientProject(ctx, a)
		}
	}
	return sel
}

// resolvedTeamConfigCtx is the team configuration of project, or of the team
// ResolveTeam gives when project is nil. With a project it is that of the team
// of the project only: no team for a project without one.
func resolvedTeamConfigCtx(ctx context.Context, a *app.App, project *domain.Project) config.ResolvedTeamConfig {
	if project != nil {
		return config.ResolveTeamForProject(a.Config, project)
	}
	t, err := ResolveTeam(ctx, a)
	if err != nil {
		return config.ResolvedTeamConfig{}
	}
	return t.ResolvedTeamConfig
}

// commandMember is the member of the user in the team the command acts on.
func commandMember(ctx context.Context, a *app.App) string {
	t, err := ResolveTeam(ctx, a)
	if err != nil {
		return ""
	}
	return t.MemberID
}

// teamEnabledFor reports whether team features are on for project, or for the
// team ResolveTeam gives when project is nil.
func teamEnabledFor(ctx context.Context, a *app.App, project *domain.Project) bool {
	return resolvedTeamConfigCtx(ctx, a, project).Enabled
}

// currentTeamMember is the member of the user in the team the TUI is on
// ("" when there is no such team).
func currentTeamMember(a *app.App) string {
	t, err := ResolveTeam(context.Background(), a)
	if err != nil {
		return ""
	}
	return t.MemberID
}

// workProject is the project the work of the TUI is about: the active
// project, none in team mode (the team is then the one chosen, not the one of
// a project that happens to be alone).
func workProject(a *app.App) *domain.Project {
	if tuiShell != nil && tuiShell.Mode() == views.ModeTeam {
		return nil
	}
	p, _ := resolveActiveProject(a)
	return p
}

// defaultTeamConfig is the configuration of the default team, whatever the
// folder or the project: the checks of the hub (oh doctor) read it.
func defaultTeamConfig(a *app.App) config.ResolvedTeamConfig {
	t, _, _ := a.Config.PickTeam(config.TeamSelection{})
	return t.Resolved()
}

// teamServiceFor is the team service of the command, with its team choice.
func teamServiceFor(ctx context.Context, a *app.App) *teamsvc.Service {
	svc := teamsvc.New(a.Config, a.Secrets, a.Projects)
	svc.Pick = teamSelectionFor(ctx, a)
	return svc
}
