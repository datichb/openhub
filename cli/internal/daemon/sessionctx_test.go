package daemon

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/limits"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionctx"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

type ctxTool struct {
	adapters.ToolAdapter
	sets []string
}

func (c *ctxTool) Capabilities() adapters.Capabilities {
	return adapters.Capabilities{SessionContext: true}
}
func (c *ctxTool) SetSessionContext(_ context.Context, _ adapters.ServerHandle, _, key string, value any) error {
	data, _ := json.Marshal(value)
	c.sets = append(c.sets, key+"="+string(data))
	return nil
}
func (c *ctxTool) ClearSessionContext(context.Context, adapters.ServerHandle, string, string) error {
	return nil
}

// QB8: the checkpoint entry lists the passed checkpoints, the current and
// the next one.
func TestCheckpointsContext(t *testing.T) {
	v := checkpointsContext(checkpoint.Status{Workflow: "ticket", Mode: "semi-auto", Next: "cp-3", Checkpoints: []checkpoint.CheckpointStatus{
		{ID: "cp-1", Label: "Start", State: checkpoint.StatePassed},
		{ID: "cp-2", Label: "Commit", State: checkpoint.StateWaiting},
		{ID: "cp-3", State: checkpoint.StateTodo},
	}})
	data, _ := json.Marshal(v)
	assert.JSONEq(t, `{"workflow":"ticket","mode":"semi-auto","passed":[{"id":"cp-1","label":"Start"}],"current":"cp-2","next":"cp-3"}`, string(data))
}

// QB8: the budget entry is written at a $ decision and after a raise, not
// at each step (spending alone does not rewrite it).
func TestSyncBudgetOnDecisionAndRaise(t *testing.T) {
	ctx := context.Background()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	usage := sqlite.NewUsageStore(st)
	dir := t.TempDir()
	require.NoError(t, limits.Save(dir, "ses_r", limits.Resolved{Limits: limits.Limits{SessionBudgetUSD: 1}}))
	tool := &ctxTool{}
	w := &watcher{d: &Daemon{opts: Options{Usage: usage, SessionsDir: dir}}, ad: tool}
	spend := func(usd float64) {
		require.NoError(t, usage.AddSession(ctx, domain.SessionUsage{Day: domain.UsageDay(time.Now()), SessionID: "ses_r", RootID: "ses_r", CostUSD: usd}))
	}

	spend(0.4)
	w.syncBudget(ctx, "ses_r", false)
	assert.Empty(t, tool.sets, "no decision yet: nothing written")
	spend(0.7)
	w.syncBudget(ctx, "ses_r", true)
	require.Len(t, tool.sets, 1)
	assert.Contains(t, tool.sets[0], `"exhausted":true`)
	assert.Contains(t, tool.sets[0], `"limit_usd":1`)

	spend(0.1)
	w.syncBudget(ctx, "ses_r", false)
	assert.Len(t, tool.sets, 1, "same allowance: not rewritten")
	require.NoError(t, usage.AddBudgetExtra(ctx, limits.SessionScope("ses_r"), "", 2))
	w.syncBudget(ctx, "ses_r", false)
	require.Len(t, tool.sets, 2, "raised")
	assert.Contains(t, tool.sets[1], `"limit_usd":3`)
	assert.Contains(t, tool.sets[1], `"remaining_usd":1.8`)
	assert.NotNil(t, w.contextWriter().Last("ses_r", sessionctx.KeyBudget))
}
