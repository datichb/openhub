package config

import (
	"fmt"
	"sort"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// ChangedOnDisk reports whether hub.toml changed (another process, an editor)
// since this Config was read from it or saved to it: a cheap test (mtime and
// size) for code that holds a Config for a long time. A Config built by hand
// has no file to compare with and a removed file is not a change: false.
func (c *Config) ChangedOnDisk() (bool, error) { return c.src.changedSince() }

// SettingChange is one setting whose value differs between two Configs.
type SettingChange struct {
	// Key is the dotted key of hub.toml (models.default,
	// provider.bedrock.aws_region, teams[0].repo…).
	Key string
	// Old and New are the values ("" = not set).
	Old, New string
}

// launchSections are the sections of hub.toml that change what a launch does:
// the model cascade, the provider, the limits, the execution, the opening of
// the session, the files given to the agents, the MCP servers, the teams.
var launchSections = []string{"models", "llm", "provider", "limits", "execution", "session", "deploy", "websearch",
	"mcp", "fanout", "remote", "tracker", "teams"}

// LaunchChanges lists the settings that touch a launch and differ between
// before and after, sorted by key.
func LaunchChanges(before, after *Config) []SettingChange {
	old, cur := launchSettings(before), launchSettings(after)
	keys := map[string]bool{}
	for k := range old {
		keys[k] = true
	}
	for k := range cur {
		keys[k] = true
	}
	var out []SettingChange
	for k := range keys {
		if old[k] != cur[k] {
			out = append(out, SettingChange{Key: k, Old: old[k], New: cur[k]})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// launchSettings flattens the launch sections of c: dotted key → value.
func launchSettings(c *Config) map[string]string {
	out := map[string]string{}
	if c == nil {
		return out
	}
	data, err := toml.Marshal(c)
	if err != nil {
		return out
	}
	var doc map[string]any
	if err := toml.Unmarshal(data, &doc); err != nil {
		return out
	}
	for _, s := range launchSections {
		if v, ok := doc[s]; ok {
			flatten(out, s, v)
		}
	}
	return out
}

func flatten(out map[string]string, key string, v any) {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			flatten(out, key+"."+k, e)
		}
	case []map[string]any:
		for i, e := range x {
			flatten(out, fmt.Sprintf("%s[%d]", key, i), e)
		}
	case []any:
		parts := make([]string, len(x))
		tables := false
		for i, e := range x {
			if m, ok := e.(map[string]any); ok {
				tables = true
				flatten(out, fmt.Sprintf("%s[%d]", key, i), m)
				continue
			}
			parts[i] = fmt.Sprint(e)
		}
		if !tables {
			out[key] = strings.Join(parts, ", ")
		}
	default:
		out[key] = fmt.Sprint(v)
	}
}
