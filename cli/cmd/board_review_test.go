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

// `oh board`: an in-progress ticket labelled ready-for-review is in REVIEW.
func TestBoardTicketsReviewColumn(t *testing.T) {
	got := map[string]string{}
	for _, b := range boardTickets(reviewFixture()) {
		got[b.ID] = b.Status
	}
	want := map[string]string{"a-1": "todo", "a-2": "in_progress", "a-3": "review", "a-4": "review", "a-5": "done"}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s: column %q, want %q", id, got[id], w)
		}
	}
	vt := boardViewTickets(boardTickets(reviewFixture()))
	if vt[2].Status != "review" || len(vt[2].Labels) != 1 {
		t.Errorf("view ticket = %+v", vt[2])
	}
}

// The Board TUI follows the same rule and leaves epics out.
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
