package fanout

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

// addSession registers a session with a cost, to be counted as spent.
func (f *fixture) addSession(t *testing.T, id string, cost float64) {
	t.Helper()
	require.NoError(t, f.store.Create(context.Background(), &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning,
		State: domain.RunActive, Cost: cost, LaunchPath: f.project}))
}

// The cap of a merger is the rule of the children (childShare), the merger
// counting as one more child to start: it leaves their part to the children
// not started and keeps apart what the running children may still spend.
func TestAuxShare(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "ev-1")
	f.addSession(t, "ses_c1", 0.30)
	f.addSession(t, "ses_c2", 0.10)
	f.addSession(t, "ses_m1", 0.20)

	running := func(ticket, session string, share float64) domain.FanoutChild {
		return domain.FanoutChild{Ticket: ticket, Session: session, State: domain.ChildRunning, BudgetUSD: share}
	}
	cases := []struct {
		name      string
		budget    float64
		children  []domain.FanoutChild
		merging   string
		wantShare float64
		wantOK    bool
	}{
		{"alone: all that is left", 3, []domain.FanoutChild{running("a", "ses_c1", 1)}, "a", 3 - 0.3, true},
		{"a running child keeps what it may still spend", 3, []domain.FanoutChild{running("a", "ses_c1", 1), running("b", "ses_c2", 1)}, "a", 3 - 0.4 - 0.9, true},
		{"the child being merged holds nothing", 3, []domain.FanoutChild{running("a", "ses_c1", 1), running("b", "", 1)}, "a", 3 - 0.3 - 1, true},
		{"the children not started keep their part", 3, []domain.FanoutChild{running("a", "ses_c1", 1),
			{Ticket: "b", State: domain.ChildPending}, {Ticket: "c", State: domain.ChildPending}}, "a", (3 - 0.3) / 3, true},
		{"done children hold nothing, only what they spent counts", 3, []domain.FanoutChild{running("a", "ses_c1", 1),
			{Ticket: "b", Session: "ses_c2", State: domain.ChildMerged, BudgetUSD: 1}}, "a", 3 - 0.4, true},
		{"the merger of another conflict still running keeps its share", 3, []domain.FanoutChild{running("a", "ses_c1", 1),
			{Ticket: "b", Session: "ses_c2", Merger: "ses_m1", State: domain.ChildRunning, BudgetUSD: 1, MergerBudgetUSD: 1}}, "a",
			3 - (0.3 + 0.1 + 0.2) - (0.9 + 0.8), true},
		{"all that is left is held by running children: not launched", 1, []domain.FanoutChild{running("a", "ses_c1", 0.3), running("b", "ses_c2", 1)}, "a", 0, false},
		{"nothing left and nothing running: the minimum", 0.3, []domain.FanoutChild{running("a", "ses_c1", 0.3)}, "a", minChildBudget, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := &domain.SessionFanout{BudgetUSD: c.budget, Children: c.children}
			share, ok := f.driver.auxShare(context.Background(), "ses_parent", rec, c.merging)
			assert.Equal(t, c.wantOK, ok)
			assert.InDelta(t, c.wantShare, share, 1e-9)
		})
	}
}

// A child that starts while a merger runs sees the share of the merger as a
// claim: the shares together never pass the budget.
func TestReserveShareCountsTheMergerClaim(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "ev-1", "ev-2")
	f.addSession(t, "ses_c1", 0.2)
	f.addSession(t, "ses_m1", 0.5)
	p, _ := f.store.Get(context.Background(), "ses_parent")
	p.Fanout.BudgetUSD = 3
	p.Fanout.Status = domain.FanoutRunning
	p.Fanout.Children = []domain.FanoutChild{
		{Ticket: "ev-1", Session: "ses_c1", Merger: "ses_m1", State: domain.ChildRunning, BudgetUSD: 1, MergerBudgetUSD: 1.6},
		{Ticket: "ev-2", State: domain.ChildPending},
	}
	require.NoError(t, f.store.SetSessionFanout(context.Background(), p.ID, p.Fanout))

	share, ok, err := f.driver.reserveShare(context.Background(), "ses_parent", "ev-2")
	require.NoError(t, err)
	require.True(t, ok)
	// spent 0.7; held: 0.8 by the child, 1.1 by its merger.
	assert.InDelta(t, 3-0.7-0.8-1.1, share, 1e-9)
}

