package daemon

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
)

// Windows option A (forced here with OH_DAEMON_INPROCESS=1): Ensure starts
// the daemon inside the process instead of spawning it, once.
func TestEnsureInProcess(t *testing.T) {
	t.Setenv("OH_DAEMON_INPROCESS", "1")
	require.True(t, InProcessMode())
	p, servers, grants := hardeningEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	var starts atomic.Int32
	opts := EnsureOptions{Version: "t", InProcess: func() error {
		starts.Add(1)
		go func() { done <- Run(ctx, Options{Paths: p, Version: "t", Servers: servers, Grants: grants, Tick: time.Hour, IdleAfter: time.Hour}) }()
		return nil
	}}
	t.Cleanup(func() { cancel(); <-done })
	_, h, err := Ensure(context.Background(), p, opts)
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), h.PID, "the daemon runs in this process")
	_, _, err = Ensure(context.Background(), p, opts)
	require.NoError(t, err)
	assert.Equal(t, int32(1), starts.Load(), "started once")

	_, _, err = Ensure(context.Background(), Paths{Dir: t.TempDir()}, EnsureOptions{})
	assert.ErrorIs(t, err, ErrUnsupported, "no in-process starter")
}

// The in-process daemon puts its server groups to sleep when it stops (the
// proxy ends with the oh process).
func TestStopServersOnExit(t *testing.T) {
	p, servers, grants := hardeningEnv(t)
	ctx := context.Background()
	require.NoError(t, servers.Upsert(ctx, &domain.Server{GroupKey: "g", Adapter: "fake", PID: os.Getpid(), Status: domain.ServerReady}))
	ad := &stopCountingAdapter{fakeAdapter: newFake()}
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Servers: servers, Grants: grants, Tick: time.Hour, IdleAfter: time.Hour, IdleSleep: time.Hour,
			StopServersOnExit: true, Adapter: func(string) adapters.ToolAdapter { return ad }})
	}()
	c := NewClient(p)
	require.Eventually(t, func() bool { h, err := c.Health(ctx); return err == nil && !h.Restoring }, 5*time.Second, 20*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("daemon did not stop")
	}
	s, err := servers.Get(ctx, "g")
	require.NoError(t, err)
	assert.Equal(t, domain.ServerSleeping, s.Status, "resumable")
	assert.Equal(t, int32(1), ad.stops.Load())
}
