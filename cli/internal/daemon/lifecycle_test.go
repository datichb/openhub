package daemon

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

type stopCountingAdapter struct {
	*fakeAdapter
	stops atomic.Int32
}

func (s *stopCountingAdapter) StopServer(context.Context, adapters.ServerHandle) error {
	s.stops.Add(1)
	return nil
}

type lcEnv struct {
	t        *testing.T
	ctx      context.Context
	sessions *sqlite.SessionStore
	servers  *sqlite.ServerStore
	ad       *stopCountingAdapter
	client   *Client
	events   chan adapters.ToolEvent
}

func startLifecycle(t *testing.T, idleSleep time.Duration, group string, sessionIDs ...string) *lcEnv {
	t.Helper()
	p := shortPaths(t)
	st, err := sqlite.Open(filepath.Join(p.Dir, "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	e := &lcEnv{t: t, ctx: ctx, sessions: sqlite.NewSessionStore(st), servers: sqlite.NewServerStore(st), ad: &stopCountingAdapter{fakeAdapter: newFake()}}
	require.NoError(t, e.servers.Upsert(ctx, &domain.Server{GroupKey: group, Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	for _, id := range sessionIDs {
		require.NoError(t, e.sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: group, State: domain.RunIdle}))
	}
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: e.servers, Sessions: e.sessions, Grants: sqlite.NewGrantStore(st),
			Tick: 50 * time.Millisecond, IdleAfter: time.Hour, IdleSleep: idleSleep,
			Adapter: func(string) adapters.ToolAdapter { return e.ad }})
	}()
	t.Cleanup(func() { cancel(); <-done })
	e.client = NewClient(p)
	require.Eventually(t, func() bool { _, err := e.client.Health(ctx); return err == nil }, 5*time.Second, 20*time.Millisecond)
	select {
	case e.events = <-e.ad.events:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not subscribe")
	}
	e.events <- adapters.ToolEvent{Kind: adapters.EventConnected}
	return e
}

func (e *lcEnv) serverStatus(group string) domain.ServerStatus {
	s, err := e.servers.Get(e.ctx, group)
	require.NoError(e.t, err)
	return s.Status
}

func (e *lcEnv) state(id string) domain.RunState {
	s, err := e.sessions.Get(e.ctx, id)
	require.NoError(e.t, err)
	return s.State
}

func TestIdleGroupSleeps(t *testing.T) {
	e := startLifecycle(t, 300*time.Millisecond, "g1", "ses_a")
	require.Eventually(t, func() bool { return e.state("ses_a") == domain.RunSleeping }, 5*time.Second, 50*time.Millisecond)
	assert.Equal(t, domain.ServerSleeping, e.serverStatus("g1"))
	s, _ := e.sessions.Get(e.ctx, "ses_a")
	require.NotNil(t, s.StateChangedAt)
	assert.Equal(t, int32(1), e.ad.stops.Load())
}

func TestAttachedClientPreventsSleep(t *testing.T) {
	e := startLifecycle(t, 200*time.Millisecond, "g1", "ses_a")
	require.NoError(t, e.client.Heartbeat(e.ctx, HeartbeatRequest{ClientID: "c1", Kind: ClientAttach, Group: "g1"}))
	time.Sleep(700 * time.Millisecond)
	assert.Equal(t, domain.ServerReady, e.serverStatus("g1"))
	require.NoError(t, e.client.ClientGone(e.ctx, "c1"))
	require.Eventually(t, func() bool { return e.serverStatus("g1") == domain.ServerSleeping }, 5*time.Second, 50*time.Millisecond)
}

func TestPendingDecisionKeptAwakeWhileOhOpen(t *testing.T) {
	e := startLifecycle(t, 200*time.Millisecond, "g1")
	require.NoError(t, e.sessions.Create(e.ctx, &domain.Session{ID: "ses_w", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunActive}))
	e.ad.set(func() { e.ad.pending["ses_w"] = 1 })
	require.NoError(t, e.client.Heartbeat(e.ctx, HeartbeatRequest{ClientID: "tui", Kind: ClientPresence}))
	e.events <- adapters.ToolEvent{Kind: adapters.EventDecisionAsked, SessionID: "ses_w"}
	require.Eventually(t, func() bool { return e.state("ses_w") == domain.RunWaiting }, 3*time.Second, 20*time.Millisecond)
	time.Sleep(600 * time.Millisecond)
	assert.Equal(t, domain.ServerReady, e.serverStatus("g1"), "decision pending and oh open: stay awake")

	require.NoError(t, e.client.ClientGone(e.ctx, "tui")) // oh closed
	require.Eventually(t, func() bool { return e.serverStatus("g1") == domain.ServerSleeping }, 5*time.Second, 50*time.Millisecond)
}

func TestSleepWhenIdlePolicyWaitsForTheTurn(t *testing.T) {
	e := startLifecycle(t, time.Hour, "g1", "ses_a")
	e.ad.set(func() { e.ad.active["ses_a"] = true })
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_a"}
	require.Eventually(t, func() bool { return e.state("ses_a") == domain.RunActive }, 3*time.Second, 20*time.Millisecond)

	require.NoError(t, e.client.SetPolicy(e.ctx, "g1", PolicySleepWhenIdle))
	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, domain.ServerReady, e.serverStatus("g1"), "the agent is still working")

	e.events <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_a", Outcome: "succeeded"}
	require.Eventually(t, func() bool { return e.serverStatus("g1") == domain.ServerSleeping }, 3*time.Second, 20*time.Millisecond)
}

func TestStopNowPolicy(t *testing.T) {
	e := startLifecycle(t, time.Hour, "g1", "ses_a")
	require.NoError(t, e.client.SetPolicy(e.ctx, "g1", PolicyStopNow))
	require.Eventually(t, func() bool { return e.state("ses_a") == domain.RunStopped }, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, domain.ServerStopped, e.serverStatus("g1"))
	s, _ := e.sessions.Get(e.ctx, "ses_a")
	assert.Equal(t, domain.RunStopped, s.State)
	assert.Equal(t, domain.SessionStatusCompleted, s.Status)
	assert.NotNil(t, s.EndedAt)
}
