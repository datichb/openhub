package cmd

import (
	"context"
	"errors"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// trackerLocalOf is the personal tracker settings in force for the team the
// command (or the TUI) is on: the [tracker] default of hub.toml with the
// section of that team on top. Every limit and every check that reads the
// hub.toml overrides goes through it, not through a.Config.Tracker.
func trackerLocalOf(ctx context.Context, a *app.App) config.TrackerLocalConfig {
	return a.Config.TrackerFor(teamIDOf(ctx, a))
}

// teamIDOf is the id of the team ResolveTeam gives ("" when there is none).
func teamIDOf(ctx context.Context, a *app.App) string {
	t, err := ResolveTeam(ctx, a)
	if err != nil {
		return ""
	}
	return t.TeamID
}

// teamTrackerConfigField is the setter of a key such as
// teams.acme.tracker.auto_sync: the same keys as [tracker], on the section of
// one team of hub.toml. A team that is not in hub.toml is refused.
func teamTrackerConfigField(key string) (configField, bool) {
	rest, ok := strings.CutPrefix(key, "teams.")
	if !ok {
		return configField{}, false
	}
	id, sub, ok := strings.Cut(rest, ".tracker.")
	if !ok || id == "" {
		return configField{}, false
	}
	base, ok := configFieldMap["tracker."+sub]
	if !ok {
		return configField{}, false
	}
	on := func(c *config.Config, fn func(*config.Config)) error {
		t := c.FindTeam(id)
		if t == nil {
			return errors.New(i18n.Tf("v54c.config.unknown_team", id))
		}
		view := config.Config{Tracker: t.Tracker}
		fn(&view)
		t.Tracker = view.Tracker
		return nil
	}
	return configField{
		Set: func(c *config.Config, v string) error {
			var setErr error
			if err := on(c, func(view *config.Config) { setErr = base.Set(view, v) }); err != nil {
				return err
			}
			if setErr != nil {
				// the view was changed in place by a failed Set: nothing to undo,
				// a failed setter leaves its field as it was
				return setErr
			}
			return nil
		},
		Unset: func(c *config.Config) { _ = on(c, base.Unset) },
	}, true
}
