package daemon

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dclient "github.com/datichb/openhub/cli/internal/daemon/client"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

// RaiseAlert, called by the jobs of the daemon (the heartbeat of the claims,
// its tasks), puts the alert in « À traiter » of the session once: the same
// alert raised at every beat is one decision, a new Ref (a renewed token that
// is refused again) is another.
func TestRaiseAlertFromTheJobsOfTheDaemon(t *testing.T) {
	p := shortPaths(t)
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	sessions, decisions := sqlite.NewSessionStore(st), sqlite.NewDecisionStore(st)
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		Mode: "auto", State: domain.RunActive}))

	var ref atomic.Value
	ref.Store("fp-1")
	var raised, beats, noDaemon atomic.Int32
	dctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() {
		done <- Run(dctx, Options{Paths: p, Version: "t", Servers: sqlite.NewServerStore(st), Sessions: sessions, Decisions: decisions,
			Tick: time.Hour, IdleAfter: time.Hour,
			ClaimsHeartbeatEvery: 10 * time.Millisecond,
			OnClaimsHeartbeat: func(ctx context.Context) error {
				beats.Add(1)
				if RaiseAlert(ctx, Alert{SessionID: "ses_a", Kind: "team_access", Ref: ref.Load().(string), Title: "t", Message: "m"}) {
					raised.Add(1)
				}
				if RaiseAlert(ctx, Alert{SessionID: "missing", Kind: "team_access"}) {
					noDaemon.Add(1)
				}
				return nil
			}})
	}()
	t.Cleanup(func() { cancel(); <-done })
	c := dclient.NewClient(p)
	require.Eventually(t, func() bool { _, err := c.Health(ctx); return err == nil }, 5*time.Second, 20*time.Millisecond)
	open := func() []domain.Decision {
		l, err := decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: "ses_a"})
		require.NoError(t, err)
		return l
	}

	require.Eventually(t, func() bool { return beats.Load() >= 4 }, 5*time.Second, 10*time.Millisecond)
	l := open()
	require.Len(t, l, 1, "one alert, however many beats")
	assert.Equal(t, domain.DecisionError, l[0].Kind)
	assert.True(t, isOhAlert(l[0]), "dismissed by the user only")
	assert.Equal(t, "t", l[0].Payload.Title)
	assert.Equal(t, int32(1), raised.Load(), "recorded once")
	assert.Zero(t, noDaemon.Load(), "an unknown session records nothing")

	ref.Store("fp-2")
	before := beats.Load()
	require.Eventually(t, func() bool { return beats.Load() >= before+3 }, 5*time.Second, 10*time.Millisecond)
	assert.Len(t, open(), 2, "a new Ref is a new alert")
	assert.Equal(t, int32(2), raised.Load())
}

// Without a daemon around (a command run by hand) there is nobody to tell.
func TestRaiseAlertWithoutADaemon(t *testing.T) {
	assert.False(t, RaiseAlert(context.Background(), Alert{SessionID: "s", Kind: "k"}))
}
