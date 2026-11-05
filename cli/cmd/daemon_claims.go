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
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Signal of life of the team claims, run by the daemon. Each beat
// renews the lease of the claims of the open sessions (one commit per team
// repository, none while the leases are long) and empties the queue of claim
// operations of every team repository, sessions or not: what a session left
// in the queue when it ended offline must not wait for the end of another
// session. The same callback serves the detached daemon and the one inside
// the oh process (Windows): both start from daemonOptions.

// claimsBeatStep is the step of the heartbeat loop of the daemon. It is fixed:
// the period of each team (heartbeat_minutes of its config.toml) is read at
// every step and decides whether the team is due (claimsBeater.due), so that a
// team joined after the daemon started, or whose period was changed, is
// followed without restarting it. The loop used to run at the shortest period
// known at the start, and not at all when no team had one then.
const claimsBeatStep = 5 * time.Minute

// claimsQueueEvery is how often the claims-queue task of the daemon tries to
// empty the queue (it also runs when the daemon starts). Nothing is read from
// git while the queue is empty.
const claimsQueueEvery = 15 * time.Minute

// claimsStore is what the daemon and the doctor ask of the team-state clone
// when they look after the claims of a member: the signal of life, the queue,
// the claims and the board to read, the claims to attach to another project.
type claimsStore interface {
	teamstate.RepoInfo
	claimEnder
	teamstate.ClaimHeartbeater
	teamstate.ClaimRehomer
}

// claimsTeam is a team-state clone with the member who works in it and the
// projects of this machine that share it.
type claimsTeam struct {
	repo     claimsStore
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
	// access remembers the repositories that refused the access (the token
	// expired): one alert, then no attempts until the credentials change.
	access *accessWatch
}

// readHubConfig reads hub.toml as it is now (a variable for the tests).
var readHubConfig = config.Read

func newClaimsBeater(a *app.App) *claimsBeater {
	return &claimsBeater{a: a, now: time.Now, last: map[string]time.Time{}, access: daemonAccess, config: func() *config.Config {
		if cfg, err := readHubConfig(); err == nil && cfg != nil {
			return cfg
		}
		return a.Config
	}}
}

// teams lists the team-state clones of the active projects, one entry per
// clone and member.
func (b *claimsBeater) teams(ctx context.Context) []*claimsTeam {
	return claimTeams(ctx, b.a, b.config())
}

// claimTeams lists the team-state clones of the active projects of a under
// the hub configuration cfg, one entry per clone and member.
func claimTeams(ctx context.Context, a *app.App, cfg *config.Config) []*claimsTeam {
	if a == nil || a.Projects == nil {
		return nil
	}
	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		slog.Debug("team claims: projects unreadable", "error", err)
		return nil
	}
	fresh := *a // the hub configuration is the only thing that changes
	fresh.Config = cfg
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

// openSessions are the sessions that still hold their claims: not over (a
// sleeping session can be resumed). One without tickets may hold the claim of
// the card of the board it was launched on (sessionRefs).
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
		return false
	})
}

// renewable reports whether the heartbeat still renews the claims of an open
// session: the renewal has a ceiling, so that a claim does not outlive for
// ever a session nobody comes back to. A session asleep for more than
// sleeping_renewal_days (3 by default) since it was put to sleep, and a remote
// session launched more than remote_renewal_days (7) ago, stop renewing; their
// claims then expire normally (oh doctor reports them).
func renewable(s domain.Session, cfg teamstate.ClaimConfig, now time.Time) bool {
	if s.State == domain.RunSleeping {
		since := s.StartedAt
		if s.StateChangedAt != nil {
			since = *s.StateChangedAt
		}
		if !since.IsZero() && now.Sub(since) > cfg.SleepingRenewalWindow() {
			return false
		}
	}
	if s.Runtime == string(sessionspec.RuntimeRemote) && !s.StartedAt.IsZero() && now.Sub(s.StartedAt) > cfg.RemoteRenewalWindow() {
		return false
	}
	return true
}

// refsOf are the claims of the open sessions of the projects of the team and
// of its member whose renewal is still allowed (renewable).
func (t *claimsTeam) refsOf(open []domain.Session, cfg teamstate.ClaimConfig, now time.Time) []teamstate.ClaimRef {
	var refs []teamstate.ClaimRef
	for _, s := range open {
		if !t.projects[s.ProjectID] || (s.MemberID != nil && *s.MemberID != t.member) || !renewable(s, cfg, now) {
			continue
		}
		for _, id := range s.Tickets {
			refs = append(refs, teamstate.ClaimRef{Project: s.ProjectID, TicketID: id})
		}
	}
	return refs
}

