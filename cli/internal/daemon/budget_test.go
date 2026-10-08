package daemon

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/limits"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// limitAdapter records the interrupts and the prompts sent by the daemon.
type limitAdapter struct {
	*fakeAdapter
	cmu        sync.Mutex
	interrupts []string
	prompts    map[string]string
}

func (a *limitAdapter) Control(_ context.Context, _ adapters.ServerHandle, id string, op adapters.ControlOp) error {
	a.cmu.Lock()
	defer a.cmu.Unlock()
	switch op.Kind {
	case adapters.ControlInterrupt:
		a.interrupts = append(a.interrupts, id)
	case adapters.ControlPrompt:
		a.prompts[id] = op.Text
	}
	return nil
}

func (a *limitAdapter) SendPrompt(_ context.Context, _ adapters.ServerHandle, id, prompt string) error {
	a.cmu.Lock()
	defer a.cmu.Unlock()
	a.prompts[id] = prompt
	return nil
}

func (a *limitAdapter) interrupted() []string {
	a.cmu.Lock()
	defer a.cmu.Unlock()
	return append([]string(nil), a.interrupts...)
}

func (a *limitAdapter) prompt(id string) (string, bool) {
	a.cmu.Lock()
	defer a.cmu.Unlock()
	p, ok := a.prompts[id]
	return p, ok
}

type limitEnv struct {
	t         *testing.T
	ctx       context.Context
	sessions  *sqlite.SessionStore
	servers   *sqlite.ServerStore
	decisions *sqlite.DecisionStore
	usage     *sqlite.UsageStore
	ad        *limitAdapter
	dir       string // sessions dir
	events    chan adapters.ToolEvent
	memoryMB  atomic.Int64
	paths     Paths
}

func newLimitEnv(t *testing.T, sessionIDs ...string) *limitEnv {
	t.Helper()
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	e := &limitEnv{t: t, ctx: ctx, sessions: sqlite.NewSessionStore(st), servers: sqlite.NewServerStore(st),
		decisions: sqlite.NewDecisionStore(st), usage: sqlite.NewUsageStore(st),
		ad: &limitAdapter{fakeAdapter: newFake(), prompts: map[string]string{}}, dir: filepath.Join(p.Dir, "sessions"), paths: p}
	require.NoError(t, e.servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	for _, id := range sessionIDs {
		require.NoError(t, e.sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunIdle}))
	}
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: e.servers, Sessions: e.sessions, Grants: sqlite.NewGrantStore(st),
			Decisions: e.decisions, Usage: e.usage, SessionsDir: e.dir, ServersDir: filepath.Join(p.Dir, "servers"),
			Tick: 50 * time.Millisecond, IdleAfter: time.Hour, IdleSleep: time.Hour,
			Adapter: func(string) adapters.ToolAdapter { return e.ad },
			Memory:  func(context.Context, []int) (int, error) { return int(e.memoryMB.Load()), nil }})
	}()
	t.Cleanup(func() { cancel(); <-done })
	c := NewClient(p)
	require.Eventually(t, func() bool { h, err := c.Health(ctx); return err == nil && !h.Restoring }, 5*time.Second, 20*time.Millisecond)
	select {
	case e.events = <-e.ad.events:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not subscribe")
	}
	e.events <- adapters.ToolEvent{Kind: adapters.EventConnected}
	return e
}

func (e *limitEnv) openBudget(id string) []domain.Decision {
	open, err := e.decisions.ListOpen(e.ctx, domain.DecisionFilter{SessionID: id})
	require.NoError(e.t, err)
	var out []domain.Decision
	for _, d := range open {
		if isLimitDecision(d) {
			out = append(out, d)
		}
	}
	return out
}

func (e *limitEnv) turn(id string, cost float64) {
	e.ad.set(func() { e.ad.usage[id] = adapters.SessionResult{Cost: cost, TokensIn: int64(cost * 1000)} })
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: id}
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: id, Outcome: "succeeded"}
}

