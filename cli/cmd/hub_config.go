package cmd

import (
	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
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