// sessionRefs are the claims of the member that name an open session of the
// team without that session having them among its tickets: the claim of the
// card of the board a launch started the work on (the session is recorded on
// it). Read from the clone, off the network.
func (t *claimsTeam) sessionRefs(ctx context.Context, open []domain.Session, cfg teamstate.ClaimConfig, now time.Time, have []teamstate.ClaimRef) []teamstate.ClaimRef {
	bySession := map[string]string{} // session → project
	for _, s := range open {
		if t.projects[s.ProjectID] && (s.MemberID == nil || *s.MemberID == t.member) && renewable(s, cfg, now) {
			bySession[s.ID] = s.ProjectID
		}
	}
	if len(bySession) == 0 {
		return nil
	}
	claims, err := t.repo.ListClaims(ctx, "")
	if err != nil {
		slog.Debug("ohd: claims heartbeat: claims unreadable", "error", err)
		return nil
	}
	var refs []teamstate.ClaimRef
	for i := range claims {
		c := &claims[i]
		if c.ClaimedBy != t.member || c.Session == "" || bySession[c.Session] != c.Project {
			continue
		}
		ref := teamstate.ClaimRef{Project: c.Project, TicketID: c.TicketID}
		if !slices.Contains(have, ref) {
			refs = append(refs, ref)
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
		if b.access.Blocked(ctx, t.repo, now) {
			continue // the access was refused: nothing to try until the token changes
		}
		if b.access.UnknownHost(ctx, t.repo.Remote(), t.sessionsOf(open)) {
			continue // the SSH server is not in known_hosts: no connection until it is
		}
		var claimCfg teamstate.ClaimConfig
		if cfg, err := t.repo.LoadConfig(ctx); err == nil && cfg != nil {
			claimCfg = cfg.Claim
		}
		refs := t.refsOf(open, claimCfg, now)
		every := claimCfg.HeartbeatInterval()
		key := t.repo.Path()
		isDue := b.due(key, every, now)
		if isDue {
			refs = append(refs, t.sessionRefs(ctx, open, claimCfg, now, refs)...)
		}
		var err error
		if len(refs) > 0 && isDue {
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
			if b.access.Report(ctx, t.repo, t.sessionsOf(open), err, now) {
				continue
			}
			errs = append(errs, err)
			continue
		}
		b.access.Accepted(key)
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
	open := b.openSessions(ctx)
	for _, t := range b.teams(ctx) {
		if b.access.Blocked(ctx, t.repo, b.now()) {
			continue
		}
		if b.access.UnknownHost(ctx, t.repo.Remote(), t.sessionsOf(open)) {
			continue
		}
		applied, dropped, err := t.repo.FlushClaimQueue(bctx)
		switch {
		case err == nil:
			if applied+dropped > 0 {
				slog.Info("ohd: claims queue emptied", "repo", t.repo.Path(), "applied", applied, "dropped", dropped)
			}
		case errors.Is(err, teamstate.ErrOffline), errors.Is(err, teamstate.ErrBusy):
			slog.Debug("ohd: claims queue not emptied", "repo", t.repo.Path(), "error", err)
		case b.access.Report(ctx, t.repo, t.sessionsOf(open), err, b.now()):
		default:
			slog.Warn("ohd: claims queue not emptied", "repo", t.repo.Path(), "error", err)
		}
	}
}

// sessionsOf are the open sessions that work on the projects of the team.
func (t *claimsTeam) sessionsOf(open []domain.Session) []domain.Session {
	var out []domain.Session
	for _, s := range open {
		if t.projects[s.ProjectID] {
			out = append(out, s)
		}
	}
	return out
}

// claimsDaemonOptions sets the claims heartbeat and the claims-queue task of
// the daemon.
func claimsDaemonOptions(_ context.Context, a *app.App, opts *daemon.Options) {
	b := newClaimsBeater(a)
	opts.OnClaimsHeartbeat = b.beat
	opts.ClaimsHeartbeatEvery = claimsBeatStep
	opts.Tasks = append(opts.Tasks, daemon.Task{Name: "claims-queue", Every: claimsQueueEvery, Run: b.flush})
}
