package teamstate

import (
	"slices"
	"strings"
)

// What a session does to a claim when it ends. DecideClaimEnd is a pure
// function of the facts the caller gathers (ticket, claim, checkpoints,
// work): no clock, no git, no file. The caller turns the decision into
// operations of the durable queue (ClaimOp).

// Labels read at the end of a session.
const (
	// LabelReview and LabelReadyForReview mark a ticket whose work waits for
	// a review (the second is the label agents put in Beads,
	// beads.LabelReadyForReview): its claim outlives the session.
	LabelReview         = "review"
	LabelReadyForReview = "ready-for-review"
	// LabelSessionEnded is put on a claim kept after its session ended with
	// work left to do (a checkpoint pending, work not committed).
	LabelSessionEnded = "session-ended"
)

// ClaimWork tells whether the session left work in its location (commits
// since it started, uncommitted or untracked changes). The zero value is
// ClaimWorkUnknown: a caller that did not look must not look like a caller
// that found nothing.
type ClaimWork int

const (
	ClaimWorkUnknown ClaimWork = iota
	ClaimWorkNone
	ClaimWorkPresent
)

// ClaimEndFacts are the facts about one claim at the end of its session.
type ClaimEndFacts struct {
	// Closed: the ticket is closed in the tracker.
	Closed bool
	// ClaimStatus is the status of the claim; ClaimTerminal says whether it
	// is a terminal status of the board.
	ClaimStatus   string
	ClaimTerminal bool
	// Labels are the labels of the ticket and of the claim.
	Labels []string
	// WaitingCheckpoint is the checkpoint of the session waiting for a
	// decision ("" = none).
	WaitingCheckpoint string
	// DeferredCheckpoint is the checkpoint that unlocks the commit and was
	// neither passed nor skipped ("" = none): the user put the commit off.
	DeferredCheckpoint string
	// Work is what the session left; only read with a DeferredCheckpoint.
	Work ClaimWork
	// CheckpointsUnreadable: the checkpoint state or the workflow of the
	// session could not be read.
	CheckpointsUnreadable bool
}

// ClaimEndAction is what happens to the claim.
type ClaimEndAction string

const (
	// ClaimEndRelease removes the claim.
	ClaimEndRelease ClaimEndAction = "release"
	// ClaimEndComplete moves the claim to the terminal status.
	ClaimEndComplete ClaimEndAction = "complete"
	// ClaimEndKeep leaves the claim as it is.
	ClaimEndKeep ClaimEndAction = "keep"
	// ClaimEndKeepMarked leaves the claim and adds LabelSessionEnded.
	ClaimEndKeepMarked ClaimEndAction = "keep_marked"
)

// ClaimEndReason is why: a code, to be translated by the caller.
type ClaimEndReason string

const (
	ClaimEndTicketClosed       ClaimEndReason = "ticket_closed"
	ClaimEndAlreadyTerminal    ClaimEndReason = "already_terminal"
	ClaimEndInReview           ClaimEndReason = "in_review"
	ClaimEndCheckpointWaiting  ClaimEndReason = "checkpoint_waiting"
	ClaimEndCheckpointDeferred ClaimEndReason = "checkpoint_deferred"
	ClaimEndCheckpointUnknown  ClaimEndReason = "checkpoint_unknown"
	ClaimEndNothingLeft        ClaimEndReason = "nothing_left"
)

// ClaimEndDecision is the outcome of DecideClaimEnd.
type ClaimEndDecision struct {
	Action ClaimEndAction
	Reason ClaimEndReason
}

// DecideClaimEnd decides what a session does to one of its claims, the first
// rule that applies wins:
//
//  1. the claim is already at a terminal status: kept (nothing to do, the
//     retention cleans it up);
//  2. the ticket is closed: the claim moves to the terminal status;
//  3. the ticket or the claim is in review (label review or
//     ready-for-review, or claim status review): kept;
//  4. a checkpoint waits for a decision: kept and marked session-ended;
//  5. the commit checkpoint was put off while the session left work (or
//     while that cannot be told): kept and marked session-ended;
//  6. the checkpoints cannot be read: kept and marked session-ended (a claim
//     released by mistake is lost, a claim kept by mistake expires with its
//     lease);
//  7. otherwise the claim is released.
func DecideClaimEnd(f ClaimEndFacts) ClaimEndDecision {
	switch {
	case f.ClaimTerminal:
		return ClaimEndDecision{ClaimEndKeep, ClaimEndAlreadyTerminal}
	case f.Closed:
		return ClaimEndDecision{ClaimEndComplete, ClaimEndTicketClosed}
	case f.ClaimStatus == ClaimStatusReview || hasReviewLabel(f.Labels):
		return ClaimEndDecision{ClaimEndKeep, ClaimEndInReview}
	case f.WaitingCheckpoint != "":
		return ClaimEndDecision{ClaimEndKeepMarked, ClaimEndCheckpointWaiting}
	case f.DeferredCheckpoint != "" && f.Work != ClaimWorkNone:
		return ClaimEndDecision{ClaimEndKeepMarked, ClaimEndCheckpointDeferred}
	case f.CheckpointsUnreadable:
		return ClaimEndDecision{ClaimEndKeepMarked, ClaimEndCheckpointUnknown}
	}
	return ClaimEndDecision{ClaimEndRelease, ClaimEndNothingLeft}
}

func hasReviewLabel(labels []string) bool {
	return slices.ContainsFunc(labels, func(l string) bool {
		l = strings.TrimSpace(l)
		return strings.EqualFold(l, LabelReview) || strings.EqualFold(l, LabelReadyForReview)
	})
}
