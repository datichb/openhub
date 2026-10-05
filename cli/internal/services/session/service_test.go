package session

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func newTestService(t *testing.T) (*Service, context.Context) {
	t.Helper()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	return &Service{Sessions: sqlite.NewSessionStore(st), Decisions: sqlite.NewDecisionStore(st), Servers: sqlite.NewServerStore(st)}, context.Background()
}

func TestRaiseAndInbox(t *testing.T) {
	svc, ctx := newTestService(t)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunWaiting}))
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_b", ProjectID: "p1", Status: domain.SessionStatusCompleted, GroupKey: "g1", State: domain.RunStopped}))

	d := &domain.Decision{SessionID: "ses_a", Kind: domain.DecisionCheckpoint, ToolRef: "cp-2",
		Payload: domain.DecisionPayload{Title: "Commit ou correction", Data: map[string]any{"checkpoint": "cp-2"}}}
	require.NoError(t, svc.Raise(ctx, d))
	assert.Equal(t, "checkpoint:ses_a:cp-2", d.ID)
	assert.Equal(t, "g1", d.GroupKey)
	require.NoError(t, svc.Raise(ctx, &domain.Decision{SessionID: "ses_b", Kind: domain.DecisionError}))

	items, err := svc.Inbox(ctx, domain.DecisionFilter{})
	require.NoError(t, err)
	require.Len(t, items, 1, "decisions of stopped sessions are hidden")
	assert.Equal(t, "ses_a", items[0].Session.ID)
	assert.Equal(t, "cp-2", items[0].Decision.Payload.Data["checkpoint"])

	assert.Error(t, svc.Raise(ctx, &domain.Decision{Kind: domain.DecisionError}))
	assert.Error(t, svc.Raise(ctx, &domain.Decision{SessionID: "ses_missing", Kind: domain.DecisionError}))
}
