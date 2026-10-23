package teamstate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecideClaimEnd(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		facts  ClaimEndFacts
		action ClaimEndAction
		reason ClaimEndReason
	}{
		// closed ticket
		{"closed ticket", ClaimEndFacts{Closed: true, ClaimStatus: ClaimStatusInProgress}, ClaimEndComplete, ClaimEndTicketClosed},
		{"closed ticket wins over a waiting checkpoint", ClaimEndFacts{Closed: true, WaitingCheckpoint: "cp-2"}, ClaimEndComplete, ClaimEndTicketClosed},
		{"closed ticket wins over the review label", ClaimEndFacts{Closed: true, Labels: []string{"ready-for-review"}}, ClaimEndComplete, ClaimEndTicketClosed},
		{"closed ticket wins over a deferred checkpoint", ClaimEndFacts{Closed: true, DeferredCheckpoint: "cp-2", Work: ClaimWorkPresent}, ClaimEndComplete, ClaimEndTicketClosed},
		{"closed ticket, claim already terminal", ClaimEndFacts{Closed: true, ClaimStatus: ClaimStatusDone, ClaimTerminal: true}, ClaimEndKeep, ClaimEndAlreadyTerminal},
		{"open ticket, claim already terminal: not released", ClaimEndFacts{ClaimStatus: ClaimStatusDone, ClaimTerminal: true}, ClaimEndKeep, ClaimEndAlreadyTerminal},

		// review
		{"label ready-for-review", ClaimEndFacts{Labels: []string{"ai-delegated", "ready-for-review"}}, ClaimEndKeep, ClaimEndInReview},
		{"label review", ClaimEndFacts{Labels: []string{"review"}}, ClaimEndKeep, ClaimEndInReview},
		{"label in another case and with blanks", ClaimEndFacts{Labels: []string{" Ready-For-Review "}}, ClaimEndKeep, ClaimEndInReview},
		{"claim status review", ClaimEndFacts{ClaimStatus: ClaimStatusReview}, ClaimEndKeep, ClaimEndInReview},
		{"review wins over a waiting checkpoint", ClaimEndFacts{Labels: []string{"review"}, WaitingCheckpoint: "cp-2"}, ClaimEndKeep, ClaimEndInReview},
		{"review wins over a deferred checkpoint", ClaimEndFacts{Labels: []string{"review"}, DeferredCheckpoint: "cp-2", Work: ClaimWorkPresent}, ClaimEndKeep, ClaimEndInReview},
		{"a label that only looks like review", ClaimEndFacts{Labels: []string{"reviewed", "needs-review"}}, ClaimEndRelease, ClaimEndNothingLeft},

		// checkpoint waiting
		{"checkpoint waiting", ClaimEndFacts{WaitingCheckpoint: "cp-1"}, ClaimEndKeepMarked, ClaimEndCheckpointWaiting},
		{"checkpoint waiting, no work", ClaimEndFacts{WaitingCheckpoint: "cp-1", Work: ClaimWorkNone}, ClaimEndKeepMarked, ClaimEndCheckpointWaiting},
		{"checkpoint waiting wins over a deferred one", ClaimEndFacts{WaitingCheckpoint: "cp-1", DeferredCheckpoint: "cp-2", Work: ClaimWorkPresent}, ClaimEndKeepMarked, ClaimEndCheckpointWaiting},

		// commit checkpoint deferred
		{"cp-2 deferred with work", ClaimEndFacts{DeferredCheckpoint: "cp-2", Work: ClaimWorkPresent}, ClaimEndKeepMarked, ClaimEndCheckpointDeferred},
		{"cp-2 deferred, work unknown", ClaimEndFacts{DeferredCheckpoint: "cp-2", Work: ClaimWorkUnknown}, ClaimEndKeepMarked, ClaimEndCheckpointDeferred},
		{"cp-2 deferred, zero value of Work is unknown", ClaimEndFacts{DeferredCheckpoint: "cp-2"}, ClaimEndKeepMarked, ClaimEndCheckpointDeferred},
		{"cp-2 deferred, nothing done", ClaimEndFacts{DeferredCheckpoint: "cp-2", Work: ClaimWorkNone}, ClaimEndRelease, ClaimEndNothingLeft},
		{"work but no deferred checkpoint", ClaimEndFacts{Work: ClaimWorkPresent}, ClaimEndRelease, ClaimEndNothingLeft},

		// unreadable checkpoints
		{"checkpoints unreadable", ClaimEndFacts{CheckpointsUnreadable: true}, ClaimEndKeepMarked, ClaimEndCheckpointUnknown},
		{"checkpoints unreadable, but closed", ClaimEndFacts{CheckpointsUnreadable: true, Closed: true}, ClaimEndComplete, ClaimEndTicketClosed},
		{"checkpoints unreadable, but in review", ClaimEndFacts{CheckpointsUnreadable: true, Labels: []string{"review"}}, ClaimEndKeep, ClaimEndInReview},

		// released
		{"open ticket, nothing pending", ClaimEndFacts{ClaimStatus: ClaimStatusInProgress}, ClaimEndRelease, ClaimEndNothingLeft},
		{"no facts at all", ClaimEndFacts{}, ClaimEndRelease, ClaimEndNothingLeft},
		{"validation claim is released (not handled)", ClaimEndFacts{ClaimStatus: ClaimStatusValidation}, ClaimEndRelease, ClaimEndNothingLeft},
		{"blocked claim is released (not handled)", ClaimEndFacts{ClaimStatus: ClaimStatusBlocked}, ClaimEndRelease, ClaimEndNothingLeft},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := DecideClaimEnd(tt.facts)
			assert.Equal(t, ClaimEndDecision{Action: tt.action, Reason: tt.reason}, got)
		})
	}
}

// The facts are not modified and the decision does not depend on anything
// else: the same facts give the same decision.
func TestDecideClaimEndIsPure(t *testing.T) {
	t.Parallel()
	labels := []string{"a", "review"}
	f := ClaimEndFacts{Labels: labels, WaitingCheckpoint: "cp-1"}
	first := DecideClaimEnd(f)
	for range 3 {
		assert.Equal(t, first, DecideClaimEnd(f))
	}
	assert.Equal(t, []string{"a", "review"}, labels)
}
