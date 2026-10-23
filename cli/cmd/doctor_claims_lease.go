package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/doctor"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/textfmt"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// Doctor › lease of the claims and queue of the claim operations (v5.1, B3b).
//
//   - claims_expired: the open claims of the active member whose lease is
//     past (or, without lease, that have been inactive longer than stale_days)
//     or that carry the session-ended label. Another member may take such a
//     claim over (oh team claim). A warning with the command to release them;
//     starting a session on the ticket renews the lease.
//   - claims_queue: operations on the claims waiting in the durable queue of a
//     team clone for longer than the daemon takes to empty it (the remote was
//     out of reach, or oh stopped before the flush ended). A warning;
//     oh doctor --fix applies the queue.

// Identifiers of the results.
const (
	doctorIDClaimsExpired = "claims_expired"
	doctorIDClaimsQueue   = "claims_queue"
)

// claimQueueTooOld is how long an operation may wait in the queue before the
// doctor reports it: the period of the claims-queue task of the daemon.
const claimQueueTooOld = claimsQueueEvery

func init() {
	registerDoctorCheck(claimsLeaseDoctorChecks)
}

// expiredClaim is a claim of the member that nobody renews or holds any more.
type expiredClaim struct {
	Project, Ticket string
	Reason          string // lease | inactive | ended
}

// expiredClaims returns the open claims of member whose lease is past at now,
// that are inactive (no lease: the rule of stale_days) or that the end of a
// session labelled. A claim that carries no lease (pool, planned, finished) is
// never expired.
func expiredClaims(project, member string, claims []teamstate.Claim, cfg *teamstate.TeamConfig, now time.Time) []expiredClaim {
	var terminals []string
	if cfg != nil {
		terminals = cfg.Board.TerminalStatuses()
	}
	var out []expiredClaim
	for i := range claims {
		c := &claims[i]
		if c.ClaimedBy != member || slices.Contains(terminals, c.Status) || !c.HasLease(cfg) {
			continue
		}
		reason := ""
		switch {
		case c.IsExpired(now, cfg) && !c.LeaseUntil.IsZero():
			reason = "lease"
		case c.IsExpired(now, cfg):
			reason = "inactive"
		case slices.Contains(c.Labels, teamstate.LabelSessionEnded):
			reason = "ended"
		default:
			continue
		}
		out = append(out, expiredClaim{Project: project, Ticket: c.TicketID, Reason: reason})
	}
	return out
}

func expiredClaimsResult(list []expiredClaim) doctor.Result {
	name := i18n.T("cmd.doctor.claims_expired.name")
	if len(list) == 0 {
		return doctor.Result{ID: doctorIDClaimsExpired, Name: name, OK: true, Detail: i18n.T("cmd.doctor.claims_expired.ok")}
	}
	parts := make([]string, len(list))
	for i, c := range list {
		parts[i] = fmt.Sprintf("%s/%s (%s)", c.Project, c.Ticket, i18n.T("cmd.doctor.claims_expired.reason."+c.Reason))
	}
	return doctor.Result{ID: doctorIDClaimsExpired, Name: name, OK: true, Warn: true,
		Detail: i18n.Tf("cmd.doctor.claims_expired.found", strings.Join(parts, ", "), list[0].Ticket, list[0].Project)}
}

// claimQueueState is the queue of a team clone.
type claimQueueState struct {
	Repo   string
	Count  int
	Oldest time.Time
	// Corrupt is the error of a queue file that could not be read.
	Corrupt error
}

// claimQueueResults reports the queues that hold operations for longer than
// claimQueueTooOld, or that cannot be read; nothing otherwise.
func claimQueueResults(states []claimQueueState, now time.Time) []doctor.Result {
	name := i18n.T("cmd.doctor.claims_queue.name")
	var out []doctor.Result
	for _, s := range states {
		switch {
		case s.Corrupt != nil:
			out = append(out, doctor.Result{ID: doctorIDClaimsQueue, Name: name, OK: true, Warn: true,
				Detail: i18n.Tf("cmd.doctor.claims_queue.corrupt", s.Corrupt)})
		case s.Count > 0 && now.Sub(s.Oldest) > claimQueueTooOld:
			out = append(out, doctor.Result{ID: doctorIDClaimsQueue, Name: name, OK: true, Warn: true,
				Detail: i18n.Tf("cmd.doctor.claims_queue.found", s.Count, textfmt.AgeAt(now, s.Oldest), s.Repo)})
		}
	}
	return out
}

