package daemon

import (
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
)

func healthOK(t *testing.T, p Paths) {
	t.Helper()
	_, err := NewClient(p).Health(context.Background())
	require.NoError(t, err, "the daemon still answers")
}

// A-02: a panic in a periodic job is logged and the job restarted; the
// daemon (and this test process) lives on.
func TestPanicInPeriodicJobDoesNotKillTheDaemon(t *testing.T) {
	logs := captureLogs(t)
	p, servers, grants := hardeningEnv(t)
	var calls atomic.Int32
	runDaemon(t, Options{Paths: p, Servers: servers, Grants: grants, Tick: time.Hour, IdleAfter: time.Hour,
		PeriodicEvery: time.Hour,
		Periodic: func(context.Context) bool {
			if calls.Add(1) == 1 {
				panic("periodic exploded")
			}
			return false
		}})
	// The loop is run again after the pause of safego.Supervise.
	require.Eventually(t, func() bool { return calls.Load() >= 2 }, 15*time.Second, 10*time.Millisecond)
	healthOK(t, p)
	assert.Contains(t, logs.String(), "recovered panic in supervised loop")
	assert.Contains(t, logs.String(), "periodic exploded")
}

// A-02: a panic in a supervision pass is logged, the next pass runs.
func TestPanicInSupervisionDoesNotKillTheDaemon(t *testing.T) {
	logs := captureLogs(t)
	p, servers, grants := hardeningEnv(t)
	ctx := context.Background()
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g", Adapter: "x", PID: os.Getpid(), Status: domain.ServerReady, CreatedAt: time.Now().Add(-time.Hour)}))
	var calls atomic.Int32
	runDaemon(t, Options{Paths: p, Servers: servers, Grants: grants, Tick: 20 * time.Millisecond, IdleAfter: time.Hour,
		Adapter: func(string) adapters.ToolAdapter {
			if calls.Add(1) <= 2 {
				panic("adapter registry exploded")
			}
			return nil
		}})
	require.Eventually(t, func() bool { return calls.Load() >= 4 }, 15*time.Second, 10*time.Millisecond, "the passes go on after the panics")
	healthOK(t, p)
	assert.Contains(t, logs.String(), "panic in the supervision pass")
}

// A-02: a panic of the notification command (flushed in its own goroutine)
// is recovered, and the next notes are sent.
func TestPanicInNotificationDoesNotKillTheDaemon(t *testing.T) {
	logs := captureLogs(t)
	sent := make(chan struct{}, 2)
	d := &Daemon{opts: Options{NotifyWindow: time.Millisecond}}
	d.notes = newNotifier(d, func(context.Context, string, string) error {
		sent <- struct{}{}
		panic("notification command exploded")
	})
	for range 2 {
		d.notes.turnDone("ses_a")
		select {
		case <-sent:
		case <-time.After(10 * time.Second):
			t.Fatal("the note was never sent")
		}
	}
	assert.Contains(t, logs.String(), "notification command exploded")
}

type panicOnceAdapter struct {
	*fakeAdapter
	calls atomic.Int32
}

func (a *panicOnceAdapter) Events(ctx context.Context, h adapters.ServerHandle) (<-chan adapters.ToolEvent, error) {
	if a.calls.Add(1) == 1 {
		panic("event stream exploded")
	}
	return a.fakeAdapter.Events(ctx, h)
}

func watcherDaemon(t *testing.T, ad adapters.ToolAdapter, sessions domain.SessionStore) *Daemon {
	t.Helper()
	d := &Daemon{
		opts:     Options{Adapter: func(string) adapters.ToolAdapter { return ad }, Sessions: sessions},
		watchers: map[string]*watcher{},
	}
	t.Cleanup(d.stopWatchers)
	return d
}

var liveServer = []domain.Server{{GroupKey: "g1", Adapter: "fake", ProjectID: "p1", PID: 1, Status: domain.ServerReady}}

