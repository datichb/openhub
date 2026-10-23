package cmd

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Doctor › team claims (v5 corrections, A31): the claims of the active
// member left behind — ticket closed while its claim is still open, or the
// sessions that worked on the ticket all ended.

func init() {
	registerDoctorCheck(claimsDoctorChecks)
	registerDoctorCheck(teamBetaDoctorChecks)
}

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

func claimsDoctorChecks(ctx context.Context) []doctor.Result {
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
	return []doctor.Result{claimsCheck(orphans)}
}

func claimsCheck(orphans []orphanClaim) doctor.Result {
	name := i18n.T("cmd.doctor.claims.name")
	if len(orphans) == 0 {
		return doctor.Result{Name: name, OK: true, Detail: i18n.T("cmd.doctor.claims.ok")}
	}
	parts := make([]string, len(orphans))
	for i, o := range orphans {
		parts[i] = fmt.Sprintf("%s/%s (%s)", o.Project, o.Ticket, i18n.T("cmd.doctor.claims."+o.Reason))
	}
	first := orphans[0]
	return doctor.Result{Name: name, OK: false,
		Detail: i18n.Tf("cmd.doctor.claims.orphans", strings.Join(parts, ", "), first.Ticket, first.Project)}
}

// teamBetaDoctorCheck reports the beta state of the team mode: a warning
// (it never fails the doctor) when a team is enabled, nothing otherwise.
func teamBetaDoctorCheck(a *app.App, projects []domain.Project) []doctor.Result {
	if !teamModeUsed(a, projects) {
		return nil
	}
	return []doctor.Result{{
		Name:   i18n.T("cmd.team.beta.doctor.name"),
		OK:     true,
		Warn:   true,
		Detail: i18n.T("cmd.team.beta.doctor.detail"),
	}}
}

// teamModeUsed reports whether the hub team or the team of a project is
// enabled (not a solo space).
func teamModeUsed(a *app.App, projects []domain.Project) bool {
	if a == nil || a.Config == nil {
		return false
	}
	if t := a.Config.ActiveTeam(); t.Enabled && !t.Solo {
		return true
	}
	for i := range projects {
		if resolvedTeamConfig(a, &projects[i]).Enabled {
			return true
		}
	}
	return false
}

// teamBetaDoctorChecks is the registered doctor check of the beta state.
func teamBetaDoctorChecks(ctx context.Context) []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	var projects []domain.Project
	if a.Projects != nil {
		projects, _ = a.Projects.List(ctx, domain.ProjectStatusActive)
	}
	return teamBetaDoctorCheck(a, projects)
}
