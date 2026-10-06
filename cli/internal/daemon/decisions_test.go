package daemon

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionresults"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func TestWatcherMirrorsDecisions(t *testing.T) {
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions := sqlite.NewSessionStore(st)
	servers := sqlite.NewServerStore(st)
	decisions := sqlite.NewDecisionStore(st)
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive}))

	fake := newFake()
	perm := adapters.PendingDecision{ID: "per_1", SessionID: "ses_a", Kind: adapters.DecisionPermission, Action: "shell", Resources: []string{"npm test"}}
	form := adapters.PendingDecision{ID: "frm_1", SessionID: "ses_a", Kind: adapters.DecisionQuestion, Title: "Color?",
		Fields: []adapters.FormField{{Key: "q0", Type: "select", Options: []adapters.FormOption{{Value: "Blue", Label: "Blue"}}}}}
	fake.set(func() { fake.decisions["ses_a"] = []adapters.PendingDecision{perm, form} })

	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: servers, Sessions: sessions, Decisions: decisions, Tick: time.Hour, IdleAfter: time.Hour,
			Adapter: func(name string) adapters.ToolAdapter {
				if name == "fake" {
					return fake
				}
				return nil
			}})
	}()
	t.Cleanup(func() { cancel(); <-done })
	var ch chan adapters.ToolEvent
	select {
	case ch = <-fake.events:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not subscribe")
	}
	open := func() []domain.Decision {
		l, err := decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: "ses_a"})
		require.NoError(t, err)
		return l
	}
	state := func() domain.RunState {
		s, err := sessions.Get(ctx, "ses_a")
		require.NoError(t, err)
		return s.State
	}

	// Resync mirrors both tool requests.
	ch <- adapters.ToolEvent{Kind: adapters.EventConnected}
	require.Eventually(t, func() bool { return len(open()) == 2 && state() == domain.RunWaiting }, 3*time.Second, 20*time.Millisecond)
	q, err := decisions.Get(ctx, domain.DecisionID(domain.DecisionQuestion, "ses_a", "frm_1"))
	require.NoError(t, err)
	assert.Equal(t, "g1", q.GroupKey)
	assert.Equal(t, "Blue", q.Payload.Fields[0].Options[0].Value)

	// The permission is answered in the tool UI → resolved by the tool.
	fake.set(func() { fake.decisions["ses_a"] = []adapters.PendingDecision{form} })
	ch <- adapters.ToolEvent{Kind: adapters.EventDecisionReplied, SessionID: "ses_a"}
	require.Eventually(t, func() bool { return len(open()) == 1 }, 3*time.Second, 20*time.Millisecond)
	pd, _ := decisions.Get(ctx, domain.DecisionID(domain.DecisionPermission, "ses_a", "per_1"))
	assert.Equal(t, domain.ResolvedByTool, pd.ResolvedBy)

	// Form answered, then the loop fails → error decision, session waiting.
	fake.set(func() { fake.decisions["ses_a"] = nil })
	ch <- adapters.ToolEvent{Kind: adapters.EventDecisionReplied, SessionID: "ses_a"}
	ch <- adapters.ToolEvent{ID: "evt_9", Kind: adapters.EventExecEnded, Outcome: "failed", SessionID: "ses_a",
		Data: map[string]any{"error": map[string]any{"message": "provider refused"}}}
	require.Eventually(t, func() bool {
		l := open()
		return len(l) == 1 && l[0].Kind == domain.DecisionError && state() == domain.RunWaiting
	}, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, "provider refused", open()[0].Payload.Message)

	// The user goes on in the tool → alert cleared.
	ch <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_a"}
	require.Eventually(t, func() bool { return len(open()) == 0 && state() == domain.RunActive }, 3*time.Second, 20*time.Millisecond)
}

func TestEventError(t *testing.T) {
	assert.Equal(t, "boom", eventError(map[string]any{"error": "boom"}))
	assert.Equal(t, "x", eventError(map[string]any{"error": map[string]any{"data": map[string]any{"message": "x"}}}))
	assert.Equal(t, "", eventError(nil))
}

func TestSleepSavesResults(t *testing.T) {
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	sessions := sqlite.NewSessionStore(st)
	decisions := sqlite.NewDecisionStore(st)
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunWaiting}))
	require.NoError(t, decisions.Upsert(ctx, &domain.Decision{ID: "permission:ses_a:per_1", SessionID: "ses_a", GroupKey: "g1", Kind: domain.DecisionPermission, ToolRef: "per_1"}))
	require.NoError(t, decisions.Upsert(ctx, &domain.Decision{ID: "error:ses_a:e1", SessionID: "ses_a", GroupKey: "g1", Kind: domain.DecisionError}))
	fake := newFake()
	fake.usage["ses_a"] = adapters.SessionResult{SessionID: "ses_a", Cost: 1.5, Branch: "feat/x",
		Changes: []adapters.FileChange{{File: "a.go", Additions: 3, Patch: "+x\n"}}}
	dir := t.TempDir()
	d := &Daemon{opts: Options{Sessions: sessions, Decisions: decisions, SessionsDir: dir, Adapter: func(string) adapters.ToolAdapter { return fake }},
		proxy: credproxy.New(), pending: map[string]domain.ProxyGrant{}, policies: map[string]QuitPolicy{}, watchers: map[string]*watcher{}}

	d.putToSleep(ctx, domain.Server{GroupKey: "g1", ProjectID: "p1", Adapter: "fake"}, false)
	sum, patch, err := sessionresults.Load(dir, "ses_a")
	require.NoError(t, err)
	assert.Equal(t, "feat/x", sum.Branch)
	assert.Equal(t, 3, sum.Additions)
	assert.Equal(t, "+x\n", patch)
	s, _ := sessions.Get(ctx, "ses_a")
	assert.Equal(t, domain.RunSleeping, s.State)
	open, _ := decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: "ses_a"})
	require.Len(t, open, 1, "tool requests die with the server, oh alerts stay")
	assert.Equal(t, domain.DecisionError, open[0].Kind)
}
