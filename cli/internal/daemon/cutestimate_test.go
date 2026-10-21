package daemon

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/limits"
)

const (
	cutStart = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":2000,\"output_tokens\":1}}}\n\n"
	cutText  = "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"text\":\"word\"}}\n\n"
	cutStop  = "event: message_delta\ndata: {\"usage\":{\"output_tokens\":42}}\n\nevent: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
)

// callStream sends a streamed model request through the proxy of the gate
// environment, to a provider that answers body and, when cut is set, drops
// the connection before the end. It returns once the proxy has accounted the
// response, with the usage the proxy counted for it.
func (e *gateEnv) callStream(body string, cut bool) credproxy.Usage {
	e.t.Helper()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, body)
		if cut {
			w.(http.Flusher).Flush()
			panic(http.ErrAbortHandler)
		}
	}))
	e.t.Cleanup(up.Close)
	tok, err := e.d.proxy.Issue(credproxy.Grant{SessionID: "g1", Provider: credproxy.ProviderAnthropic,
		Upstream: credproxy.Upstream{BaseURL: up.URL, Auth: credproxy.HeaderAuth{Name: "x-api-key", Value: "k"}}})
	require.NoError(e.t, err)
	req, _ := http.NewRequest(http.MethodPost, e.d.proxy.BaseURL(credproxy.ProviderAnthropic)+"/messages", strings.NewReader(`{"model":"m","stream":true,"max_tokens":4096}`))
	req.Header.Set("x-api-key", tok)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(e.t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	require.Eventually(e.t, func() bool { u, _ := e.d.proxy.Usage(tok); return u.InputTokens > 0 }, 5*time.Second, 5*time.Millisecond)
	u, _ := e.d.proxy.Usage(tok)
	return u
}

func (e *gateEnv) total() domain.SessionUsage {
	e.t.Helper()
	tot, err := e.usage.SessionTotal(e.ctx, "ses_a")
	require.NoError(e.t, err)
	return tot
}

func (e *gateEnv) estimated() float64 {
	e.t.Helper()
	tot, err := e.usage.ToolSessionTotal(e.ctx, domain.EstimatedUsageID("ses_a"))
	require.NoError(e.t, err)
	return tot.CostUSD
}

func (e *gateEnv) waitEstimated() float64 {
	e.t.Helper()
	require.Eventually(e.t, func() bool { return e.estimated() > 0 }, 5*time.Second, 5*time.Millisecond, "the estimate is in the ledger")
	return e.estimated()
}

// S-27 follow-up: a response cut before its end is counted in dollars. The
// tool records nothing for it; the daemon adds the tokens the proxy counted,
// at the reservation rates, to the ledger of the session, under the mark of an
// estimate.
func TestCutResponseIsCountedInDollars(t *testing.T) {
	e := newGateEnv(t)
	e.d.ledger.last = map[string]adapters.SessionResult{}
	got := e.callStream(cutStart+strings.Repeat(cutText, 40), true)
	require.Equal(t, int64(1), got.Cut)

	want := limits.CutCost(got.InputTokens, got.OutputTokens, got.CacheReadTokens, got.CacheWriteTokens, 0)
	assert.Positive(t, want)
	assert.InDelta(t, want, e.waitEstimated(), 1e-9)
	assert.InDelta(t, want, e.total().CostUSD, 1e-9, "the totals of the session and of the day see it")
	day, err := e.usage.DayCost(e.ctx, domain.UsageDay(time.Now()), "p1")
	require.NoError(t, err)
	assert.InDelta(t, want, day, 1e-9)
	require.Eventually(t, func() bool {
		s, err := e.d.opts.Sessions.Get(e.ctx, "ses_a")
		return err == nil && s.Cost > 0
	}, 5*time.Second, 5*time.Millisecond, "the cost shown for the session")
}

func TestCompleteResponseIsNotEstimated(t *testing.T) {
	e := newGateEnv(t)
	e.d.ledger.last = map[string]adapters.SessionResult{}
	got := e.callStream(cutStart+strings.Repeat(cutText, 40)+cutStop, false)
	require.Zero(t, got.Cut)
	require.Eventually(t, func() bool { return e.held() == 0 }, 5*time.Second, 5*time.Millisecond)
	assert.Zero(t, e.estimated())
	assert.Zero(t, e.total().CostUSD)
}

// A cut response counts against the budget: it raises the decision, though
// the tool reported nothing. Bounded by a tenth of the allowance, like a
// reservation.
func TestCutResponseRaisesTheBudgetDecision(t *testing.T) {
	e := newGateEnv(t)
	e.d.ledger.last = map[string]adapters.SessionResult{}
	e.budget(0.5)
	e.spend(0.46)
	require.Zero(t, e.openBudget())
	got := e.callStream(cutStart+strings.Repeat(cutText, 40), true)
	require.Greater(t, limits.CutCost(got.InputTokens, got.OutputTokens, 0, 0, 0), 0.05, "above the bound")
	assert.InDelta(t, 0.05, e.waitEstimated(), 1e-9, "a tenth of the allowance of 0.5 USD")
	require.Eventually(t, func() bool { return e.openBudget() == 1 }, 5*time.Second, 5*time.Millisecond, "0.46 + 0.05 passes the budget")
	assert.InDelta(t, 0.51, e.total().CostUSD, 1e-9)
}

