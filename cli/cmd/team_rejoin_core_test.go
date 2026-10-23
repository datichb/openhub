package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/config/configtest"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

func TestTeamRejoinCore_Success(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	a, repo, _ := setupClaimTestApp(t)

	configtest.Home(t)

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

	configtest.Home(t)

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

	configtest.Home(t)

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

	s := sqlitetest.New(t)
	origApp := application
	application = &app.App{Sessions: sqlite.NewSessionStore(s)}
	t.Cleanup(func() { application = origApp })

	now := time.Now()
	sqlitetest.Seed(t, s).Project("p1", "/p1").
		Session(&domain.Session{ID: "s1", ProjectID: "p1", StartedAt: now, Status: domain.SessionStatusCompleted}).
		Session(&domain.Session{ID: "s2", ProjectID: "p1", StartedAt: now, Status: domain.SessionStatusRunning})

	ctx := context.Background()
	count, err := retroTagSessions(ctx, "alice")
	require.NoError(t, err)
	assert.Equal(t, 2, count, "both sessions had no member")

	var memberID string
	require.NoError(t, s.DB().QueryRowContext(ctx, `SELECT member_id FROM sessions WHERE id = 's1'`).Scan(&memberID))
	assert.Equal(t, "alice", memberID)
}

// A degraded startup has no session store: the retro-tagging says so.
func TestRetroTagSessionsWithoutDatabase(t *testing.T) {
	origApp := application
	t.Cleanup(func() { application = origApp })

	application = &app.App{}
	_, err := retroTagSessions(context.Background(), "alice")
	require.Error(t, err, "no session store")

	application = nil
	_, err = retroTagSessions(context.Background(), "alice")
	require.Error(t, err, "no application")
}
