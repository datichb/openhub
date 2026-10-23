package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// Team claims of the tickets of a session at its end (v5 corrections, A31;
// v5.1, B3a): teamstate.DecideClaimEnd decides for each claim of the member
// who ran the session, from the facts gathered here (ticket, claim,
// checkpoints, work left). The decisions become operations of the durable
// queue of the team-state clone (conditioned on the member and on the
// session), which is then flushed for a bounded time: a remote that does not
// answer leaves them in the queue and never holds the end of the session.

const (
	// claimEndLockWait and claimEndFlushLimit bound what the end of a
	// session waits for the team-state when a person is waiting for oh to
	// return: the lock of the clone, and the whole flush (pull, commit, push).
	claimEndLockWait   = 10 * time.Second
	claimEndFlushLimit = 15 * time.Second
	// claimEndBackgroundLimit is the flush of the daemon, which waits for
	// nobody.
	claimEndBackgroundLimit = 2 * time.Minute
)

// ticketClosed reports whether a ticket is closed (err: unknown).
type ticketClosed func(id string) (bool, error)

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

// ticketFacts is what the tracker says about a ticket.
type ticketFacts struct {
	Closed bool
	Labels []string
}

// ticketLookup reads a ticket (err: unknown).
type ticketLookup func(id string) (ticketFacts, error)

// sessionEndState is what the workflow of the session left at its end, the
// same for every ticket of the session.
type sessionEndState struct {
	// Waiting is the checkpoint waiting for a decision, Deferred the commit
	// checkpoint that was put off ("" = none).
	Waiting, Deferred string
	// Work is what the session left in its location (read only when a
	// checkpoint was deferred).
	Work teamstate.ClaimWork
	// Unreadable: the state or the workflow of the session could not be read.
	Unreadable bool
}

// claimEnd is what the end of a session does to a claim.
type claimEnd string

const (
	claimDone     claimEnd = "done"     // moved to the terminal status
	claimReleased claimEnd = "released" // removed
	claimKept     claimEnd = "kept"     // left as it is
	claimMarked   claimEnd = "marked"   // left, labelled session-ended
)

// claimEndOutcome is the decision taken for a claim and why.
type claimEndOutcome struct {
	End    claimEnd
	Reason teamstate.ClaimEndReason
	// Checkpoint is the checkpoint that explains a kept claim.
	Checkpoint string
}

// sessionClaimsInput is a session whose claims end.
type sessionClaimsInput struct {
	Project, Member string
	// Session is the ID of the session: the claims of another session are
	// left alone, and the queued operations only apply while the claim is
	// still on this session.
	Session string
	Tickets []string
	Ticket  ticketLookup
	// State reads the checkpoint state and the work of the session; called
	// once, and only when a ticket needs it.
	State func(ctx context.Context) sessionEndState
}

// endTicketClaims decides what happens to the claims of the tickets of an
// ended session and queues the operations (EnqueueClaimOp: local, never
// waits for git). The claims of other members, those on another session,
// those of unknown tickets and the ones with no claim are left as they are.
// Nothing is written to the team-state: FlushClaimQueue applies the queue.
func endTicketClaims(ctx context.Context, repo *teamstate.Repo, in sessionClaimsInput) map[string]claimEndOutcome {
	out := map[string]claimEndOutcome{}
	var board teamstate.BoardConfig
	if cfg, err := repo.LoadConfig(); err == nil && cfg != nil {
		board = cfg.Board
	}
	terminals := board.TerminalStatuses()
	var state *sessionEndState
	for _, t := range in.Tickets {
		c, err := repo.GetClaim(in.Project, t)
		if err != nil || c.ClaimedBy != in.Member {
			continue // no claim, or someone else's
		}
		// A session that took the claim over while the remote was out of
		// reach (a queued adoption) is the one the claim belongs to.
		owner := c.Session
		if queued, ok := repo.QueuedAdoption(in.Project, t, in.Member); ok {
			owner = queued
		}
		if in.Session != "" && owner != "" && owner != in.Session {
			slog.Info("team claim left to its session", "ticket", t, "session", in.Session, "claim_session", owner)
			continue
		}
		tk, err := in.Ticket(t)
		if err != nil {
			continue // ticket unknown: left as is (Doctor reports it)
		}
		facts := teamstate.ClaimEndFacts{
			Closed:        tk.Closed,
			ClaimStatus:   c.Status,
			ClaimTerminal: slices.Contains(terminals, c.Status),
			Labels:        slices.Concat(tk.Labels, c.Labels),
		}
		if !tk.Closed && !facts.ClaimTerminal && in.State != nil {
			if state == nil {
				s := in.State(ctx)
				state = &s
			}
			facts.WaitingCheckpoint, facts.DeferredCheckpoint = state.Waiting, state.Deferred
			facts.Work, facts.CheckpointsUnreadable = state.Work, state.Unreadable
		}
		d := teamstate.DecideClaimEnd(facts)
		op := teamstate.ClaimOp{Project: in.Project, TicketID: t, ClaimedBy: in.Member, Session: c.Session}
		outcome := claimEndOutcome{Reason: d.Reason}
		switch d.Action {
		case teamstate.ClaimEndComplete:
			op.Kind, op.Status, op.AnyTransition = teamstate.ClaimOpStatus, terminals[0], true
			outcome.End = claimDone
		case teamstate.ClaimEndRelease:
			op.Kind = teamstate.ClaimOpRelease
			outcome.End = claimReleased
		case teamstate.ClaimEndKeepMarked:
			outcome.End, outcome.Checkpoint = claimMarked, firstNonEmpty(facts.WaitingCheckpoint, facts.DeferredCheckpoint)
			if !slices.Contains(c.Labels, teamstate.LabelSessionEnded) {
				op.Kind, op.Label = teamstate.ClaimOpLabel, teamstate.LabelSessionEnded
			}
		default:
			outcome.End = claimKept
		}
		if op.Kind != "" {
			if err := repo.EnqueueClaimOp(op); err != nil {
				slog.Warn("team claim not queued", "ticket", t, "error", err)
				continue
			}
		}
		out[t] = outcome
	}
	return out
}

