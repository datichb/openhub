package cmd

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Signal of life of the team claims, run by the daemon (v5.1, B3b). Each beat
// renews the lease of the claims of the open sessions (one commit per team
// repository, none while the leases are long) and empties the queue of claim
// operations of every team repository, sessions or not: what a session left
// in the queue when it ended offline must not wait for the end of another
// session. The same callback serves the detached daemon and the one inside
// the oh process (Windows): both start from daemonOptions.

// claimsQueueEvery is how often the claims-queue task of the daemon tries to
// empty the queue (it also runs when the daemon starts). Nothing is read from
// git while the queue is empty.
const claimsQueueEvery = 15 * time.Minute

// claimsTeam is a team-state clone with the member who works in it and the
// projects of this machine that share it.
type claimsTeam struct {
	repo     *teamstate.Repo
	member   string
	projects map[string]bool
}

// claimsBeater is the signal of life of the claims of the daemon.
type claimsBeater struct {
	a *app.App
	// config gives the hub configuration as it is now (a daemon that lives
	// for days must see the teams joined since it started).
	config func() *config.Config
	now    func() time.Time

	mu   sync.Mutex // one beat or flush at a time
	last map[string]time.Time
}

// readHubConfig reads hub.toml as it is now (a variable for the tests).
var readHubConfig = config.Read

func newClaimsBeater(a *app.App) *claimsBeater {
	return &claimsBeater{a: a, now: time.Now, last: map[string]time.Time{}, config: func() *config.Config {
		if cfg, err := readHubConfig(); err == nil && cfg != nil {
			return cfg
		}
		return a.Config
	}}
}

// teams lists the team-state clones of the active projects, one entry per
// clone and member.
func (b *claimsBeater) teams(ctx context.Context) []*claimsTeam {
	if b.a == nil || b.a.Projects == nil {
		return nil
	}
	projects, err := b.a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		slog.Debug("ohd: claims heartbeat: projects unreadable", "error", err)
		return nil
	}
	fresh := *b.a // the hub configuration is the only thing that changes
	fresh.Config = b.config()
	var out []*claimsTeam
	byClone := map[string]*claimsTeam{}
	for i := range projects {
		repo, member, ok := teamClaimsRepo(&fresh, &projects[i])
		if !ok {
			continue
		}
		key := repo.Path() + "\x00" + member
		t := byClone[key]
		if t == nil {
			t = &claimsTeam{repo: repo, member: member, projects: map[string]bool{}}
			byClone[key] = t
			out = append(out, t)
		}
		t.projects[projects[i].ID] = true
	}
	return out
}

// every is the period of the heartbeat loop: the shortest heartbeat of the
// teams of this machine (0: no team has one, the loop does not run). A team
// whose period is longer is beaten less often (beat).
func (b *claimsBeater) every(ctx context.Context) time.Duration {
	var every time.Duration
	for _, t := range b.teams(ctx) {
		if d := heartbeatOf(t.repo); d > 0 && (every == 0 || d < every) {
			every = d
		}
	}
	return every
}

func heartbeatOf(repo *teamstate.Repo) time.Duration {
	cfg, err := repo.LoadConfig()
	if err != nil || cfg == nil {
		return 0
	}
	return cfg.Claim.HeartbeatInterval()
}

// openSessions are the sessions that still hold their claims: not over (a
// sleeping session can be resumed).
func (b *claimsBeater) openSessions(ctx context.Context) []domain.Session {
	if b.a == nil || b.a.Sessions == nil {
		return nil
	}
	running, err := b.a.Sessions.ListRunning(ctx, "")
	if err != nil {
		slog.Debug("ohd: claims heartbeat: sessions unreadable", "error", err)
		return nil
	}
	return slices.DeleteFunc(running, func(s domain.Session) bool {
		switch s.State {
		case domain.RunCompleted, domain.RunFailed, domain.RunStopped:
			return true
		}
		return len(s.Tickets) == 0
	})
}

// refsOf are the claims of the open sessions of the projects of the team and
// of its member.
func (t *claimsTeam) refsOf(open []domain.Session) []teamstate.ClaimRef {
	var refs []teamstate.ClaimRef
	for _, s := range open {
		if !t.projects[s.ProjectID] || (s.MemberID != nil && *s.MemberID != t.member) {
			continue
		}
		for _, id := range s.Tickets {
			refs = append(refs, teamstate.ClaimRef{Project: s.ProjectID, TicketID: id})
		}
	}
	return refs
}

// beat is Options.OnClaimsHeartbeat: for each team repository, renew the
// leases of the claims of the open sessions and empty the queue; a team
// without heartbeat (or not due yet) only has its queue emptied. A remote
// that cannot be reached is not an error of the beat (what could not be done
// stays queued for the next one).
func (b *claimsBeater) beat(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	teams := b.teams(ctx)
	if len(teams) == 0 {
		return nil
	}
	open := b.openSessions(ctx)
	bctx := teamstate.WithLockWait(ctx, teamstate.BackgroundLockWait)
	now := b.now()
	var errs []error
	for _, t := range teams {
		refs := t.refsOf(open)
		every := heartbeatOf(t.repo)
		key := t.repo.Path()
		var err error
		if len(refs) > 0 && b.due(key, every, now) {
			b.last[key] = now
			_, err = t.repo.Heartbeat(bctx, refs, t.member, "", now)
		} else {
			_, _, err = t.repo.FlushClaimQueue(bctx)
		}
		if err != nil {
			if errors.Is(err, teamstate.ErrOffline) || errors.Is(err, teamstate.ErrBusy) {
				slog.Debug("ohd: claims heartbeat: team-state not reached", "repo", key, "error", err)
				continue
			}
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// due tells whether the team repository key is beaten at now: its heartbeat
// is on (every > 0) and a period (less a tenth, for the jitter of the timer)
// has passed since the last time. The loop runs at the shortest period of the
// teams, so a team with a longer one skips some beats.
func (b *claimsBeater) due(key string, every time.Duration, now time.Time) bool {
	return every > 0 && now.Sub(b.last[key]) >= every-every/10
}

// flush is the claims-queue task: empty the queue of every team repository.
func (b *claimsBeater) flush(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()
	bctx := teamstate.WithLockWait(ctx, teamstate.BackgroundLockWait)
	for _, t := range b.teams(ctx) {
		applied, dropped, err := t.repo.FlushClaimQueue(bctx)
		switch {
		case err == nil:
			if applied+dropped > 0 {
				slog.Info("ohd: claims queue emptied", "repo", t.repo.Path(), "applied", applied, "dropped", dropped)
			}
		case errors.Is(err, teamstate.ErrOffline), errors.Is(err, teamstate.ErrBusy):
			slog.Debug("ohd: claims queue not emptied", "repo", t.repo.Path(), "error", err)
		default:
			slog.Warn("ohd: claims queue not emptied", "repo", t.repo.Path(), "error", err)
		}
	}
}

// claimsDaemonOptions sets the claims heartbeat and the claims-queue task of
// the daemon.
func claimsDaemonOptions(ctx context.Context, a *app.App, opts *daemon.Options) {
	b := newClaimsBeater(a)
	opts.OnClaimsHeartbeat = b.beat
	opts.ClaimsHeartbeatEvery = b.every(ctx)
	opts.Tasks = append(opts.Tasks, daemon.Task{Name: "claims-queue", Every: claimsQueueEvery, Run: b.flush})
}
