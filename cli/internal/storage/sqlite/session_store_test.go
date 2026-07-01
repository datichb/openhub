package sqlite

import (
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

	// Create a project first (FK constraint)
	now := time.Now().Truncate(time.Second)
	require.NoError(t, ps.Create(&domain.Project{
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
	require.NoError(t, ss.Create(session))

	// Get
	got, err := ss.Get("sess-1")
	require.NoError(t, err)
	assert.Equal(t, "sess-1", got.ID)
	assert.Equal(t, "proj-1", got.ProjectID)
	assert.Equal(t, domain.SessionStatusRunning, got.Status)
	assert.Equal(t, "bedrock", got.Provider)
	assert.Nil(t, got.EndedAt)

	// List by project
	sessions, err := ss.List("proj-1")
	require.NoError(t, err)
	assert.Len(t, sessions, 1)

	// Update (complete session)
	endTime := now.Add(30 * time.Minute)
	got.EndedAt = &endTime
	got.Status = domain.SessionStatusCompleted
	got.TokensIn = 5000
	got.TokensOut = 2000
	require.NoError(t, ss.Update(got))

	updated, err := ss.Get("sess-1")
	require.NoError(t, err)
	assert.Equal(t, domain.SessionStatusCompleted, updated.Status)
	assert.NotNil(t, updated.EndedAt)
	assert.Equal(t, int64(5000), updated.TokensIn)
	assert.Equal(t, int64(2000), updated.TokensOut)
}

func TestSessionStore_GetNotFound(t *testing.T) {
	s := openTestStore(t)
	ss := NewSessionStore(s)

	_, err := ss.Get("nonexistent")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSessionStore_ListAll(t *testing.T) {
	s := openTestStore(t)
	ps := NewProjectStore(s)
	ss := NewSessionStore(s)

	now := time.Now()
	require.NoError(t, ps.Create(&domain.Project{ID: "proj-1", Name: "P1", Path: "/p1", Status: domain.ProjectStatusActive, CreatedAt: now, UpdatedAt: now}))
	require.NoError(t, ps.Create(&domain.Project{ID: "proj-2", Name: "P2", Path: "/p2", Status: domain.ProjectStatusActive, CreatedAt: now, UpdatedAt: now}))

	require.NoError(t, ss.Create(&domain.Session{ID: "s1", ProjectID: "proj-1", StartedAt: now, Status: domain.SessionStatusCompleted}))
	require.NoError(t, ss.Create(&domain.Session{ID: "s2", ProjectID: "proj-1", StartedAt: now, Status: domain.SessionStatusRunning}))
	require.NoError(t, ss.Create(&domain.Session{ID: "s3", ProjectID: "proj-2", StartedAt: now, Status: domain.SessionStatusCompleted}))

	// List all
	all, err := ss.List("")
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// List by project
	proj1, err := ss.List("proj-1")
	require.NoError(t, err)
	assert.Len(t, proj1, 2)
}
