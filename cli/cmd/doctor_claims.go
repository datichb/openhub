package cmd

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Doctor › team claims: the claims of the active
// member left behind — ticket closed while its claim is still open, or the
// sessions that worked on the ticket all ended.

func init() {
	registerDoctorCheck(claimsDoctorChecks)
}

// orphanClaim is a claim of the member that nothing holds any more.
type orphanClaim struct {
	Project, Ticket string
	// ProjectName is the name of the project, shown next to its ID (empty: the
	// ID alone).
	ProjectName string
	Reason      string // closed | ended
}

// claimProjectLabel names a project in the lists of the doctor: its name and, in
// brackets, the ID the commands take (-p). A project without a name, or named
// like its ID, is shown by its ID.
func claimProjectLabel(id, name string) string {
	if name == "" || name == id {
		return id
	}
	return name + " [" + id + "]"
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
		return []doctor.Result{claimsReport(nil, nil, nil, i18n.Tf("cmd.doctor.claims.unverified.projects", err))}
	}
	var (
		orphans    []orphanClaim
		unverified []string
		teamed     bool
	)
	for i := range projects {
		p := &projects[i]
		repo, member, ok := teamClaimsRepo(a, p)
		if !ok {
			continue
		}
		teamed = true
		claims, err := repo.ListClaims(ctx, p.ID)
		if err != nil {
			unverified = append(unverified, i18n.Tf("cmd.doctor.claims.unverified.list", claimProjectLabel(p.ID, p.Name), err))
			continue
		}
		var board teamstate.BoardConfig
		if cfg, err := repo.LoadConfig(ctx); err == nil && cfg != nil {
			board = cfg.Board
		}
		sessions, err := a.Sessions.List(ctx, p.ID)
		if err != nil {
			unverified = append(unverified, i18n.Tf("cmd.doctor.claims.unverified.sessions", claimProjectLabel(p.ID, p.Name), err))
			sessions = nil
		}
		closed := recordClosedFailures(claimsClosedSource(p.Path))
		found := orphanClaims(p.ID, member, claims, board.TerminalStatuses(), sessions, closed.check)
		for j := range found {
			found[j].ProjectName = p.Name
		}
		orphans = append(orphans, found...)
		if len(closed.tickets) > 0 {
			unverified = append(unverified, i18n.Tf("cmd.doctor.claims.unverified.closed", claimProjectLabel(p.ID, p.Name),
				len(closed.tickets), shortList(closed.tickets), closed.first))
		}
	}
	if !teamed {
		return nil
	}
	found, unresolved, notes := findClaimsRehomes(ctx, a)
	unverified = append(unverified, notes...)
	return []doctor.Result{claimsReport(orphans, found, unresolved, unverified...)}
}

// claimsClosedSource reads the status of the tickets in the Beads database of
// the project. A project without Beads keeps its tickets elsewhere (the
// tracker): there is no status to read, which is not a failed read.
func claimsClosedSource(projectPath string) ticketClosed {
	if !beads.IsInitialized(projectPath) {
		return func(string) (bool, error) { return false, nil }
	}
	return beadsClosed(projectPath)
}

// closedFailures remembers the tickets whose status could not be read: the
// claims on them are not known to be open.
type closedFailures struct {
	check   ticketClosed
	tickets []string
	first   error
}

func recordClosedFailures(closed ticketClosed) *closedFailures {
	f := &closedFailures{}
	f.check = func(id string) (bool, error) {
		isClosed, err := closed(id)
		if err != nil {
			f.tickets = append(f.tickets, id)
			if f.first == nil {
				f.first = err
			}
		}
		return isClosed, err
	}
	return f
}

func claimsCheck(orphans []orphanClaim) doctor.Result { return claimsReport(orphans, nil, nil) }

// claimsReport is the result of the claims check: the orphan claims of the
// member and the claims under an ID no project has any more. Orphans and
// claims that --fix can attach fail the check; claims under an unknown ID that
// nothing attaches only warn. unverified tells what could not be read (and so
// not checked): it turns a green result into a warning, and is added to the
// others; the check never says "no orphan claim" after a failed read.
func claimsReport(orphans []orphanClaim, found []claimsRehome, unresolved []claimsUnresolved, unverified ...string) doctor.Result {
	res := claimsFindings(orphans, found, unresolved)
	if len(unverified) == 0 {
		return res
	}
	text := i18n.Tf("cmd.doctor.claims.unverified", strings.Join(unverified, " ; "))
	if res.OK && !res.Warn && len(orphans) == 0 {
		res.Detail = text // replaces "no orphan claim"
	} else {
		res.Detail += " — " + text
	}
	if res.OK {
		res.Warn = true
	}
	return res
}

func claimsFindings(orphans []orphanClaim, found []claimsRehome, unresolved []claimsUnresolved) doctor.Result {
	res := orphanClaimsResult(orphans)
	detail, repairable := claimsProjectDetail(found, unresolved)
	if detail == "" {
		return res
	}
	res.ID = doctorIDClaimsUnknown
	if repairable {
		res.ID = doctorIDClaimsProject
	}
	if res.OK {
		res.Detail = detail
	} else {
		res.Detail += " — " + detail
	}
	if !repairable && res.OK {
		// Claims under an ID no project of this machine has may belong to a
		// project that lives on another machine: a warning, not a failure
		// that would change the exit code of oh doctor.
		res.Warn = true
		return res
	}
	res.OK = false
	return res
}

func orphanClaimsResult(orphans []orphanClaim) doctor.Result {
	name := i18n.T("cmd.doctor.claims.name")
	if len(orphans) == 0 {
		return doctor.Result{Name: name, OK: true, Detail: i18n.T("cmd.doctor.claims.ok")}
	}
	parts := make([]string, len(orphans))
	for i, o := range orphans {
		parts[i] = fmt.Sprintf("%s/%s (%s)", claimProjectLabel(o.Project, o.ProjectName), o.Ticket, i18n.T("cmd.doctor.claims."+o.Reason))
	}
	first := orphans[0]
	return doctor.Result{Name: name, OK: false,
		Detail: i18n.Tf("cmd.doctor.claims.orphans", strings.Join(parts, ", "), first.Ticket, first.Project)}
}
