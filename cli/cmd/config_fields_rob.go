package cmd

import "github.com/datichb/openhub/cli/internal/config"

// Settings of the robustness track (N17): [beads].
func init() {
	configFieldMap["beads.zero_impact"] = enumField("beads.zero_impact",
		func(c *config.Config) *string { return &c.Beads.ZeroImpact }, zeroImpactAsk, zeroImpactAsk, zeroImpactAuto, zeroImpactOff)
	configDefaults["beads.zero_impact"] = zeroImpactAsk
}
