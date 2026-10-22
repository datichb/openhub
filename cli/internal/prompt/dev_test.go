package prompt

import (
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/beads"
)

// The prompt names the label the boards read as the REVIEW column.
func TestBuildDevPromptUsesReviewLabel(t *testing.T) {
	for _, status := range []string{"open", "in_progress"} {
		p := BuildDevPrompt([]beads.Ticket{{ID: "a-1", Title: "t", Status: status}})
		want := "bd update <ID> --add-label " + beads.LabelReadyForReview + " — prêt pour la review\n"
		if !strings.Contains(p, want) {
			t.Errorf("status %s: prompt lacks %q", status, want)
		}
	}
}