// The session budget: the step that overruns it finishes, then a $ decision
// is raised; a new step while it is open is interrupted; a raise lets it go
// on, until the raised allowance is spent too. The usage (subagents
// included) is in the ledger.
func TestSessionBudgetDecisionAndHold(t *testing.T) {
	e := newLimitEnv(t, "ses_a")
	require.NoError(t, limits.Save(e.dir, "ses_a", limits.Resolve(limits.Input{Workflow: limits.Limits{SessionBudgetUSD: 1}, ProjectID: "p1"})))

	e.turn("ses_a", 0.4)
	time.Sleep(200 * time.Millisecond)
	assert.Empty(t, e.openBudget("ses_a"), "under budget")

	e.turn("ses_a", 1.2) // overruns during the step: no interrupt, decision at the end
	require.Eventually(t, func() bool { return len(e.openBudget("ses_a")) == 1 }, 3*time.Second, 20*time.Millisecond)
	assert.Empty(t, e.ad.interrupted(), "the step was allowed to finish")
	dec := e.openBudget("ses_a")[0]
	assert.Equal(t, limits.BudgetSession, dec.Payload.Data[limits.DataBudgetLimit])
	tot, err := e.usage.SessionTotal(e.ctx, "ses_a")
	require.NoError(t, err)
	assert.InDelta(t, 1.2, tot.CostUSD, 1e-9, "ledger = deltas of the reported cost")

	// A new step while the decision is open is interrupted; the decision stays.
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_a"}
	require.Eventually(t, func() bool { return len(e.ad.interrupted()) == 1 }, 3*time.Second, 20*time.Millisecond)
	assert.Len(t, e.openBudget("ses_a"), 1)
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_a", Outcome: "failed"}

	// Raised by one budget unit (oh budget raise / decision "raise").
	require.NoError(t, e.usage.AddBudgetExtra(e.ctx, limits.SessionScope("ses_a"), "", 1))
	_, err = e.decisions.Resolve(e.ctx, dec.ID, domain.ResolvedByOh, &domain.DecisionResolution{Decision: "raise"}, time.Now())
	require.NoError(t, err)
	e.turn("ses_a", 1.5)
	time.Sleep(200 * time.Millisecond)
	assert.Len(t, e.ad.interrupted(), 1, "no interrupt after the raise")
	assert.Empty(t, e.openBudget("ses_a"))

	// A subagent session spends for its root: over the raised allowance.
	e.events <- adapters.ToolEvent{Kind: adapters.EventSessionCreated, SessionID: "ses_child", ParentID: "ses_a"}
	e.ad.set(func() { e.ad.usage["ses_child"] = adapters.SessionResult{Cost: 0.6} })
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_a"}
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_child"}
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_child", Outcome: "succeeded"}
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_a", Outcome: "succeeded"}
	require.Eventually(t, func() bool { return len(e.openBudget("ses_a")) == 1 }, 3*time.Second, 20*time.Millisecond)
	tot, _ = e.usage.SessionTotal(e.ctx, "ses_a")
	assert.InDelta(t, 2.1, tot.CostUSD, 1e-9)
}

// A21: the session row holds the oh session total (root + subagents), as
// the budget counts it: what oh session list and the views show.
func TestSessionRowHoldsTheTotalCost(t *testing.T) {
	e := newLimitEnv(t, "ses_a")
	e.turn("ses_a", 0.88)
	require.Eventually(t, func() bool {
		s, err := e.sessions.Get(e.ctx, "ses_a")
		return err == nil && s.Cost > 0.87
	}, 3*time.Second, 20*time.Millisecond)

	e.events <- adapters.ToolEvent{Kind: adapters.EventSessionCreated, SessionID: "ses_dev", ParentID: "ses_a"}
	e.ad.set(func() { e.ad.usage["ses_dev"] = adapters.SessionResult{Cost: 1.94, TokensIn: 100} })
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_dev"}
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_dev", Outcome: "succeeded"}
	require.Eventually(t, func() bool {
		s, err := e.sessions.Get(e.ctx, "ses_a")
		return err == nil && s.Cost > 2.81 && s.Cost < 2.83
	}, 3*time.Second, 20*time.Millisecond, "the subagent cost counts at once")

	e.turn("ses_a", 1.0) // the root spends again: the total stays the total
	require.Eventually(t, func() bool {
		s, err := e.sessions.Get(e.ctx, "ses_a")
		return err == nil && s.Cost > 2.93 && s.Cost < 2.95
	}, 3*time.Second, 20*time.Millisecond)
}

