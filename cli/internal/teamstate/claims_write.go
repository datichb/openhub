package teamstate

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// Grouped write of a claim: what a launch or a review verdict does to one
// claim (take it, move it on the board, tie it to a session, label it, log
// events) is one transaction, so one pull, one commit and one push, instead of
// one round trip to the remote per step.

// ClaimStatusMove moves a claim to a status of the board.
type ClaimStatusMove struct {
	// To is the status to reach.
	To string
	// From limits the move to claims at one of these statuses; empty: any.
	From []string
	// Board, when set, routes the move through the board of the team: a claim
	// still at work whose transition to a terminal status is not allowed goes
	// through the review column when the board has one, and a board without
	// review column applies the move directly, as the tracker sync does. Nil:
	// the transitions of the board are enforced as UpdateClaimStatus does.
	Board *BoardConfig
	// BestEffort: a move that is refused (invalid status or transition) is
	// reported in ClaimWriteResult.StatusErr and does not stop the write.
	BestEffort bool
}

// ClaimWrite is the grouped write on one claim (WriteClaim). The steps run in
// this order, all in the same transaction: take, move, adopt, label, events.
type ClaimWrite struct {
	Project  string
	TicketID string
	// Member is the member the write is for. A claim held by somebody else is
	// left alone (ClaimWriteResult.Held). Empty: whoever holds the claim.
	Member string
	// Create takes the ticket when it has no claim yet (the claim.taken event
	// of Member goes in the same commit). Nil: a missing claim is
	// ErrClaimNotFound.
	Create *Claim
	// Status moves a claim that already existed (a new claim is created with
	// the status of Create). Nil: the status is left as it is.
	Status *ClaimStatusMove
	// Session ties the claim to the session that takes the work up (AdoptClaim).
	Session string
	// Labels are added to the claim.
	Labels []string
	// Events are appended to the activity log.
	Events []Event
	// Identity records the identity of the project (projects/<id>/project.toml)
	// in the same commit when it has none yet (EnsureProjectIdentity); an
	// existing file is left alone. Nothing is recorded for a claim of another
	// member. Nil: not recorded.
	Identity *ProjectIdentity
	// Message is the commit message; empty: the one of the steps that ran.
	Message string
}

// ClaimWriteResult tells what WriteClaim did.
type ClaimWriteResult struct {
	// Created: the claim did not exist and was taken.
	Created bool
	// IdentityRecorded: the identity of the project was written.
	IdentityRecorded bool
	// Held is the claim of another member, when there is one: nothing was
	// written.
	Held *Claim
	// From is the status the claim had before the move; Moved tells it moved.
	From  string
	Moved bool
	// Adopted: the claim was tied to Session (it was not already).
	Adopted bool
	// StatusErr is the refusal of a BestEffort move.
	StatusErr error
}

// ClaimGroupWriter is the grouped write of a claim.
type ClaimGroupWriter interface {
	WriteClaim(ctx context.Context, w ClaimWrite) (ClaimWriteResult, error)
}

var _ ClaimGroupWriter = (*Repo)(nil)

// WriteClaim runs the steps of w in one transaction: a single commit and a
// single push, whatever the number of steps. A remote that cannot be reached
// gives ErrOffline (nothing is committed), like any transaction; any other
// failure leaves nothing written. A claim of another member (w.Member), or
// nothing to change, commits nothing.
func (r *Repo) WriteClaim(ctx context.Context, w ClaimWrite) (ClaimWriteResult, error) {
	if _, err := SafeName(w.Project); err != nil {
		return ClaimWriteResult{}, fmt.Errorf("invalid project name: %w", err)
	}
	if _, err := SafeName(w.TicketID); err != nil {
		return ClaimWriteResult{}, fmt.Errorf("invalid ticket ID: %w", err)
	}
	var res ClaimWriteResult
	err := r.Transact(ctx, func(ctx context.Context, _ *Tx) (TxResult, error) {
		res = ClaimWriteResult{} // the transaction may run again
		return TxResult{Message: w.Message}, r.writeClaimSteps(ctx, w, &res)
	})
	if err != nil {
		return ClaimWriteResult{}, err
	}
	if res.Adopted {
		// The claim is now on this session: what the other sessions of the
		// claim had queued is cancelled, as AdoptClaimDurable does. The queue
		// is local, so a deadline that ended the push must not stop it.
		r.supersedeClaimOps(context.WithoutCancel(ctx), w.Project, w.TicketID, w.Member, w.Session)
	}
	return res, nil
}

