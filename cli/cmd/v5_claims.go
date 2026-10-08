package cmd

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Team claims of the tickets of a session at its end (v5 corrections, A31):
// a closed ticket moves to the terminal status of the board (then cleaned up
// after the retention), a ticket still open is released. Only the claims of
// the member who ran the session are touched.

// ticketClosed reports whether a ticket is closed (err: unknown).
type ticketClosed func(id string) (bool, error)

// claimEnd is what the end of a session does to a claim.
type claimEnd string

const (
	claimKeep     claimEnd = ""
	claimDone     claimEnd = "done"
	claimReleased claimEnd = "released"
)

// endTicketClaims applies the end of a session to the claims of its tickets.
func endTicketClaims(ctx context.Context, repo *teamstate.Repo, projectID, member string, tickets []string, closed ticketClosed) map[string]claimEnd {
	out := map[string]claimEnd{}
	var board teamstate.BoardConfig
	if cfg, err := repo.LoadConfig(); err == nil && cfg != nil {
		board = cfg.Board
	}
	terminal := board.TerminalStatuses()[0]
	for _, t := range tickets {
		c, err := repo.GetClaim(projectID, t)
		if err != nil || c.ClaimedBy != member {
			continue // no claim, or someone else's
		}
		isClosed, err := closed(t)
		if err != nil {
			continue // ticket unknown: left as is (Doctor reports it)
		}
		switch {
		case isClosed && c.Status != terminal:
			if err := repo.UpdateClaimStatusFromTracker(ctx, projectID, t, terminal); err != nil {
				slog.Warn("team claim not completed", "ticket", t, "error", err)
				continue
			}
			out[t] = claimDone
		case !isClosed:
			if err := repo.ReleaseClaim(ctx, projectID, t); err != nil && !errors.Is(err, teamstate.ErrClaimNotFound) {
				slog.Warn("team claim not released", "ticket", t, "error", err)
				continue
			}
			out[t] = claimReleased
		}
	}
	return out
}

// beadsClosed reads the status of a ticket in the Beads database of a
// project.
func beadsClosed(projectPath string) ticketClosed {
	return func(id string) (bool, error) {
		d, err := beads.Show(projectPath, id)
		if err != nil {
			return false, err
		}
		return beads.IsClosedStatus(d.Status), nil
	}
}

// teamClaimsRepo is the team-state of a project and the active member, as
// used for the claims taken at launch (startTicketClaims).
func teamClaimsRepo(a *app.App, proj *domain.Project) (*teamstate.Repo, string, bool) {
	if a == nil || a.Config == nil || proj == nil {
		return nil, "", false
	}
	tc := config.ResolveTeamForProject(a.Config, proj)
	if !tc.Enabled || tc.MemberID == "" || tc.StateRepo == "" {
		return nil, "", false
	}
	statePath := tc.StatePath
	if statePath == "" {
		statePath = defaultTeamStatePath(a)
	}
	repo := teamstate.NewRepo(tc.StateRepo, statePath)
	if !repo.IsCloned() {
		return nil, "", false
	}
	return repo, tc.MemberID, true
}

// sessionClaimsEnd releases or completes the team claims of an ended
// session (async: in the background, the daemon must not wait for git).
func sessionClaimsEnd(ctx context.Context, a *app.App, proj *domain.Project, s domain.Session, async bool) {
	if len(s.Tickets) == 0 {
		return
	}
	repo, member, ok := teamClaimsRepo(a, proj)
	if !ok {
		return
	}
	run := func(ctx context.Context) {
		endTicketClaims(ctx, repo, s.ProjectID, member, s.Tickets, beadsClosed(proj.Path))
	}
	if !async {
		run(ctx)
		return
	}
	go func() {
		bctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		run(bctx)
	}()
}
