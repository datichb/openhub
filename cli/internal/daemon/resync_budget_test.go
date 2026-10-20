package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/limits"
)

// reconnect makes the tool report subagent sessions the daemon did not see
// run (their events were missed), then replays the reconnection.
func (e *limitEnv) reconnect(children map[string]adapters.SessionResult) {
	e.ad.set(func() {
		e.ad.parents = map[string]string{}
		for id, res := range children {
			e.ad.parents[id] = "ses_a"
			e.ad.usage[id] = res
		}
	})
	e.events <- adapters.ToolEvent{Kind: adapters.EventConnected}
}

func (e *limitEnv) rootCost() float64 {
	tot, err := e.usage.SessionTotal(e.ctx, "ses_a")
	require.NoError(e.t, err)
	return tot.CostUSD
}

// A subagent that ended while the daemon was disconnected is counted when
// it reconnects: its cost reaches the ledger and the session total. A second
// reconnection does not count it again, and what a subagent spends after
// the first one is added to what the ledger held.
func TestResyncCountsASubagentWhoseEndWasMissed(t *testing.T) {
	e := newLimitEnv(t, "ses_a")
	costIs := func(want float64) func() bool {
		return func() bool { return e.rootCost() > want-1e-9 && e.rootCost() < want+1e-9 }
	}
	// The end of a subagent is the marker of what comes before it: events
	// are handled in order, so a cost reached with the marker counted once
	// shows that the reconnection before it counted nothing twice.
	end := func(cost float64) {
		e.ad.set(func() { e.ad.usage["ses_c"] = adapters.SessionResult{Cost: cost, TokensIn: int64(cost * 1000)} })
		e.events <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_c", Outcome: "succeeded"}
	}

	e.reconnect(map[string]adapters.SessionResult{"ses_c": {Cost: 0.4, TokensIn: 400}})
	require.Eventually(t, costIs(0.4), 10*time.Second, 10*time.Millisecond, "counted at the resync")
	tot, err := e.usage.ToolSessionTotal(e.ctx, "ses_c")
	require.NoError(t, err)
	assert.InDelta(t, 0.4, tot.CostUSD, 1e-9)
	require.Eventually(t, func() bool {
		s, err := e.sessions.Get(e.ctx, "ses_a")
		return err == nil && s.Cost > 0.39
	}, 10*time.Second, 10*time.Millisecond, "the session shows the total")

	// Counted already: a new reconnection adds nothing, the marker adds its 0.05.
	e.reconnect(map[string]adapters.SessionResult{"ses_c": {Cost: 0.4, TokensIn: 400}})
	end(0.45)
	require.Eventually(t, costIs(0.45), 10*time.Second, 10*time.Millisecond, "0.4 once, then 0.05")

	// It spent more while the daemon was away: only the difference is added.
	e.reconnect(map[string]adapters.SessionResult{"ses_c": {Cost: 0.55, TokensIn: 550}})
	require.Eventually(t, costIs(0.55), 10*time.Second, 10*time.Millisecond, "0.1 more at the resync")
	end(0.6)
	require.Eventually(t, costIs(0.6), 10*time.Second, 10*time.Millisecond)
	assert.Empty(t, e.openBudget("ses_a"))
}

// The budget is checked on what the resync counted: a subagent that
// overran it unseen raises the decision at once, without waiting for an
// event.
func TestResyncRaisesTheBudgetDecision(t *testing.T) {
	e := newLimitEnv(t, "ses_a")
	require.NoError(t, limits.Save(e.dir, "ses_a", limits.Resolve(limits.Input{Workflow: limits.Limits{SessionBudgetUSD: 1}, ProjectID: "p1"})))
	e.reconnect(map[string]adapters.SessionResult{"ses_c1": {Cost: 0.7}, "ses_c2": {Cost: 0.5}})
	require.Eventually(t, func() bool { return len(e.openBudget("ses_a")) == 1 }, 10*time.Second, 10*time.Millisecond)
	assert.InDelta(t, 1.2, e.rootCost(), 1e-9, "both subagents counted")
}
