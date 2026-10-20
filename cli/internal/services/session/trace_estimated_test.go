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

// The cost of the responses cut before their end is estimated by oh, not
// reported by the tool: the summary of the trace says how much of the ledger
// cost it is.
func TestTraceSummaryShowsTheEstimatedCost(t *testing.T) {
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	usage := sqlite.NewUsageStore(st)
	sess := &domain.Session{ID: "ses_root"}
	require.NoError(t, usage.AddSession(ctx, domain.SessionUsage{Day: "2026-10-20", SessionID: "ses_root", RootID: "ses_root", CostUSD: 0.5}))

	sum := summaryEvent(ctx, sess, nil, "live", usage).Data
	assert.InDelta(t, 0.5, sum["ledger_cost_usd"], 1e-9)
	assert.NotContains(t, sum, "ledger_estimated_cost_usd", "nothing estimated: no mark")

	require.NoError(t, usage.AddSession(ctx, domain.SessionUsage{Day: "2026-10-20", SessionID: domain.EstimatedUsageID("ses_root"), RootID: "ses_root", CostUSD: 0.2}))
	sum = summaryEvent(ctx, sess, nil, "live", usage).Data
	assert.InDelta(t, 0.7, sum["ledger_cost_usd"], 1e-9, "the estimate is in the ledger total")
	assert.InDelta(t, 0.2, sum["ledger_estimated_cost_usd"], 1e-9, "and shown apart")
}
