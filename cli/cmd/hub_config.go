package cmd

import (
	"context"
	"errors"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// updateHubConfig changes hub.toml: fn receives the file as it is on disk,
// under the lock, and changes only the settings of its caller (config.Update).
// The Config the app shares (the one the views hold) then takes the result in
// place, so nobody keeps a copy that is older than the file.
func updateHubConfig(a *app.App, fn func(*config.Config) error) error {
	fresh, err := config.UpdateAndGet(fn)
	if err != nil {
		return err
	}
	shareConfig(a, fresh)
	return nil
}

// updateHubConfigOffLoop is updateHubConfig for a view that writes off the
// event loop of the TUI: the file is written where it is called, and the
// shared Config (read on the loop) takes the result there, queued before the
// end of the write is given back to the view.
func updateHubConfigOffLoop(a *app.App, fn func(*config.Config) error) error {
	fresh, err := config.UpdateAndGet(fn)
	if err != nil {
		return err
	}
	if tuiShell != nil {
		runOnLoop(func() { shareConfig(a, fresh) })
	} else {
		shareConfig(a, fresh)
	}
	return nil
}

// teamConfigWriter is called on the event loop: it resolves the team the views
// work on, the same as they do (it reads the Config), and gives the function
// that writes the team config to its team-state, which the view runs off the
// loop.
func teamConfigWriter(a *app.App) views.TeamConfigWriter {
	tc := makeResolveTeamFunc(a)()
	return func(ctx context.Context, cfg *teamstate.TeamConfig) error {
		if !tc.Enabled {
			return errors.New(i18n.T("tui.views.team_not_configured"))
		}
		return teamstate.NewRepo(tc.StateRepo, tc.StatePath).SaveConfig(ctx, cfg)
	}
}

// shareConfig copies c into the Config of the app, which the views and the
// wizard steps hold by pointer: the pointer stays, the content follows the file.
func shareConfig(a *app.App, c *config.Config) {
	if a != nil && a.Config != nil && c != nil && a.Config != c {
		*a.Config = *c
	}
}

// refreshSharedConfig reads hub.toml again into the Config of the app, for
// what was written through another path (a wizard step, another process).
func refreshSharedConfig(a *app.App) error {
	fresh, err := config.Read()
	if err != nil {
		return err
	}
	shareConfig(a, fresh)
	return nil
}
