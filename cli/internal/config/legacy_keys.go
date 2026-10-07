package config

import "github.com/spf13/viper"

// Former keys of hub.toml, still read (D19: the tool-specific names live
// here only; no migration, hub.toml is written with the neutral keys at the
// next save).
const (
	legacyDefaultProvider = "opencode.default_provider"
	legacyToolVersion     = "execution.opencode_version"
)

// LegacyKeyAliases maps the former `oh config set|get|unset` keys to the
// current ones.
var LegacyKeyAliases = map[string]string{legacyDefaultProvider: "llm.default_provider"}

// applyLegacyKeys fills the neutral settings from their former keys when
// only those are set.
func applyLegacyKeys(v *viper.Viper, c *Config) {
	if c.LLM.DefaultProvider == "" {
		c.LLM.DefaultProvider = v.GetString(legacyDefaultProvider)
	}
	if c.Execution.ToolVersion == "" {
		c.Execution.ToolVersion = v.GetString(legacyToolVersion)
	}
}
