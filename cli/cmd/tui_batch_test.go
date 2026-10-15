package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/domain"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// The choices of the TUI batch become the batch of the CLI: only the
// decisions chosen and still waiting, each with its action.
func TestChoicesBatch(t *testing.T) {
	cp := func(id string) sessionsvc.InboxItem {
		return sessionsvc.InboxItem{Decision: domain.Decision{ID: id, SessionID: "ses_" + id, Kind: domain.DecisionCheckpoint,
			Payload: domain.DecisionPayload{Data: map[string]any{"checkpoint": "cp-2"}}}}
	}
	items := []sessionsvc.InboxItem{cp("d1"), cp("d2"), cp("d3"), cp("other")}
	got := choicesBatch(items, []views.BatchChoice{
		{ID: "d1", Action: views.BatchApprove},
		{ID: "d2", Action: views.BatchReject, Message: "corriger"},
		{ID: "d3", Action: "nope"},
		{ID: "gone", Action: views.BatchApprove},
	})
	if assert.Len(t, got, 3, "not chosen and gone left out") {
		assert.Equal(t, sessionsvc.BatchApprove, got[0].Action)
		assert.Equal(t, sessionsvc.BatchReject, got[1].Action)
		assert.Equal(t, "corriger", got[1].Message)
		assert.Equal(t, sessionsvc.BatchKeep, got[2].Action, "an unknown action answers nothing")
	}
}

func TestApprovingChoices(t *testing.T) {
	for _, c := range []string{"", "once", "approve", "override", "always"} {
		assert.True(t, approving(c), c)
	}
	for _, c := range []string{"fix", "other", "reject"} {
		assert.False(t, approving(c), c)
	}
}
