package cmd

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func TestTeamRejoinCore_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	a, repo, _ := setupClaimTestApp(t)

	// Override HOME for config.Save()
	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	// Clear teams to simulate a fresh install
	a.Config.Teams = nil

	result, err := teamRejoinCore(context.Background(), a, teamRejoinParams{
		StateRepo: repo.Remote(),
		StatePath: repo.Path(),
		MemberID:  "testuser",
	})

	require.NoError(t, err)
	assert.Equal(t, "testuser", result.Member.ID)
	assert.Equal(t, "Test User", result.Member.DisplayName)
	assert.NotEmpty(t, result.TeamID)

	// Verify that the team config was written
	require.Len(t, a.Config.Teams, 1)
	assert.Equal(t, "testuser", a.Config.Teams[0].MemberID)
	assert.True(t, a.Config.Teams[0].Enabled)
}

func TestTeamRejoinCore_MemberNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	a, repo, _ := setupClaimTestApp(t)

	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	_, err := teamRejoinCore(context.Background(), a, teamRejoinParams{
		StateRepo: repo.Remote(),
		StatePath: repo.Path(),
		MemberID:  "nonexistent",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "not found")
}

func TestTeamRejoinCore_UpdatesExistingTeam(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	a, repo, _ := setupClaimTestApp(t)

	origHome := os.Getenv("HOME")
	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	// Pre-populate with an existing team (same repo, different member)
	a.Config.Teams = []config.TeamConfig{
		{
			ID:        "test-team",
			Enabled:   true,
			StateRepo: repo.Remote(),
			StatePath: repo.Path(),
			MemberID:  "old-member",
		},
	}

	result, err := teamRejoinCore(context.Background(), a, teamRejoinParams{
		StateRepo: repo.Remote(),
		StatePath: repo.Path(),
		MemberID:  "testuser",
	})

	require.NoError(t, err)
	assert.Equal(t, "testuser", result.Member.ID)

	// Should update the existing team, not add a new one
	require.Len(t, a.Config.Teams, 1)
	assert.Equal(t, "testuser", a.Config.Teams[0].MemberID)
}

func TestListTeamMembers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	_, repo, _ := setupClaimTestApp(t)

	_, members, _, err := listTeamMembers(context.Background(), repo.Remote(), repo.Path())
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, "testuser", members[0].ID)
	assert.Equal(t, "Test User", members[0].DisplayName)
}

func TestRetroTagSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Open a test SQLite store
	tmpDB := t.TempDir()
	s, err := sqlite.Open(tmpDB + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	// Set package-level store
	origStore := store
	store = s
	t.Cleanup(func() { store = origStore })

	ctx := context.Background()
	db := s.DB()

	// Create project + sessions without member_id
	_, err = db.ExecContext(ctx, `INSERT INTO projects (id, name, path, status) VALUES ('p1', 'P1', '/p1', 'active')`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO sessions (id, project_id, started_at, status) VALUES ('s1', 'p1', datetime('now'), 'completed')`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO sessions (id, project_id, started_at, status) VALUES ('s2', 'p1', datetime('now'), 'running')`)
	require.NoError(t, err)

	count, err := retroTagSessions(ctx, "alice")
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify
	var memberID string
	err = db.QueryRowContext(ctx, `SELECT member_id FROM sessions WHERE id = 's1'`).Scan(&memberID)
	require.NoError(t, err)
	assert.Equal(t, "alice", memberID)
}