// A22: after a raise answered in oh, the session is no longer « waiting »
// and the step interrupted while the decision was open is resumed.
func TestRaiseResumesTheInterruptedStep(t *testing.T) {
	e := newLimitEnv(t, "ses_a")
	require.NoError(t, limits.Save(e.dir, "ses_a", limits.Resolve(limits.Input{Workflow: limits.Limits{SessionBudgetUSD: 1}, ProjectID: "p1"})))
	e.turn("ses_a", 1.2)
	require.Eventually(t, func() bool { return len(e.openBudget("ses_a")) == 1 }, 3*time.Second, 20*time.Millisecond)
	dec := e.openBudget("ses_a")[0]
	c := NewClient(e.paths)

	// No interrupted step yet: a raise only updates the state.
	require.NoError(t, e.usage.AddBudgetExtra(e.ctx, limits.SessionScope("ses_a"), "", 1))
	_, err := e.decisions.Resolve(e.ctx, dec.ID, domain.ResolvedByOh, &domain.DecisionResolution{Decision: "raise"}, time.Now())
	require.NoError(t, err)
	require.NoError(t, c.SessionDecided(e.ctx, "ses_a"))
	s, err := e.sessions.Get(e.ctx, "ses_a")
	require.NoError(t, err)
	assert.Equal(t, domain.RunIdle, s.State, "not waiting without an open decision")
	_, prompted := e.ad.prompt("ses_a")
	assert.False(t, prompted)

	// Over the raised budget, the cp-2 correction step starts: interrupted.
	e.turn("ses_a", 2.3)
	require.Eventually(t, func() bool { return len(e.openBudget("ses_a")) == 1 }, 3*time.Second, 20*time.Millisecond)
	dec = e.openBudget("ses_a")[0]
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_a"}
	require.Eventually(t, func() bool { return len(e.ad.interrupted()) == 1 }, 3*time.Second, 20*time.Millisecond)
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_a", Outcome: "failed"}
	require.Eventually(t, func() bool {
		s, err := e.sessions.Get(e.ctx, "ses_a")
		return err == nil && s.State == domain.RunWaiting
	}, 3*time.Second, 20*time.Millisecond)

	require.NoError(t, e.usage.AddBudgetExtra(e.ctx, limits.SessionScope("ses_a"), "", 3))
	_, err = e.decisions.Resolve(e.ctx, dec.ID, domain.ResolvedByOh, &domain.DecisionResolution{Decision: "raise"}, time.Now())
	require.NoError(t, err)
	require.NoError(t, c.SessionDecided(e.ctx, "ses_a"))
	p, ok := e.ad.prompt("ses_a")
	require.True(t, ok, "the interrupted step is resumed")
	assert.Contains(t, p, "[oh]")
	s, err = e.sessions.Get(e.ctx, "ses_a")
	require.NoError(t, err)
	assert.NotEqual(t, domain.RunWaiting, s.State)
}

// The daily budget counts every session of its scope.
func TestDailyBudget(t *testing.T) {
	e := newLimitEnv(t, "ses_a", "ses_b")
	lim := limits.Resolve(limits.Input{Hub: limits.Limits{DailyBudgetUSD: 1}, ProjectID: "p1"})
	for _, id := range []string{"ses_a", "ses_b"} {
		require.NoError(t, limits.Save(e.dir, id, lim))
	}
	e.turn("ses_a", 0.7)
	time.Sleep(200 * time.Millisecond)
	assert.Empty(t, e.openBudget("ses_a"))
	e.turn("ses_b", 0.4)
	require.Eventually(t, func() bool { return len(e.openBudget("ses_b")) == 1 }, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, limits.BudgetDaily, e.openBudget("ses_b")[0].Payload.Data[limits.DataBudgetLimit])
}

// Queued first prompts are sent when a slot frees up, by priority.
func TestQueuedSessionsStartWhenASlotFrees(t *testing.T) {
	e := newLimitEnv(t, "ses_busy")
	ctx := e.ctx
	busy, _ := e.sessions.Get(ctx, "ses_busy")
	busy.State = domain.RunActive
	require.NoError(t, e.sessions.Update(ctx, busy))
	e.ad.set(func() { e.ad.active["ses_busy"] = true })
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_busy"}
	now := time.Now()
	for i, q := range []struct {
		id  string
		pri int
	}{{"ses_head", limits.PriorityHeadless}, {"ses_inter", limits.PriorityInteractive}} {
		require.NoError(t, e.sessions.Create(ctx, &domain.Session{ID: q.id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunQueued}))
		require.NoError(t, limits.SaveQueued(e.dir, q.id, limits.Queued{Prompt: "go " + q.id, Priority: q.pri, QueuedAt: now.Add(time.Duration(i) * time.Second), Scope: limits.ScopeGlobal, MaxActive: 1}))
	}
	time.Sleep(300 * time.Millisecond)
	_, sent := e.ad.prompt("ses_inter")
	assert.False(t, sent, "no free slot")
	s, _ := e.sessions.Get(ctx, "ses_inter")
	assert.Equal(t, domain.RunQueued, s.State, "kept queued by the watcher")
	srv, _ := e.servers.Get(ctx, "g1")
	assert.Equal(t, domain.ServerReady, srv.Status)

	// The working session ends its turn: the interactive one goes first.
	e.ad.set(func() { e.ad.active["ses_busy"] = false })
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_busy", Outcome: "succeeded"}
	require.Eventually(t, func() bool { p, ok := e.ad.prompt("ses_inter"); return ok && p == "go ses_inter" }, 10*time.Second, 20*time.Millisecond)
	// The prompt is sent before the queue entry is removed and the state
	// updated (kept queued when sending fails): wait for both.
	require.Eventually(t, func() bool {
		_, queued, _ := limits.LoadQueued(e.dir, "ses_inter")
		s, _ := e.sessions.Get(ctx, "ses_inter")
		return !queued && s.State == domain.RunActive
	}, 10*time.Second, 20*time.Millisecond)
	_, sent = e.ad.prompt("ses_head")
	assert.False(t, sent, "one slot only")
}

