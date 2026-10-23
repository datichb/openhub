package daemon

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// heartbeatDaemon is a daemon whose grants are restored, enough to run the loop.
func heartbeatDaemon(beat func(context.Context) error, every time.Duration) *Daemon {
	d := &Daemon{
		opts:     Options{OnClaimsHeartbeat: beat, ClaimsHeartbeatEvery: every},
		restored: make(chan struct{}),
		stop:     make(chan struct{}),
	}
	close(d.restored)
	return d
}

func runLoop(ctx context.Context, d *Daemon) <-chan struct{} {
	done := make(chan struct{})
	go func() { defer close(done); d.runClaimsHeartbeat(ctx) }()
	return done
}

func TestClaimsHeartbeatCallsTheCallbackAtTheGivenRhythm(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	d := heartbeatDaemon(func(context.Context) error { calls.Add(1); return nil }, 10*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, d)

	require.Eventually(t, func() bool { return calls.Load() >= 4 }, 10*time.Second, 5*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the loop did not stop with the context")
	}
	n := calls.Load()
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, n, calls.Load(), "no call after the stop")
}

func TestClaimsHeartbeatFirstBeatIsImmediate(t *testing.T) {
	t.Parallel()
	called := make(chan struct{}, 1)
	d := heartbeatDaemon(func(context.Context) error {
		select {
		case called <- struct{}{}:
		default:
		}
		return nil
	}, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runLoop(ctx, d)
	select {
	case <-called:
	case <-time.After(5 * time.Second):
		t.Fatal("no first beat")
	}
}

func TestClaimsHeartbeatZeroOrMissingIsOff(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	beat := func(context.Context) error { calls.Add(1); return nil }
	for name, d := range map[string]*Daemon{
		"zero period":      heartbeatDaemon(beat, 0),
		"negative period":  heartbeatDaemon(beat, -time.Second),
		"missing callback": heartbeatDaemon(nil, time.Millisecond),
	} {
		select {
		case <-runLoop(context.Background(), d):
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: the loop should return at once", name)
		}
	}
	assert.Zero(t, calls.Load())
}

func TestClaimsHeartbeatErrorsAndPanicsAreNotFatal(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	d := heartbeatDaemon(func(context.Context) error {
		switch calls.Add(1) {
		case 1:
			return errors.New("offline")
		case 2:
			panic("boom")
		}
		return nil
	}, 5*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, d)
	require.Eventually(t, func() bool { return calls.Load() >= 5 }, 10*time.Second, 5*time.Millisecond, "the loop goes on after an error and a panic")
	cancel()
	<-done
}

func TestClaimsHeartbeatWaitsForTheGrantsAndStopsWithTheDaemon(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	d := heartbeatDaemon(func(context.Context) error { calls.Add(1); return nil }, 5*time.Millisecond)
	d.restored = make(chan struct{}) // not restored yet
	done := runLoop(context.Background(), d)
	time.Sleep(50 * time.Millisecond)
	assert.Zero(t, calls.Load(), "not before the restoration")

	close(d.restored)
	require.Eventually(t, func() bool { return calls.Load() >= 2 }, 10*time.Second, 5*time.Millisecond)
	close(d.stop)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the loop did not stop with the daemon")
	}
}

func TestClaimsHeartbeatContextIsPassedToTheCallback(t *testing.T) {
	t.Parallel()
	got := make(chan error, 1)
	d := heartbeatDaemon(func(ctx context.Context) error {
		<-ctx.Done()
		got <- ctx.Err()
		return ctx.Err()
	}, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	done := runLoop(ctx, d)
	time.Sleep(20 * time.Millisecond)
	cancel()
	assert.ErrorIs(t, <-got, context.Canceled, "a callback blocked on git is interrupted")
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the loop did not stop")
	}
}

// Run starts the loop only when both options are set.
func TestRunStartsTheClaimsHeartbeat(t *testing.T) {
	var on, off atomic.Int32
	p, servers, grants := hardeningEnv(t)
	runDaemon(t, Options{Paths: p, Servers: servers, Grants: grants, Tick: 30 * time.Millisecond, IdleAfter: time.Hour,
		OnClaimsHeartbeat: func(context.Context) error { on.Add(1); return nil }, ClaimsHeartbeatEvery: 10 * time.Millisecond})
	require.Eventually(t, func() bool { return on.Load() >= 3 }, 10*time.Second, 10*time.Millisecond)

	p2, servers2, grants2 := hardeningEnv(t)
	runDaemon(t, Options{Paths: p2, Servers: servers2, Grants: grants2, Tick: 30 * time.Millisecond, IdleAfter: time.Hour,
		OnClaimsHeartbeat: func(context.Context) error { off.Add(1); return nil }})
	assert.Never(t, func() bool { return off.Load() > 0 }, 200*time.Millisecond, 10*time.Millisecond, "a zero period is off")
}

func TestRunStopsTheClaimsHeartbeatWithItsContext(t *testing.T) {
	var calls atomic.Int32
	p, servers, grants := hardeningEnv(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{Paths: p, Servers: servers, Grants: grants, Tick: 30 * time.Millisecond, IdleAfter: time.Hour,
			OnClaimsHeartbeat: func(context.Context) error { calls.Add(1); return nil }, ClaimsHeartbeatEvery: 10 * time.Millisecond})
	}()
	require.Eventually(t, func() bool { return calls.Load() >= 2 }, 10*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the daemon did not stop")
	}
	n := calls.Load()
	time.Sleep(80 * time.Millisecond)
	assert.Equal(t, n, calls.Load())
}
