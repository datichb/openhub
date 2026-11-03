package views

import (
	"context"

	"github.com/datichb/openhub/cli/internal/teamstate"
)

// BoardTicket represents a kanban ticket.
type BoardTicket struct {
	ID          string
	Title       string
	Status      string // "planned", "in_progress", "review", "done", "blocked"
	Priority    string
	Type        string
	ExternalRef string // external tracker reference (e.g. "gitlab-693") — ADR-032
	Labels      []string
}

// TeamTicket extends BoardTicket with team assignment and label info.
type TeamTicket struct {
	ID       string
	Title    string
	Status   string
	Priority string
	Assignee string
	// AssigneeID is the ID of the member who holds the claim; empty for a
	// ticket of the pool (no owner, to take with the take key).
	AssigneeID  string
	Project     string // directory-based ID (used for filtering)
	ProjectName string // human-friendly display name (ADR-032)
	Description string
	// Labels are displayed as compact tags on the board item.
	Labels []string
	// MRURL is the merge request URL (from claim). Empty if no MR published.
	MRURL string
}

// ProjectNameResolver maps a project directory ID to a human-friendly display name.
// Returns the input as-is if no mapping is found.
type ProjectNameResolver func(projectDirID string) string

// TeamTicketSource is what the board reads of the team-state: the members
// (their display names) and the claims (the tickets).
type TeamTicketSource interface {
	teamstate.MemberReader
	teamstate.ClaimReader
}

// FetchTeamTickets builds the ticket list from team-state claims for the board views.
// Members with no active claims are omitted — the board shows tickets, not people.
// resolver is optional — if nil, project directory names are used as-is.
// Uses DefaultColumns() for status mapping. For custom columns, use FetchTeamTicketsWithColumns.
func FetchTeamTickets(ctx context.Context, repo TeamTicketSource, resolver ProjectNameResolver) []TeamTicket {
	return FetchTeamTicketsWithColumns(ctx, repo, resolver, nil)
}

// FetchTeamTicketsWithColumns is like FetchTeamTickets but uses the provided
// column definitions for status mapping. If columns is nil, falls back to
// the legacy MapClaimStatus (DefaultColumns-compatible).
func FetchTeamTicketsWithColumns(ctx context.Context, repo TeamTicketSource, resolver ProjectNameResolver, columns []BoardColumnDef) []TeamTicket {
	members, err := repo.ListMembers(ctx)
	if err != nil {
		return nil
	}

	claims, err := repo.ListClaims(ctx, "")
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
		// Resolve project display name
		projName := c.Project
		if resolver != nil {
			projName = resolver(c.Project)
		}
		// Map claim status to board column key.
		var status string
		if len(columns) > 0 {
			status = MapClaimStatusWithConfig(c.Status, columns)
		} else {
			status = MapClaimStatus(c.Status)
		}
		tickets = append(tickets, TeamTicket{
			ID:          c.TicketID,
			Title:       title,
			Project:     c.Project,
			ProjectName: projName,
			Status:      status,
			Assignee:    name,
			AssigneeID:  c.ClaimedBy,
			Labels:      c.Labels,
			Description: c.Description,
			MRURL:       c.MRURL,
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

// MapClaimStatusWithConfig converts a claim status to a board column key
// using the provided column definitions. With custom columns the status IS
// the column ID, so this is mostly an identity function plus legacy compat.
//
// Resolution order:
//  1. Direct match: status == column.Status → return status
//  2. Legacy compat: "planned" → first column (the initial/todo column)
//  3. Fallback: first column with role "active", or the first column overall
func MapClaimStatusWithConfig(status string, columns []BoardColumnDef) string {
	// Direct match against column keys.
	for _, col := range columns {
		if col.Status == status {
			return status
		}
	}
	// Legacy compat: "planned" → first column (initial/todo).
	if status == teamstate.ClaimStatusPlanned && len(columns) > 0 {
		return columns[0].Status
	}
	// Fallback: first column.
	if len(columns) > 0 {
		return columns[0].Status
	}
	return "in_progress"
}
