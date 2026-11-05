package teamstate

// RoleCounts is how many claims stand in the columns of each role of a board.
type RoleCounts struct {
	// Active: columns of work (role active, or none).
	Active int
	// Review: columns that wait for a review.
	Review int
	// Blocked: columns of impediment.
	Blocked int
}

// CountByRole counts the claims by the role of the column their status is in:
// the roles of the board of the team, not the names of the statuses of the
// default board (a board of its own has its own). The default board is used
// when the team did not configure columns. The claims in the initial and the
// terminal columns, and in a status the board does not have, are in none; the
// legacy status planned is the initial column.
func CountByRole(claims []Claim, board BoardConfig) RoleCounts {
	if !board.HasCustomColumns() {
		board = DefaultBoardConfig()
	}
	roles := make(map[string]string, len(board.Columns))
	for _, c := range board.Columns {
		role := c.Role
		if role == "" {
			role = ColumnRoleActive
		}
		roles[c.ID] = role
	}
	// A board that declares no review column: the review status stands in.
	if _, declared := roles[ClaimStatusReview]; declared && !hasRole(roles, ColumnRoleReview) && roles[ClaimStatusReview] == ColumnRoleActive {
		roles[ClaimStatusReview] = ColumnRoleReview
	}
	var n RoleCounts
	for i := range claims {
		switch roles[claims[i].Status] {
		case ColumnRoleActive:
			n.Active++
		case ColumnRoleReview:
			n.Review++
		case ColumnRoleBlocked:
			n.Blocked++
		}
	}
	return n
}

func hasRole(roles map[string]string, role string) bool {
	for _, r := range roles {
		if r == role {
			return true
		}
	}
	return false
}
