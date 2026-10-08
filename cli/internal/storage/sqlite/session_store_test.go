package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestSessionStore_CRUD(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ss := NewSessionStore(s)
	ctx := context.Background()

	// Create a project first (FK constraint)
	now := time.Now().Truncate(time.Second)
	require.NoError(t, ps.Create(ctx, &domain.Project{
		ID: "proj-1", Name: "P1", Path: "/p1", Status: domain.ProjectStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}))

	// Create session
	session := &domain.Session{
		ID:        "sess-1",
		ProjectID: "proj-1",
		StartedAt: now,
		Status:    domain.SessionStatusRunning,
		Provider:  "bedrock",
		Model:     "claude-opus-4",
	}
	require.NoError(t, ss.Create(ctx, session))

	// Get
	got, err := ss.Get(ctx, "sess-1")
	require.NoError(t, err)
	assert.Equal(t, "sess-1", got.ID)
	assert.Equal(t, "proj-1", got.ProjectID)
	assert.Equal(t, domain.SessionStatusRunning, got.Status)
	assert.Equal(t, "bedrock", got.Provider)
	assert.Nil(t, got.EndedAt)

	// List by project
	sessions, err := ss.List(ctx, "proj-1")
	require.NoError(t, err)
	assert.Len(t, sessions, 1)

	// Update (complete session)
	endTime := now.Add(30 * time.Minute)
	got.EndedAt = &endTime
	got.Status = domain.SessionStatusCompleted
	got.TokensIn = 5000
	got.TokensOut = 2000
	require.NoError(t, ss.Update(ctx, got))

	updated, err := ss.Get(ctx, "sess-1")
	require.NoError(t, err)
	assert.Equal(t, domain.SessionStatusCompleted, updated.Status)
	assert.NotNil(t, updated.EndedAt)
	assert.Equal(t, int64(5000), updated.TokensIn)
	assert.Equal(t, int64(2000), updated.TokensOut)
}

