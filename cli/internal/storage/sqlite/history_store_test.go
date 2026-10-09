package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// N16: the history past its retention is counted then deleted; the rest
// (recent rows, sessions not stopped) stays.
func TestHistoryStore(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	ss := NewSessionStore(s)
	old := time.Now().Add(-200 * 24 * time.Hour)
	recent := time.Now().Add(-time.Hour)
	_, err := s.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p', 'p', '/p')`)
	require.NoError(t, err)
	for _, c := range []struct {
		id    string
		state domain.RunState
		ended time.Time
	}{{"old-stopped", domain.RunStopped, old}, {"old-completed", domain.RunCompleted, old}, {"recent-stopped", domain.RunStopped, recent}} {
		e := c.ended
		require.NoError(t, ss.Create(ctx, &domain.Session{ID: c.id, ProjectID: "p", State: c.state, StartedAt: old, EndedAt: &e, Status: domain.SessionStatusCompleted}))
	}
	for i, at := range []time.Time{old, old, recent} {
		_, err := s.DB().Exec(`INSERT INTO agent_events (id, session_id, project_id, agent_name, started_at) VALUES (?, 's', 'p', 'a', ?)`, "ev"+string(rune('0'+i)), at)
		require.NoError(t, err)
	}
	_, err = s.DB().Exec(`INSERT INTO pending_decisions (id, session_id, kind, resolved_at) VALUES ('d-old', 's', 'k', ?), ('d-open', 's', 'k', NULL), ('d-new', 's', 'k', ?)`, old, recent)
	require.NoError(t, err)
	_, err = s.DB().Exec(`INSERT INTO usage_sessions (day, session_id) VALUES ('2024-01-01', 'a'), ('2099-01-01', 'b');
INSERT INTO usage_proxy (day, group_key) VALUES ('2024-01-01', 'g');
INSERT INTO budget_extra (scope, day, extra_usd) VALUES ('s', '2024-01-01', 1), ('global', '', 2)`)
	require.NoError(t, err)

	h := NewHistoryStore(s)
	history, usage := time.Now().Add(-180*24*time.Hour), time.Now().Add(-400*24*time.Hour)
	want := HistoryRows{Sessions: 1, AgentEvents: 2, Decisions: 1, UsageSessions: 1, UsageProxy: 1, BudgetExtra: 1}
	n, err := h.Count(ctx, history, usage)
	require.NoError(t, err)
	assert.Equal(t, want, n)
	n, err = h.Delete(ctx, history, usage)
	require.NoError(t, err)
	assert.Equal(t, want, n)
	n, err = h.Count(ctx, history, usage)
	require.NoError(t, err)
	assert.Equal(t, HistoryRows{}, n)
	_, err = ss.Get(ctx, "old-completed")
	assert.NoError(t, err, "a completed session can be resumed: kept")
	_, err = ss.Get(ctx, "old-stopped")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// v43: the worktree created by oh is kept on the session.
func TestSessionStore_Worktree(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	_, err := s.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p', 'p', '/p')`)
	require.NoError(t, err)
	ss := NewSessionStore(s)
	require.NoError(t, ss.Create(ctx, &domain.Session{ID: "s1", ProjectID: "p", WorktreeAuto: true, WorktreeBranch: "feat/x"}))
	got, err := ss.Get(ctx, "s1")
	require.NoError(t, err)
	assert.True(t, got.WorktreeAuto)
	assert.Equal(t, "feat/x", got.WorktreeBranch)
	got.WorktreeAuto = false
	require.NoError(t, ss.Update(ctx, got))
	got, err = ss.Get(ctx, "s1")
	require.NoError(t, err)
	assert.False(t, got.WorktreeAuto)
}
