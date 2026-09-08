package views

import (
	"github.com/datichb/openhub/cli/internal/teamstate"
)

// FetchTeamTickets builds the ticket list from team-state claims for the board views.
// Members with no active claims are omitted — the board shows tickets, not people.
func FetchTeamTickets(repo teamstate.TeamStateReader) []TeamTicket {
	members, err := repo.ListMembers()
	if err != nil {
		return nil
	}

	claims, err := repo.ListClaims("")
	if err != nil {
		return nil
	}

	// Build a display-name lookup keyed by member ID.
	displayName := make(map[string]string, len(members))
	for _, m := range members {
		displayName[m.ID] = m.DisplayName
	}

	var tickets []TeamTicket
	for _, c := range claims {
		name := displayName[c.ClaimedBy]
		if name == "" {
			name = c.ClaimedBy // fallback to ID if member no longer in registry
		}
		// Use the real title from the tracker if available, fall back to ticket ID.
		title := c.Title
		if title == "" {
			title = c.TicketID
		}
		tickets = append(tickets, TeamTicket{
			ID:          c.TicketID,
			Title:       title,
			Project:     c.Project,
			Status:      MapClaimStatus(c.Status),
			Assignee:    name,
			Labels:      c.Labels,
			Description: c.Description,
		})
	}

	return tickets
}

// MapClaimStatus converts a claim status string to a board column key.
// The mapping is 1-to-1 with the column definitions in DefaultColumns().
func MapClaimStatus(status string) string {
	switch status {
	case teamstate.ClaimStatusPlanned:
		return "todo"
	case teamstate.ClaimStatusInProgress:
		return "in_progress"
	case teamstate.ClaimStatusReview:
		return "review"
	case teamstate.ClaimStatusValidation:
		return "validation"
	case teamstate.ClaimStatusBlocked:
		return "blocked"
	case teamstate.ClaimStatusDone:
		return "done"
	default:
		return "in_progress"
	}
}
