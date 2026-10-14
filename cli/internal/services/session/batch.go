package session

import (
	"context"
	"slices"

	"github.com/datichb/openhub/cli/internal/domain"
)

// Validation by batch (R6, CP-R6): the checkpoints waiting in several
// sessions (the children of a fan-out, remote sessions, the merge
// checkpoints of several fan-outs) are validated in one go, each one
// still listed with its session, its checkpoint and its summary, and each
// one refusable or left waiting on its own. Each approval goes through the
// same resolver as one at a time: a failed verification refuses that one
// only.

// BatchSelection chooses the checkpoint decisions of a batch.
type BatchSelection struct {
	// Sessions restricts the batch to these sessions (empty = all).
	Sessions []string
	// Checkpoints restricts it to these checkpoints (empty = all).
	Checkpoints []string
	// Except are decisions left waiting.
	Except []string
	// Reject are decisions refused, with their message ("" = none).
	Reject map[string]string
}

// BatchAction is what a batch does with a decision.
type BatchAction string

const (
	BatchApprove BatchAction = "approve"
	BatchReject  BatchAction = "reject"
	BatchKeep    BatchAction = "keep"
)

// BatchItem is a decision of a batch.
type BatchItem struct {
	Decision   domain.Decision
	Session    *domain.Session
	Checkpoint string
	Action     BatchAction
	Message    string
	// Error is the reason the decision could not be answered.
	Error string
}

// SelectBatch returns the checkpoint decisions of items the selection
// covers, each with its action, in the order of the inbox.
func SelectBatch(items []InboxItem, sel BatchSelection) []BatchItem {
	var out []BatchItem
	for _, it := range items {
		d := it.Decision
		if d.Kind != domain.DecisionCheckpoint {
			continue
		}
		cp, _ := d.Payload.Data["checkpoint"].(string)
		if len(sel.Sessions) > 0 && !slices.Contains(sel.Sessions, d.SessionID) {
			continue
		}
		if len(sel.Checkpoints) > 0 && !slices.Contains(sel.Checkpoints, cp) {
			continue
		}
		b := BatchItem{Decision: d, Session: it.Session, Checkpoint: cp, Action: BatchApprove}
		if msg, ok := sel.Reject[d.ID]; ok {
			b.Action, b.Message = BatchReject, msg
		} else if slices.Contains(sel.Except, d.ID) {
			b.Action = BatchKeep
		}
		out = append(out, b)
	}
	return out
}

// ApplyBatch answers the decisions of a batch one by one (decide = the
// SessionService Decide), recording the error of each one that fails.
func ApplyBatch(ctx context.Context, items []BatchItem, decide func(context.Context, Reply) error) []BatchItem {
	for i := range items {
		it := &items[i]
		var err error
		switch it.Action {
		case BatchApprove:
			err = decide(ctx, Reply{DecisionID: it.Decision.ID, Decision: "once", Message: it.Message})
		case BatchReject:
			err = decide(ctx, Reply{DecisionID: it.Decision.ID, Decision: "reject", Message: it.Message})
		}
		if err != nil {
			it.Error = err.Error()
		}
	}
	return items
}
