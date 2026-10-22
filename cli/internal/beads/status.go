package beads

import "strings"

// LabelReadyForReview is the label an agent puts on an in-progress ticket
// when the work is ready for review (Beads has no review status).
const LabelReadyForReview = "ready-for-review"

// BoardColumn maps a Beads status and the labels of the ticket to the board
// column (todo, in_progress, review, validation, done, blocked). An
// in-progress ticket labelled LabelReadyForReview is in review. Unknown
// statuses (deferred, pinned, hooked…) are shown as todo.
func BoardColumn(status string, labels []string) string {
	switch strings.ToLower(status) {
	case "todo", "to_do", "backlog", "open", "planned":
		return "todo"
	case "in_progress", "in-progress", "doing", "wip":
		if hasLabelName(labels, LabelReadyForReview) {
			return "review"
		}
		return "in_progress"
	case "review", "in_review", "in-review":
		return "review"
	case "validation", "testing", "qa", "pre-release":
		return "validation"
	case "done", "completed", "closed", "cancelled":
		return "done"
	case "blocked", "stuck":
		return "blocked"
	default:
		return "todo"
	}
}

func hasLabelName(labels []string, want string) bool {
	for _, l := range labels {
		if strings.EqualFold(strings.TrimSpace(l), want) {
			return true
		}
	}
	return false
}
