package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestAgentEventStore_CreateWithMemberID(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ss := NewSessionStore(s)
	es := NewAgentEventStore(s)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	require.NoError(t, ps.Create(ctx, &domain.Project{
		ID: "proj-1", Name: "P1", Path: "/p1", Status: domain.ProjectStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, ss.Create(ctx, &domain.Session{
		ID: "sess-1", ProjectID: "proj-1", StartedAt: now, Status: domain.SessionStatusRunning,
	}))

	memberID := "alice"
	event := &domain.AgentEvent{
		ID:           "evt-1",
		SessionID:    "sess-1",
		ProjectID:    "proj-1",
		AgentName:    "developer",
		SkillsLoaded: []string{"frontend"},
		StartedAt:    now,
		Status:       domain.AgentEventSuccess,
		MemberID:     &memberID,
	}
	require.NoError(t, es.Create(ctx, event))

	events, err := es.ListBySession(ctx, "sess-1")
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.NotNil(t, events[0].MemberID)
	assert.Equal(t, "alice", *events[0].MemberID)
	assert.Equal(t, "developer", events[0].AgentName)
}

func TestAgentEventStore_CreateWithoutMemberID(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ss := NewSessionStore(s)
	es := NewAgentEventStore(s)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	require.NoError(t, ps.Create(ctx, &domain.Project{
		ID: "proj-1", Name: "P1", Path: "/p1", Status: domain.ProjectStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, ss.Create(ctx, &domain.Session{
		ID: "sess-1", ProjectID: "proj-1", StartedAt: now, Status: domain.SessionStatusRunning,
	}))

	event := &domain.AgentEvent{
		ID:           "evt-no-member",
		SessionID:    "sess-1",
		ProjectID:    "proj-1",
		AgentName:    "reviewer",
		SkillsLoaded: []string{},
		StartedAt:    now,
		Status:       domain.AgentEventSuccess,
	}
	require.NoError(t, es.Create(ctx, event))

	events, err := es.ListBySession(ctx, "sess-1")
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Nil(t, events[0].MemberID, "MemberID should be nil when not set")
}

func TestAgentEventStore_UpdateMemberID(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ss := NewSessionStore(s)
	es := NewAgentEventStore(s)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	require.NoError(t, ps.Create(ctx, &domain.Project{
		ID: "proj-1", Name: "P1", Path: "/p1", Status: domain.ProjectStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, ss.Create(ctx, &domain.Session{
		ID: "sess-1", ProjectID: "proj-1", StartedAt: now, Status: domain.SessionStatusRunning,
	}))

	// Create without member_id
	event := &domain.AgentEvent{
		ID:           "evt-update",
		SessionID:    "sess-1",
		ProjectID:    "proj-1",
		AgentName:    "developer",
		SkillsLoaded: []string{},
		StartedAt:    now,
		Status:       domain.AgentEventSuccess,
	}
	require.NoError(t, es.Create(ctx, event))

	// Update with member_id
	mid := "bob"
	event.MemberID = &mid
	completedAt := now.Add(5 * time.Minute)
	event.CompletedAt = &completedAt
	require.NoError(t, es.Update(ctx, event))

	// Verify
	events, err := es.ListBySession(ctx, "sess-1")
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.NotNil(t, events[0].MemberID)
	assert.Equal(t, "bob", *events[0].MemberID)
	assert.NotNil(t, events[0].CompletedAt)
}

func TestAgentEventStore_RetroTagMemberID(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ss := NewSessionStore(s)
	es := NewAgentEventStore(s)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	require.NoError(t, ps.Create(ctx, &domain.Project{
		ID: "proj-1", Name: "P1", Path: "/p1", Status: domain.ProjectStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}))
	require.NoError(t, ss.Create(ctx, &domain.Session{
		ID: "sess-1", ProjectID: "proj-1", StartedAt: now, Status: domain.SessionStatusRunning,
	}))

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
