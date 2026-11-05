package cmd

import (
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The one window that starts a team in the TUI: the « Nouvelle équipe » row of
// the home, the key n of the Teams view, the palette (« team.new »), the team
// mode and Ctrl+T without any team all open it. The three ways are the same
// everywhere: create a team (a new team-state repository), rejoin one (an
// existing repository: new member or reinstallation) or a solo space.
const (
	newTeamCreate = "create"
	newTeamRejoin = "rejoin"
	newTeamSolo   = "solo"
)

// refreshAfterNewTeam draws again the views that list the teams once a solo
// space is created (the assistants refresh the view they return to). Event
// loop only.
func refreshAfterNewTeam() {
	if tuiShell != nil {
		tuiShell.RemountIf("home", "team.mode", "project.mode", "teams")
	}
}

// actionTeamNew is « Nouvelle équipe… »: create a team, rejoin one, or a solo
// space.
func actionTeamNew() {
	openNewTeamWindow(i18n.T("v54u.new.title"), true)
}

// actionTeamNewOrJoin is the window of Ctrl+T when there is no team to enter:
// create or join one (a solo space has no team mode to enter).
func actionTeamNewOrJoin() {
	openNewTeamWindow(i18n.T("v54u.new.no_team_title"), false)
}

func openNewTeamWindow(title string, withSolo bool) {
	sh := tuiShell
	if sh == nil {
		return
	}
	opts := []views.SelectOption{
		{Label: i18n.T("v54u.new.create"), Value: newTeamCreate},
		{Label: i18n.T("v54u.new.rejoin"), Value: newTeamRejoin},
	}
	if withSolo {
		opts = append(opts, views.SelectOption{Label: i18n.T("v54u.new.solo"), Value: newTeamSolo})
	}
	sh.ShowSelectModal(title, opts, "", func(choice string) {
		// The window closes first: the assistant opens on the next tick, as
		// the other choices of a modal that start a view.
		safego.Go(func() {
			queueOnShell(sh, func() { startNewTeam(choice) })
		})
	})
}

// startNewTeam runs the choice of the window.
func startNewTeam(choice string) {
	switch choice {
	case newTeamCreate:
		actionTeamInit()
	case newTeamRejoin:
		actionTeamRejoin()
	case newTeamSolo:
		tuiCreateSoloSpace(refreshAfterNewTeam)
	}
}

// enterTeamMode opens the team mode on the team id (its name as hub.toml
// gives it).
func enterTeamMode(a *app.App, id string) {
	sh := tuiShell
	if sh == nil {
		return
	}
	name := id
	if t := a.Config.FindTeam(id); t != nil {
		name = t.DisplayName()
	}
	sh.SetActiveTeam(&views.ActiveTeam{ID: id, Name: name})
	sh.SetMode(views.ModeTeam)
}

// actionSwitchTeam is « Changer d'équipe… » of the team mode: the teams of
// Ctrl+T (no solo space, no archived team), the team open being preselected.
// Without any team it offers to create or to join one.
func actionSwitchTeam() {
	sh := tuiShell
	if sh == nil {
		return
	}
	a := MustApp()
	opts := teamSelectorOptions(a.Config)
	if len(opts) == 0 {
		actionTeamNewOrJoin()
		return
	}
	sh.ShowSelectModal(i18n.T("tui.select_team"), opts, tuiTeamID(), func(id string) {
		safego.Go(func() {
			queueOnShell(sh, func() {
				enterTeamMode(a, id)
				// Already on the landing of the team mode, the navigation
				// above is a no-op: mount it again for the new team.
				sh.RemountIf("team.mode")
			})
		})
	})
}
