package cmd

import (
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
)

// teamStatePathFor is the folder of the clone of the team-state stateRepo: the
// one of the team hub.toml already has for this repository, else the folder
// of a new clone. Every team assistant (wizards, CLI, TUI) asks it, none
// derives the path from the URL itself: joining a team that is already
// configured reuses its clone instead of cloning it a second time.
func teamStatePathFor(a *app.App, stateRepo string) string {
	return teamService(a).StatePathFor(stateRepo)
}

// teamIDFor is the ID the team of stateRepo has in hub.toml, or the one it
// would get (config.Config.TeamIDFor): the keychain key of its token is
// derived from it.
func teamIDFor(a *app.App, stateRepo string) string {
	if a == nil || a.Config == nil {
		return config.TeamIDSlug(config.RepoNameFromRemote(stateRepo))
	}
	return a.Config.TeamIDFor(stateRepo)
}
