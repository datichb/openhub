package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestServerStoreCRUD(t *testing.T) {
	st := NewServerStore(openTestStore(t))
	ctx := context.Background()
	srv := &domain.Server{GroupKey: "p-abc-local", Adapter: "tool-v2", Runtime: "local", PID: 42, URL: "http://127.0.0.1:1", Password: "pw", Status: domain.ServerReady}
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
	gs := NewGrantStore(openTestStore(t))
	ctx := context.Background()
	src := domain.CredentialSource{Kind: domain.CredentialBearer, KeychainKey: "openhub.provider.bedrock.token", Scope: "hub"}
	require.NoError(t, gs.Insert(ctx, &domain.ProxyGrant{TokenHash: "ohs_1", Owner: "g1", Provider: "amazon-bedrock", Region: "eu-west-1", Source: src, AllowedModels: []string{"eu.*"}}))
	require.NoError(t, gs.Insert(ctx, &domain.ProxyGrant{TokenHash: "ohs_2", Owner: "g2", Provider: "anthropic"}))

	active, err := gs.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, active, 2)
	assert.Equal(t, src, active[0].Source)
	assert.Equal(t, []string{"eu.*"}, active[0].AllowedModels)

	require.NoError(t, gs.RevokeOwner(ctx, "g1", time.Now()))
	active, _ = gs.ListActive(ctx)
	require.Len(t, active, 1)
	assert.Equal(t, "ohs_2", active[0].TokenHash)
	require.NoError(t, gs.Revoke(ctx, "ohs_2", time.Now()))
	active, _ = gs.ListActive(ctx)
	assert.Empty(t, active)
}

// S-23 (audit v5): a grant whose model list cannot be read is not restored
// (never "any model").
func TestGrantStoreUnreadableModels(t *testing.T) {
	st := openTestStore(t)
	gs := NewGrantStore(st)
	ctx := context.Background()
	require.NoError(t, gs.Insert(ctx, &domain.ProxyGrant{TokenHash: "ohs_1", Owner: "g1", Provider: "amazon-bedrock", AllowedModels: []string{"eu.*"}}))
	require.NoError(t, gs.Insert(ctx, &domain.ProxyGrant{TokenHash: "ohs_2", Owner: "g2", Provider: "amazon-bedrock", AllowedModels: []string{"eu.*"}}))
	require.NoError(t, gs.Insert(ctx, &domain.ProxyGrant{TokenHash: "ohs_3", Owner: "g3", Provider: "amazon-bedrock"}))
	_, err := st.DB().Exec(`UPDATE proxy_grants SET allowed_models = '["eu.*"' WHERE token = 'ohs_1'`)
	require.NoError(t, err)
	_, err = st.DB().Exec(`UPDATE proxy_grants SET source = 'not json' WHERE token = 'ohs_2'`)
	require.NoError(t, err)
	active, err := gs.ListActive(ctx)
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, "ohs_3", active[0].TokenHash)
	assert.Empty(t, active[0].AllowedModels, "no list stays no list")
}

func TestSessionStoreV5Fields(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	seedProject(t, s, "p1")
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
	s := openTestStore(t)
	ctx := context.Background()
	seedProject(t, s, "p1")
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

// S-29: RevokeOrphans revokes the active grants whose owner has no starting
// or ready server, old enough, whether they can be listed or not.
func TestGrantStoreRevokeOrphans(t *testing.T) {
	st := openTestStore(t)
	gs, ss := NewGrantStore(st), NewServerStore(st)
	ctx := context.Background()
	now := time.Now()
	old := now.Add(-time.Hour)
	for _, s := range []domain.Server{
		{GroupKey: "ready", Adapter: "x", Status: domain.ServerReady, CreatedAt: old},
		{GroupKey: "starting", Adapter: "x", Status: domain.ServerStarting, CreatedAt: old},
		{GroupKey: "sleeping", Adapter: "x", Status: domain.ServerSleeping, CreatedAt: old},
		{GroupKey: "stopped", Adapter: "x", Status: domain.ServerStopped, CreatedAt: old},
	} {
		require.NoError(t, ss.Upsert(ctx, &s))
	}
	for _, g := range []domain.ProxyGrant{
		{TokenHash: "t-ready", Owner: "ready", CreatedAt: old},
		{TokenHash: "t-starting", Owner: "starting", CreatedAt: old},
		{TokenHash: "t-sleeping", Owner: "sleeping", CreatedAt: old},
		{TokenHash: "t-stopped", Owner: "stopped", CreatedAt: old},
		{TokenHash: "t-nobody", Owner: "nobody", CreatedAt: old},
		{TokenHash: "t-recent", Owner: "nobody", CreatedAt: now}, // a client is starting its server
		{TokenHash: "t-done", Owner: "nobody", CreatedAt: old},
	} {
		g.Provider = "amazon-bedrock"
		require.NoError(t, gs.Insert(ctx, &g))
	}
	require.NoError(t, gs.Revoke(ctx, "t-done", now)) // already revoked: not counted
	_, err := st.DB().Exec(`UPDATE proxy_grants SET source = 'not json' WHERE token = 't-nobody'`)
	require.NoError(t, err)

	n, err := gs.RevokeOrphans(ctx, now.Add(-time.Minute), now)
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	var left []string
	rows, err := st.DB().Query(`SELECT token FROM proxy_grants WHERE revoked_at IS NULL ORDER BY token`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var tok string
		require.NoError(t, rows.Scan(&tok))
		left = append(left, tok)
	}
	assert.Equal(t, []string{"t-ready", "t-recent", "t-starting"}, left)

	n, err = gs.RevokeOrphans(ctx, now.Add(-time.Minute), now)
	require.NoError(t, err)
	assert.Zero(t, n, "idempotent")
}
