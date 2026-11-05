package daemon

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// sleepingWithADecision starts a daemon, opens a permission on ses_w, lets the
// group go to sleep (no oh UI present) and returns the environment.
func sleepingWithADecision(t *testing.T, sent *notes) (*lcEnv, string) {
	t.Helper()
	e := startLifecycleWith(t, 300*time.Millisecond, "g1", func(e *lcEnv, o *Options) {
		o.Decisions = e.decisions
		o.Notify = sent.add
		o.NotifyWindow = 50 * time.Millisecond
		o.ProjectName = func(context.Context, string) string { return "openhub" }
	})
	require.NoError(t, e.sessions.Create(e.ctx, &domain.Session{ID: "ses_w", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive, EntryAgent: "lead"}))
	e.ad.set(func() { e.ad.pending["ses_w"] = 1 })
	e.events <- adapters.ToolEvent{Kind: adapters.EventDecisionAsked, SessionID: "ses_w"}
	id := domain.DecisionID(domain.DecisionPermission, "ses_w", "per_0")
	require.Eventually(t, func() bool { d, err := e.decisions.Get(e.ctx, id); return err == nil && d.Open() }, 3*time.Second, 20*time.Millisecond)
	// The decision opened while nobody listened is notified once; the sleep adds its own note.
	require.Eventually(t, func() bool { return e.serverStatus("g1") == domain.ServerSleeping }, 5*time.Second, 50*time.Millisecond)
	// The status changes before the decisions are marked.
	require.Eventually(t, func() bool { d, err := e.decisions.Get(e.ctx, id); return err == nil && d.Asleep() }, 3*time.Second, 20*time.Millisecond)
	return e, id
}

func TestSleepKeepsThePendingDecisionAndSaysSo(t *testing.T) {
	var sent notes
	e, id := sleepingWithADecision(t, &sent)

	d, err := e.decisions.Get(e.ctx, id)
	require.NoError(t, err)
	assert.True(t, d.Open(), "the decision is not resolved `gone` any more")
	assert.True(t, d.Asleep())

	want := i18n.T("v54n.notify.title.sleep")
	require.Eventually(t, func() bool {
		for _, n := range sent.list() {
			if n[0] == want {
				return true
			}
		}
		return false
	}, 3*time.Second, 20*time.Millisecond, "a note says the session went to sleep with a decision waiting")
	for _, n := range sent.list() {
		if n[0] == want {
			assert.Equal(t, "openhub · lead", n[1])
		}
	}
}

func TestWakeLosingTheRequestAsksTheAgentAgain(t *testing.T) {
	var sent notes
	e, id := sleepingWithADecision(t, &sent)

	// The session wakes up: the tool does not list the request any more.
	e.ad.set(func() { e.ad.pending["ses_w"] = 0 })
	require.NoError(t, e.servers.SetStatus(e.ctx, "g1", domain.ServerReady))
	var ch chan adapters.ToolEvent
	select {
	case ch = <-e.ad.events:
	case <-time.After(5 * time.Second):
		t.Fatal("the watcher did not subscribe again")
	}
	ch <- adapters.ToolEvent{Kind: adapters.EventConnected}

	require.Eventually(t, func() bool { d, _ := e.decisions.Get(e.ctx, id); return d != nil && !d.Open() }, 3*time.Second, 20*time.Millisecond)
	d, _ := e.decisions.Get(e.ctx, id)
	assert.Equal(t, domain.ResolvedByGone, d.ResolvedBy)
	require.Eventually(t, func() bool { return len(e.ad.controlCalls()) == 1 }, 3*time.Second, 20*time.Millisecond)
	call := e.ad.controlCalls()[0]
	assert.Equal(t, "ses_w", call.session)
	assert.Equal(t, adapters.ControlPrompt, call.op.Kind)
	assert.Equal(t, adapters.DeliveryQueue, call.op.Delivery)
	assert.Equal(t, i18n.T("v54n.decision.resume_after_sleep"), call.op.Text)
}

func TestWakeWithTheRequestAskedAgainKeepsTheDecision(t *testing.T) {
	var sent notes
	e, id := sleepingWithADecision(t, &sent)

	// The tool asks the request again after the wake-up: same decision, no longer asleep.
	require.NoError(t, e.servers.SetStatus(e.ctx, "g1", domain.ServerReady))
	var ch chan adapters.ToolEvent
	select {
	case ch = <-e.ad.events:
	case <-time.After(5 * time.Second):
		t.Fatal("the watcher did not subscribe again")
	}
	ch <- adapters.ToolEvent{Kind: adapters.EventConnected}

	require.Eventually(t, func() bool { d, _ := e.decisions.Get(e.ctx, id); return d != nil && d.Open() && !d.Asleep() }, 3*time.Second, 20*time.Millisecond)
	assert.Empty(t, e.ad.controlCalls(), "the agent is not asked: the tool asks")
}

func TestFinalStopStillClosesEverything(t *testing.T) {
	var sent notes
	e := startLifecycleWith(t, time.Hour, "g1", func(e *lcEnv, o *Options) {
		o.Decisions = e.decisions
		o.Notify = sent.add
	})
	require.NoError(t, e.sessions.Create(e.ctx, &domain.Session{ID: "ses_w", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive}))
	e.ad.set(func() { e.ad.pending["ses_w"] = 1 })
	e.events <- adapters.ToolEvent{Kind: adapters.EventDecisionAsked, SessionID: "ses_w"}
	id := domain.DecisionID(domain.DecisionPermission, "ses_w", "per_0")
	require.Eventually(t, func() bool { d, err := e.decisions.Get(e.ctx, id); return err == nil && d.Open() }, 3*time.Second, 20*time.Millisecond)

	require.NoError(t, e.client.SetPolicy(e.ctx, "g1", api.PolicyStopNow))
	require.Eventually(t, func() bool { d, _ := e.decisions.Get(e.ctx, id); return d != nil && !d.Open() }, 3*time.Second, 20*time.Millisecond)
	d, _ := e.decisions.Get(e.ctx, id)
	assert.Equal(t, domain.ResolvedByGone, d.ResolvedBy)
}

func TestDaemonStartRemindsOldDecisionsOnce(t *testing.T) {
	var sent notes
	d := &Daemon{opts: Options{Decisions: &fakeOpenDecisions{list: []domain.Decision{
		{ID: "a", SessionID: "s1", Kind: domain.DecisionQuestion, CreatedAt: time.Now().Add(-time.Hour)},
		{ID: "b", SessionID: "s2", Kind: domain.DecisionPermission, CreatedAt: time.Now().Add(-3 * time.Minute)},
		{ID: "old", SessionID: "s3", Kind: domain.DecisionPermission, CreatedAt: time.Now().Add(-72 * time.Hour)},
	}}}}
	n := newNotifier(d, sent.add)
	n.window = 20 * time.Millisecond
	n.scan(context.Background())
	require.Eventually(t, func() bool { return len(sent.list()) == 1 }, time.Second, 10*time.Millisecond)
	assert.Equal(t, i18n.Tf("tui.notify.many_decisions", 2, 2), sent.list()[0][1])

	n.scan(context.Background())
	assert.Never(t, func() bool { return len(sent.list()) > 1 }, 150*time.Millisecond, 10*time.Millisecond, "once")
}

// fakeOpenDecisions is a DecisionStore that only lists.
type fakeOpenDecisions struct {
	domain.DecisionStore
	list []domain.Decision
}

func (f *fakeOpenDecisions) ListOpen(context.Context, domain.DecisionFilter) ([]domain.Decision, error) {
	return f.list, nil
}
