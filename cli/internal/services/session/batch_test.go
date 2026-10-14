package session

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/domain"
)

// CP-R6: several checkpoints validated in one go, each one listed and
// refusable on its own.
func TestBatch(t *testing.T) {
	cp := func(id, session, checkpoint string) InboxItem {
		return InboxItem{Decision: domain.Decision{ID: id, SessionID: session, Kind: domain.DecisionCheckpoint,
			Payload: domain.DecisionPayload{Data: map[string]any{"checkpoint": checkpoint}}}}
	}
	items := []InboxItem{
		cp("d1", "ses_c1", "cp-2"), cp("d2", "ses_c2", "cp-2"), cp("d3", "ses_p", "cp-merge"), cp("d4", "ses_x", "cp-2"),
		{Decision: domain.Decision{ID: "q1", SessionID: "ses_c1", Kind: domain.DecisionQuestion}},
	}
	got := SelectBatch(items, BatchSelection{Sessions: []string{"ses_c1", "ses_c2", "ses_p"}, Except: []string{"d2"}, Reject: map[string]string{"d3": "pas encore"}})
	if assert.Len(t, got, 3, "questions and other sessions left out") {
		assert.Equal(t, BatchApprove, got[0].Action)
		assert.Equal(t, BatchKeep, got[1].Action)
		assert.Equal(t, BatchReject, got[2].Action)
		assert.Equal(t, "pas encore", got[2].Message)
	}
	assert.Len(t, SelectBatch(items, BatchSelection{Checkpoints: []string{"cp-merge"}}), 1)

	var replies []Reply
	done := ApplyBatch(context.Background(), got, func(_ context.Context, r Reply) error {
		replies = append(replies, r)
		if r.DecisionID == "d1" {
			return errors.New("verification failed")
		}
		return nil
	})
	assert.Equal(t, []Reply{{DecisionID: "d1", Decision: "once"}, {DecisionID: "d3", Decision: "reject", Message: "pas encore"}}, replies, "the kept one is not answered")
	assert.Equal(t, "verification failed", done[0].Error, "a failure stays on its item")
	assert.Empty(t, done[2].Error)
}
