package cmd

import (
	"github.com/spf13/cobra"
)

// `oh skill` checks the skill catalogue. The community registry
// (add/list/remove/search) and the former per-agent budget (`budget`) were
// disconnected in v5 (ADR-051): the skills of a session are those of the hub
// and of the team catalogue, and `oh bundle show <workflow> --budget` gives
// the budget of the real bundle.
var skillCmd = &cobra.Command{
	Use: "skill",
}

func init() {
	rootCmd.AddCommand(skillCmd)
	skillCmd.AddCommand(skillCheckCmd())
}
