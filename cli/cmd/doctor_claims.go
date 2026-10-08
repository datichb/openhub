package cmd

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Doctor › team claims (v5 corrections, A31): the claims of the active
// member left behind — ticket closed while its claim is still open, or the
// sessions that worked on the ticket all ended.

func init() { registerDoctorCheck(claimsDoctorChecks) }

// orphanClaim is a claim of the member that nothing holds any more.
type orphanClaim struct {
	Project, Ticket string
	Reason          string // closed | ended
}

// orphanClaims returns the open claims of member on a project whose ticket
// is closed, or whose sessions (those that recorded the ticket) all ended.
func orphanClaims(projectID, member string, claims []teamstate.Claim, terminal []string, sessions []domain.Session, closed ticketClosed) []orphanClaim {
	var out []orphanClaim
	for _, c := range claims {
		if c.ClaimedBy != member || slices.Contains(terminal, c.Status) {
			continue
		}
		if isClosed, err := closed(c.TicketID); err == nil && isClosed {
			out = append(out, orphanClaim{Project: projectID, Ticket: c.TicketID, Reason: "closed"})
			continue
		}
		worked, open := false, false
		for _, s := range sessions {
			if !slices.Contains(s.Tickets, c.TicketID) {
				continue
			}
			worked = true
			switch s.State {
			case domain.RunCompleted, domain.RunFailed, domain.RunStopped:
			default:
				open = true
			}
		}
		if worked && !open {
			out = append(out, orphanClaim{Project: projectID, Ticket: c.TicketID, Reason: "ended"})
		}
	}
	return out
}

func claimsDoctorChecks(ctx context.Context) []views.DoctorCheck {
	a := TryApp()
	if a == nil || a.Projects == nil || a.Sessions == nil {
		return nil
	}
	projects, err := a.Projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		return nil
	}
	var (
		orphans []orphanClaim
		teamed  bool
	)
	for i := range projects {
		p := &projects[i]
		repo, member, ok := teamClaimsRepo(a, p)
		if !ok {
			continue
		}
		teamed = true
		claims, err := repo.ListClaims(p.ID)
		if err != nil {
			continue
		}
		var board teamstate.BoardConfig
		if cfg, err := repo.LoadConfig(); err == nil && cfg != nil {
			board = cfg.Board
		}
		sessions, _ := a.Sessions.List(ctx, p.ID)
		orphans = append(orphans, orphanClaims(p.ID, member, claims, board.TerminalStatuses(), sessions, beadsClosed(p.Path))...)
	}
	if !teamed {
		return nil
	}
	return []views.DoctorCheck{claimsCheck(orphans)}
}

func claimsCheck(orphans []orphanClaim) views.DoctorCheck {
	name := i18n.T("cmd.doctor.claims.name")
	if len(orphans) == 0 {
		return views.DoctorCheck{Name: name, OK: true, Detail: i18n.T("cmd.doctor.claims.ok")}
	}
	parts := make([]string, len(orphans))
	for i, o := range orphans {
		parts[i] = fmt.Sprintf("%s/%s (%s)", o.Project, o.Ticket, i18n.T("cmd.doctor.claims."+o.Reason))
	}
	first := orphans[0]
	return views.DoctorCheck{Name: name, OK: false,
		Detail: i18n.Tf("cmd.doctor.claims.orphans", strings.Join(parts, ", "), first.Ticket, first.Project)}
}