// Above the memory cap: queued sessions wait, an idle group sleeps.
func TestMemoryCap(t *testing.T) {
	e := newLimitEnv(t, "ses_a")
	e.memoryMB.Store(9000)
	require.NoError(t, limits.Save(e.dir, "ses_a", limits.Resolve(limits.Input{Hub: limits.Limits{MemoryMB: 8000}})))
	require.Eventually(t, func() bool {
		s, _ := e.servers.Get(e.ctx, "g1")
		return s.Status == domain.ServerSleeping
	}, 5*time.Second, 50*time.Millisecond, "idle group put to sleep over the cap")
}

func TestTreeRSS(t *testing.T) {
	ps := "  1     0  1000\n 10     1  2048\n 11    10  1024\n 12    11  1024\n 20     1  4096\n"
	assert.Equal(t, 4, treeRSS(ps, []int{10}))
	assert.Equal(t, 8, treeRSS(ps, []int{10, 20}))
	assert.Equal(t, 0, treeRSS(ps, []int{99}))
}

// The proxy traffic of a group is persisted and continued by its next grant
// (daemon restart, server restart).
func TestProxyUsageLedger(t *testing.T) {
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	us := sqlite.NewUsageStore(st)
	d := &Daemon{opts: Options{Usage: us}, proxy: credproxy.New()}
	d.onProxyUsage("g1", credproxy.Usage{Requests: 1})
	d.onProxyUsage("g1", credproxy.Usage{InputTokens: 100, OutputTokens: 20})
	d.flushProxyUsage(ctx)
	d.onProxyUsage("g1", credproxy.Usage{Requests: 1, InputTokens: 5})
	tot, err := us.ProxyTotal(ctx, "g1")
	require.NoError(t, err)
	assert.Equal(t, domain.ProxyUsage{Requests: 1, TokensIn: 100, TokensOut: 20}, tot)

	tok := credproxy.NewToken()
	require.NoError(t, d.proxy.IssueWithToken(tok, credproxy.Grant{SessionID: "g1", Provider: credproxy.ProviderOpenAI,
		Upstream: credproxy.Upstream{BaseURL: "https://x", Auth: credproxy.BearerAuth{Token: "s"}}}))
	d.continueProxyUsage(ctx, "g1", credproxy.TokenHash(tok))
	u, ok := d.proxy.Usage(tok)
	require.True(t, ok)
	assert.Equal(t, credproxy.Usage{Requests: 2, InputTokens: 105, OutputTokens: 20}, u, "stored + not yet flushed")
}

// A fork reports the copied history from its first turn: only what it spends
// above that baseline is added to the ledger (v5 finalisation, Q3-6).
func TestForkBaselineNotCountedTwice(t *testing.T) {
	e := newLimitEnv(t, "ses_a")
	e.turn("ses_a", 0.5)
	require.Eventually(t, func() bool {
		tot, _ := e.usage.SessionTotal(e.ctx, "ses_a")
		return tot.CostUSD == 0.5
	}, 3*time.Second, 20*time.Millisecond)

	// ses_f forked from ses_a: it reports 0.5 copied, then spends 0.2.
	// As runsvc.ForkSession: the baseline is saved before the oh session.
	e.ad.set(func() { e.ad.usage["ses_f"] = adapters.SessionResult{Cost: 0.5, TokensIn: 500} })
	require.NoError(t, limits.SaveBaseline(e.dir, "ses_f", limits.UsageBaseline{CostUSD: 0.5, TokensIn: 500}))
	require.NoError(t, e.sessions.Create(e.ctx, &domain.Session{ID: "ses_f", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunIdle}))
	e.turn("ses_f", 0.7)
	require.Eventually(t, func() bool {
		tot, _ := e.usage.SessionTotal(e.ctx, "ses_f")
		return tot.CostUSD > 0
	}, 3*time.Second, 20*time.Millisecond)
	tot, err := e.usage.SessionTotal(e.ctx, "ses_f")
	require.NoError(t, err)
	assert.InDelta(t, 0.2, tot.CostUSD, 1e-9)
	assert.Equal(t, int64(200), tot.TokensIn)
	day, err := e.usage.DayCost(e.ctx, domain.UsageDay(time.Now()), "")
	require.NoError(t, err)
	assert.InDelta(t, 0.7, day, 1e-9, "0.5 + 0.2, the copied history once")
}