// claimFlush is the result of the flush of the queue at the end of a session.
type claimFlush struct {
	Applied, Dropped int
	// Pending is the number of operations left in the queue.
	Pending int
	Err     error
}

// flushClaimQueue applies the queue of the clone, waiting at most limit (and
// lockWait for its lock). A remote that cannot be reached, a clone that
// stays busy or the end of limit leave the operations in the queue.
func flushClaimQueue(ctx context.Context, repo *teamstate.Repo, lockWait, limit time.Duration) claimFlush {
	fctx, cancel := context.WithTimeout(teamstate.WithLockWait(ctx, lockWait), limit)
	defer cancel()
	var f claimFlush
	f.Applied, f.Dropped, f.Err = repo.FlushClaimQueue(fctx)
	if f.Err != nil {
		slog.Warn("team claims queue not flushed", "error", f.Err)
		if ops, err := repo.ClaimQueue(); err == nil {
			f.Pending = len(ops)
		}
	}
	return f
}

// claimFlushNote tells the person that operations stay in the queue ("" when
// there is nothing to say).
func claimFlushNote(f claimFlush) string {
	if f.Err == nil {
		return ""
	}
	var cause string
	switch {
	case errors.Is(f.Err, teamstate.ErrOffline):
		cause = i18n.T("cmd.claims.end.cause.offline")
	case errors.Is(f.Err, context.DeadlineExceeded), errors.Is(f.Err, teamstate.ErrBusy):
		cause = i18n.T("cmd.claims.end.cause.timeout")
	default:
		cause = f.Err.Error()
	}
	return i18n.Tf("cmd.claims.end.queued", f.Pending, cause)
}

// claimKeptNote explains a claim kept after the session (marked
// session-ended); "" for the others.
func claimKeptNote(ticket string, o claimEndOutcome) string {
	if o.End != claimMarked {
		return ""
	}
	switch o.Reason {
	case teamstate.ClaimEndCheckpointWaiting:
		return i18n.Tf("cmd.claims.end.kept.checkpoint_waiting", ticket, o.Checkpoint)
	case teamstate.ClaimEndCheckpointDeferred:
		return i18n.Tf("cmd.claims.end.kept.checkpoint_deferred", ticket, o.Checkpoint)
	case teamstate.ClaimEndCheckpointUnknown:
		return i18n.Tf("cmd.claims.end.kept.checkpoint_unknown", ticket)
	}
	return ""
}

// ticketFactsFromBeads reads a ticket in the Beads database of a project.
func ticketFactsFromBeads(projectPath string) ticketLookup {
	return func(id string) (ticketFacts, error) {
		d, err := beads.Show(projectPath, id)
		if err != nil {
			return ticketFacts{}, err
		}
		return ticketFacts{Closed: beads.IsClosedStatus(d.Status), Labels: d.Labels}, nil
	}
}

