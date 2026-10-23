package cmd

import (
	"log/slog"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// maxConfigChanges is the number of changed settings a launch names; the
// others are counted.
const maxConfigChanges = 5

// runOnLoop runs fn on the event loop of the TUI, where the views read the
// Config they share. Outside the TUI (command line, daemon, fan-out driver)
// no view holds it: nothing to do.
var runOnLoop = func(fn func()) {
	if sh := tuiShell; sh != nil {
		sh.App().QueueUpdateDraw(fn)
	}
}

// launchAppNow is the app a launch works with. A process that runs for a
// long time (the TUI, the daemon, the fan-out driver) read hub.toml once; when
// another process changed it since (`oh config model default …` in another
// terminal), the launch must not build its bundle with the old settings. If
// hub.toml changed since the Config of a was read, the result is an app whose
// Config is the file as it is now (a copy of its own: nothing else reads it),
// and the shared Config takes the file in place on the event loop. Otherwise
// it is a itself. If the file can no longer be read, it is a and err says why.
func launchAppNow(a *app.App) (*app.App, error) {
	if a == nil || a.Config == nil {
		return a, nil
	}
	changed, err := a.Config.ChangedOnDisk()
	if err != nil {
		slog.Debug("hub.toml not compared with the configuration in memory", "error", err)
		return a, nil
	}
	if !changed {
		return a, nil
	}
	fresh, err := config.Read()
	if err != nil {
		slog.Warn("hub.toml changed but cannot be read: the launch keeps the configuration in memory", "error", err)
		return a, err
	}
	fa := *a
	fa.Config = fresh
	runOnLoop(func() { shareConfig(a, fresh) })
	return &fa, nil
}

// freshLaunchApp is launchAppNow and the lines that tell the recap what
// changed for the launch (models, provider, region, limits… that differ from
// the Config the app had); empty when nothing did. A file that cannot be read
// is one line.
func freshLaunchApp(a *app.App) (fa *app.App, changes []string) {
	fa, err := launchAppNow(a)
	return fa, launchChangeLines(a, fa, err, nil)
}

// launchChangeLines words what a launch changed: the settings of fa that
// differ from baseline (the Config of a when nil), or the file that cannot be
// read (err).
func launchChangeLines(a, fa *app.App, err error, baseline *config.Config) []string {
	if err != nil {
		return []string{i18n.Tf("cmd.run.config_unreadable", localizeError(err))}
	}
	if a == nil || fa == nil || fa.Config == nil {
		return nil
	}
	if baseline == nil {
		baseline = a.Config
	}
	if baseline == fa.Config {
		return nil
	}
	return configChangeLines(config.LaunchChanges(baseline, fa.Config))
}

// configChangeLines words the changed settings, one line each (the first
// maxConfigChanges, then the count of the others).
func configChangeLines(changes []config.SettingChange) []string {
	var out []string
	notSet := i18n.T("cmd.config.model.show.not_set")
	for i, c := range changes {
		if i == maxConfigChanges {
			out = append(out, i18n.Tf("cmd.run.config_changed_more", len(changes)-maxConfigChanges))
			break
		}
		oldV, newV := c.Old, c.New
		if oldV == "" {
			oldV = notSet
		}
		if newV == "" {
			newV = notSet
		}
		out = append(out, i18n.Tf("cmd.run.config_changed", c.Key, oldV, newV))
	}
	return out
}
