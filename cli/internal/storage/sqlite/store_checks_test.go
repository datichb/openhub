package sqlite

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestStore_QuickCheckAndCountProjects(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	require.NoError(t, s.QuickCheck(), "a fresh database is healthy")
	n, err := s.CountProjects(ctx)
	require.NoError(t, err)
	assert.Zero(t, n, "no project yet")

	seedProject(t, s, "a")
	seedProject(t, s, "b")
	n, err = s.CountProjects(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, n, "both projects are counted")

	require.NoError(t, s.Close())
	assert.Error(t, s.QuickCheck(), "a closed database cannot be checked")
	_, err = s.CountProjects(ctx)
	assert.Error(t, err, "nor counted")
}

func TestStore_QuickCheckReportsCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oh.db")
	s, err := Open(path)
	require.NoError(t, err)
	for i := 0; i < 200; i++ {
		seedProject(t, s, "p"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	require.NoError(t, s.Close())

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Greater(t, len(raw), 8192, "the database has several pages")
	for i := 4096; i < len(raw) && i < 4096+2048; i++ {
		raw[i] = 0xFF
	}
	require.NoError(t, os.WriteFile(path, raw, 0o600))

	s2, err := OpenRaw(path)
	if err != nil {
		t.Skipf("the damage is detected on open already: %v", err)
	}
	t.Cleanup(func() { s2.Close() })
	assert.Error(t, s2.QuickCheck(), "the damaged page is reported")
}

func TestSessionStore_TagUnownedMember(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	seedProject(t, s, "p1")
	ss := NewSessionStore(s)
	es := NewAgentEventStore(s)
	now := time.Now().Truncate(time.Second)
	bob := "bob"
	for _, sess := range []*domain.Session{
		{ID: "own", ProjectID: "p1", StartedAt: now, Status: domain.SessionStatusCompleted, MemberID: &bob},
		{ID: "free1", ProjectID: "p1", StartedAt: now, Status: domain.SessionStatusCompleted},
		{ID: "free2", ProjectID: "p1", StartedAt: now, Status: domain.SessionStatusRunning},
	} {
		require.NoError(t, ss.Create(ctx, sess))
	}
	for _, id := range []string{"free1", "own"} {
		require.NoError(t, es.Create(ctx, &domain.AgentEvent{ID: "ev-" + id, SessionID: id, ProjectID: "p1", AgentName: "dev", StartedAt: now}))
	}

	n, err := ss.TagUnownedMember(ctx, "alice")
	require.NoError(t, err)
	assert.Equal(t, 2, n, "the two sessions without member are tagged")

	member := func(table, id string) string {
		var m string
		require.NoError(t, s.DB().QueryRowContext(ctx, `SELECT COALESCE(member_id, '') FROM `+table+` WHERE id = ?`, id).Scan(&m))
		return m
	}
	assert.Equal(t, "alice", member("sessions", "free1"))
	assert.Equal(t, "alice", member("sessions", "free2"))
	assert.Equal(t, "bob", member("sessions", "own"), "a session that has a member keeps it")
	assert.Equal(t, "alice", member("agent_events", "ev-free1"), "the events follow")
	assert.Equal(t, "alice", member("agent_events", "ev-own"), "an event without member is tagged whatever its session")

	n, err = ss.TagUnownedMember(ctx, "carol")
	require.NoError(t, err)
	assert.Zero(t, n, "nothing is left to tag: replayable")
	assert.Equal(t, "alice", member("sessions", "free1"), "a second run does not take over")
}

func TestSessionStore_TagUnownedMemberErrors(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	ss := NewSessionStore(s)

	_, err := s.DB().Exec(`DROP TABLE agent_events`)
	require.NoError(t, err)
	n, err := ss.TagUnownedMember(ctx, "alice")
	require.ErrorIs(t, err, domain.ErrTagAgentEvents, "the failure on the events is told apart")
	assert.Zero(t, n)

	require.NoError(t, s.Close())
	_, err = ss.TagUnownedMember(ctx, "alice")
	require.Error(t, err)
	assert.NotErrorIs(t, err, domain.ErrTagAgentEvents, "a failure on the sessions is not the one of the events")
}

func TestProjectStore_MigrateTeamConfigToTeamID(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	ps := NewProjectStore(s)
	for _, id := range []string{"inherit", "empty", "custom", "disabled", "unknown", "broken", "plain", "done"} {
		seedProject(t, s, id)
	}
	for id, cfg := range map[string]string{
		"inherit":  `{"mode":"inherit"}`,
		"empty":    `{}`,
		"custom":   `{"mode":"custom"}`,
		"disabled": `{"mode":"disabled"}`,
		"unknown":  `{"mode":"whatever"}`,
		"broken":   `{not json`,
		"done":     `{"mode":"inherit"}`,
	} {
		_, err := s.DB().Exec(`UPDATE projects SET team_config = ? WHERE id = ?`, cfg, id)
		require.NoError(t, err)
	}
	_, err := s.DB().Exec(`UPDATE projects SET team_id = 'old' WHERE id = 'done'`)
	require.NoError(t, err)

	n, err := ps.MigrateTeamConfigToTeamID(ctx, "")
	require.NoError(t, err)
	assert.Zero(t, n, "without a team at the hub level nothing is migrated")

	n, err = ps.MigrateTeamConfigToTeamID(ctx, "team-1")
	require.NoError(t, err)
	assert.Equal(t, 3, n, "inherit, empty and custom are attached to the active team")

	teamID := func(id string) string {
		var v string
		require.NoError(t, s.DB().QueryRowContext(ctx, `SELECT COALESCE(team_id, '') FROM projects WHERE id = ?`, id).Scan(&v))
		return v
	}
	for _, id := range []string{"inherit", "empty", "custom"} {
		assert.Equal(t, "team-1", teamID(id), id)
	}
	for _, id := range []string{"disabled", "unknown", "broken", "plain"} {
		assert.Empty(t, teamID(id), "%s is left without team", id)
	}
	assert.Equal(t, "old", teamID("done"), "a project that has a team keeps it")

	n, err = ps.MigrateTeamConfigToTeamID(ctx, "team-1")
	require.NoError(t, err)
	assert.Zero(t, n, "replayable")
}
