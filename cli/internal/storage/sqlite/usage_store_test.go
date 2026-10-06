package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestUsageStore(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	us := NewUsageStore(st)
	ctx := context.Background()

	require.NoError(t, us.AddSession(ctx, domain.SessionUsage{Day: "2026-10-06", SessionID: "s1", ProjectID: "p1", GroupKey: "g", CostUSD: 0.5, TokensIn: 10, TokensOut: 2}))
	require.NoError(t, us.AddSession(ctx, domain.SessionUsage{Day: "2026-10-06", SessionID: "s1", ProjectID: "p1", GroupKey: "g", CostUSD: 0.25, TokensIn: 5}))
	require.NoError(t, us.AddSession(ctx, domain.SessionUsage{Day: "2026-10-07", SessionID: "s1", ProjectID: "p1", GroupKey: "g", CostUSD: 1}))
	require.NoError(t, us.AddSession(ctx, domain.SessionUsage{Day: "2026-10-06", SessionID: "s2", ProjectID: "p2", CostUSD: 2}))
	require.NoError(t, us.AddSession(ctx, domain.SessionUsage{Day: "2026-10-07", SessionID: "child", RootID: "s1", ProjectID: "p1", CostUSD: 0.5}))
	child, err := us.ToolSessionTotal(ctx, "child")
	require.NoError(t, err)
	assert.InDelta(t, 0.5, child.CostUSD, 1e-9)
	tot, err := us.SessionTotal(ctx, "s1")
	require.NoError(t, err)
	assert.InDelta(t, 2.25, tot.CostUSD, 1e-9, "with its subagent session")
	assert.Equal(t, int64(15), tot.TokensIn)
	assert.Equal(t, "p1", tot.ProjectID)
	day, err := us.DayCost(ctx, "2026-10-06", "")
	require.NoError(t, err)
	assert.InDelta(t, 2.75, day, 1e-9)
	day, err = us.DayCost(ctx, "2026-10-06", "p1")
	require.NoError(t, err)
	assert.InDelta(t, 0.75, day, 1e-9)

	require.NoError(t, us.AddProxy(ctx, "g", "2026-10-06", domain.ProxyUsage{Requests: 1, TokensIn: 100, TokensOut: 7}))
	require.NoError(t, us.AddProxy(ctx, "g", "2026-10-07", domain.ProxyUsage{Requests: 2, TokensIn: 1}))
	pu, err := us.ProxyTotal(ctx, "g")
	require.NoError(t, err)
	assert.Equal(t, domain.ProxyUsage{Requests: 3, TokensIn: 101, TokensOut: 7}, pu)

	require.NoError(t, us.AddBudgetExtra(ctx, "session:s1", "", 2))
	require.NoError(t, us.AddBudgetExtra(ctx, "session:s1", "", 3))
	x, err := us.BudgetExtra(ctx, "session:s1", "")
	require.NoError(t, err)
	assert.InDelta(t, 5, x, 1e-9)
	x, err = us.BudgetExtra(ctx, "global", "2026-10-06")
	require.NoError(t, err)
	assert.Zero(t, x)
}