func (r *Repo) writeClaimSteps(ctx context.Context, w ClaimWrite, res *ClaimWriteResult) error {
	claim, err := r.getClaim(w.Project, w.TicketID)
	switch {
	case errors.Is(err, ErrClaimNotFound) && w.Create != nil:
		c := *w.Create
		c.Project, c.TicketID = w.Project, w.TicketID
		if c.ClaimedBy == "" {
			c.ClaimedBy = w.Member
		}
		if c.Session == "" {
			c.Session = w.Session
		}
		cctx := ctx
		if w.Member != "" {
			cctx = WithClaimEvent(ctx, w.Member)
		}
		if _, err := r.CreateClaim(cctx, c); err != nil {
			return err
		}
		res.Created = true
	case err != nil:
		return err
	case w.Member != "" && claim.ClaimedBy != w.Member:
		res.Held = claim
		return nil
	default:
		res.From = claim.Status
		if mv := w.Status; mv != nil && claim.Status != mv.To && (len(mv.From) == 0 || slices.Contains(mv.From, claim.Status)) {
			if err := r.moveClaim(ctx, w.Project, w.TicketID, claim.Status, *mv); err != nil {
				if !mv.BestEffort {
					return err
				}
				res.StatusErr = err
			} else {
				res.Moved = true
			}
		}
		if w.Session != "" && w.Member != "" {
			adopted, err := r.AdoptClaim(ctx, w.Project, w.TicketID, w.Member, w.Session)
			if err != nil {
				return err
			}
			res.Adopted = adopted
		}
	}
	for _, label := range w.Labels {
		if err := r.AddClaimLabel(ctx, w.Project, w.TicketID, label); err != nil {
			return fmt.Errorf("adding the %s label: %w", label, err)
		}
	}
	for _, e := range w.Events {
		if err := r.AppendEvent(ctx, e); err != nil {
			return fmt.Errorf("recording the event: %w", err)
		}
	}
	if w.Identity != nil && w.Identity.Remote != "" {
		id := *w.Identity
		id.ID = w.Project
		created, err := r.writeProjectIdentity(ctx, id, false)
		if err != nil {
			return err
		}
		res.IdentityRecorded = created
	}
	return nil
}

// moveClaim moves the claim (now at status from) as mv says, within the
// transaction of ctx.
func (r *Repo) moveClaim(ctx context.Context, project, ticketID, from string, mv ClaimStatusMove) error {
	board := mv.Board
	if board == nil || IsValidBoardTransition(from, mv.To) {
		return r.UpdateClaimStatus(ctx, project, ticketID, mv.To)
	}
	if !slices.Contains(board.TerminalStatuses(), mv.To) || from != board.DefaultWorkStatus() {
		return r.UpdateClaimStatus(ctx, project, ticketID, mv.To) // refused, with the reason
	}
	for _, review := range board.ReviewStatuses() {
		if !slices.Contains(board.AllStatuses(), review) {
			continue
		}
		if IsValidBoardTransition(from, review) && IsValidBoardTransition(review, mv.To) {
			if err := r.UpdateClaimStatus(ctx, project, ticketID, review); err != nil {
				return err
			}
			return r.UpdateClaimStatus(ctx, project, ticketID, mv.To)
		}
	}
	// No review column on this board: the move is applied directly.
	return r.UpdateClaimStatusFromTracker(ctx, project, ticketID, mv.To)
}
