package cmd

import (
	"context"
	"errors"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// startTicketClaims returns the hook run when a session starts on Beads
// tickets (any workflow): each ticket is
// claimed in the team-state for the active member, or its claim moves from
// the initial / planned status to the work status. Nil without a team.
func startTicketClaims(a *app.App, project *domain.Project) func(ctx context.Context, projectID string, tickets []string) []string {
	if project == nil {
		return nil
	}
	tc := resolvedTeamConfig(a, project)
	if !tc.Enabled || tc.MemberID == "" {
		return nil
	}
	statePath := tc.StatePath
	repo := teamstate.NewRepo(tc.StateRepo, statePath)
	member := tc.MemberID
	return func(ctx context.Context, projectID string, tickets []string) []string {
		if !repo.IsCloned() {
			return nil
		}
		// the first claim of the project records its remote in the team-state
		ctx = withProjectIdentity(ctx, project, member)
		var notes []string
		for _, t := range tickets {
			if n := startTicketClaim(ctx, repo, projectID, member, t); n != "" {
				notes = append(notes, n)
			}
		}
		return notes
	}
}

// ticketClaimStore is what claiming a ticket at launch asks of the team-state.
type ticketClaimStore interface {
	teamstate.ConfigReader
	teamstate.ClaimWriter
	teamstate.ClaimAdopter
	teamstate.ClaimGroupWriter
}

// startTicketClaim claims a ticket for member, or moves its claim from the
// initial / planned status to the work status; it returns a note ("" when
// nothing changed). The session that starts (runsvc.StartedSession, when the
// caller knows it) is recorded on the claim, new or already held, so that the
// end of the session applies to the claim it worked on.
//
// The claim, its status, its session and the claim.taken event are one write
// (teamstate.WriteClaim): one commit and one push, not one per step.
func startTicketClaim(ctx context.Context, repo ticketClaimStore, projectID, member, ticketID string) string {
	var boardCfg teamstate.BoardConfig
	if cfg, err := repo.LoadConfig(ctx); err == nil && cfg != nil {
		boardCfg = cfg.Board
	}
	session := runsvc.StartedSession(ctx)
	workStatus := boardCfg.DefaultWorkStatus()
	res, err := repo.WriteClaim(ctx, teamstate.ClaimWrite{
		Project: projectID, TicketID: ticketID, Member: member, Session: session,
		Create:   &teamstate.Claim{Status: workStatus},
		Identity: projectIdentityFrom(ctx),
		Status: &teamstate.ClaimStatusMove{
			To: workStatus, From: []string{boardCfg.InitialStatus(), teamstate.ClaimStatusPlanned}, BestEffort: true,
		},
	})
	switch {
	case errors.Is(err, teamstate.ErrOffline), errors.Is(err, teamstate.ErrBusy), errors.Is(err, context.DeadlineExceeded):
		// The team-state cannot be reached. A claim the member already holds
		// (read in the clone) is handed to the session when the network is
		// back, ahead of what the previous session had queued for it; a ticket
		// nobody holds (in the clone) is taken then, if it is still free.
		if session != "" {
			if queued, qerr := repo.QueueAdoptClaim(context.WithoutCancel(ctx), projectID, ticketID, member, session); qerr == nil && queued {
				return i18n.Tf("cmd.claims.start.adopt_queued", projectID, ticketID)
			}
		}
		queued, qerr := repo.QueueCreateClaim(context.WithoutCancel(ctx), projectID, ticketID, member, session, workStatus)
		switch {
		case qerr != nil:
			return i18n.Tf("v54l.claims.start.offline", projectID, ticketID)
		case queued:
			return i18n.Tf("v54l.claims.start.create_queued", projectID, ticketID)
		}
		return "" // the clone has a claim on the ticket already
	case teamstate.IsAuthError(err):
		// A refused access is not offline: the claim is not queued, the member
		// is told (oh doctor) instead of finding the ticket unclaimed later.
		return i18n.Tf("v54a.claims.start.access", projectID, ticketID)
	case err != nil:
		slog.Warn("team claim not started", "ticket", ticketID, "session", session, "error", err)
		return ""
	case res.Created:
		return i18n.Tf("cmd.run.claim.taken", projectID, ticketID)
	case res.Held != nil:
		return i18n.Tf("cmd.run.claim.other", ticketID, res.Held.ClaimedBy)
	case res.Moved:
		return i18n.Tf("cmd.run.claim.started", projectID, ticketID, res.From, workStatus)
	}
	return ""
}
