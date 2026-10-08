package cmd

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/limits"
	"github.com/datichb/openhub/cli/internal/semver"
)

// v5 sections of `oh config set|unset` (A15): [session], [execution],
// [limits] and [remote], with the same values as the TUI Settings.

func init() {
	for k, f := range v5ConfigFields() {
		configFieldMap[k] = f
	}
}

func invalidConfigValue(key, value, expected string) error {
	return errors.New(i18n.Tf("cmd.config.invalid_value", value, key, expected))
}

// enumField sets a string among allowed values; def ("" = none) is stored
// as an empty value (default of oh).
func enumField(key string, get func(c *config.Config) *string, def string, allowed ...string) configField {
	return configField{
		Set: func(c *config.Config, v string) error {
			v = strings.TrimSpace(v)
			if !slices.Contains(allowed, v) {
				return invalidConfigValue(key, v, strings.Join(allowed, " | "))
			}
			if v == def {
				v = ""
			}
			*get(c) = v
			return nil
		},
		Unset: func(c *config.Config) { *get(c) = "" },
	}
}

func intField(key string, get func(c *config.Config) *int, minV, maxV int) configField {
	return configField{
		Set: func(c *config.Config, v string) error {
			n, err := strconv.Atoi(strings.TrimSpace(v))
			if err != nil || n < minV || n > maxV {
				return invalidConfigValue(key, v, i18n.Tf("cmd.config.expected_int", minV, maxV))
			}
			*get(c) = n
			return nil
		},
		Unset: func(c *config.Config) { *get(c) = 0 },
	}
}

func boolField(key string, get func(c *config.Config) *bool) configField {
	return configField{
		Set: func(c *config.Config, v string) error {
			b, err := parseBoolValue(v)
			if err != nil {
				return invalidConfigValue(key, v, "true | false")
			}
			*get(c) = b
			return nil
		},
		Unset: func(c *config.Config) { *get(c) = false },
	}
}

// validToolVersion accepts a version such as 2.0.20 or 2.0 (optional "v").
func validToolVersion(s string) bool {
	parts := strings.Split(strings.TrimPrefix(s, "v"), ".")
	if len(parts) == 2 {
		parts = append(parts, "0")
	}
	_, ok := semver.ParseRelease(strings.Join(parts, "."))
	return ok && len(parts) == 3
}

func v5ConfigFields() map[string]configField {
	fields := map[string]configField{
		"session.attach": enumField("session.attach", func(c *config.Config) *string { return &c.Session.Attach },
			"", "auto", "iterm", "terminal", "tmux", "browser", "suspend"),
		"session.iterm_style": enumField("session.iterm_style", func(c *config.Config) *string { return &c.Session.ITermStyle },
			"", "tab", "split", "window"),
		"session.idle_sleep_minutes": intField("session.idle_sleep_minutes", func(c *config.Config) *int { return &c.Session.IdleSleepMinutes }, 1, 1440),
		"session.notify": enumField("session.notify", func(c *config.Config) *string { return &c.Session.Notify },
			"on", "on", "off"),

		"execution.runtime": enumField("execution.runtime", func(c *config.Config) *string { return &c.Execution.Runtime },
			"", "local", "container"),
		"execution.engine": enumField("execution.engine", func(c *config.Config) *string { return &c.Execution.Engine },
			"auto", "auto", "colima", "podman", "docker"),
		"execution.keep_images":      intField("execution.keep_images", func(c *config.Config) *int { return &c.Execution.KeepImages }, 1, 20),
		"execution.strict_isolation": boolField("execution.strict_isolation", func(c *config.Config) *bool { return &c.Execution.StrictIsolation }),
		"execution.tool_version": {
			Set: func(c *config.Config, v string) error {
				v = strings.TrimSpace(v)
				if !validToolVersion(v) {
					return invalidConfigValue("execution.tool_version", v, "2.0.20")
				}
				c.Execution.ToolVersion = strings.TrimPrefix(v, "v")
				return nil
			},
			Unset: func(c *config.Config) { c.Execution.ToolVersion = "" },
		},
	}
	for _, f := range limits.Fields {
		key := "limits." + f
		fields[key] = configField{
			Set: func(c *config.Config, v string) error {
				l := c.Limits
				if err := l.Set(f, v); err != nil {
					return invalidConfigValue(key, v, i18n.T("cmd.config.expected_limit."+f))
				}
				c.Limits = l
				return nil
			},
			Unset: func(c *config.Config) {
				l := c.Limits
				_ = l.Set(f, "")
				c.Limits = l
			},
		}
	}
	return fields
}

