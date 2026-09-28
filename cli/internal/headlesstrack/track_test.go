package headlesstrack

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/platform"
)

// --- mock session store ---

type mockSessionStore struct {
	created []*domain.Session
	updated []*domain.Session
}

func (m *mockSessionStore) Create(_ context.Context, s *domain.Session) error {
	m.created = append(m.created, s)
	return nil
}

func (m *mockSessionStore) Update(_ context.Context, s *domain.Session) error {
	m.updated = append(m.updated, s)
	return nil
}

func (m *mockSessionStore) List(_ context.Context, _ string) ([]domain.Session, error) {
	return nil, nil
}
func (m *mockSessionStore) Get(_ context.Context, _ string) (*domain.Session, error) {
	return nil, domain.ErrNotFound
}
func (m *mockSessionStore) ListRunning(_ context.Context, _ string) ([]domain.Session, error) {
	return nil, nil
}

// --- tests ---

func TestTrack_SuccessfulRun(t *testing.T) {
	store := &mockSessionStore{}
	result := &platform.HeadlessResult{
		Content:         "enriched brief",
		Model:           "claude-opus",
		Cost:            0.05,
		TokensIn:        1000,
		TokensOut:       500,
		TokensReasoning: 200,
	}

	got, err := Track(context.Background(), Opts{
		Sessions:     store,
		PlatformName: "opencode",
		ProjectID:    "proj-1",
		ProjectPath:  "/tmp/project",
		Provider:     "bedrock",
		Label:        "brief-enrichment",
	}, func(_ context.Context) (*platform.HeadlessResult, error) {
		return result, nil
	})

	require.NoError(t, err)
	assert.Equal(t, "enriched brief", got.Content)

	// Session was created and updated
	require.Len(t, store.created, 1)
	require.Len(t, store.updated, 1)

	created := store.created[0]
	assert.Equal(t, domain.SessionTypeHeadless, created.Type)
	// Note: created and updated point to the same Session object, so after
	// Track() returns the status reflects the final state (completed).
	assert.Equal(t, domain.SessionStatusCompleted, created.Status)
	assert.Equal(t, "proj-1", created.ProjectID)
	assert.Equal(t, "bedrock", created.Provider)
	assert.NotNil(t, created.Label)
	assert.Equal(t, "brief-enrichment", *created.Label)

	updated := store.updated[0]
	assert.Equal(t, domain.SessionStatusCompleted, updated.Status)
	assert.Equal(t, "claude-opus", updated.Model)
	assert.Equal(t, 0.05, updated.Cost)
	assert.Equal(t, int64(1000), updated.TokensIn)
	assert.Equal(t, int64(500), updated.TokensOut)
	assert.NotNil(t, updated.EndedAt)
}

func TestTrack_FailedRun(t *testing.T) {
	store := &mockSessionStore{}

	_, err := Track(context.Background(), Opts{
		Sessions:     store,
		PlatformName: "opencode",
		ProjectID:    "proj-1",
		Provider:     "bedrock",
		Label:        "sweep-decomposition",
	}, func(_ context.Context) (*platform.HeadlessResult, error) {
		return nil, assert.AnError
	})

	require.Error(t, err)
	require.Len(t, store.updated, 1)
	assert.Equal(t, domain.SessionStatusFailed, store.updated[0].Status)
}

func TestTrack_CorrelationID(t *testing.T) {
	store := &mockSessionStore{}

	_, _ = Track(context.Background(), Opts{
		Sessions:      store,
		PlatformName:  "opencode",
		CorrelationID: "sweep-abc-123",
		Label:         "sweep-decomposition",
	}, func(_ context.Context) (*platform.HeadlessResult, error) {
		return &platform.HeadlessResult{Content: "ok"}, nil
	})

	require.Len(t, store.created, 1)
	assert.NotNil(t, store.created[0].CorrelationID)
	assert.Equal(t, "sweep-abc-123", *store.created[0].CorrelationID)
}

func TestTrack_NilSessionStore_NoTracking(t *testing.T) {
	got, err := Track(context.Background(), Opts{
		Sessions: nil, // no tracking
	}, func(_ context.Context) (*platform.HeadlessResult, error) {
		return &platform.HeadlessResult{Content: "result"}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, "result", got.Content)
}

func TestTrack_MemberID(t *testing.T) {
	store := &mockSessionStore{}

	_, _ = Track(context.Background(), Opts{
		Sessions:     store,
		PlatformName: "opencode",
		MemberID:     "alice",
	}, func(_ context.Context) (*platform.HeadlessResult, error) {
		return &platform.HeadlessResult{Content: "ok"}, nil
	})

	require.Len(t, store.created, 1)
	assert.NotNil(t, store.created[0].MemberID)
	assert.Equal(t, "alice", *store.created[0].MemberID)
}
