package cmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/notifypolicy"
	teamsvc "github.com/datichb/openhub/cli/internal/services/team"
	"github.com/datichb/openhub/cli/internal/tracker"
)

type fakeSyncService struct {
	intervals map[string]time.Duration // team → interval; absent: auto_sync off
	errs      map[string]error
	calls     []string
	autos     []bool
}

func (f *fakeSyncService) TeamSyncInterval(_ context.Context, p *domain.Project) (time.Duration, bool, error) {
	d, ok := f.intervals[*p.TeamID]
	return d, ok, nil
}

func (f *fakeSyncService) SyncTracker(_ context.Context, opts teamsvc.SyncOptions) (*tracker.SyncResult, error) {
	id := *opts.Project.TeamID
	f.calls = append(f.calls, id)
	f.autos = append(f.autos, opts.Auto)
	if err := f.errs[id]; err != nil {
		return nil, err
	}
	return &tracker.SyncResult{}, nil
}

func newTestSyncer(svc *fakeSyncService, teams ...config.TeamConfig) (*trackerSyncer, *time.Time) {
	now := time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	return &trackerSyncer{
		config:  func() *config.Config { return &config.Config{Teams: teams} },
		service: func(*config.Config) trackerSyncService { return svc },
		now:     func() time.Time { return now },
		last:    map[string]time.Time{},
		failed:  map[string]string{},
	}, &now
}

func syncTeam(id string) config.TeamConfig {
	return config.TeamConfig{ID: id, Enabled: true, StateRepo: "git@h:acme/" + id + ".git", MemberID: "alice"}
}

// The task synchronizes the teams that turned auto_sync on, once per interval
// of each, automatically (the lease and the circuit breaker apply), and leaves
// the others alone: archived teams, solo spaces, auto_sync off.
func TestTrackerSyncTaskSynchronizesPerIntervalOfTheTeam(t *testing.T) {
	svc := &fakeSyncService{intervals: map[string]time.Duration{"core": 5 * time.Minute, "ops": 10 * time.Minute}}
	solo := syncTeam("solo")
	solo.Solo = true
	archived := syncTeam("old")
	archived.Enabled = false
	s, now := newTestSyncer(svc, syncTeam("core"), syncTeam("ops"), syncTeam("quiet"), solo, archived)

	s.run(t.Context())
	assert.Equal(t, []string{"core", "ops"}, svc.calls, "the teams with auto_sync, once")
	assert.Equal(t, []bool{true, true}, svc.autos, "automatic runs")

	*now = now.Add(time.Minute)
	s.run(t.Context())
	assert.Len(t, svc.calls, 2, "no interval has passed")

	*now = now.Add(4 * time.Minute)
	s.run(t.Context())
	assert.Equal(t, []string{"core", "ops", "core"}, svc.calls, "core's 5 minutes passed, ops' 10 not")

	*now = now.Add(5 * time.Minute)
	s.run(t.Context())
	assert.Equal(t, []string{"core", "ops", "core", "core", "ops"}, svc.calls)
}

// A lease held by another member is waited out: the team is looked at again
// when it ends, not a full interval later.
func TestTrackerSyncTaskLooksAgainWhenTheLeaseEnds(t *testing.T) {
	svc := &fakeSyncService{intervals: map[string]time.Duration{"core": 5 * time.Minute}}
	s, now := newTestSyncer(svc, syncTeam("core"))
	svc.errs = map[string]error{"core": &tracker.LeaseHeldError{Member: "bob", Until: now.Add(2 * time.Minute)}}

	s.run(t.Context())
	require.Len(t, svc.calls, 1)
	*now = now.Add(time.Minute)
	s.run(t.Context())
	assert.Len(t, svc.calls, 1)
	*now = now.Add(time.Minute + time.Second)
	s.run(t.Context())
	assert.Len(t, svc.calls, 2, "the lease of bob ended")
}

// A failure is said once, the same at the next intervals is silent.
func TestTrackerSyncTaskSaysAFailureOnce(t *testing.T) {
	svc := &fakeSyncService{intervals: map[string]time.Duration{"core": time.Minute}, errs: map[string]error{"core": errors.New("boom")}}
	s, _ := newTestSyncer(svc, syncTeam("core"))
	s.syncTeam(t.Context(), svc, syncTeam("core"))
	assert.Equal(t, "boom", s.failed["core"])
	s.report(t.Context(), "core", &tracker.SyncResult{}, nil)
	assert.Empty(t, s.failed, "a sync that gets through forgets the failure")
}

func TestTrackerSyncTaskIsRegisteredInTheDaemon(t *testing.T) {
	var opts daemon.Options
	trackerSyncDaemonOptions(&app.App{Config: &config.Config{}}, &opts)
	require.Len(t, opts.Tasks, 1)
	assert.Equal(t, "tracker-sync", opts.Tasks[0].Name)
	assert.Equal(t, trackerSyncTick, opts.Tasks[0].Every)
}

// tellingDaemon is a daemon that records what a task tells the user.
type tellingDaemon struct{ told [][2]string }

func (d *tellingDaemon) RaiseAlert(context.Context, daemon.Alert) bool { return false }
func (d *tellingDaemon) NotifyUser(_ notifypolicy.Kind, title, message string) {
	d.told = append(d.told, [2]string{title, message})
}

// The summary of an automatic sync that changed something is told to the user,
// a failure once (then silence), and a sync that did nothing not at all.
func TestTrackerSyncTaskTellsTheUser(t *testing.T) {
	i18ntest.Set(t, "fr")
	svc := &fakeSyncService{intervals: map[string]time.Duration{"core": time.Minute}}
	s, _ := newTestSyncer(svc, syncTeam("core"))
	d := &tellingDaemon{}
	ctx := daemon.WithAlerts(t.Context(), d)

	s.report(ctx, "core", &tracker.SyncResult{}, nil)
	assert.Empty(t, d.told, "nothing changed: nothing to say")

	s.report(ctx, "core", &tracker.SyncResult{ClaimsCreated: 2, ClaimsUpdated: 1}, nil)
	require.Len(t, d.told, 1)
	assert.Equal(t, "Synchro du tracker · core", d.told[0][0])
	assert.Equal(t, syncSummary(&tracker.SyncResult{ClaimsCreated: 2, ClaimsUpdated: 1}), d.told[0][1])

	boom := errors.New("boom")
	s.report(ctx, "core", nil, boom)
	s.report(ctx, "core", nil, boom)
	s.report(ctx, "core", nil, boom)
	require.Len(t, d.told, 2, "a failure that repeats is said once")
	assert.Equal(t, "Synchro du tracker en échec · core", d.told[1][0])
	assert.Equal(t, "boom", d.told[1][1])
}
