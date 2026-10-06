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

func openTemp(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "oh.db")
	s, err := Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	return s
}

func TestServerStoreCRUD(t *testing.T) {
	st := NewServerStore(openTemp(t))
	ctx := context.Background()
	srv := &domain.Server{GroupKey: "p-abc-local", Adapter: "opencode-v2", Runtime: "local", PID: 42, URL: "http://127.0.0.1:1", Password: "pw", Status: domain.ServerReady}
	require.NoError(t, st.Upsert(ctx, srv))
	srv.PID = 43
	require.NoError(t, st.Upsert(ctx, srv))

	got, err := st.Get(ctx, "p-abc-local")
	require.NoError(t, err)
	assert.Equal(t, 43, got.PID)
	assert.Equal(t, "pw", got.Password)

	require.NoError(t, st.SetStatus(ctx, "p-abc-local", domain.ServerSleeping))
	at := time.Now().Add(time.Minute).Truncate(time.Second)
	require.NoError(t, st.Touch(ctx, "p-abc-local", at))
	got, _ = st.Get(ctx, "p-abc-local")
	assert.Equal(t, domain.ServerSleeping, got.Status)
	assert.True(t, got.LastActivityAt.Equal(at))

	list, err := st.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	require.NoError(t, st.Delete(ctx, "p-abc-local"))
	_, err = st.Get(ctx, "p-abc-local")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGrantStore(t *testing.T) {
	gs := NewGrantStore(openTemp(t))
	ctx := context.Background()
	src := domain.CredentialSource{Kind: domain.CredentialBearer, KeychainKey: "openhub.provider.bedrock.token", Scope: "hub"}
	require.NoError(t, gs.Insert(ctx, &domain.ProxyGrant{Token: "ohs_1", Owner: "g1", Provider: "amazon-bedrock", Region: "eu-west-1", Source: src, AllowedModels: []string{"eu.*"}}))
	require.NoError(t, gs.Insert(ctx, &domain.ProxyGrant{Token: "ohs_2", Owner: "g2", Provider: "anthropic"}))

	active, err := gs.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, active, 2)
	assert.Equal(t, src, active[0].Source)
	assert.Equal(t, []string{"eu.*"}, active[0].AllowedModels)

	require.NoError(t, gs.RevokeOwner(ctx, "g1", time.Now()))
	active, _ = gs.ListActive(ctx)
	require.Len(t, active, 1)
	assert.Equal(t, "ohs_2", active[0].Token)
	require.NoError(t, gs.Revoke(ctx, "ohs_2", time.Now()))
	active, _ = gs.ListActive(ctx)
	assert.Empty(t, active)
}

func TestSessionStoreV5Fields(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	_, err := s.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1', 'p1', '/p1')`)
	require.NoError(t, err)
	ss := NewSessionStore(s)
	sess := &domain.Session{ID: "ses_1", ProjectID: "p1", Status: domain.SessionStatusRunning,
		WorkflowID: "ticket", EntryAgent: "orchestrator-dev", BundleHash: "h", GroupKey: "g", Runtime: "local", Mode: "semi-auto", State: domain.RunActive}
	require.NoError(t, ss.Create(ctx, sess))
	sess.State = domain.RunIdle
	require.NoError(t, ss.Update(ctx, sess))
	got, err := ss.Get(ctx, "ses_1")
	require.NoError(t, err)
	assert.Equal(t, domain.RunIdle, got.State)
	assert.Equal(t, "orchestrator-dev", got.EntryAgent)
	assert.Equal(t, "g", got.GroupKey)
}

func TestSessionStoreWorkflowLaunchFields(t *testing.T) {
	s := openTemp(t)
	ctx := context.Background()
	_, err := s.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1', 'p1', '/p1')`)
	require.NoError(t, err)
	ss := NewSessionStore(s)
	sess := &domain.Session{ID: "ses_1", ProjectID: "p1", Status: domain.SessionStatusRunning, WorkflowID: "ticket",
		WorkflowLayer: "hub", WorkflowVersion: 3, WorkflowRisk: "write", Location: "worktree", ParentSessionID: "ses_0"}
	require.NoError(t, ss.Create(ctx, sess))
	got, err := ss.Get(ctx, "ses_1")
	require.NoError(t, err)
	assert.Equal(t, "hub", got.WorkflowLayer)
	assert.Equal(t, 3, got.WorkflowVersion)
	assert.Equal(t, "write", got.WorkflowRisk)
	assert.Equal(t, "worktree", got.Location)
	assert.Equal(t, "ses_0", got.ParentSessionID)
	assert.Empty(t, got.Outputs)

	got.Outputs = map[string]any{"branch": "feat/bd-1", "tickets": []any{"bd-2"}}
	require.NoError(t, ss.Update(ctx, got))
	again, err := ss.Get(ctx, "ses_1")
	require.NoError(t, err)
	assert.Equal(t, got.Outputs, again.Outputs)
}