// gitlabDuration is a GitLab job timeout ("3h", "90m", "1h 30m", "2 days").
var gitlabDuration = regexp.MustCompile(`^(\d+\s*(s|sec|seconds?|m|min|minutes?|h|hr|hours?|d|days?)\s*)+$`)

// remoteConfigField handles the keys of existing remote targets and
// project assignments: remote.targets.<name>.{tag,builder,arch,timeout} and
// remote.projects.<project-id> = <target>.
func remoteConfigField(key string) (configField, bool) {
	parts := strings.Split(key, ".")
	if len(parts) < 3 || parts[0] != "remote" {
		return configField{}, false
	}
	unknownTarget := func(name string) error {
		return errors.New(i18n.Tf("cmd.config.remote_unknown_target", name))
	}
	switch {
	case parts[1] == "projects" && len(parts) == 3:
		projectID := parts[2]
		return configField{
			Set: func(c *config.Config, v string) error {
				v = strings.TrimSpace(v)
				if c.Remote.Target(v) == nil {
					return unknownTarget(v)
				}
				if c.Remote.Projects == nil {
					c.Remote.Projects = map[string]string{}
				}
				c.Remote.Projects[projectID] = v
				return nil
			},
			Unset: func(c *config.Config) { delete(c.Remote.Projects, projectID) },
		}, true
	case parts[1] == "targets" && len(parts) == 4:
		name, field := parts[2], parts[3]
		allowed := map[string][]string{"builder": {"kaniko", "dind"}, "arch": {"amd64", "arm64"}}
		var set func(t *config.RemoteTarget, v string)
		switch field {
		case "tag":
			set = func(t *config.RemoteTarget, v string) { t.Tag = v }
		case "builder":
			set = func(t *config.RemoteTarget, v string) { t.Builder = v }
		case "arch":
			set = func(t *config.RemoteTarget, v string) { t.Arch = v }
		case "timeout":
			set = func(t *config.RemoteTarget, v string) { t.Timeout = v }
		default:
			return configField{}, false
		}
		return configField{
			Set: func(c *config.Config, v string) error {
				t := c.Remote.Target(name)
				if t == nil {
					return unknownTarget(name)
				}
				v = strings.TrimSpace(v)
				switch {
				case allowed[field] != nil && !slices.Contains(allowed[field], v):
					return invalidConfigValue(key, v, strings.Join(allowed[field], " | "))
				case field == "timeout" && !gitlabDuration.MatchString(v):
					return invalidConfigValue(key, v, "3h, 90m, 1h 30m")
				case field == "tag" && v == "":
					return invalidConfigValue(key, v, "oh")
				}
				set(t, v)
				return nil
			},
			Unset: func(c *config.Config) {
				if t := c.Remote.Target(name); t != nil {
					set(t, "")
				}
			},
		}, true
	}
	return configField{}, false
}

// lookupConfigField returns the setter of a settable key.
func lookupConfigField(key string) (configField, bool) {
	if f, ok := configFieldMap[key]; ok {
		return f, true
	}
	return remoteConfigField(key)
}

// configKeyPatterns are the dynamic keys shown by `oh config list --keys`.
var configKeyPatterns = []string{
	"remote.projects.<project-id>",
	"remote.targets.<target>.arch",
	"remote.targets.<target>.builder",
	"remote.targets.<target>.tag",
	"remote.targets.<target>.timeout",
}
