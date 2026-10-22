package config

import "strings"

// Former keys of hub.toml, still read (D19: the tool-specific names live
// here only; no migration, hub.toml is written with the neutral keys at the
// next save). Reading one is announced once per process (notices.go) and
// reported by oh doctor; the support ends in v6.
const (
	legacyDefaultProvider = "opencode.default_provider"
	legacyToolVersion     = "execution.opencode_version"
)

// LegacyKeyAliases maps the former `oh config set|get|unset` keys to the
// current ones.
var LegacyKeyAliases = map[string]string{legacyDefaultProvider: "llm.default_provider"}

// legacyReplacement is the current key of each former hub.toml key.
var legacyReplacement = map[string]string{
	legacyDefaultProvider: "llm.default_provider",
	legacyToolVersion:     "execution.tool_version",
}

// historicalKeys are the keys and sections of hub.toml that earlier versions
// wrote and nothing reads any more (the managed install of the tool, the
// native agents switch, the former workflow overrides, which `oh workflow
// migrate` reads from the file itself). They are ignored without a warning:
// a hub.toml of an earlier version stays quiet.
var historicalKeys = []string{
	"opencode",
	"deploy.disable_native_agents",
	"workflow",
}

// isHistoricalKey reports whether a key (dotted, any case) is a historical
// key or lies under a historical section.
func isHistoricalKey(key string) bool {
	k := strings.ToLower(key)
	for _, h := range historicalKeys {
		if k == h || strings.HasPrefix(k, h+".") {
			return true
		}
	}
	return false
}

// legacyFile reads the former keys apart from the configuration: a pointer
// tells a key that is set (even empty) from an absent one.
type legacyFile struct {
	Tool struct {
		DefaultProvider *string `toml:"default_provider"`
	} `toml:"opencode"`
	Execution struct {
		ToolVersion *string `toml:"opencode_version"`
	} `toml:"execution"`
}

// applyLegacyKeys fills the neutral settings from their former keys when
// only those are set, and returns the former keys found in the file.
func applyLegacyKeys(l legacyFile, c *Config) []string {
	var found []string
	if p := l.Tool.DefaultProvider; p != nil {
		found = append(found, legacyDefaultProvider)
		if c.LLM.DefaultProvider == "" {
			c.LLM.DefaultProvider = *p
		}
	}
	if p := l.Execution.ToolVersion; p != nil {
		found = append(found, legacyToolVersion)
		if c.Execution.ToolVersion == "" {
			c.Execution.ToolVersion = *p
		}
	}
	return found
}
