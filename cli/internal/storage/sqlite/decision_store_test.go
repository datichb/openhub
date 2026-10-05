package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestDecisionStoreLifecycle(t *testing.T) {
	ds := NewDecisionStore(openTemp(t))
	ctx := context.Background()
	id := domain.DecisionID(domain.DecisionPermission, "ses_a", "per_1")
	d := &domain.Decision{ID: id, SessionID: "ses_a", GroupKey: "g1", Kind: domain.DecisionPermission, ToolRef: "per_1",
		Payload: domain.DecisionPayload{Action: "shell", Resources: []string{"npm test"}}}
	require.NoError(t, ds.Upsert(ctx, d))
	created := d.CreatedAt

	// Refresh keeps the creation date.
	d2 := &domain.Decision{ID: id, SessionID: "ses_a", Kind: domain.DecisionPermission, ToolRef: "per_1",
		Payload: domain.DecisionPayload{Action: "shell", Resources: []string{"npm run e2e"}}, CreatedAt: created.Add(time.Hour)}
	require.NoError(t, ds.Upsert(ctx, d2))
	got, err := ds.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []string{"npm run e2e"}, got.Payload.Resources)
	assert.Equal(t, "g1", got.GroupKey)
	assert.True(t, got.CreatedAt.Equal(created))
	assert.True(t, got.Open())

	q := &domain.Decision{ID: domain.DecisionID(domain.DecisionQuestion, "ses_b", "frm_1"), SessionID: "ses_b", GroupKey: "g1",
		Kind: domain.DecisionQuestion, ToolRef: "frm_1", Payload: domain.DecisionPayload{Title: "Color?",
			Fields: []domain.DecisionField{{Key: "q0", Type: "select", Options: []domain.DecisionOption{{Value: "Blue"}}}}}}
	require.NoError(t, ds.Upsert(ctx, q))

	open, err := ds.ListOpen(ctx, domain.DecisionFilter{GroupKey: "g1"})
	require.NoError(t, err)
	require.Len(t, open, 2)
	open, _ = ds.ListOpen(ctx, domain.DecisionFilter{SessionID: "ses_b", Kind: domain.DecisionQuestion})
	require.Len(t, open, 1)
	assert.Equal(t, "Blue", open[0].Payload.Fields[0].Options[0].Value)

	// First answer wins.
	before := time.Now().Add(-time.Second)
	ok, err := ds.Resolve(ctx, id, domain.ResolvedByOh, &domain.DecisionResolution{Decision: "once", Message: "go"}, time.Now())
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = ds.Resolve(ctx, id, domain.ResolvedByTool, nil, time.Now())
	require.NoError(t, err)
	assert.False(t, ok)
	got, _ = ds.Get(ctx, id)
	assert.Equal(t, domain.ResolvedByOh, got.ResolvedBy)
	require.NotNil(t, got.Resolution)
	assert.Equal(t, "once", got.Resolution.Decision)

	since, err := ds.ListSince(ctx, before)
	require.NoError(t, err)
	assert.Len(t, since, 2)

	require.NoError(t, ds.Reopen(ctx, id))
	got, _ = ds.Get(ctx, id)
	assert.True(t, got.Open())
	assert.Nil(t, got.Resolution)

	_, err = ds.Get(ctx, "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}
