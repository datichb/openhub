package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// localEdits gathers the changes an assistant makes to hub.toml, to write them
// at once with config.Update (under the lock, backed up, on the file as it is
// on disk, the shared Config of the app refreshed). A key is set or unset with
// the typed setters of `oh config set` (configFieldMap), so a value is never
// written with another type than the one the key has.
type localEdits struct {
	// team, when set, makes the [tracker] keys the ones of that team
	// ([teams.tracker], teams.<id>.tracker.<key>): the settings the
	// assistant asks about are those of the team it configures.
	team string
	ops  []func(*config.Config) error
	said []string // what to tell the user once it is written
}

// keyOf is the key as written for the team of the edits.
func (l *localEdits) keyOf(key string) string {
	if l.team != "" && strings.HasPrefix(key, "tracker.") {
		return "teams." + l.team + "." + key
	}
	return key
}

// set changes the key of hub.toml to value (configFieldMap).
func (l *localEdits) set(key, value string) {
	key = l.keyOf(key)
	l.ops = append(l.ops, func(c *config.Config) error {
		f, ok := lookupConfigField(key)
		if !ok {
			return fmt.Errorf("%s", i18n.Tf("cmd.config.unknown_key", key))
		}
		return f.Set(c, value)
	})
	l.said = append(l.said, i18n.Tf("cmd.team_config.updated", config.ConfigPath(), key, value))
}

// unset removes the key from hub.toml: the setting is inherited again.
func (l *localEdits) unset(key string) {
	key = l.keyOf(key)
	l.ops = append(l.ops, func(c *config.Config) error {
		f, ok := lookupConfigField(key)
		if !ok {
			return fmt.Errorf("%s", i18n.Tf("cmd.config.unknown_key", key))
		}
		f.Unset(c)
		return nil
	})
	l.said = append(l.said, i18n.Tf("v533b.team_config.local_unset", config.ConfigPath(), key))
}

// change applies fn to the Config (for what has no key of configFieldMap).
func (l *localEdits) change(fn func(*config.Config) error) { l.ops = append(l.ops, fn) }

// tri records the new state of an optional boolean: nil removes the key.
func (l *localEdits) tri(key string, current, next *bool) {
	switch {
	case current == nil && next == nil, current != nil && next != nil && *current == *next:
	case next == nil:
		l.unset(key)
	default:
		l.set(key, strconv.FormatBool(*next))
	}
}

// apply writes the changes, in one update of hub.toml.
func (l *localEdits) apply(a *app.App) error {
	if len(l.ops) == 0 {
		return nil
	}
	err := updateHubConfig(a, func(c *config.Config) error {
		for _, op := range l.ops {
			if err := op(c); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, s := range l.said {
		fmt.Fprint(a.IO.Out, s)
	}
	return nil
}
