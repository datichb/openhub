package cmd

import (
	"testing"

	"github.com/datichb/openhub/cli/internal/beads"
)

func reviewFixture() []beads.Ticket {
	return []beads.Ticket{
		{ID: "a-1", Title: "open", Status: "open", Type: "task"},
		{ID: "a-2", Title: "wip", Status: "in_progress", Type: "task", Labels: []string{"ai-delegated"}},
		{ID: "a-3", Title: "wip ready", Status: "in_progress", Type: "task", Labels: []string{beads.LabelReadyForReview}},
		{ID: "a-4", Title: "epic", Status: "in_progress", Type: "epic", Labels: []string{beads.LabelReadyForReview}},
		{ID: "a-5", Title: "closed", Status: "closed", Type: "task", Labels: []string{beads.LabelReadyForReview}},
	}
}

// The board follows the rule of the columns (an in-progress ticket labelled
// ready-for-review is in REVIEW) and leaves epics out.
func TestTUIBoardTicketsReviewColumn(t *testing.T) {
	out := tuiBoardTickets(reviewFixture())
	if len(out) != 4 {
		t.Fatalf("got %d tickets, want 4 (epic left out)", len(out))
	}
	got := map[string]string{}
	for _, b := range out {
		got[b.ID] = b.Status
	}
	if got["a-3"] != "review" || got["a-2"] != "in_progress" || got["a-5"] != "done" {
		t.Errorf("columns = %v", got)
	}
}
