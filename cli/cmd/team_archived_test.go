package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

// oh team init --reconfigure on an archived team does not push to it, and does
// not suggest creating another one: the way back is oh teams restore.
func TestTeamInitReconfigureOfAnArchivedTeamPointsToRestore(t *testing.T) {
	i18ntest.Set(t, "en")
	setupTeamsTestApp(t, []config.TeamConfig{
		{ID: "old", Enabled: false, StateRepo: "git@h:o/old.git", MemberID: "me"},
	}, nil)
	cmd := &cobra.Command{Use: "init"}
	cmd.Flags().Bool("reconfigure", true, "")
	cmd.Flags().String("team", "old", "")
	cmd.SetContext(t.Context())

	err := runTeamInit(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Team old is archived")
	assert.Contains(t, err.Error(), "oh teams restore old")
}