// The report of the tool takes the place of the estimate, so that a step
// that the tool does report after all is not counted twice: the cost of the
// minute is the larger of the two. After the minute, the estimate is final.
func TestToolReportTakesThePlaceOfTheEstimate(t *testing.T) {
	e := newGateEnv(t)
	e.d.ledger.last = map[string]adapters.SessionResult{}
	now := time.Now()
	e.d.cuts.now = func() time.Time { return now }
	got := e.callStream(cutStart+strings.Repeat(cutText, 40), true)
	est := limits.CutCost(got.InputTokens, got.OutputTokens, got.CacheReadTokens, got.CacheWriteTokens, 0)
	require.InDelta(t, est, e.waitEstimated(), 1e-9)

	// The tool reports a part of the estimate for the session: counted once.
	part := est / 4
	e.w.account(e.ctx, "ses_a", "ses_a", adapters.SessionResult{Cost: part})
	assert.InDelta(t, est-part, e.estimated(), 1e-9)
	assert.InDelta(t, est, e.total().CostUSD, 1e-9, "reported + what is left of the estimate: the estimate")

	// More than what is left: the estimate is gone, the report counts.
	more := est * 2
	e.w.account(e.ctx, "ses_a", "ses_a", adapters.SessionResult{Cost: part + more})
	assert.InDelta(t, 0.0, e.estimated(), 1e-9)
	assert.InDelta(t, part+more, e.total().CostUSD, 1e-9, "the larger of the report and the estimate, not their sum")
}

func TestEstimateIsFinalAfterTheWindow(t *testing.T) {
	e := newGateEnv(t)
	e.d.ledger.last = map[string]adapters.SessionResult{}
	now := time.Now()
	e.d.cuts.now = func() time.Time { return now }
	got := e.callStream(cutStart+strings.Repeat(cutText, 40), true)
	est := limits.CutCost(got.InputTokens, got.OutputTokens, got.CacheReadTokens, got.CacheWriteTokens, 0)
	require.InDelta(t, est, e.waitEstimated(), 1e-9)

	now = now.Add(cutAbsorbWindow + time.Second)
	e.w.account(e.ctx, "ses_a", "ses_a", adapters.SessionResult{Cost: 3 * est})
	assert.InDelta(t, est, e.estimated(), 1e-9, "a report after the minute does not replace it")
	assert.InDelta(t, 4*est, e.total().CostUSD, 1e-9)
}

func TestCutEstimatesAbsorb(t *testing.T) {
	now := time.Now()
	c := &cutEstimates{now: func() time.Time { return now }}
	c.add("ses_a", "2026-10-20", 0.5)
	now = now.Add(10 * time.Second)
	c.add("ses_a", "2026-10-20", 0.2)
	c.add("ses_b", "2026-10-20", 1)

	assert.Nil(t, c.absorb("ses_a", 0), "nothing reported")
	assert.Nil(t, c.absorb("ses_c", 5), "another session")
	taken := c.absorb("ses_a", 0.6)
	assert.InDelta(t, 0.6, taken["2026-10-20"], 1e-9, "oldest first: the 0.5, then 0.1 of the 0.2")
	taken = c.absorb("ses_a", 5)
	assert.InDelta(t, 0.1, taken["2026-10-20"], 1e-9, "what is left of the second")
	assert.Nil(t, c.absorb("ses_a", 5), "nothing left")
	assert.InDelta(t, 1.0, c.absorb("ses_b", 1)["2026-10-20"], 1e-9, "the other session is untouched")

	c.add("ses_a", "2026-10-20", 0.4)
	now = now.Add(cutAbsorbWindow + time.Second)
	assert.Nil(t, c.absorb("ses_a", 5), "expired")
	assert.Empty(t, c.open, "forgotten")

	// The day of the estimate is kept: the ledger row to correct.
	c.add("ses_a", "2026-10-19", 0.3)
	taken = c.absorb("ses_a", 0.1)
	assert.Equal(t, map[string]float64{"2026-10-19": 0.1}, taken)
}

func TestCutRoot(t *testing.T) {
	w := &watcher{srv: domain.Server{GroupKey: "g1"}, tracks: map[string]*sessionTrack{"ses_a": {}}}
	assert.Equal(t, "ses_a", w.cutRoot([]string{"ses_a"}))
	assert.Equal(t, "ses_b", w.cutRoot([]string{"ses_b", "ses_a"}), "the first one working (those with a budget come first)")
	assert.Equal(t, "ses_a", w.cutRoot(nil), "none working: the only session of the group")
	w.tracks["ses_b"] = &sessionTrack{}
	assert.Empty(t, w.cutRoot(nil), "none working and several sessions: no guess")
}
