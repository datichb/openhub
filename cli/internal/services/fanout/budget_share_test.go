package fanout

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
)

func TestChildShare(t *testing.T) {
	cases := []struct {
		name                  string
		budget, spent, claims float64
		pending, running      int
		wantShare             float64
		wantOK                bool
	}{
		{"three children, nothing spent", 3, 0, 0, 3, 0, 1, true},
		{"the second sees the claim of the first", 3, 0, 1, 2, 1, 1, true},
		{"the last has what the others hold not", 3, 0, 2, 1, 2, 1, true},
		{"spent counts", 3, 0.6, 0, 2, 0, 1.2, true},
		{"a child ended under its share: the next ones get more", 3, 0.2, 2, 1, 2, 0.8, true},
		{"alone", 10, 1, 0, 1, 0, 9, true},
		{"no pending count: at least itself", 1, 0, 0, 0, 0, 1, true},
		{"everything is held by the running children: wait", 1, 0.5, 0.5, 2, 2, 0, false},
		{"over-held: wait", 1, 0.5, 0.7, 1, 1, 0, false},
		{"nothing running and nothing free: the minimum", 1, 1, 0, 2, 0, minChildBudget, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			share, ok := childShare(c.budget, c.spent, c.claims, c.pending, c.running)
			assert.Equal(t, c.wantOK, ok)
			assert.InDelta(t, c.wantShare, share, 1e-9)
		})
	}
}

// F-04: children at the same time do not each get all that remains. With a
// budget of 3 USD and three children, they get 1 USD each, not 3 each.
func TestDriverChildrenShareTheBudget(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2", "ev-3")
	f.launcher.together = 3 // the three children run at the same time
	for i, ticket := range []string{"ev-1", "ev-2", "ev-3"} {
		f.launcher.work[ticket] = [2]string{string(rune('a'+i)) + ".go", "package calc\n"}
	}
	p, _ := f.store.Get(context.Background(), "ses_parent")
	p.Fanout.BudgetUSD = 3
	require.NoError(t, f.store.SetSessionFanout(context.Background(), p.ID, p.Fanout))

	fo := f.runDriver(t)
	require.Equal(t, domain.FanoutReady, fo.Status)
	f.launcher.mu.Lock()
	defer f.launcher.mu.Unlock()
	assert.Equal(t, 3, f.launcher.maxSeen, "at the same time")
	require.Len(t, f.launcher.started, 3)
	total := 0.0
	for _, req := range f.launcher.started {
		assert.InDelta(t, 1.0, req.BudgetUSD, 1e-9, req.Ticket)
		assert.Less(t, req.BudgetUSD, 3.0)
		total += req.BudgetUSD
	}
	assert.LessOrEqual(t, total, 3.0+1e-9, "the shares together never exceed the budget")
	for _, c := range fo.Children {
		assert.InDelta(t, 1.0, c.BudgetUSD, 1e-9, "the share is kept in the record")
	}
}

// What a child did not spend goes to the children after it: the share is
// computed again at each start.
func TestDriverUnspentShareGoesToTheNextChildren(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2", "ev-3")
	f.wf.Fanout.Parallel = 1
	f.launcher.costOf = map[string]float64{"ev-1": 0.2, "ev-2": 0.2, "ev-3": 0}
	for i, ticket := range []string{"ev-1", "ev-2", "ev-3"} {
		f.launcher.work[ticket] = [2]string{string(rune('a'+i)) + ".go", "package calc\n"}
	}
	p, _ := f.store.Get(context.Background(), "ses_parent")
	p.Fanout.BudgetUSD = 3
	require.NoError(t, f.store.SetSessionFanout(context.Background(), p.ID, p.Fanout))

	fo := f.runDriver(t)
	require.Equal(t, domain.FanoutReady, fo.Status)
	f.launcher.mu.Lock()
	defer f.launcher.mu.Unlock()
	require.Len(t, f.launcher.started, 3)
	byTicket := map[string]float64{}
	for _, req := range f.launcher.started {
		byTicket[req.Ticket] = req.BudgetUSD
	}
	assert.InDelta(t, 1.0, byTicket["ev-1"], 1e-9, "3 USD among 3 children")
	assert.InDelta(t, 1.4, byTicket["ev-2"], 1e-9, "(3 - 0.2) among the 2 left: the unspent 0.8 of the first is shared")
	assert.InDelta(t, 2.6, byTicket["ev-3"], 1e-9, "(3 - 0.4) for the last")
}

// When the running children hold all that is left, the next ones wait for
// one of them to end instead of starting with nothing.
func TestDriverWaitsWhenTheBudgetIsHeld(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2", "ev-3")
	for i, ticket := range []string{"ev-1", "ev-2", "ev-3"} {
		f.launcher.work[ticket] = [2]string{string(rune('a'+i)) + ".go", "package calc\n"}
	}
	p, _ := f.store.Get(context.Background(), "ses_parent")
	p.Fanout.BudgetUSD = 0.025
	require.NoError(t, f.store.SetSessionFanout(context.Background(), p.ID, p.Fanout))

	fo := f.runDriver(t)
	require.Equal(t, domain.FanoutReady, fo.Status)
	f.launcher.mu.Lock()
	defer f.launcher.mu.Unlock()
	require.Len(t, f.launcher.started, 3)
	assert.InDelta(t, minChildBudget, f.launcher.started[0].BudgetUSD, 1e-9, "alone, the first has the minimum")
	assert.InDelta(t, 0.0125, f.launcher.started[1].BudgetUSD, 1e-9, "the others waited for its claim to be released")
	assert.InDelta(t, 0.0125, f.launcher.started[2].BudgetUSD, 1e-9)
	assert.LessOrEqual(t, f.launcher.maxSeen, 2, "the second did not start while the first held the budget")
}

// Without a global budget, no cap is given.
func TestDriverNoBudgetNoShare(t *testing.T) {
	f := newFixture(t, "ev-1", "ev-2")
	f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
	f.launcher.work["ev-2"] = [2]string{"b.go", "package calc\n"}
	f.runDriver(t)
	for _, req := range f.launcher.started {
		assert.Zero(t, req.BudgetUSD)
	}
}

// A driver that stopped between the reservation of the share and the start
// of the child leaves it "running" without a session: the next driver starts
// it again.
func TestDriverRestartsAChildReservedButNeverStarted(t *testing.T) {
	f := newFixture(t, "ev-1")
	f.launcher.work["ev-1"] = [2]string{"a.go", "package calc\n"}
	p, _ := f.store.Get(context.Background(), "ses_parent")
	p.Fanout.BudgetUSD = 2
	p.Fanout.Status = domain.FanoutRunning
	p.Fanout.Children = []domain.FanoutChild{{Ticket: "ev-1", State: domain.ChildRunning, BudgetUSD: 2}}
	require.NoError(t, f.store.SetSessionFanout(context.Background(), p.ID, p.Fanout))

	fo := f.runDriver(t)
	c, ok := fo.Child("ev-1")
	require.True(t, ok)
	assert.Equal(t, domain.ChildMerged, c.State)
	require.Len(t, f.launcher.started, 1)
	assert.InDelta(t, 2.0, f.launcher.started[0].BudgetUSD, 1e-9, "a new share, computed again")
}
