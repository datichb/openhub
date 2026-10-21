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
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/filelock"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
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
	servDir  string
	events   chan adapters.ToolEvent
	endedMu  sync.Mutex
	ended    []string // OnSessionEnd calls
}

func (e *lcEnv) endedIDs() []string {
	e.endedMu.Lock()
	defer e.endedMu.Unlock()
	return append([]string(nil), e.ended...)
}

func startLifecycle(t *testing.T, idleSleep time.Duration, group string, sessionIDs ...string) *lcEnv {
	t.Helper()
	p := shortPaths(t)
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	e := &lcEnv{t: t, ctx: ctx, sessions: sqlite.NewSessionStore(st), servers: sqlite.NewServerStore(st), ad: &stopCountingAdapter{fakeAdapter: newFake()},
		servDir: filepath.Join(p.Dir, "servers")}
	require.NoError(t, e.servers.Upsert(ctx, &domain.Server{GroupKey: group, Adapter: "fake", ProjectID: "p1", PID: os.Getpid(), Status: domain.ServerReady}))
	for _, id := range sessionIDs {
		require.NoError(t, e.sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: group, State: domain.RunIdle}))
	}
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: e.servers, Sessions: e.sessions, Grants: sqlite.NewGrantStore(st),
			Tick: 50 * time.Millisecond, IdleAfter: time.Hour, IdleSleep: idleSleep, ServersDir: e.servDir,
			Adapter: func(string) adapters.ToolAdapter { return e.ad },
			OnSessionEnd: func(_ context.Context, s domain.Session) {
				e.endedMu.Lock()
				e.ended = append(e.ended, s.ID)
				e.endedMu.Unlock()
			}})
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

// staysReady fails the test if the server of the group leaves the ready
// status during window (checked all along, not once at the end).
func (e *lcEnv) staysReady(group string, window time.Duration, msg string) {
	e.t.Helper()
	assert.Never(e.t, func() bool {
		s, err := e.servers.Get(e.ctx, group)
		return err != nil || s.Status != domain.ServerReady
	}, window, 10*time.Millisecond, msg)
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
	e.staysReady("g1", 500*time.Millisecond, "an attached client keeps the server awake")
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
	e.staysReady("g1", 500*time.Millisecond, "decision pending and oh open: stay awake")

	require.NoError(t, e.client.ClientGone(e.ctx, "tui")) // oh closed
	require.Eventually(t, func() bool { return e.serverStatus("g1") == domain.ServerSleeping }, 5*time.Second, 50*time.Millisecond)
}

func TestSleepWhenIdlePolicyWaitsForTheTurn(t *testing.T) {
	e := startLifecycle(t, time.Hour, "g1", "ses_a")
	e.ad.set(func() { e.ad.active["ses_a"] = true })
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_a"}
	require.Eventually(t, func() bool { return e.state("ses_a") == domain.RunActive }, 3*time.Second, 20*time.Millisecond)

	require.NoError(t, e.client.SetPolicy(e.ctx, "g1", PolicySleepWhenIdle))
	e.staysReady("g1", 250*time.Millisecond, "the agent is still working")

	e.ad.set(func() { e.ad.active["ses_a"] = false })
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecEnded, SessionID: "ses_a", Outcome: "succeeded"}
	require.Eventually(t, func() bool { return e.serverStatus("g1") == domain.ServerSleeping }, 3*time.Second, 20*time.Millisecond)
}

// E14-M2: a session the tool runs but oh does not track keeps the group awake.
func TestUntrackedToolSessionKeepsGroupAwake(t *testing.T) {
	e := startLifecycle(t, 100*time.Millisecond, "g1")
	e.ad.set(func() { e.ad.active["ses_untracked"] = true })
	e.staysReady("g1", 500*time.Millisecond, "the tool reports an executing session")
	e.ad.set(func() { e.ad.active["ses_untracked"] = false })
	require.Eventually(t, func() bool { return e.serverStatus("g1") == domain.ServerSleeping }, 5*time.Second, 50*time.Millisecond)
}

// E14-M2: an event for a session whose row appears just after is tracked.
func TestLateSessionRowIsTracked(t *testing.T) {
	// A session unknown to oh stays unknown for unknownTTL (5 s): a shorter
	// one, set before the daemon starts and restored after it stopped.
	oldTTL := unknownTTL
	unknownTTL = 200 * time.Millisecond
	t.Cleanup(func() { unknownTTL = oldTTL })
	e := startLifecycle(t, time.Hour, "g1")
	e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_late"}
	require.Eventually(t, func() bool { return len(e.events) == 0 }, 5*time.Second, time.Millisecond, "the watcher took the event before the row exists")
	require.NoError(t, e.sessions.Create(e.ctx, &domain.Session{ID: "ses_late", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", State: domain.RunIdle}))
	require.Eventually(t, func() bool {
		e.events <- adapters.ToolEvent{Kind: adapters.EventExecStarted, SessionID: "ses_late"}
		return e.state("ses_late") == domain.RunActive
	}, 10*time.Second, 50*time.Millisecond)
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
	assert.Equal(t, []string{"ses_a"}, e.endedIDs(), "E14-M10: session.complete hook")
}

// A group locked by a client (starting or resuming a session) never sleeps
// under its feet: the sleep waits for the lock.
func TestSleepWaitsForTheGroupLock(t *testing.T) {
	e := startLifecycle(t, 200*time.Millisecond, "g1", "ses_a")
	unlock, err := filelock.Lock(GroupLockPath(e.servDir, "g1"))
	require.NoError(t, err)
	e.staysReady("g1", 500*time.Millisecond, "group locked by a client")
	unlock()
	require.Eventually(t, func() bool { return e.serverStatus("g1") == domain.ServerSleeping }, 5*time.Second, 50*time.Millisecond)
}
