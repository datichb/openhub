package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/notifypolicy"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
)

// The claims of the member whose lease ends soon, or has ended, and that no open
// session renews: the person is told once per claim (a system notification
// when they chose « system » for claim_expired — the default —, a line in the
// notification centre, and the webhook of the team for the destinations that
// subscribed to claim.expiring / claim.expired, with a mention). A claim a
// session works on is renewed by the heartbeat and never reaches this.

// claimExpiryTick is how often the task looks at the claims (a local read:
// the clones are read, nothing is fetched).
const claimExpiryTick = 10 * time.Minute

// claimExpiringWithin is how long before the end of its lease a claim is
// announced.
const claimExpiringWithin = 2 * time.Hour

type claimExpiryWatch struct {
	a      *app.App
	config func() *config.Config
	now    func() time.Time

	mu   sync.Mutex
	told map[string]bool // claim, lease end and state already told
}

func newClaimExpiryWatch(a *app.App) *claimExpiryWatch {
	return &claimExpiryWatch{a: a, now: time.Now, told: map[string]bool{}, config: func() *config.Config {
		if cfg, err := readHubConfig(); err == nil && cfg != nil {
			return cfg
		}
		return a.Config
	}}
}

func (w *claimExpiryWatch) run(ctx context.Context) {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.now()
	for _, t := range claimTeams(ctx, w.a, w.config()) {
		if ctx.Err() != nil {
			return
		}
		tc, err := loadTeamConfigOf(ctx, t.repo)
		if err != nil {
			continue
		}
		claims, err := t.repo.ListClaims(ctx, "")
		if err != nil {
			slog.Debug("ohd: claim expiry: claims unreadable", "repo", t.repo.Path(), "error", err)
			continue
		}
		for i := range claims {
			c := &claims[i]
			end := c.LeaseUntil.Time()
			if c.ClaimedBy != t.member || end.IsZero() || !c.HasLease(tc) || end.Sub(now) > claimExpiringWithin {
				continue
			}
			state := teamstate.EventClaimExpiring
			if now.After(end) {
				state = teamstate.EventClaimExpired
			}
			key := fmt.Sprintf("%s/%s/%d/%s", c.Project, c.TicketID, end.Unix(), state)
			if w.told[key] {
				continue
			}
			w.told[key] = true
			w.tell(ctx, t, c, state, end)
		}
	}
}

// loadTeamConfigOf reads the configuration of a team-state clone.
func loadTeamConfigOf(ctx context.Context, repo any) (*teamstate.TeamConfig, error) {
	src, ok := repo.(teamstate.ConfigReader)
	if !ok {
		return &teamstate.TeamConfig{}, nil
	}
	return src.LoadConfig(ctx)
}

func (w *claimExpiryWatch) tell(ctx context.Context, t *claimsTeam, c *teamstate.Claim, state string, end time.Time) {
	subject := c.Project + "/" + c.TicketID
	if w.a != nil && w.a.Projects != nil {
		if p, err := w.a.Projects.Get(ctx, c.Project); err == nil && p != nil {
			subject = p.Name + "/" + c.TicketID
		}
	}
	title := i18n.T("v54n.claim.expiring.title")
	text := i18n.Tf("v54n.claim.expiring.text", subject, end.Local().Format("15:04"))
	level := shell.ToastWarning
	if state == teamstate.EventClaimExpired {
		title, text = i18n.T("v54n.claim.expired.title"), i18n.Tf("v54n.claim.expired.text", subject)
	}
	_ = shell.AppendToFile(shell.NotificationsFilePath(), level, title+" · "+text)
	daemon.NotifyUser(ctx, notifypolicy.KindClaimExpired, title, text)
	if src, ok := t.repo.(teamstate.EventSource); ok {
		teamEventSink(ctx, src, teamstate.Event{Type: state, Project: c.Project, Ticket: c.TicketID,
			Data: map[string]any{"owner": t.member, "lease_until": end.UTC().Format(time.RFC3339)}})
	}
}

// claimExpiryDaemonOptions adds the task to the daemon.
func claimExpiryDaemonOptions(a *app.App, opts *daemon.Options) {
	opts.Tasks = append(opts.Tasks, daemon.Task{Name: "claims-expiry", Every: claimExpiryTick, Run: newClaimExpiryWatch(a).run})
}
