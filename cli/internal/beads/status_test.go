package beads

import "testing"

func TestBoardColumn(t *testing.T) {
	tests := []struct {
		name   string
		status string
		labels []string
		want   string
	}{
		{"open", "open", nil, "todo"},
		{"backlog", "backlog", nil, "todo"},
		{"in progress", "in_progress", nil, "in_progress"},
		{"in progress with other label", "in_progress", []string{"ai-delegated"}, "in_progress"},
		{"in progress ready for review", "in_progress", []string{"ai-delegated", LabelReadyForReview}, "review"},
		{"label is case-insensitive", "In_Progress", []string{"Ready-For-Review"}, "review"},
		{"doing ready for review", "doing", []string{LabelReadyForReview}, "review"},
		{"custom review status", "review", nil, "review"},
		{"custom in_review status", "in_review", nil, "review"},
		{"custom review status with label", "review", []string{LabelReadyForReview}, "review"},
		{"label on open ticket is ignored", "open", []string{LabelReadyForReview}, "todo"},
		{"label on blocked ticket is ignored", "blocked", []string{LabelReadyForReview}, "blocked"},
		{"validation", "qa", nil, "validation"},
		{"closed", "closed", nil, "done"},
		{"closed keeps done with label", "closed", []string{LabelReadyForReview}, "done"},
		{"deferred", "deferred", nil, "todo"},
		{"pinned", "pinned", nil, "todo"},
		{"hooked", "hooked", nil, "todo"},
		{"unknown", "whatever", nil, "todo"},
		{"empty", "", nil, "todo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BoardColumn(tt.status, tt.labels); got != tt.want {
				t.Fatalf("BoardColumn(%q, %v) = %q, want %q", tt.status, tt.labels, got, tt.want)
			}
		})
	}
}
