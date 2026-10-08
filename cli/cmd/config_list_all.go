package cmd

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/datichb/openhub/cli/internal/config"
)

// configDefaults are the values oh uses when a settable key is not in
// hub.toml ("" = no value: the feature is off or the workflow decides).
var configDefaults = map[string]string{
	"cli.language":               "en",
	"worktree.auto_cleanup":      "true",
	"websearch.enabled":          "false",
	"session.attach":             "auto",
	"session.iterm_style":        "tab",
	"session.idle_sleep_minutes": "5",
	"session.notify":             "on",
	"execution.engine":           "auto",
	"execution.keep_images":      strconv.Itoa(config.DefaultKeepImages),
	"execution.strict_isolation": "false",
	"mcp.figma.token_key":        config.DefaultFigmaTokenKey,
	"mcp.gitlab.token_key":       config.DefaultGitLabTokenKey,
	"mcp.jira.token_key":         config.DefaultJiraTokenKey,
	"mcp.gslides.token_key":      config.DefaultGslidesTokenKey,
	"mcp.figma.enabled":          "false",
	"mcp.gitlab.enabled":         "false",
	"mcp.gitlab.write_enabled":   "false",
	"mcp.jira.enabled":           "false",
	"mcp.gslides.enabled":        "false",
}

// configListRow is one settable key of `oh config list --all`.
type configListRow struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Origin string `json:"origin"` // hub.toml | default
}

// configListAll lists every settable key with its value in hub.toml, else
// the default of oh.
func configListAll(cfg *config.Config) []configListRow {
	set := cfg.ToMap()
	keys := configFieldKeys()
	sort.Strings(keys)
	rows := make([]configListRow, 0, len(keys))
	for _, k := range keys {
		if v, ok := set[k]; ok {
			rows = append(rows, configListRow{Key: k, Value: fmt.Sprint(v), Origin: "hub.toml"})
			continue
		}
		rows = append(rows, configListRow{Key: k, Value: configDefaults[k], Origin: "default"})
	}
	return rows
}
