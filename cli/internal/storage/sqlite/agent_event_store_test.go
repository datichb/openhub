package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// agentEventFixture is a store with a project (proj-1) and a running session
// (sess-1) to attach events to.
func agentEventFixture(t *testing.T) (*Store, *AgentEventStore, time.Time) {
	t.Helper()
	s := openTestStore(t)
	now := time.Now().Truncate(time.Second)
	seedProject(t, s, "proj-1")
	require.NoError(t, NewSessionStore(s).Create(context.Background(), &domain.Session{
		ID: "sess-1", ProjectID: "proj-1", StartedAt: now, Status: domain.SessionStatusRunning,
	}))
	return s, NewAgentEventStore(s), now
}

func TestAgentEventStore_MemberID(t *testing.T) {
	ctx := context.Background()

	t.Run("an event keeps its member", func(t *testing.T) {
		_, es, now := agentEventFixture(t)
		memberID := "alice"
		require.NoError(t, es.Create(ctx, &domain.AgentEvent{
			ID: "evt-1", SessionID: "sess-1", ProjectID: "proj-1", AgentName: "developer",
			SkillsLoaded: []string{"frontend"}, StartedAt: now, Status: domain.AgentEventSuccess, MemberID: &memberID,
		}))

		events, err := es.ListBySession(ctx, "sess-1")
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.NotNil(t, events[0].MemberID)
		assert.Equal(t, "alice", *events[0].MemberID)
		assert.Equal(t, "developer", events[0].AgentName)
	})

	t.Run("an event without member stays without", func(t *testing.T) {
		_, es, now := agentEventFixture(t)
		require.NoError(t, es.Create(ctx, &domain.AgentEvent{
			ID: "evt-no-member", SessionID: "sess-1", ProjectID: "proj-1", AgentName: "reviewer",
			SkillsLoaded: []string{}, StartedAt: now, Status: domain.AgentEventSuccess,
		}))

		events, err := es.ListBySession(ctx, "sess-1")
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Nil(t, events[0].MemberID, "MemberID should be nil when not set")
	})

	t.Run("the member is set by an update", func(t *testing.T) {
		_, es, now := agentEventFixture(t)
		event := &domain.AgentEvent{
			ID: "evt-update", SessionID: "sess-1", ProjectID: "proj-1", AgentName: "developer",
			SkillsLoaded: []string{}, StartedAt: now, Status: domain.AgentEventSuccess,
		}
		require.NoError(t, es.Create(ctx, event))

		mid := "bob"
		event.MemberID = &mid
		completedAt := now.Add(5 * time.Minute)
		event.CompletedAt = &completedAt
		require.NoError(t, es.Update(ctx, event))

		events, err := es.ListBySession(ctx, "sess-1")
		require.NoError(t, err)
		require.Len(t, events, 1)
		require.NotNil(t, events[0].MemberID)
		assert.Equal(t, "bob", *events[0].MemberID)
		assert.NotNil(t, events[0].CompletedAt)
	})
}

func TestAgentEventStore_RetroTagMemberID(t *testing.T) {
	s, es, now := agentEventFixture(t)
	ctx := context.Background()

	// Create two events without member_id
	for _, id := range []string{"evt-a", "evt-b"} {
		require.NoError(t, es.Create(ctx, &domain.AgentEvent{
			ID: id, SessionID: "sess-1", ProjectID: "proj-1",
			AgentName: "dev", SkillsLoaded: []string{}, StartedAt: now,
			Status: domain.AgentEventSuccess,
		}))
	}

	// Retro-tag via raw SQL (simulates retroTagSessions)
	result, err := s.DB().ExecContext(ctx, `UPDATE agent_events SET member_id = ? WHERE member_id IS NULL`, "charlie")
	require.NoError(t, err)
	rows, _ := result.RowsAffected()
	assert.Equal(t, int64(2), rows)

	// Verify
	events, err := es.ListBySession(ctx, "sess-1")
	require.NoError(t, err)
	require.Len(t, events, 2)
	for _, e := range events {
		require.NotNil(t, e.MemberID)
		assert.Equal(t, "charlie", *e.MemberID)
	}
}