func claimQueueStates(teams []*claimsTeam) []claimQueueState {
	var out []claimQueueState
	for _, t := range teams {
		ops, err := t.repo.ClaimQueue()
		s := claimQueueState{Repo: t.repo.Path(), Count: len(ops), Corrupt: err}
		for _, op := range ops {
			if s.Oldest.IsZero() || op.QueuedAt.Before(s.Oldest) {
				s.Oldest = op.QueuedAt
			}
		}
		out = append(out, s)
	}
	return out
}

func claimsLeaseDoctorChecks(ctx context.Context) []doctor.Result {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	return claimsLeaseResults(ctx, a, time.Now())
}

// claimsLeaseResults runs both checks over the team clones of the active
// projects. Nothing is reported without a team.
func claimsLeaseResults(ctx context.Context, a *app.App, now time.Time) []doctor.Result {
	teams := claimTeams(ctx, a, a.Config)
	if len(teams) == 0 {
		return nil
	}
	var expired []expiredClaim
	for _, t := range teams {
		cfg, _ := t.repo.LoadConfig()
		projects := make([]string, 0, len(t.projects))
		for p := range t.projects {
			projects = append(projects, p)
		}
		slices.Sort(projects)
		for _, p := range projects {
			claims, err := t.repo.ListClaims(p)
			if err != nil {
				continue
			}
			expired = append(expired, expiredClaims(p, t.member, claims, cfg, now)...)
		}
	}
	results := []doctor.Result{expiredClaimsResult(expired)}
	return append(results, claimQueueResults(claimQueueStates(teams), now)...)
}

// fixClaimsQueue is oh doctor --fix for the claims queue.
func fixClaimsQueue(cmd *cobra.Command) error {
	return repairClaimsQueue(cmd.Context(), cmd.OutOrStdout(), cmdConfirm(cmd))
}

// repairClaimsQueue applies the operations waiting in the queue of the team
// clones, after one confirmation; a remote that does not answer leaves them
// where they are. It asks nothing and does nothing when no queue holds
// operations.
func repairClaimsQueue(ctx context.Context, w io.Writer, confirm confirmFunc) error {
	a := TryApp()
	if a == nil || a.Config == nil {
		return nil
	}
	var todo []*claimsTeam
	count := 0
	for _, t := range claimTeams(ctx, a, a.Config) {
		if ops, err := t.repo.ClaimQueue(); err == nil && len(ops) > 0 {
			todo = append(todo, t)
			count += len(ops)
		}
	}
	if len(todo) == 0 {
		return nil
	}
	fmt.Fprintln(w, theme.Bold.Render(i18n.T("cmd.doctor.claims_queue.fix_title")))
	fmt.Fprintln(w, i18n.Tf("cmd.doctor.claims_queue.fix_plan", count, len(todo)))
	ok, err := confirm(i18n.T("cmd.doctor.claims_queue.fix_confirm"))
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(w, i18n.T("cmd.doctor.fix.cancelled"))
		return nil
	}
	for _, t := range todo {
		f := flushClaimQueue(ctx, t.repo, claimEndLockWait, claimEndFlushLimit)
		switch {
		case f.Err == nil:
			fmt.Fprintf(w, "%s %s\n", theme.SuccessStyle.Render(theme.IconSuccess),
				i18n.Tf("cmd.doctor.claims_queue.fix_done", f.Applied, f.Dropped))
		case errors.Is(f.Err, teamstate.ErrOffline):
			fmt.Fprintf(w, "%s %s\n", theme.WarningStyle.Render(theme.IconWarning),
				i18n.Tf("cmd.doctor.claims_queue.fix_offline", f.Pending))
		default:
			return fmt.Errorf("%s", i18n.Tf("cmd.doctor.claims_queue.fix_failed", f.Err))
		}
	}
	return nil
}

// planClaimsQueueFix is the plan shown by the Doctor view.
func planClaimsQueueFix(ctx context.Context) (string, error) {
	a := TryApp()
	if a == nil || a.Config == nil {
		return i18n.T("cmd.doctor.fix.nothing"), nil
	}
	count, repos := 0, 0
	for _, t := range claimTeams(ctx, a, a.Config) {
		if ops, err := t.repo.ClaimQueue(); err == nil && len(ops) > 0 {
			count += len(ops)
			repos++
		}
	}
	if count == 0 {
		return i18n.T("cmd.doctor.fix.nothing"), nil
	}
	return i18n.Tf("cmd.doctor.claims_queue.fix_plan", count, repos), nil
}

// claimsQueueFix is the repair of the claims queue, for the Doctor view.
func claimsQueueFix(r doctor.Result) *doctor.Fix {
	if r.ID != doctorIDClaimsQueue {
		return nil
	}
	return &doctor.Fix{Plan: planClaimsQueueFix, Apply: doctorFixRun(func(ctx context.Context, w io.Writer) error {
		return repairClaimsQueue(ctx, w, alwaysConfirm)
	})}
}
