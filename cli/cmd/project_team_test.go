package cmd

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
)

func TestTeamPickers(t *testing.T) {
	m := newMultiTeam(t, false)
	m.a.Config.Teams = append(m.a.Config.Teams,
		config.TeamConfig{ID: "solo", Enabled: true, Solo: true, StatePath: "/s", MemberID: "me"},
		config.TeamConfig{ID: "old", Enabled: false, StateRepo: "git@x:org/old.git", MemberID: "me"},
		config.TeamConfig{ID: "c", Enabled: true, StateRepo: "git@x:org/c.git", MemberID: "carol", Default: true})

	// Ctrl+T: no solo space, no archived team, the default team first.
	var ids []string
	for _, o := range teamSelectorOptions(m.a.Config) {
		ids = append(ids, o.Value)
	}
	assert.Equal(t, []string{"c", "a", "b"}, ids)

	// The project wizards offer the same teams, and the default is chosen.
	teams := attachableTeams(m.a.Config)
	require.Len(t, teams, 3)
	assert.Equal(t, "c", teams[0].ID)
}

func TestProjectSetTeam(t *testing.T) {
	m := newMultiTeam(t, false)
	m.a.Config.Teams = append(m.a.Config.Teams,
		config.TeamConfig{ID: "old", Enabled: false, StateRepo: "git@x:org/old.git", MemberID: "me"})
	ctx := context.Background()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)

	require.NoError(t, runProjectSetTeam(cmd, []string{"proj-none", "b"}))
	p, _ := m.a.Projects.Get(ctx, "pn")
	require.NotNil(t, p.TeamID)
	assert.Equal(t, "b", *p.TeamID)

	require.NoError(t, runProjectSetTeam(cmd, []string{"proj-a", "b"}))
	p, _ = m.a.Projects.Get(ctx, "pa")
	assert.Equal(t, "b", *p.TeamID, "a project moves to another team")
	assert.Contains(t, m.stdout.String(), "a", "the claims left in the old team are mentioned")

	err := runProjectSetTeam(cmd, []string{"proj-a", "zzz"})
	require.Error(t, err)
	err = runProjectSetTeam(cmd, []string{"proj-a", "old"})
	require.Error(t, err, "an archived team is refused")
	assert.Contains(t, err.Error(), "oh teams restore old")
	p, _ = m.a.Projects.Get(ctx, "pa")
	assert.Equal(t, "b", *p.TeamID, "a refusal changes nothing")
	require.Error(t, runProjectSetTeam(cmd, []string{"nope", "a"}))
}

func TestProjectAddMinimal_WithTeam(t *testing.T) {
	m := newMultiTeam(t, false)
	ctx := context.Background()
	dir := t.TempDir()
	require.NoError(t, doCreateProjectMinimal(ctx, m.a, "fresh", dir, "go", "b"))
	p, err := m.a.Projects.GetByName(ctx, "fresh")
	require.NoError(t, err)
	require.NotNil(t, p.TeamID)
	assert.Equal(t, "b", *p.TeamID)

	require.NoError(t, doCreateProjectMinimal(ctx, m.a, "plain", t.TempDir(), "go", ""))
	p, _ = m.a.Projects.GetByName(ctx, "plain")
	assert.Nil(t, p.TeamID)
}