// A-02: the loop of a watcher that panics is restarted (the group would
// otherwise never be followed again), and the panic is logged.
func TestPanicInWatcherRestartsIt(t *testing.T) {
	logs := captureLogs(t)
	ad := &panicOnceAdapter{fakeAdapter: newFake()}
	d := watcherDaemon(t, ad, blockedSessions{})
	d.syncWatchers(context.Background(), liveServer)

	require.Eventually(t, func() bool {
		ad.mu.Lock()
		defer ad.mu.Unlock()
		return ad.subs >= 1
	}, 15*time.Second, 10*time.Millisecond, "the watcher subscribed again after its panic")
	assert.Equal(t, int32(2), ad.calls.Load())
	assert.Contains(t, logs.String(), "ohd watcher g1")
	assert.Contains(t, logs.String(), "event stream exploded")
}

// blockedSessions is a session store whose List blocks the watcher that
// resynchronizes: it signals that it entered, waits for the watcher to be
// cancelled, signals that, and waits to be released.
type blockedSessions struct {
	domain.SessionStore
	entered, cancelled chan struct{}
	release            chan struct{}
}

func newBlockedSessions() blockedSessions {
	return blockedSessions{entered: make(chan struct{}), cancelled: make(chan struct{}), release: make(chan struct{})}
}

func (b blockedSessions) List(ctx context.Context, _ string) ([]domain.Session, error) {
	if b.entered == nil {
		return nil, errors.New("not used")
	}
	close(b.entered)
	<-ctx.Done()
	close(b.cancelled)
	<-b.release
	return nil, errors.New("stopped")
}

// A-04: stopping the watchers waits until they have returned, so that none
// writes after the daemon is stopped.
func TestStopWatchersWaitsForThem(t *testing.T) {
	bs := newBlockedSessions()
	ad := newFake()
	d := watcherDaemon(t, ad, bs)
	d.syncWatchers(context.Background(), liveServer)
	events := <-ad.events
	events <- adapters.ToolEvent{Kind: adapters.EventConnected} // resync: blocks in the session store
	<-bs.entered

	returned := make(chan struct{})
	go func() {
		d.stopWatchers()
		close(returned)
	}()
	<-bs.cancelled // the watcher was cancelled and is still working
	select {
	case <-returned:
		t.Fatal("stopWatchers returned while a watcher was still running")
	case <-time.After(100 * time.Millisecond):
	}
	close(bs.release)
	select {
	case <-returned:
	case <-time.After(10 * time.Second):
		t.Fatal("stopWatchers never returned")
	}
}

// A-04: the wait is bounded: a watcher stuck on a tool server that does not
// answer does not hold the daemon.
func TestStopWatchersWaitIsBounded(t *testing.T) {
	logs := captureLogs(t)
	bs := newBlockedSessions()
	ad := newFake()
	d := watcherDaemon(t, ad, bs)
	d.watchersWait = 50 * time.Millisecond
	d.syncWatchers(context.Background(), liveServer)
	events := <-ad.events
	events <- adapters.ToolEvent{Kind: adapters.EventConnected}
	<-bs.entered
	t.Cleanup(func() { close(bs.release) })

	returned := make(chan struct{})
	go func() {
		d.stopWatchers()
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(10 * time.Second):
		t.Fatal("stopWatchers waited for a stuck watcher")
	}
	assert.Contains(t, logs.String(), "watchers still running at exit")
}

// A-04: the daemon that stops has no watcher running any more when shutdown
// returns.
func TestShutdownWaitsForWatchers(t *testing.T) {
	bs := newBlockedSessions()
	ad := newFake()
	d := watcherDaemon(t, ad, bs)
	d.opts.Paths = shortPaths(t)
	d.feed = newHub()
	d.syncWatchers(context.Background(), liveServer)
	events := <-ad.events
	events <- adapters.ToolEvent{Kind: adapters.EventConnected}
	<-bs.entered

	returned := make(chan error, 1)
	go func() { returned <- d.shutdown() }()
	<-bs.cancelled
	select {
	case <-returned:
		t.Fatal("shutdown returned while a watcher was still running")
	case <-time.After(100 * time.Millisecond):
	}
	close(bs.release)
	select {
	case err := <-returned:
		assert.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("shutdown never returned")
	}
}
