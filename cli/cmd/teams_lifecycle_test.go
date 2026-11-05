package cmd

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

func lifecycleTeams() []config.TeamConfig {
	return []config.TeamConfig{
		{ID: "core", Enabled: true, Default: true, StateRepo: "git@x:core.git", MemberID: "me"},
		{ID: "side", Enabled: true, StateRepo: "git@x:side.git", MemberID: "me"},
	}
}

// Archiving the default team (the Teams view and oh teams archive share it)
// takes its mark away and says where the default went.
func TestArchiveTeamMovesTheDefault(t *testing.T) {
	i18ntest.Set(t, "en")
	a, _ := setupTeamsTestApp(t, lifecycleTeams(), nil)

	note, err := archiveTeam(a, "core")
	require.NoError(t, err)
	assert.Equal(t, "core was the default team: the default team is now side.", note)
	assert.False(t, a.Config.FindTeam("core").Enabled)
	assert.False(t, a.Config.FindTeam("core").Default)
	read, err := config.Read()
	require.NoError(t, err)
	assert.False(t, read.FindTeam("core").Enabled, "written to hub.toml")

	require.NoError(t, restoreTeam(a, "core"))
	assert.True(t, a.Config.FindTeam("core").Enabled)

	note, err = archiveTeam(a, "side")
	require.NoError(t, err)
	assert.Empty(t, note, "side was not the default team (core is, as the first enabled team)")

	_, err = archiveTeam(a, "ghost")
	var unknown *config.UnknownTeamError
	assert.ErrorAs(t, err, &unknown)
	assert.ErrorAs(t, restoreTeam(a, "ghost"), &unknown)
}

// Removing a team detaches its projects (the TUI used to leave them attached to
// a team that was gone) and tells which ones, and where the default went.
func TestRemoveTeamDetachesItsProjects(t *testing.T) {
	i18ntest.Set(t, "en")
	core, side := "core", "side"
	projects := []domain.Project{
		{ID: "p1", Name: "api", TeamID: &core},
		{ID: "p2", Name: "web", TeamID: &core},
		{ID: "p3", Name: "docs", TeamID: &side},
		{ID: "p4", Name: "lone"},
	}
	a, _ := setupTeamsTestApp(t, lifecycleTeams(), projects)
	ctx := context.Background()

	assert.Equal(t, []string{"api", "web"}, teamProjectNames(ctx, a, "core"))

	detached, note, err := removeTeam(ctx, a, "core")
	require.NoError(t, err)
	assert.Equal(t, []string{"api", "web"}, detached)
	assert.Equal(t, "core was the default team: the default team is now side.", note)
	assert.Nil(t, a.Config.FindTeam("core"))
	for _, p := range a.Projects.(*mockProjectStore).projects {
		switch p.Name {
		case "api", "web", "lone":
			assert.Nil(t, p.TeamID, p.Name)
		case "docs":
			require.NotNil(t, p.TeamID)
			assert.Equal(t, "side", *p.TeamID, "another team keeps its project")
		}
	}
	read, err := config.Read()
	require.NoError(t, err)
	assert.Nil(t, read.FindTeam("core"), "written to hub.toml")

	_, _, err = removeTeam(ctx, a, "core")
	var unknown *config.UnknownTeamError
	assert.ErrorAs(t, err, &unknown)
}

// oh teams remove goes through the same function: the projects are detached
// and the default team that moves is told.
func TestRunTeamsRemoveSaysWhereTheDefaultWent(t *testing.T) {
	i18ntest.Set(t, "en")
	core := "core"
	a, stdout := setupTeamsTestApp(t, lifecycleTeams(), []domain.Project{{ID: "p1", Name: "api", TeamID: &core}})

	cmd := &cobra.Command{Use: "remove"}
	addYesFlag(cmd)
	require.NoError(t, cmd.Flags().Set("yes", "true"))
	require.NoError(t, runTeamsRemove(cmd, []string{"core"}))

	assert.Contains(t, stdout.String(), "core was the default team: the default team is now side.")
	assert.Nil(t, a.Projects.(*mockProjectStore).projects[0].TeamID)
	assert.Nil(t, a.Config.FindTeam("core"))
}
