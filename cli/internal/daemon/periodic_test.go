package daemon

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
)

// The runtime environment of a group is removed when the daemon stops its
// server (sleep) and when it finds the server dead (container orphan).
func TestTeardownOnSleepAndDeadServer(t *testing.T) {
	p, servers, grants := hardeningEnv(t)
	ctx := context.Background()
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "live", Adapter: "fake", Runtime: "container", PID: os.Getpid(), Status: domain.ServerReady}))
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "dead", Adapter: "fake", Runtime: "container", PID: 999999, Status: domain.ServerReady, CreatedAt: time.Now().Add(-time.Hour)}))
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "local", Adapter: "fake", Runtime: "local", PID: 999998, Status: domain.ServerReady, CreatedAt: time.Now().Add(-time.Hour)}))
	var mu sync.Mutex
	torn := map[string]int{}
	ad := &stopCountingAdapter{fakeAdapter: newFake()}
	c := NewClient(p)
	runDaemon(t, Options{Paths: p, Servers: servers, Grants: grants, Tick: 30 * time.Millisecond, IdleAfter: time.Hour, IdleSleep: time.Hour,
		Adapter: func(string) adapters.ToolAdapter { return ad },
		Teardown: func(_ context.Context, s domain.Server) {
			mu.Lock()
			torn[s.GroupKey]++
			mu.Unlock()
		}})
	get := func(g string) int { mu.Lock(); defer mu.Unlock(); return torn[g] }
	require.Eventually(t, func() bool { return get("dead") == 1 }, 3*time.Second, 20*time.Millisecond)
	require.NoError(t, c.SetPolicy(ctx, "live", PolicyStopNow))
	require.Eventually(t, func() bool { return get("live") == 1 }, 3*time.Second, 20*time.Millisecond)
	assert.Equal(t, 0, get("local"), "nothing to remove for a local server")
}

// Work that outlives the local servers (remote sessions) keeps the daemon
// running; without it, the daemon stops when idle.
func TestPeriodicTaskKeepsTheDaemonAlive(t *testing.T) {
	p, servers, grants := hardeningEnv(t)
	var busy atomic.Bool
	busy.Store(true)
	var runs atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{Paths: p, Servers: servers, Grants: grants, Tick: 20 * time.Millisecond, IdleAfter: 150 * time.Millisecond,
			PeriodicEvery: 20 * time.Millisecond,
			Periodic:      func(context.Context) bool { runs.Add(1); return busy.Load() }})
	}()
	select {
	case <-done:
		t.Fatal("stopped while the periodic task has work")
	case <-time.After(600 * time.Millisecond):
	}
	assert.Greater(t, runs.Load(), int32(5))
	busy.Store(false)
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("did not stop once idle")
	}
}