// sessionEndStateOf reads the checkpoint state and the work of a session
// from the checkpoint service and the bundle of the session. A session
// without workflow has nothing pending. Everything it cannot read is
// reported as Unreadable, not as nothing: the caller keeps the claims.
func sessionEndStateOf(a *app.App, s domain.Session) func(ctx context.Context) sessionEndState {
	return func(ctx context.Context) sessionEndState {
		if s.BundleHash == "" {
			return sessionEndState{}
		}
		if store == nil {
			return sessionEndState{Unreadable: true}
		}
		b, err := bundle.Load(ohBundlesDir(), s.BundleHash)
		if err != nil {
			slog.Warn("team claims: bundle of the session unreadable", "session", s.ID, "error", err)
			return sessionEndState{Unreadable: true}
		}
		wf := b.Spec.Workflow
		if wf == nil {
			return sessionEndState{}
		}
		cs, err := newCheckpointService(a).States.GetCheckpointState(ctx, s.ID)
		if err != nil {
			slog.Warn("team claims: checkpoint state unreadable", "session", s.ID, "error", err)
			return sessionEndState{Unreadable: true}
		}
		st := sessionEndState{Waiting: cs.Waiting, Deferred: deferredCommitCheckpoint(wf, &cs)}
		if st.Deferred != "" {
			st.Work = sessionWork(ctx, s)
		}
		return st
	}
}

// deferredCommitCheckpoint is the first checkpoint of the workflow that
// unlocks the commit (cp-2 of ticket and implement-spec) and that the session
// did not pass ("" = none): the commit was put off. No mode skips such a
// checkpoint (CheckpointDef.Behavior).
func deferredCommitCheckpoint(wf *sessionspec.WorkflowRuntime, cs *domain.CheckpointState) string {
	for _, c := range wf.Checkpoints {
		if slices.Contains(c.Unlocks, sessionspec.UnlockCommit) && !cs.HasPassed(c.ID) {
			return c.ID
		}
	}
	return ""
}

// sessionWork tells whether the session left work in its location: commits
// since it started, uncommitted or untracked changes (outside .beads). A
// location that cannot be read, or that is not on this machine (remote
// runtime), gives ClaimWorkUnknown.
func sessionWork(ctx context.Context, s domain.Session) teamstate.ClaimWork {
	if s.LaunchPath == "" || s.Runtime == string(sessionspec.RuntimeRemote) {
		return teamstate.ClaimWorkUnknown
	}
	paths, err := checkpoint.GitChanged(ctx, s.LaunchPath, s.StartRef)
	if err != nil {
		slog.Warn("team claims: work of the session unreadable", "session", s.ID, "error", err)
		return teamstate.ClaimWorkUnknown
	}
	for _, p := range paths {
		if p = filepath.ToSlash(p); p != ".beads" && !strings.HasPrefix(p, ".beads/") {
			return teamstate.ClaimWorkPresent
		}
	}
	return teamstate.ClaimWorkNone
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

// sessionClaimsEnd ends the team claims of an ended session. The decisions
// are queued then the queue is flushed: for a bounded time and while the
// person waits (async=false: the CLI says what stays in the queue), or in the
// background for the daemon (async), which must not wait for git and has no
// terminal.
func sessionClaimsEnd(ctx context.Context, a *app.App, proj *domain.Project, s domain.Session, async bool) {
	if len(s.Tickets) == 0 {
		return
	}
	repo, member, ok := teamClaimsRepo(a, proj)
	if !ok {
		return
	}
	in := sessionClaimsInput{
		Project: s.ProjectID, Member: member, Session: s.ID, Tickets: s.Tickets,
		Ticket: ticketFactsFromBeads(proj.Path), State: sessionEndStateOf(a, s),
	}
	if !async {
		reportClaimsEnd(a, endSessionClaims(ctx, repo, in, claimEndLockWait, claimEndFlushLimit))
		return
	}
	safego.Go(func() {
		bctx, cancel := context.WithTimeout(context.Background(), claimEndBackgroundLimit+time.Minute)
		defer cancel()
		endSessionClaims(bctx, repo, in, teamstate.BackgroundLockWait, claimEndBackgroundLimit)
	})
}

// claimsEndReport is what the end of a session did to the claims.
type claimsEndReport struct {
	Outcomes map[string]claimEndOutcome
	Flush    claimFlush
}

// endSessionClaims queues the end of the claims then flushes the queue.
func endSessionClaims(ctx context.Context, repo *teamstate.Repo, in sessionClaimsInput, lockWait, limit time.Duration) claimsEndReport {
	r := claimsEndReport{Outcomes: endTicketClaims(ctx, repo, in)}
	r.Flush = flushClaimQueue(ctx, repo, lockWait, limit)
	return r
}

// reportClaimsEnd tells the person which claims stay after the session and
// what stays in the queue; the end of the session is not held by it.
func reportClaimsEnd(a *app.App, r claimsEndReport) {
	if a == nil || a.IO == nil || a.IO.ErrOut == nil {
		return
	}
	tickets := make([]string, 0, len(r.Outcomes))
	for t := range r.Outcomes {
		tickets = append(tickets, t)
	}
	slices.Sort(tickets)
	for _, t := range tickets {
		if note := claimKeptNote(t, r.Outcomes[t]); note != "" {
			fmt.Fprintln(a.IO.ErrOut, note)
		}
	}
	if note := claimFlushNote(r.Flush); note != "" {
		fmt.Fprintln(a.IO.ErrOut, note)
	}
}
