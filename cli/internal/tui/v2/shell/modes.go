package shell

import (
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Navigation modes (ADR-032): hub, project, team, and the Ctrl+T cycle
// between them.

// SetProjectMode activates or deactivates project mode.
// Passing nil deactivates project mode (hub mode).
func (s *Shell) SetProjectMode(project *views.ActiveProject) {
	s.activeProject = project
	if project != nil {
		s.lastProject = project.ID
		s.activeMode = views.ModeProject
		theme.SetActiveMode(string(views.ModeProject))
		s.router.NavigateTo("project.mode")
	} else {
		s.SetMode(views.ModeHub)
	}
}

// SetActiveProject sets the active project without triggering navigation.
// Use this to pre-set the project before NavigateHome is called.
func (s *Shell) SetActiveProject(project *views.ActiveProject) {
	s.activeProject = project
	if project != nil {
		s.lastProject = project.ID
	}
}

// ActiveProject returns the currently active project, or nil if in hub mode.
func (s *Shell) ActiveProject() *views.ActiveProject {
	return s.activeProject
}

// ─────────────────────────────────────────────────────────────────────────────
// Mode navigation (ADR-032 Phase 3)
// ─────────────────────────────────────────────────────────────────────────────

// SetMode switches the shell to the given navigation mode and navigates to its landing view.
func (s *Shell) SetMode(mode views.Mode) {
	s.activeMode = mode
	theme.SetActiveMode(string(mode))
	switch mode {
	case views.ModeHub:
		s.activeProject = nil
		s.activeTeam = nil
		s.router.NavigateTo("home")
	case views.ModeTeam:
		s.router.NavigateTo("team.mode")
	case views.ModeProject:
		if s.activeProject != nil {
			s.router.NavigateTo("project.mode")
		} else {
			s.router.NavigateTo("projects.list")
		}
	}
}

// Mode returns the current navigation mode.
func (s *Shell) Mode() views.Mode {
	return s.activeMode
}

// modeRoots maps the landing view of each mode to that mode.
var modeRoots = map[string]views.Mode{
	"home":         views.ModeHub,
	"team.mode":    views.ModeTeam,
	"project.mode": views.ModeProject,
}

// syncModeWithStack derives the mode from the nearest landing view in the
// navigation stack, before the view is mounted: going back (Esc, « home »
// command) to a landing restores its mode, its theme and its context.
func (s *Shell) syncModeWithStack(stack []views.View) {
	for i := len(stack) - 1; i >= 0; i-- {
		mode, ok := modeRoots[stack[i].ID()]
		if !ok {
			continue
		}
		if mode == views.ModeProject && s.activeProject == nil {
			continue
		}
		s.activeMode = mode
		switch mode {
		case views.ModeHub:
			s.activeProject = nil
			s.activeTeam = nil
		case views.ModeTeam:
			s.activeProject = nil
		}
		break
	}
	theme.SetActiveMode(string(s.activeMode))
}

// SetActiveTeam sets the active team context without triggering navigation.
func (s *Shell) SetActiveTeam(team *views.ActiveTeam) {
	s.activeTeam = team
}

// ActiveTeam returns the currently active team, or nil outside team mode.
func (s *Shell) ActiveTeam() *views.ActiveTeam {
	return s.activeTeam
}

// cycleMode is Ctrl+T: Hub → Project → Team → Hub, skipping the modes that
// are not configured (it used to go only between the hub and the
// team). From the hub, the project is the one the user left (or the only one,
// or one to choose); from the project, the team (selector when several teams
// are configured); from the team, back to the hub. With no project the hub
// goes straight to the team, with no team the project goes back to the hub,
// with neither a toast says so.
func (s *Shell) cycleMode() {
	switch s.activeMode {
	case views.ModeTeam:
		s.SetMode(views.ModeHub)
	case views.ModeProject:
		if s.teamsConfigured() {
			s.enterTeamMode()
			return
		}
		s.SetMode(views.ModeHub)
	default:
		s.enterProjectMode(func() {
			if s.teamsConfigured() {
				s.enterTeamMode()
				return
			}
			s.ShowToast(i18n.T("tui.no_mode_configured"), ToastWarning)
		})
	}
}

// teamsConfigured reports whether Ctrl+T can enter a team: a team is
// configured, or the shell has no list of them and falls back on the teams
// view.
func (s *Shell) teamsConfigured() bool {
	return s.cfg.TeamsProvider == nil || len(s.cfg.TeamsProvider()) > 0
}

// enterProjectMode enters the project mode: the project the user left when it
// is still there, the only project, or the one chosen in a selector. The
// projects are read off the event loop. none runs when there is no project.
func (s *Shell) enterProjectMode(none func()) {
	if s.cfg.ProjectsProvider == nil {
		none()
		return
	}
	last := s.lastProject
	safego.Go(func() {
		opts := s.cfg.ProjectsProvider()
		s.app.QueueUpdateDraw(func() {
			for _, o := range opts {
				if last != "" && o.Value == last {
					s.enterProject(o.Value)
					return
				}
			}
			switch len(opts) {
			case 0:
				none()
			case 1:
				s.enterProject(opts[0].Value)
			default:
				s.ShowSelectModal(i18n.T("tui.select_project"), opts, "", s.enterProject)
			}
		})
	})
}

// enterProject makes the project active and enters the project mode. The
// project is completed (path, branch) off the event loop.
func (s *Shell) enterProject(id string) {
	safego.Go(func() {
		var p *views.ActiveProject
		if s.cfg.ProjectResolver != nil {
			p = s.cfg.ProjectResolver(id)
		}
		s.app.QueueUpdateDraw(func() {
			if p == nil {
				s.ShowToast(i18n.T("tui.project_not_found"), ToastWarning)
				return
			}
			s.SetActiveProject(p)
			s.SetMode(views.ModeProject)
		})
	})
}

// enterTeamMode enters the team mode (ADR-032 Phase 3): the only team, or the
// one chosen in a selector.
func (s *Shell) enterTeamMode() {
	if s.cfg.TeamsProvider == nil {
		// No TeamsProvider — fallback to the teams list view.
		s.activeMode = views.ModeTeam
		theme.SetActiveMode(string(views.ModeTeam))
		s.router.NavigateTo("teams")
		return
	}
	teams := s.cfg.TeamsProvider()
	switch len(teams) {
	case 0:
		s.ShowToast(i18n.T("tui.no_team_configured"), ToastWarning)
	case 1:
		s.activeTeam = &views.ActiveTeam{ID: teams[0].Value, Name: teams[0].Label}
		s.SetMode(views.ModeTeam)
	default:
		s.ShowSelectModal(i18n.T("tui.select_team"), teams, "", func(selected string) {
			for _, t := range teams {
				if t.Value == selected {
					s.activeTeam = &views.ActiveTeam{ID: t.Value, Name: t.Label}
					s.SetMode(views.ModeTeam)
					return
				}
			}
		})
	}
}