func TestReserveMergerShareRecordsItInTheChild(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "ev-1", "ev-2")
	p, _ := f.store.Get(context.Background(), "ses_parent")
	p.Fanout.BudgetUSD = 2
	p.Fanout.Children = []domain.FanoutChild{{Ticket: "ev-1", State: domain.ChildRunning, BudgetUSD: 1}, {Ticket: "ev-2", State: domain.ChildPending}}
	require.NoError(t, f.store.SetSessionFanout(context.Background(), p.ID, p.Fanout))

	share, ok, err := f.driver.reserveMergerShare(context.Background(), "ses_parent", "ev-1")
	require.NoError(t, err)
	require.True(t, ok)
	assert.InDelta(t, 1.0, share, 1e-9, "(2 - 0) among the pending child and the merger")
	p, _ = f.store.Get(context.Background(), "ses_parent")
	c, _ := p.Fanout.Child("ev-1")
	assert.InDelta(t, 1.0, c.MergerBudgetUSD, 1e-9)

	f.driver.releaseMergerShare(context.Background(), "ses_parent", "ev-1")
	p, _ = f.store.Get(context.Background(), "ses_parent")
	c, _ = p.Fanout.Child("ev-1")
	assert.Zero(t, c.MergerBudgetUSD)

	// No global budget: no share and nothing recorded.
	p.Fanout.BudgetUSD = 0
	require.NoError(t, f.store.SetSessionFanout(context.Background(), p.ID, p.Fanout))
	share, ok, err = f.driver.reserveMergerShare(context.Background(), "ses_parent", "ev-1")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Zero(t, share)
}

func conflictFixture(t *testing.T, budget float64, costs map[string]float64, tickets ...string) *fixture {
	t.Helper()
	f := newFixture(t, tickets...)
	f.launcher.together = 2
	f.launcher.costOf = costs
	f.launcher.work["ev-1"] = [2]string{"calc.go", "package calc\n\nfunc Sub() {}\n"}
	f.launcher.work["ev-2"] = [2]string{"calc.go", "package calc\n\nfunc Mul() {}\n"}
	f.launcher.work["ev-3"] = [2]string{"c.go", "package calc\n"}
	f.launcher.resolve = "package calc\n\nfunc Sub() {}\n\nfunc Mul() {}\n"
	p, _ := f.store.Get(context.Background(), "ses_parent")
	p.Fanout.BudgetUSD = budget
	require.NoError(t, f.store.SetSessionFanout(context.Background(), p.ID, p.Fanout))
	return f
}

func (f *fixture) mergerRequests() []LaunchRequest {
	f.launcher.mu.Lock()
	defer f.launcher.mu.Unlock()
	var out []LaunchRequest
	for _, r := range f.launcher.started {
		if r.Record.Role == domain.RoleMerger {
			out = append(out, r)
		}
	}
	return out
}

// The merger no longer gets "all that remains": a child still running keeps
// what it may spend (here 1 USD of the 3), and the merger gets the rest.
func TestDriverMergerLeavesTheShareOfRunningChildren(t *testing.T) {
	t.Parallel()
	f := conflictFixture(t, 3, map[string]float64{"ev-1": 0.2, "ev-2": 0.2, "ev-3": 0}, "ev-1", "ev-2", "ev-3")
	// ev-3 works until the merger started: it is running during the merge.
	mergerStarted := make(chan struct{})
	var once sync.Once
	f.launcher.onWork = func(req LaunchRequest) {
		switch {
		case req.Record.Role == domain.RoleMerger:
			once.Do(func() { close(mergerStarted) })
		case req.Ticket == "ev-3":
			select {
			case <-mergerStarted:
			case <-time.After(runGuard):
			}
		}
	}
	fo := f.runDriver(t)
	require.Equal(t, domain.FanoutReady, fo.Status)
	mergers := f.mergerRequests()
	require.Len(t, mergers, 1)
	// spent 0.4, held 1.0 by ev-3: 3 - 0.4 - 1.0. All that remained (2.6)
	// was given before.
	assert.InDelta(t, 1.6, mergers[0].BudgetUSD, 1e-9)
	assert.Less(t, mergers[0].BudgetUSD, 3-0.4)
	c := conflicted(t, fo)
	assert.Equal(t, domain.ChildMerged, c.State)
	assert.InDelta(t, 1.6, c.MergerBudgetUSD, 1e-9, "the share is kept in the record")
}

