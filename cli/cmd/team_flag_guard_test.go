package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// teamFlagTree is `oh team` with its pre-run (the one that runs before init and
// rejoin), and the two commands that must not ignore --team in silence.
func teamFlagTree(ran *[]string, args ...string) *cobra.Command {
	parent := &cobra.Command{Use: "team", PersistentPreRunE: teamPreRunE, SilenceUsage: true, SilenceErrors: true}
	addTeamFlag(parent)
	record := func(name string) func(*cobra.Command, []string) error {
		return func(*cobra.Command, []string) error { *ran = append(*ran, name); return nil }
	}
	init := &cobra.Command{Use: "init", RunE: record("init")}
	init.Flags().Bool("reconfigure", false, "")
	rejoin := &cobra.Command{Use: "rejoin", RunE: record("rejoin")}
	rejoin.Flags().String("repo", "", "")
	parent.AddCommand(init, rejoin)
	parent.SetArgs(args)
	return parent
}

// --team names the team of --reconfigure and nothing else: `oh team init`
// creates a new team, `oh team rejoin` joins the team of --repo; both used to
// ignore --team in silence.
func TestTeamFlagIsRefusedWhereItWouldBeIgnored(t *testing.T) {
	i18ntest.Set(t, "en")
	cases := []struct {
		name    string
		args    []string
		ran     string
		refuses string
	}{
		{"init creates a new team", []string{"init", "--team", "a"}, "", "--reconfigure"},
		{"init with an empty --team", []string{"init", "--team="}, "", "--reconfigure"},
		{"init --reconfigure uses it", []string{"init", "--reconfigure", "--team", "a"}, "init", ""},
		{"init without it", []string{"init"}, "init", ""},
		{"rejoin takes the team of the repository", []string{"rejoin", "--repo", "git@x:o/a.git", "--team", "a"}, "", "--repo"},
		{"rejoin without it", []string{"rejoin", "--repo", "git@x:o/a.git"}, "rejoin", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ran []string
			err := teamFlagTree(&ran, tc.args...).Execute()
			if tc.refuses != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.refuses)
				assert.Empty(t, ran, "the command does not run")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, []string{tc.ran}, ran)
		})
	}
}
