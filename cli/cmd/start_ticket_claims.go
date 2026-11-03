package cmd

import (
	"context"
	"errors"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// startTicketClaims returns the hook run when a session starts on Beads
// tickets (any workflow, QB2): each ticket is
// claimed in the team-state for the active member, or its claim moves from
// the initial / planned status to the work status. Nil without a team.
func startTicketClaims(a *app.App, project *domain.Project) func(ctx context.Context, projectID string, tickets []string) []string {
	if project == nil || !teamEnabledForProject(a, project) || a.Config.ActiveTeam().MemberID == "" {
		return nil
	}
	tc := resolvedTeamConfig(a, project)
	statePath := tc.StatePath
	if statePath == "" {
		statePath = defaultTeamStatePath(a)
	}
	repo := teamstate.NewRepo(tc.StateRepo, statePath)
	member := a.Config.ActiveTeam().MemberID
	return func(ctx context.Context, projectID string, tickets []string) []string {
		if !repo.IsCloned() {
			return nil
		}
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
}

// startTicketClaim claims a ticket for member, or moves its claim from the
// initial / planned status to the work status; it returns a note ("" when
// nothing changed). The session that starts (runsvc.StartedSession, when the
// caller knows it) is recorded on the claim, new or already held, so that the
// end of the session applies to the claim it worked on.
func startTicketClaim(ctx context.Context, repo ticketClaimStore, projectID, member, ticketID string) string {
	var boardCfg teamstate.BoardConfig
	if cfg, err := repo.LoadConfig(ctx); err == nil && cfg != nil {
		boardCfg = cfg.Board
	}
	session := runsvc.StartedSession(ctx)
	workStatus := boardCfg.DefaultWorkStatus()
	existing, err := repo.CreateClaim(ctx, teamstate.Claim{TicketID: ticketID, Project: projectID, ClaimedBy: member, Status: workStatus, Session: session})
	switch {
	case err == nil:
		return i18n.Tf("cmd.run.claim.taken", projectID, ticketID)
	case errors.Is(err, teamstate.ErrOffline), errors.Is(err, teamstate.ErrBusy):
		// The team-state cannot be reached: a claim the member already holds
		// (read in the clone) is handed to the session when the network is
		// back, ahead of what the previous session had queued for it.
		if session == "" {
			return ""
		}
		if queued, qerr := repo.QueueAdoptClaim(projectID, ticketID, member, session); qerr == nil && queued {
			return i18n.Tf("cmd.claims.start.adopt_queued", projectID, ticketID)
		}
		return ""
	case errors.Is(err, teamstate.ErrClaimExists) && existing != nil:
		if existing.ClaimedBy != member {
			return i18n.Tf("cmd.run.claim.other", ticketID, existing.ClaimedBy)
		}
		note := ""
		if existing.Status == boardCfg.InitialStatus() || existing.Status == teamstate.ClaimStatusPlanned {
			if err := repo.UpdateClaimStatus(ctx, projectID, ticketID, workStatus); err == nil {
				note = i18n.Tf("cmd.run.claim.started", projectID, ticketID, existing.Status, workStatus)
			}
		}
		_, queued, err := repo.AdoptClaimDurable(ctx, projectID, ticketID, member, session)
		if err != nil {
			slog.Warn("team claim not tied to the session", "ticket", ticketID, "session", session, "error", err)
		}
		if queued && note == "" {
			note = i18n.Tf("cmd.claims.start.adopt_queued", projectID, ticketID)
		}
		return note
	}
	return ""
}

// defaultTeamStatePath returns the team state path from config or default.
func defaultTeamStatePath(a *app.App) string {
	if a.Config.ActiveTeam().StatePath != "" {
		return a.Config.ActiveTeam().StatePath
	}
	return config.DefaultTeamStatePath()
}