func TestSessionStore_GetNotFound(t *testing.T) {
	s := openTestStore(t)
	ss := NewSessionStore(s)
	ctx := context.Background()

	_, err := ss.Get(ctx, "nonexistent")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSessionStore_ListAll(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ss := NewSessionStore(s)
	ctx := context.Background()

	now := time.Now()
	require.NoError(t, ps.Create(ctx, &domain.Project{ID: "proj-1", Name: "P1", Path: "/p1", Status: domain.ProjectStatusActive, CreatedAt: now, UpdatedAt: now}))
	require.NoError(t, ps.Create(ctx, &domain.Project{ID: "proj-2", Name: "P2", Path: "/p2", Status: domain.ProjectStatusActive, CreatedAt: now, UpdatedAt: now}))

	require.NoError(t, ss.Create(ctx, &domain.Session{ID: "s1", ProjectID: "proj-1", StartedAt: now, Status: domain.SessionStatusCompleted}))
	require.NoError(t, ss.Create(ctx, &domain.Session{ID: "s2", ProjectID: "proj-1", StartedAt: now, Status: domain.SessionStatusRunning}))
	require.NoError(t, ss.Create(ctx, &domain.Session{ID: "s3", ProjectID: "proj-2", StartedAt: now, Status: domain.SessionStatusCompleted}))

	// List all
	all, err := ss.List(ctx, "")
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// List by project
	proj1, err := ss.List(ctx, "proj-1")
	require.NoError(t, err)
	assert.Len(t, proj1, 2)
}

func TestSessionStore_MemberID(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ss := NewSessionStore(s)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	require.NoError(t, ps.Create(ctx, &domain.Project{
		ID: "proj-1", Name: "P1", Path: "/p1", Status: domain.ProjectStatusActive,
		CreatedAt: now, UpdatedAt: now,
	}))

	// Create session without member_id
	require.NoError(t, ss.Create(ctx, &domain.Session{
		ID: "s-no-member", ProjectID: "proj-1", StartedAt: now, Status: domain.SessionStatusRunning,
	}))
	got, err := ss.Get(ctx, "s-no-member")
	require.NoError(t, err)
	assert.Nil(t, got.MemberID, "MemberID should be nil when not set")

	// Create session with member_id
	mid := "alice"
	require.NoError(t, ss.Create(ctx, &domain.Session{
		ID: "s-with-member", ProjectID: "proj-1", StartedAt: now, Status: domain.SessionStatusRunning,
		MemberID: &mid,
	}))
	got2, err := ss.Get(ctx, "s-with-member")
	require.NoError(t, err)
	require.NotNil(t, got2.MemberID)
	assert.Equal(t, "alice", *got2.MemberID)

	// Update member_id
	newMid := "bob"
	got2.MemberID = &newMid
	require.NoError(t, ss.Update(ctx, got2))
	got3, err := ss.Get(ctx, "s-with-member")
	require.NoError(t, err)
	require.NotNil(t, got3.MemberID)
	assert.Equal(t, "bob", *got3.MemberID)

	// Retro-tag: update NULL member_id via raw SQL
	_, err = s.DB().ExecContext(ctx, `UPDATE sessions SET member_id = ? WHERE member_id IS NULL`, "charlie")
	require.NoError(t, err)
	got4, err := ss.Get(ctx, "s-no-member")
	require.NoError(t, err)
	require.NotNil(t, got4.MemberID)
	assert.Equal(t, "charlie", *got4.MemberID)
}

// v42: the starting point and the tickets of a session are kept (A23, A31).
func TestSessionStore_StartRefAndTickets(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	require.NoError(t, NewProjectStore(s).Create(ctx, &domain.Project{ID: "p", Name: "P", Path: "/p", Status: domain.ProjectStatusActive, CreatedAt: now, UpdatedAt: now}))
	ss := NewSessionStore(s)
	require.NoError(t, ss.Create(ctx, &domain.Session{ID: "s1", ProjectID: "p", StartedAt: now, Status: domain.SessionStatusRunning,
		StartRef: "abc123", Tickets: []string{"pt-1", "pt-2"}}))
	got, err := ss.Get(ctx, "s1")
	require.NoError(t, err)
	assert.Equal(t, "abc123", got.StartRef)
	assert.Equal(t, []string{"pt-1", "pt-2"}, got.Tickets)
	got.Tickets = nil
	require.NoError(t, ss.Update(ctx, got))
	got, err = ss.Get(ctx, "s1")
	require.NoError(t, err)
	assert.Empty(t, got.Tickets)
	assert.Equal(t, "abc123", got.StartRef)
}

// v42 (A21): the sessions recorded before show their total cost with their
// subagent sessions (usage ledger), as the budget counted it.
func TestMigrationV42SessionTotals(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	require.NoError(t, NewProjectStore(s).Create(ctx, &domain.Project{ID: "p", Name: "P", Path: "/p", Status: domain.ProjectStatusActive, CreatedAt: now, UpdatedAt: now}))
	ss := NewSessionStore(s)
	for _, id := range []string{"ticket", "alone"} {
		require.NoError(t, ss.Create(ctx, &domain.Session{ID: id, ProjectID: "p", StartedAt: now, Status: domain.SessionStatusCompleted, Cost: 0.88, TokensIn: 10}))
	}
	us := NewUsageStore(s)
	for _, u := range []domain.SessionUsage{
		{Day: "2026-10-07", SessionID: "ticket", RootID: "ticket", CostUSD: 0.88, TokensIn: 10},
		{Day: "2026-10-07", SessionID: "ses_dev", RootID: "ticket", CostUSD: 1.94, TokensIn: 20},
		{Day: "2026-10-07", SessionID: "ses_rev", RootID: "ticket", CostUSD: 0.59, TokensIn: 5},
		{Day: "2026-10-07", SessionID: "alone", RootID: "alone", CostUSD: 0.88, TokensIn: 10},
	} {
		require.NoError(t, us.AddSession(ctx, u))
	}
	_, err := s.DB().Exec(`DELETE FROM schema_migrations WHERE version = 42;
		ALTER TABLE sessions DROP COLUMN tickets; ALTER TABLE sessions DROP COLUMN start_ref`)
	require.NoError(t, err)
	require.NoError(t, s.migrate())
	got, err := ss.Get(ctx, "ticket")
	require.NoError(t, err)
	assert.InDelta(t, 3.41, got.Cost, 1e-9)
	assert.Equal(t, int64(35), got.TokensIn)
	got, err = ss.Get(ctx, "alone")
	require.NoError(t, err)
	assert.InDelta(t, 0.88, got.Cost, 1e-9)
}
