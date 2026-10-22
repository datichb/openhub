package sqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestUsageStore(t *testing.T) {
	st := openTestStore(t)
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

// The estimates of responses cut before their end are rows of the ledger under
// the mark of domain.EstimatedUsageID, summed with the others in the totals of
// the oh session and of the day, and given back by a negative delta when the
// tool reports the cost after all (no column, no migration).
func TestUsageStoreEstimatedRows(t *testing.T) {
	st := openTestStore(t)
	us := NewUsageStore(st)
	ctx := context.Background()
	est := domain.EstimatedUsageID("s1")

	require.NoError(t, us.AddSession(ctx, domain.SessionUsage{Day: "2026-10-20", SessionID: "s1", ProjectID: "p1", CostUSD: 1}))
	require.NoError(t, us.AddSession(ctx, domain.SessionUsage{Day: "2026-10-20", SessionID: est, RootID: "s1", ProjectID: "p1", CostUSD: 0.4}))
	require.NoError(t, us.AddSession(ctx, domain.SessionUsage{Day: "2026-10-20", SessionID: est, RootID: "s1", ProjectID: "p1", CostUSD: 0.2}))

	tot, err := us.SessionTotal(ctx, "s1")
	require.NoError(t, err)
	assert.InDelta(t, 1.6, tot.CostUSD, 1e-9, "the estimates count in the session")
	only, err := us.ToolSessionTotal(ctx, est)
	require.NoError(t, err)
	assert.InDelta(t, 0.6, only.CostUSD, 1e-9, "and can be told apart")
	day, err := us.DayCost(ctx, "2026-10-20", "p1")
	require.NoError(t, err)
	assert.InDelta(t, 1.6, day, 1e-9, "and in the day")

	require.NoError(t, us.AddSession(ctx, domain.SessionUsage{Day: "2026-10-20", SessionID: est, RootID: "s1", ProjectID: "p1", CostUSD: -0.5}))
	tot, _ = us.SessionTotal(ctx, "s1")
	assert.InDelta(t, 1.1, tot.CostUSD, 1e-9)
	only, _ = us.ToolSessionTotal(ctx, est)
	assert.InDelta(t, 0.1, only.CostUSD, 1e-9, "given back")
}

// The report joins the ledger with the sessions (outer): a session deleted by
// gc keeps its dollars, under no workflow; the project comes from the ledger.
func TestUsageStoreReport(t *testing.T) {
	st := openTestStore(t)
	us := NewUsageStore(st)
	ss := NewSessionStore(st)
	ctx := context.Background()
	seedProject(t, st, "p1")
	seedProject(t, st, "p2")
	for _, s := range []*domain.Session{
		{ID: "s1", ProjectID: "p1", WorkflowID: "feature", Status: domain.SessionStatusRunning},
		{ID: "s2", ProjectID: "p2", WorkflowID: "feature", Status: domain.SessionStatusRunning},
		{ID: "s3", ProjectID: "p1", Status: domain.SessionStatusRunning}, // before workflows
	} {
		require.NoError(t, ss.Create(ctx, s))
	}
	add := func(day, id, root, project string, cost float64) {
		require.NoError(t, us.AddSession(ctx, domain.SessionUsage{Day: day, SessionID: id, RootID: root, ProjectID: project, CostUSD: cost, TokensIn: 10, TokensOut: 1}))
	}
	add("2026-10-20", "s1", "", "p1", 1)
	add("2026-10-21", "sub", "s1", "p1", 0.5)
	add("2026-10-21", "s2", "", "p2", 3)
	add("2026-10-21", "s3", "", "p1", 0.25)
	add("2026-10-21", "gone", "", "p1", 2) // session deleted
	add("2026-10-01", "old", "", "p2", 9)  // outside the period

	byWorkflow, err := us.UsageReport(ctx, domain.UsageByWorkflow, "2026-10-15")
	require.NoError(t, err)
	require.Len(t, byWorkflow, 2)
	assert.Equal(t, domain.UsageGroup{Key: "feature", Sessions: 2, CostUSD: 4.5, TokensIn: 30, TokensOut: 3}, byWorkflow[0])
	assert.Equal(t, "", byWorkflow[1].Key, "deleted and older sessions are unknown")
	assert.Equal(t, 2, byWorkflow[1].Sessions)
	assert.InDelta(t, 2.25, byWorkflow[1].CostUSD, 1e-9)

	byProject, err := us.UsageReport(ctx, domain.UsageByProject, "2026-10-15")
	require.NoError(t, err)
	require.Len(t, byProject, 2)
	assert.Equal(t, "p1", byProject[0].Key)
	assert.InDelta(t, 3.75, byProject[0].CostUSD, 1e-9)
	assert.Equal(t, "p2", byProject[1].Key)
	assert.InDelta(t, 3, byProject[1].CostUSD, 1e-9)

	all, err := us.UsageReport(ctx, domain.UsageByProject, "")
	require.NoError(t, err)
	assert.InDelta(t, 15.75, all[0].CostUSD+all[1].CostUSD, 1e-9, "no period: everything")

	_, err = us.UsageReport(ctx, "model", "")
	assert.Error(t, err)
}