// The merger counts as one more child to start: the children not started
// keep their part.
func TestDriverMergerKeepsAPartForThePendingChildren(t *testing.T) {
	t.Parallel()
	f := conflictFixture(t, 3, map[string]float64{"ev-1": 0.2, "ev-2": 0.2, "ev-3": 0}, "ev-1", "ev-2", "ev-3")
	f.deps["ev-3"] = []string{"ev-1", "ev-2"} // ev-3 is not started while the conflict is being resolved
	fo := f.runDriver(t)
	require.Equal(t, domain.FanoutReady, fo.Status)
	mergers := f.mergerRequests()
	require.Len(t, mergers, 1)
	assert.InDelta(t, (3-0.4)/2, mergers[0].BudgetUSD, 1e-9, "(3 - 0.4) among ev-3 and the merger")
	f.launcher.mu.Lock()
	defer f.launcher.mu.Unlock()
	for _, r := range f.launcher.started {
		if r.Ticket == "ev-3" && r.Record.Role == domain.RoleChild {
			assert.InDelta(t, 2.6, r.BudgetUSD, 1e-9, "ev-3 has what is left once the merger spent nothing")
		}
	}
}

// When the budget left is held by running children, the merger is not
// launched (it cannot wait for them: they need the integration worktree to
// end): the conflict stays unresolved and the trace says why.
func TestDriverMergerNotLaunchedWhenTheBudgetIsHeld(t *testing.T) {
	t.Parallel()
	f := conflictFixture(t, 1, map[string]float64{"ev-1": 0.4, "ev-2": 0.4, "ev-3": 0}, "ev-1", "ev-2", "ev-3")
	// ev-3 runs until the conflict is recorded.
	f.launcher.onWork = func(req LaunchRequest) {
		if req.Ticket != "ev-3" {
			return
		}
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		deadline := time.After(runGuard)
		for {
			if p, err := f.store.Get(context.Background(), "ses_parent"); err == nil && p.Fanout != nil {
				for _, c := range p.Fanout.Children {
					if c.State == domain.ChildConflict {
						return
					}
				}
			}
			select {
			case <-tick.C:
			case <-deadline:
				return
			}
		}
	}
	fo := f.runDriver(t)
	assert.Empty(t, f.mergerRequests(), "no merger")
	c := conflicted(t, fo)
	assert.Equal(t, domain.ChildConflict, c.State)
	assert.Zero(t, c.MergerBudgetUSD)
	held := false
	for _, e := range fo.Events {
		if e.Kind == domain.FanoutEventBudget && e.Ticket == c.Ticket {
			held = true
			assert.Contains(t, e.Message, "0.80 / 1.00 USD")
		}
	}
	assert.True(t, held, "the trace says the budget was held")
}

// A fix round starts when every child is done: nothing is held, it gets what
// is left, as before.
func TestFixRoundGetsWhatIsLeft(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "ev-1")
	f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
	f.launcher.costOf = map[string]float64{"ev-1": 0.5}
	f.wf.Fanout.Fix, f.wf.Fanout.FixInput = "quick", "request"
	f.wf.MaxFixRounds = 1
	p, _ := f.store.Get(context.Background(), "ses_parent")
	p.Fanout.BudgetUSD = 2
	require.NoError(t, f.store.SetSessionFanout(context.Background(), p.ID, p.Fanout))
	ctx, cancel := context.WithTimeout(context.Background(), runGuard)
	done := make(chan error, 1)
	go func() { done <- f.driver.Run(ctx, "ses_parent") }()
	t.Cleanup(func() { cancel(); <-done })
	f.waitStatus(t, domain.FanoutReady)

	f.launcher.fix = [2]string{"fix.go", "package calc // fixed\n"}
	require.NoError(t, f.requestFix(t, "corrige"))
	f.waitStatus(t, domain.FanoutReady)
	f.launcher.mu.Lock()
	defer f.launcher.mu.Unlock()
	fixer := f.launcher.started[len(f.launcher.started)-1]
	require.Equal(t, domain.RoleFixer, fixer.Record.Role)
	assert.InDelta(t, 1.5, fixer.BudgetUSD, 1e-9, "2 - 0.5 spent by the child")
}
