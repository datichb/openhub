package daemon

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/limits"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

// F-04: a step that delegates is checked while it runs. The cost of a
// subagent is read when its usage changes, and the decision is raised at
// once: the next subagent is interrupted, the root step has not ended.
func TestSubagentUsageRaisesTheDecisionMidStep(t *testing.T) {
	for _, tc := range []struct {
		name string
		// signal is what tells the daemon the subagent spent.
		signal adapters.ToolEvent
	}{
		{"usage of a running subagent", adapters.ToolEvent{Kind: adapters.EventUsage, SessionID: "ses_c1"}},
		{"end of a subagent", adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_c1", Outcome: "succeeded"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newLimitEnv(t, "ses_a")
			require.NoError(t, limits.Save(e.dir, "ses_a", limits.Resolve(limits.Input{Workflow: limits.Limits{SessionBudgetUSD: 1}, ProjectID: "p1"})))

			// The root step starts and delegates; it does not end in this test.
			e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_a"}
			e.events <- adapters.ToolEvent{Kind: adapters.EventSessionCreated, SessionID: "ses_c1", ParentID: "ses_a"}
			e.ad.set(func() { e.ad.usage["ses_c1"] = adapters.SessionResult{Cost: 1.2, TokensIn: 1200} })
			e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_c1"}
			e.events <- tc.signal

			require.Eventually(t, func() bool { return len(e.openBudget("ses_a")) == 1 }, 5*time.Second, 10*time.Millisecond,
				"the decision is raised while the root step runs")
			assert.Empty(t, e.ad.interrupted(), "nothing was running that the decision stops")

			// The next delegation of the same step is stopped.
			e.events <- adapters.ToolEvent{Kind: adapters.EventSessionCreated, SessionID: "ses_c2", ParentID: "ses_a"}
			e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_c2"}
			require.Eventually(t, func() bool {
				for _, id := range e.ad.interrupted() {
					if id == "ses_c2" {
						return true
					}
				}
				return false
			}, 5*time.Second, 10*time.Millisecond, "the second subagent is interrupted")
			assert.Len(t, e.openBudget("ses_a"), 1)
		})
	}
}

// A dismissed decision lets the step go on: a subagent spending meanwhile
// does not open it again; the end of the step does.
func TestDismissedDecisionNotReopenedMidStep(t *testing.T) {
	e := newLimitEnv(t, "ses_a")
	require.NoError(t, limits.Save(e.dir, "ses_a", limits.Resolve(limits.Input{Workflow: limits.Limits{SessionBudgetUSD: 1}, ProjectID: "p1"})))
	e.turn("ses_a", 1.2)
	require.Eventually(t, func() bool { return len(e.openBudget("ses_a")) == 1 }, 5*time.Second, 10*time.Millisecond)
	dec := e.openBudget("ses_a")[0]
	_, err := e.decisions.Resolve(e.ctx, dec.ID, domain.ResolvedByOh, &domain.DecisionResolution{Decision: "dismiss"}, time.Now())
	require.NoError(t, err)

	e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_a"}
	e.events <- adapters.ToolEvent{Kind: adapters.EventSessionCreated, SessionID: "ses_c1", ParentID: "ses_a"}
	e.ad.set(func() { e.ad.usage["ses_c1"] = adapters.SessionResult{Cost: 0.3} })
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_c1"}
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_c1", Outcome: "succeeded"}
	// The subagent was accounted (the next event is handled after it).
	require.Eventually(t, func() bool {
		tot, err := e.usage.ToolSessionTotal(e.ctx, "ses_c1")
		return err == nil && tot.CostUSD > 0
	}, 5*time.Second, 10*time.Millisecond)
	assert.Empty(t, e.openBudget("ses_a"), "one more step: not asked again before it ends")

	e.events <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_a", Outcome: "succeeded"}
	require.Eventually(t, func() bool { return len(e.openBudget("ses_a")) == 1 }, 5*time.Second, 10*time.Millisecond,
		"asked again at the end of the step")
}

