package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

func runProfile(t *testing.T, args ...string) error {
	t.Helper()
	c := &cobra.Command{Use: "profile", RunE: runTeamProfile}
	f := teamProfileCmd.Flags()
	f.VisitAll(func(fl *pflag.Flag) { c.Flags().String(fl.Name, fl.DefValue, fl.Usage) })
	c.SetContext(t.Context())
	require.NoError(t, c.ParseFlags(args))
	return runTeamProfile(c, nil)
}

// Bug 12: no command changed the profile of a member; oh team profile shows it
// and changes only the fields it is given, in one commit.
func TestTeamProfileShowsAndChangesTheMemberProfile(t *testing.T) {
	_, repo, out := setupClaimTestApp(t)
	configtest.Home(t)
	i18ntest.Set(t, "en")

	require.NoError(t, runProfile(t))
	assert.Contains(t, out.String(), "Test User")
	assert.Contains(t, out.String(), "tracker_username is empty", "an empty tracker name is said, with the command")
	assert.Contains(t, out.String(), "oh team profile --tracker-username")

	out.Reset()
	require.NoError(t, runProfile(t, "--tracker-username=@benjamin.datiche"))
	assert.Contains(t, out.String(), "Profile updated")
	assert.Contains(t, out.String(), "benjamin.datiche")
	assert.NotContains(t, out.String(), "tracker_username is empty")
	m, err := repo.GetMember("testuser")
	require.NoError(t, err)
	assert.Equal(t, "benjamin.datiche", m.TrackerUsername)
	assert.Equal(t, "Test User", m.DisplayName)
	assert.Equal(t, "members: update testuser", gittest.Run(t, repo.Remote(), "log", "-1", "--format=%s", "main"))

	require.NoError(t, runProfile(t, "--display-name=Ben", "--tracker-username="))
	m, err = repo.GetMember("testuser")
	require.NoError(t, err)
	assert.Equal(t, "Ben", m.DisplayName)
	assert.Empty(t, m.TrackerUsername, "given empty, the field is cleared")

	err = runProfile(t, "--display-name= ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "display name")

	err = runProfile(t, "--team=ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No team")
}
