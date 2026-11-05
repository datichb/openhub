package cmd

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	teamsvc "github.com/datichb/openhub/cli/internal/services/team"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// Automatic tracker sync, run by the daemon. A team that turned auto_sync on
// is synchronized every sync_interval_minutes (5 by default): the task wakes up
// every minute, a team whose interval has not passed since its last attempt is
// left alone. The members of a team each have a daemon, but one of them
// synchronizes per interval: the sync takes a lease (sync.lock of the
// team-state: member and expiry) and the others find it taken.

// trackerSyncTick is how often the task looks at the teams. The interval of a
// team is a multiple of it, read from the team config at each look (a daemon
// lives for days: it must see a team joined, or an interval changed, since it
// started).
const trackerSyncTick = time.Minute

// trackerSyncService is what the task asks of the team service.
type trackerSyncService interface {
	TeamSyncInterval(ctx context.Context, project *domain.Project) (time.Duration, bool, error)
	SyncTracker(ctx context.Context, opts teamsvc.SyncOptions) (*tracker.SyncResult, error)
}

// trackerSyncer is the automatic sync task of the daemon.
type trackerSyncer struct {
	// config gives the hub configuration as it is now.
	config func() *config.Config
	// service is the team service on that configuration.
	service func(cfg *config.Config) trackerSyncService
	now     func() time.Time

	mu     sync.Mutex
	last   map[string]time.Time // team → last attempt
	failed map[string]string    // team → message of the failure already said
}

func newTrackerSyncer(a *app.App) *trackerSyncer {
	return &trackerSyncer{
		config: func() *config.Config {
			if cfg, err := readHubConfig(); err == nil && cfg != nil {
				return cfg
			}
			return a.Config
		},
		service: func(cfg *config.Config) trackerSyncService { return teamsvc.New(cfg, a.Secrets, a.Projects) },
		now:     time.Now,
		last:    map[string]time.Time{},
		failed:  map[string]string{},
	}
}

// run is the task: one look at every team.
func (s *trackerSyncer) run(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg := s.config()
	if cfg == nil {
		return
	}
	svc := s.service(cfg)
	for i := range cfg.Teams {
		t := cfg.Teams[i]
		if !t.Enabled || t.Solo || t.StateRepo == "" || t.ID == "" {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		s.syncTeam(ctx, svc, t)
	}
}

// syncTeam synchronizes a team if it asked for it and its interval has passed.
func (s *trackerSyncer) syncTeam(ctx context.Context, svc trackerSyncService, t config.TeamConfig) {
	// The team is named by a project that belongs to it: the service resolves
	// its tracker, its member and its token as for any project of the team.
	project := &domain.Project{TeamID: &t.ID}
	interval, on, err := svc.TeamSyncInterval(ctx, project)
	if err != nil || !on {
		if err != nil {
			slog.Debug("ohd: tracker sync: team skipped", "team", t.ID, "reason", err)
		}
		return
	}
	now := s.now()
	if last, ok := s.last[t.ID]; ok && now.Sub(last) < interval-interval/10 {
		return
	}
	s.last[t.ID] = now

	res, err := svc.SyncTracker(ctx, teamsvc.SyncOptions{Project: project, Auto: true})
	var held *tracker.LeaseHeldError
	if errors.As(err, &held) {
		// The sync of another run covers this interval: look again when its
		// lease ends (not a full interval from now, which would let ours drift).
		if wait := held.Until.Sub(now); wait > 0 && wait < interval {
			s.last[t.ID] = held.Until.Add(-interval)
		}
	}
	s.report(t.ID, res, err)
}

// report logs the outcome of the sync of a team. A failure is said once (a
// warning); the same one at the next intervals is only a debug line, until the
// sync gets through again.
func (s *trackerSyncer) report(teamID string, res *tracker.SyncResult, err error) {
	var held *tracker.LeaseHeldError
	switch {
	case err == nil:
		delete(s.failed, teamID)
		slog.Info("ohd: tracker synchronized", "team", teamID, "created", res.ClaimsCreated, "updated", res.ClaimsUpdated,
			"released", res.ClaimsReleased, "warnings", len(res.Warnings), "errors", len(res.Errors))
	case errors.As(err, &held):
		slog.Debug("ohd: tracker sync left to another sync", "team", teamID, "member", held.Member, "until", held.Until)
	case errors.Is(err, tracker.ErrAutoSyncDisabled), errors.Is(err, teamsvc.ErrNoProjects):
		slog.Debug("ohd: tracker sync: nothing to do", "team", teamID, "reason", err)
	case errors.Is(err, teamstate.ErrOffline), errors.Is(err, teamstate.ErrBusy), errors.Is(err, context.Canceled):
		slog.Debug("ohd: tracker sync: team-state not reached", "team", teamID, "error", err)
	default:
		msg := err.Error()
		if s.failed[teamID] == msg {
			slog.Debug("ohd: tracker sync failed again", "team", teamID, "error", err)
			return
		}
		s.failed[teamID] = msg
		slog.Warn("ohd: tracker sync failed", "team", teamID, "error", err)
	}
}

// trackerSyncDaemonOptions adds the automatic tracker sync to the tasks of the
// daemon.
func trackerSyncDaemonOptions(a *app.App, opts *daemon.Options) {
	opts.Tasks = append(opts.Tasks, daemon.Task{Name: "tracker-sync", Every: trackerSyncTick, Run: newTrackerSyncer(a).run})
}
