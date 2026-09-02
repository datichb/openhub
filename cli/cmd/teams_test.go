package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
)

func TestRunTeamsDetach_Success(t *testing.T) {
	teamID := "acme"
	projects := []domain.Project{
		{ID: "p1", Name: "frontend", TeamID: &teamID},
	}
	teams := []config.TeamConfig{
		{ID: "acme", Enabled: true, StateRepo: "git@example.com:acme/ts.git", MemberID: "alice"},
	}
	_, stdout := setupTeamsTestApp(t, teams, projects)

	cmd := &cobra.Command{Use: "detach"}
	err := runTeamsDetach(cmd, []string{"frontend"})
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "frontend")
	assert.Contains(t, output, "acme")
}

func TestRunTeamsDetach_NotAttached(t *testing.T) {
	projects := []domain.Project{
		{ID: "p1", Name: "solo-project", TeamID: nil},
	}
	_, stdout := setupTeamsTestApp(t, nil, projects)

	cmd := &cobra.Command{Use: "detach"}
	err := runTeamsDetach(cmd, []string{"solo-project"})
	require.NoError(t, err)

	// Should show "not attached" warning
	assert.Contains(t, stdout.String(), "solo-project")
}

func TestRunTeamsArchive_Success(t *testing.T) {
	teams := []config.TeamConfig{
		{ID: "acme", Enabled: true, StateRepo: "git@example.com:acme/ts.git", MemberID: "alice"},
	}
	a, stdout := setupTeamsTestApp(t, teams, nil)

	cmd := &cobra.Command{Use: "archive"}
	err := runTeamsArchive(cmd, []string{"acme"})
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "acme")

	// Verify Enabled is now false
	team := a.Config.FindTeam("acme")
	require.NotNil(t, team)
	assert.False(t, team.Enabled)
}

func TestRunTeamsArchive_AlreadyArchived(t *testing.T) {
	teams := []config.TeamConfig{
		{ID: "acme", Enabled: false, StateRepo: "git@example.com:acme/ts.git", MemberID: "alice"},
	}
	_, stdout := setupTeamsTestApp(t, teams, nil)

	cmd := &cobra.Command{Use: "archive"}
	err := runTeamsArchive(cmd, []string{"acme"})
	require.NoError(t, err)

	// Should show "already archived" warning
	assert.Contains(t, stdout.String(), "acme")
}

func TestRunTeamsRestore_Success(t *testing.T) {
	teams := []config.TeamConfig{
		{ID: "acme", Enabled: false, StateRepo: "git@example.com:acme/ts.git", MemberID: "alice"},
	}
	a, stdout := setupTeamsTestApp(t, teams, nil)

	cmd := &cobra.Command{Use: "restore"}
	err := runTeamsRestore(cmd, []string{"acme"})
	require.NoError(t, err)

	assert.Contains(t, stdout.String(), "acme")

	// Verify Enabled is now true
	team := a.Config.FindTeam("acme")
	require.NotNil(t, team)
	assert.True(t, team.Enabled)
}