// gateEnv is a daemon with the budget gate in front of a real proxy, and a
// provider that answers when the test lets it.
type gateEnv struct {
	t         *testing.T
	ctx       context.Context
	d         *Daemon
	w         *watcher
	usage     *sqlite.UsageStore
	decisions *sqlite.DecisionStore
	dir       string
	proxy     *credproxy.Proxy
	token     string
	entered   chan struct{} // a request reached the provider
	release   chan struct{} // closed: the provider answers
}

func newGateEnv(t *testing.T) *gateEnv {
	t.Helper()
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	e := &gateEnv{t: t, ctx: ctx, usage: sqlite.NewUsageStore(st), decisions: sqlite.NewDecisionStore(st), dir: filepath.Join(t.TempDir(), "sessions"),
		entered: make(chan struct{}, 8), release: make(chan struct{})}
	sessions := sqlite.NewSessionStore(st)
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive}))
	e.d = &Daemon{opts: Options{Usage: e.usage, Sessions: sessions, Decisions: e.decisions, SessionsDir: e.dir},
		proxy: credproxy.New(), watchers: map[string]*watcher{}, feed: newHub()}
	e.w = &watcher{d: e.d, srv: domain.Server{GroupKey: "g1", ProjectID: "p1"}, tracks: map[string]*sessionTrack{"ses_a": {executing: true}}, known: map[string]bool{"ses_a": true}}
	e.d.watchers["g1"] = e.w
	e.d.proxy.Gate = e.d.admitModel
	require.NoError(t, e.d.proxy.Start("127.0.0.1:0"))
	t.Cleanup(func() { _ = e.d.proxy.Close(context.Background()) })

	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		e.entered <- struct{}{}
		<-e.release
		fmt.Fprint(w, `{"usage":{"input_tokens":10,"output_tokens":10}}`)
	}))
	t.Cleanup(up.Close)
	var err error
	e.token, err = e.d.proxy.Issue(credproxy.Grant{SessionID: "g1", Provider: credproxy.ProviderAnthropic,
		Upstream: credproxy.Upstream{BaseURL: up.URL, Auth: credproxy.HeaderAuth{Name: "x-api-key", Value: "k"}}})
	require.NoError(t, err)
	return e
}

func (e *gateEnv) budget(usd float64) {
	e.t.Helper()
	require.NoError(e.t, limits.Save(e.dir, "ses_a", limits.Resolve(limits.Input{Workflow: limits.Limits{SessionBudgetUSD: usd}, ProjectID: "p1"})))
}

func (e *gateEnv) spend(usd float64) {
	e.t.Helper()
	require.NoError(e.t, e.usage.AddSession(e.ctx, domain.SessionUsage{Day: domain.UsageDay(time.Now()), SessionID: "ses_a", RootID: "ses_a", ProjectID: "p1", CostUSD: usd}))
}

// call sends a model request through the proxy (max_tokens: 16000, which
// the reservation rates price above the allowance).
func (e *gateEnv) call() (int, http.Header, string) {
	e.t.Helper()
	req, err := http.NewRequest(http.MethodPost, e.proxy.BaseURL(credproxy.ProviderAnthropic)+"/messages", strings.NewReader(`{"model":"m","max_tokens":16000}`))
	require.NoError(e.t, err)
	req.Header.Set("x-api-key", e.token)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(e.t, err)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(b)
}

func (e *gateEnv) held() float64 {
	e.d.reservations.mu.Lock()
	defer e.d.reservations.mu.Unlock()
	total := 0.0
	for _, v := range e.d.reservations.held {
		total += v
	}
	return total
}

func (e *gateEnv) openBudget() int {
	open, err := e.decisions.ListOpen(e.ctx, domain.DecisionFilter{SessionID: "ses_a"})
	require.NoError(e.t, err)
	n := 0
	for _, d := range open {
		if isLimitDecision(d) {
			n++
		}
	}
	return n
}

