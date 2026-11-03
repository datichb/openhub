package cmd

import (
	"context"
	"errors"
	"log/slog"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// The limit of tickets a member has in progress (max_in_progress_per_member,
// teamstate.CheckInProgressRoom) is checked where the current member takes or
// starts a ticket:
//
//   - oh team claim on a ticket nobody holds, and the takeover of an expired
//     claim (claim.go);
//   - oh claim transfer to oneself (claim.go);
//   - the take key of the team board and a card moved to in progress or
//     transferred to oneself (tui_team_board.go);
//   - a session launched on tickets of a team project, local or remote
//     (ticketRoom, checked before anything is started: services/launch).
//
// The tracker sync never goes through here: it reflects the tracker, so a
// ticket already in progress there comes onto the board whatever the limit.
// The merge-request review verdict of the team MCP server sends a claim back
// to in progress for its owner; that one is not refused either (it would leave
// the verdict half applied).

// wipStore is what the check asks of the team-state.
type wipStore interface {
	teamstate.ClaimReader
	teamstate.ConfigReader
}

// checkInProgressRoom refuses (*teamstate.InProgressLimitError) to put the
// claims named by becoming in progress for member when that goes past their
// limit: hub.toml first, else the team's, else 5; 0 is no limit. What the
// team-state does not tell (unreadable config or claims) never blocks.
func checkInProgressRoom(ctx context.Context, repo wipStore, local config.TrackerLocalConfig, member string, becoming ...teamstate.ClaimRef) error {
	if member == "" || len(becoming) == 0 {
		return nil
	}
	cfg, err := repo.LoadConfig(ctx)
	if err != nil || cfg == nil {
		slog.Warn("in-progress limit not checked: team config unreadable", "error", err)
		return nil //nolint:nilerr // an unreadable team state never blocks the work
	}
	limit := tracker.InProgressLimit(&cfg.Tracker, local)
	if limit <= 0 {
		return nil
	}
	claims, err := repo.ListClaims(ctx, "")
	if err != nil {
		slog.Warn("in-progress limit not checked: claims unreadable", "error", err)
		return nil //nolint:nilerr // an unreadable team state never blocks the work
	}
	return teamstate.CheckInProgressRoom(claims, member, cfg.Board, limit, becoming)
}

// checkTransferToSelfRoom is checkInProgressRoom for a claim handed over to
// member: it counts only when its status is work in progress (the transfer
// keeps the status).
func checkTransferToSelfRoom(ctx context.Context, repo wipStore, local config.TrackerLocalConfig, member string, c *teamstate.Claim) error {
	if c == nil || member == "" || c.ClaimedBy == member {
		return nil
	}
	cfg, err := repo.LoadConfig(ctx)
	if err != nil || cfg == nil {
		return nil //nolint:nilerr // an unreadable team state never blocks the work
	}
	if !c.CountsInProgress(cfg.Board) {
		return nil
	}
	return checkInProgressRoom(ctx, repo, local, member, teamstate.ClaimRef{Project: c.Project, TicketID: c.TicketID})
}

// checkStatusMoveRoom is checkInProgressRoom for a claim of member moved to
// newStatus (a card dragged to a column of the board): it counts when the
// claim would then be work in progress. The claims of the others are not
// checked here.
func checkStatusMoveRoom(ctx context.Context, repo wipStore, local config.TrackerLocalConfig, member string, c *teamstate.Claim, newStatus string) error {
	if c == nil || member == "" || c.ClaimedBy != member {
		return nil
	}
	cfg, err := repo.LoadConfig(ctx)
	if err != nil || cfg == nil {
		return nil //nolint:nilerr // an unreadable team state never blocks the work
	}
	moved := *c
	moved.Status = newStatus
	if !moved.CountsInProgress(cfg.Board) {
		return nil
	}
	return checkInProgressRoom(ctx, repo, local, member, teamstate.ClaimRef{Project: c.Project, TicketID: c.TicketID})
}

// ticketRoom returns the check run before a session starts on tickets of a
// team project (launch.Deps.TicketRoom): it refuses when the tickets that the
// session would start (no claim yet, or the member's own claim still planned)
// go past the limit of the active member. Nil without a team.
func ticketRoom(a *app.App, project *domain.Project) func(ctx context.Context, projectID string, tickets []string) error {
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
	local := a.Config.Tracker
	return func(ctx context.Context, projectID string, tickets []string) error {
		if !repo.IsCloned() {
			return nil
		}
		return checkTicketsRoom(ctx, repo, local, member, projectID, tickets)
	}
}

// checkTicketsRoom is the check of ticketRoom: which of the tickets a session
// would start (startTicketClaim) are new work for member.
func checkTicketsRoom(ctx context.Context, repo wipStore, local config.TrackerLocalConfig, member, projectID string, tickets []string) error {
	var starting []teamstate.ClaimRef
	var initial string
	if cfg, err := repo.LoadConfig(ctx); err == nil && cfg != nil {
		initial = cfg.Board.InitialStatus()
	}
	for _, t := range tickets {
		c, err := repo.GetClaim(ctx, projectID, t)
		switch {
		case err != nil && !errors.Is(err, teamstate.ErrClaimNotFound):
			continue // not readable: startTicketClaim will tell
		case err != nil:
			// No claim: the session claims it in progress.
		case c.ClaimedBy != member || (c.Status != initial && c.Status != teamstate.ClaimStatusPlanned):
			// Somebody else's, or already started or in review: the session
			// starts nothing new.
			continue
		}
		starting = append(starting, teamstate.ClaimRef{Project: projectID, TicketID: t})
	}
	return checkInProgressRoom(ctx, repo, local, member, starting...)
}
