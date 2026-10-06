package session

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/limits"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func TestDecideBudgetOfTheRestrictions(t *testing.T) {
	svc, ctx := newTestService(t)
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "u.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	usage := sqlite.NewUsageStore(st)
	var stopped []string
	svc.Resolvers = map[domain.DecisionKind]Resolver{domain.DecisionBudget: BudgetResolver(usage, func(_ context.Context, id string) error {
		stopped = append(stopped, id)
		return nil
	})}
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunWaiting}))
	raise := func(ref string) string {
		d := &domain.Decision{SessionID: "ses_a", Kind: domain.DecisionBudget, ToolRef: ref, Payload: domain.DecisionPayload{Data: map[string]any{
			limits.DataBudgetLimit: limits.BudgetSession, limits.DataBudgetScope: limits.SessionScope("ses_a"),
			limits.DataBudgetUSD: 2.0, limits.DataBudgetSpent: 2.5, limits.DataBudgetUnit: 2.0}}}
		require.NoError(t, svc.Raise(ctx, d))
		return d.ID
	}

	id := raise("r1")
	assert.Error(t, svc.Decide(ctx, Reply{DecisionID: id, Decision: "always"}))
	require.NoError(t, svc.Decide(ctx, Reply{DecisionID: id, Decision: "raise"}))
	x, err := usage.BudgetExtra(ctx, limits.SessionScope("ses_a"), "")
	require.NoError(t, err)
	assert.InDelta(t, 2.5, x, 1e-9, "the overrun plus one budget")

	id = raise("r2")
	require.NoError(t, svc.Decide(ctx, Reply{DecisionID: id, Decision: "raise", Message: "$10"}))
	x, _ = usage.BudgetExtra(ctx, limits.SessionScope("ses_a"), "")
	assert.InDelta(t, 12.5, x, 1e-9)

	id = raise("r3")
	require.NoError(t, svc.Decide(ctx, Reply{DecisionID: id, Decision: "stop"}))
	assert.Equal(t, []string{"ses_a"}, stopped)

	id = raise("r4")
	require.NoError(t, svc.Decide(ctx, Reply{DecisionID: id}), "dismiss: one more step")
	d, _ := svc.Decisions.Get(ctx, id)
	assert.Equal(t, "dismiss", d.Resolution.Decision)

	// The proxy token budget is only acknowledged.
	tok := &domain.Decision{SessionID: "ses_a", Kind: domain.DecisionBudget, ToolRef: "tok"}
	require.NoError(t, svc.Raise(ctx, tok))
	assert.Error(t, svc.Decide(ctx, Reply{DecisionID: tok.ID, Decision: "raise"}))
	require.NoError(t, svc.Decide(ctx, Reply{DecisionID: tok.ID}))
}