// S-27: two requests that would pass the budget together are not both
// admitted. The second one is told to retry; it goes through once the first
// is over, when its reservation is replaced by what it really cost.
func TestGateReservesTheBudgetOfConcurrentRequests(t *testing.T) {
	e := newGateEnv(t)
	e.proxy = e.d.proxy
	e.budget(1)
	e.spend(0.85) // 0.15 left; a reservation is 0.10 (a tenth of the budget)

	first := make(chan int, 1)
	go func() { status, _, _ := e.call(); first <- status }()
	<-e.entered
	assert.InDelta(t, 0.1, e.held(), 1e-9, "the first request holds a reservation")

	status, hdr, msg := e.call()
	assert.Equal(t, http.StatusTooManyRequests, status, msg)
	assert.NotEmpty(t, hdr.Get("Retry-After"))
	assert.Zero(t, e.openBudget(), "momentary: no budget decision")

	close(e.release)
	assert.Equal(t, http.StatusOK, <-first)
	require.Eventually(t, func() bool { return e.held() == 0 }, 5*time.Second, 5*time.Millisecond, "released at the end")
	status, _, msg = e.call()
	assert.Equal(t, http.StatusOK, status, msg)
}

// A request alone goes through whatever its estimate (soft cap), even when
// the budget left is smaller.
func TestGateAdmitsALoneRequest(t *testing.T) {
	e := newGateEnv(t)
	e.proxy = e.d.proxy
	e.budget(1)
	e.spend(0.99)
	close(e.release)
	status, _, msg := e.call()
	assert.Equal(t, http.StatusOK, status, msg)
}

// F-04: once the budget is spent, the next model request is refused, the
// decision is raised, and the step is marked as interrupted by the budget
// (no error decision; it resumes after the answer). Dismissed, the step goes
// on. No working session, no budget: nothing is checked.
func TestGateRefusesASpentBudget(t *testing.T) {
	e := newGateEnv(t)
	e.proxy = e.d.proxy
	close(e.release)
	e.budget(1)
	e.spend(1.2)

	status, _, msg := e.call()
	assert.Equal(t, http.StatusPaymentRequired, status)
	assert.Contains(t, msg, "ses_a")
	require.Equal(t, 1, e.openBudget(), "decision raised by the request")
	e.w.mu.Lock()
	assert.True(t, e.w.tracks["ses_a"].held, "interrupted by the budget, not failed")
	e.w.mu.Unlock()
	assert.Zero(t, e.held(), "a refused request holds nothing")

	status, _, _ = e.call()
	assert.Equal(t, http.StatusPaymentRequired, status, "while the decision is open")

	// Dismissed: one more step.
	open, err := e.decisions.ListOpen(e.ctx, domain.DecisionFilter{SessionID: "ses_a"})
	require.NoError(t, err)
	for _, d := range open {
		_, err := e.decisions.Resolve(e.ctx, d.ID, domain.ResolvedByOh, &domain.DecisionResolution{Decision: "dismiss"}, time.Now())
		require.NoError(t, err)
	}
	status, _, msg = e.call()
	assert.Equal(t, http.StatusOK, status, msg)
	assert.Zero(t, e.openBudget(), "not asked again in the middle of the step")
	require.True(t, e.w.raiseBudget(e.ctx, "ses_a", true), "asked again at the end of the step")

	// Raised: the budget allows it again.
	open, _ = e.decisions.ListOpen(e.ctx, domain.DecisionFilter{SessionID: "ses_a"})
	for _, d := range open {
		_, err := e.decisions.Resolve(e.ctx, d.ID, domain.ResolvedByOh, &domain.DecisionResolution{Decision: "raise"}, time.Now())
		require.NoError(t, err)
	}
	require.NoError(t, e.usage.AddBudgetExtra(e.ctx, limits.SessionScope("ses_a"), "", 1))
	status, _, msg = e.call()
	assert.Equal(t, http.StatusOK, status, msg)

	// No session of the group is working: the request cannot be attributed.
	e.w.mu.Lock()
	e.w.tracks["ses_a"].executing = false
	e.w.mu.Unlock()
	e.spend(5)
	status, _, msg = e.call()
	assert.Equal(t, http.StatusOK, status, msg)
}

func TestGateWithoutBudgetAddsNothing(t *testing.T) {
	e := newGateEnv(t)
	e.proxy = e.d.proxy
	close(e.release)
	status, _, msg := e.call()
	assert.Equal(t, http.StatusOK, status, msg)
	assert.Zero(t, e.held())
}
