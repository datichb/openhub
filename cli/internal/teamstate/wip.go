package teamstate

import (
	"slices"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// The work in progress of a member is bounded by max_in_progress_per_member
// (TrackerConfig.InProgressLimit). A claim counts as in progress when its
// owner works on it: its status is one of the "active" columns of the board
// (in_progress on the default board), and it does not wait for a review. A
// claim in review (InReview: status review, or the review or ready-for-review
// label) waits for a person, not for the member's hands: it does not count.
// Neither do planned, blocked, finished and validation claims (the work passed
// review and is being validated): on the default board the count is exactly
// the claims in in_progress, whether the team left its columns unset or saved
// the default ones (where review and validation are "active" columns too).
//
// The limit is checked where a member takes or starts a ticket (oh team
// claim, the take key of the board, a session launched on a ticket, a card
// moved to in progress). The tracker sync never refuses anything.

// CountsInProgress reports whether the claim counts towards the in-progress
// limit of its owner on the given board. A claim without an owner (pool)
// counts for nobody.
func (c *Claim) CountsInProgress(board BoardConfig) bool {
	return c.ClaimedBy != "" && !c.InReview() && StatusCountsInProgress(board, c.Status)
}

// StatusCountsInProgress reports whether a claim in the status is work in
// progress on the board: an active column, the review and validation ones
// apart.
func StatusCountsInProgress(board BoardConfig, status string) bool {
	return status != ClaimStatusReview && status != ClaimStatusValidation && slices.Contains(board.ActiveStatuses(), status)
}

// InProgressOf returns the claims of member that count as in progress.
func InProgressOf(claims []Claim, member string, board BoardConfig) []ClaimRef {
	var out []ClaimRef
	for i := range claims {
		if c := &claims[i]; c.ClaimedBy == member && c.CountsInProgress(board) {
			out = append(out, ClaimRef{Project: c.Project, TicketID: c.TicketID})
		}
	}
	return out
}

// InProgressLimitError is the refusal to give a member one more ticket in
// progress: they already work on Count, which is the Limit.
type InProgressLimitError struct {
	Member string
	// Count is how many tickets the member has in progress, Limit the most
	// they may have (max_in_progress_per_member).
	Count, Limit int
	// Adding is how many more the refused operation would have put in progress.
	Adding int
}

func (e *InProgressLimitError) Error() string {
	if e.Adding > 1 {
		return i18n.Tf("teamstate.wip.limit_many", e.Member, e.Count, e.Limit, e.Adding)
	}
	return i18n.Tf("teamstate.wip.limit", e.Member, e.Count, e.Limit)
}

// CheckInProgressRoom refuses (InProgressLimitError) to put the claims named
// by becoming in progress for member when that would go past limit. The
// claims member already has in progress do not add to the count a second
// time; limit 0 is no limit. claims are all the claims of the team.
func CheckInProgressRoom(claims []Claim, member string, board BoardConfig, limit int, becoming []ClaimRef) error {
	if limit <= 0 {
		return nil
	}
	current := InProgressOf(claims, member, board)
	adding := 0
	var seen []ClaimRef
	for _, ref := range becoming {
		if slices.Contains(current, ref) || slices.Contains(seen, ref) {
			continue
		}
		seen = append(seen, ref)
		adding++
	}
	if adding == 0 || len(current)+adding <= limit {
		return nil
	}
	return &InProgressLimitError{Member: member, Count: len(current), Limit: limit, Adding: adding}
}
