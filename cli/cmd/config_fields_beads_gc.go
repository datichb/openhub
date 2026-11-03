package cmd

import (
	"strconv"

	"github.com/datichb/openhub/cli/internal/config"
)

// Settings of the robustness track: [beads] (N17) and [gc] (N16).
func init() {
	configFieldMap["beads.zero_impact"] = enumField("beads.zero_impact",
		func(c *config.Config) *string { return &c.Beads.ZeroImpact }, zeroImpactAsk, zeroImpactAsk, zeroImpactAuto, zeroImpactOff)
	configDefaults["beads.zero_impact"] = zeroImpactAsk

	enabled := boolField("gc.enabled", func(c *config.Config) *bool { return &c.GC.Enabled })
	enabled.Unset = func(c *config.Config) { c.GC.Enabled = true } // default on
	configFieldMap["gc.enabled"] = enabled
	configDefaults["gc.enabled"] = "true"
	for _, f := range []struct {
		key      string
		get      func(c *config.Config) *int
		def, max int
	}{
		{"gc.every_hours", func(c *config.Config) *int { return &c.GC.EveryHours }, config.DefaultGCEveryHours, 24 * 30},
		{"gc.session_days", func(c *config.Config) *int { return &c.GC.SessionDays }, config.DefaultGCSessionDays, 3650},
		{"gc.history_days", func(c *config.Config) *int { return &c.GC.HistoryDays }, config.DefaultGCHistoryDays, 3650},
		{"gc.usage_days", func(c *config.Config) *int { return &c.GC.UsageDays }, config.DefaultGCUsageDays, 3650},
		{"gc.bundle_grace_hours", func(c *config.Config) *int { return &c.GC.BundleGraceHours }, config.DefaultGCBundleGraceHours, 24 * 365},
		{"gc.log_max_mb", func(c *config.Config) *int { return &c.GC.LogMaxMB }, config.DefaultGCLogMaxMB, 10240},
		{"gc.log_keep", func(c *config.Config) *int { return &c.GC.LogKeep }, config.DefaultGCLogKeep, 100},
	} {
		configFieldMap[f.key] = intField(f.key, f.get, 1, f.max)
		configDefaults[f.key] = strconv.Itoa(f.def)
	}
}
