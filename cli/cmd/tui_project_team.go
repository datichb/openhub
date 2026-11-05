package cmd

import (
	"context"
	"errors"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// « Rattacher le projet à une équipe »: the team of a project is chosen in a
// list of all the teams, the solo spaces, or none.

const (
	// attachSoloNew is the choice « a new solo space ».
	attachSoloNew = "\x00solo"
	// attachNewTeam is the choice « a new team »: the window that starts one.
	attachNewTeam = "\x00new"
)

// soloSpaces are the solo spaces a project can be attached to: the enabled
// ones.
func soloSpaces(cfg *config.Config) []config.TeamConfig {
	var out []config.TeamConfig
	for _, t := range cfg.Teams {
		if t.Solo && t.Enabled {
			out = append(out, t)
		}
	}
	return out
}

// projectTeamChoices are the teams a project can be attached to, for a list
// where one is chosen: every team (the default first), the solo spaces, then
// « none » (Value ""). current, the team the project is in now, stays in the
// list even when it is archived or gone from hub.toml, so that the list never
// hides where the project is.
func projectTeamChoices(cfg *config.Config, current string) []views.SelectOption {
	var out []views.SelectOption
	seen := map[string]bool{}
	for _, t := range attachableTeams(cfg) {
		out = append(out, views.SelectOption{Label: teamChoiceLabel(cfg, t), Value: t.ID})
		seen[t.ID] = true
	}
	for _, t := range soloSpaces(cfg) {
		out = append(out, views.SelectOption{Label: i18n.Tf("v54u.team.solo_choice", t.DisplayName()), Value: t.ID})
		seen[t.ID] = true
	}
	if current != "" && !seen[current] {
		label := i18n.Tf("v54u.team.unknown_choice", current)
		if t := cfg.FindTeam(current); t != nil {
			label = i18n.Tf("v54u.team.archived_choice", t.DisplayName())
		}
		out = append(out, views.SelectOption{Label: label, Value: current})
	}
	return append(out, views.SelectOption{Label: i18n.T("tui.team.configure.no_team"), Value: ""})
}

// projectTeamID is the team a project is in ("" for none).
func projectTeamID(p *domain.Project) string {
	if p == nil || p.TeamID == nil {
		return ""
	}
	return *p.TeamID
}

// actionTeamConfigure is « Rattacher le projet à une équipe » (project mode):
// the active project, else one chosen in a list, goes to a team, a solo space
// or none.
func actionTeamConfigure() {
	sh := tuiShell
	if sh == nil {
		return
	}
	a := MustApp()
	if p := sh.ActiveProject(); p != nil {
		attachProjectWindow(a, p.ID)
		return
	}
	// No project is open (the palette is not the only way in): choose which
	// one before the team, instead of failing.
	projects, _ := a.Projects.List(context.Background(), domain.ProjectStatusActive)
	if len(projects) == 0 {
		sh.ShowToast(i18n.T("tui.team.configure.no_project"), shell.ToastError)
		return
	}
	opts := make([]views.SelectOption, 0, len(projects))
	for _, p := range projects {
		opts = append(opts, views.SelectOption{Label: p.Name, Value: p.ID})
	}
	sh.ShowSelectModal(i18n.T("v54u.team.pick_project"), opts, "", func(id string) {
		safego.Go(func() { queueOnShell(sh, func() { attachProjectWindow(a, id) }) })
	})
}

// attachProjectWindow asks the team of the project projectID.
func attachProjectWindow(a *app.App, projectID string) {
	sh := tuiShell
	project, err := a.Projects.Get(context.Background(), projectID)
	if err != nil || project == nil {
		sh.ShowToast(i18n.T("tui.team.configure.project_not_found"), shell.ToastError)
		return
	}
	current := projectTeamID(project)
	opts := projectTeamChoices(a.Config, current)
	// « none » stays the last choice of the teams; a new solo space is offered
	// while there is none, and a new team always.
	none := opts[len(opts)-1]
	opts = opts[: len(opts)-1 : len(opts)-1]
	if len(soloSpaces(a.Config)) == 0 {
		opts = append(opts, views.SelectOption{Label: i18n.T("v54u.team.solo_new_choice"), Value: attachSoloNew})
	}
	opts = append(opts, none, views.SelectOption{Label: i18n.T("v54u.team.new_choice"), Value: attachNewTeam})

	// The team the project is in now, else the default team
	preset := current
	if preset == "" {
		if t := a.Config.DefaultTeam(); t != nil {
			preset = t.ID
		}
	}
	sh.ShowSelectModal(i18n.Tf("tui.team.configure.modal_title", project.Name), opts, preset, func(teamID string) {
		safego.Go(func() {
			queueOnShell(sh, func() {
				switch teamID {
				case attachNewTeam:
					actionTeamNew()
				default:
					applyProjectTeam(a, project.ID, teamID)
				}
			})
		})
	})
}

// applyProjectTeam attaches the project to teamID ("" detaches it,
// attachSoloNew attaches it to a new solo space) and says so. The store and
// the solo space are written off the event loop.
func applyProjectTeam(a *app.App, projectID, teamID string) {
	sh := tuiShell
	ctx := sh.Context()
	safego.Go(func() {
		label, name, err := attachProject(ctx, a, projectID, teamID)
		queueOnShell(sh, func() {
			if err != nil {
				sh.ShowToast(i18n.T("tui.team.configure.error_prefix")+err.Error(), shell.ToastError)
				return
			}
			sh.ShowToast(i18n.Tf("tui.team.configure.team_set", name, label), shell.ToastSuccess)
			sh.RemountIf("project.mode", "project.config")
		})
	})
}

// attachProject does the attachment of applyProjectTeam; it returns the team
// as it is told to the member and the name of the project.
func attachProject(ctx context.Context, a *app.App, projectID, teamID string) (label, name string, err error) {
	project, err := a.Projects.Get(ctx, projectID)
	if err != nil || project == nil {
		return "", "", errors.New(i18n.T("tui.team.configure.project_not_found"))
	}
	name = project.Name
	switch teamID {
	case "":
		project.TeamID, project.TeamConfig = nil, nil
		return i18n.T("tui.team.configure.team_none"), name, a.Projects.Update(ctx, project)
	case attachSoloNew:
		// initSoloTeam refuses a project that is in a team: detach it first.
		project.TeamID, project.TeamConfig = nil, nil
		if err := a.Projects.Update(ctx, project); err != nil {
			return "", name, err
		}
		sp, err := attachProjectSolo(ctx, a, project)
		if err != nil {
			return "", name, err
		}
		return sp.Team.ID, name, nil
	default:
		return teamID, name, setProjectTeam(ctx, a, project, teamID)
	}
}
