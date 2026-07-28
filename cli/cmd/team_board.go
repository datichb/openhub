package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

var teamBoardCmd = &cobra.Command{
	Use:   "board",
	Short: "Kanban board interactif de l'équipe",
	Long: `Lance un tableau kanban plein écran montrant qui travaille sur quoi.
Colonnes par status (IDLE / IN PROGRESS / REVIEW / BLOCKED).
Navigation: h/l colonnes, j/k items, d detail, r refresh, q quit.`,
	RunE: runTeamBoard,
}

func init() {
	teamCmd.AddCommand(teamBoardCmd)
	teamBoardCmd.Flags().Bool("watch", false, "Rafraîchissement automatique (touche r pour rafraîchir manuellement)")
}

func runTeamBoard(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	tickets := fetchTeamTicketsV2(repo)

	if len(tickets) == 0 {
		fmt.Fprintf(a.IO.Out, "%s Aucun membre dans l'équipe. Lance %s\n",
			theme.Subtitle.Render(theme.IconInfo),
			theme.Bold.Render("oh team init"))
		return nil
	}

	cfg := views.TeamBoardConfig{
		Layout: layout.Config{
			ProjectName: a.Config.Name,
			Command:     "team board",
			StatusHints: "←→ columns · ↑↓ scroll · r refresh · q quit",
		},
		Tickets: tickets,
		RefreshFunc: func() []views.TeamTicket {
			r, err := ensureTeamRepo(ctx, a)
			if err != nil {
				return tickets
			}
			return fetchTeamTicketsV2(r)
		},
	}

	return views.RunTeamBoard(cfg)
}

// fetchTeamTicketsV2 builds the list of team tickets from claims.
// Members with no active claims are omitted — the board shows tickets, not people.
func fetchTeamTicketsV2(repo *teamstate.Repo) []views.TeamTicket {
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

	var tickets []views.TeamTicket
	for _, c := range claims {
		name := displayName[c.ClaimedBy]
		if name == "" {
			name = c.ClaimedBy // fallback to ID if member no longer in registry
		}
		tickets = append(tickets, views.TeamTicket{
			ID:       c.TicketID,
			Title:    fmt.Sprintf("%s/%s", c.Project, c.TicketID),
			Status:   mapClaimStatus(c.Status),
			Assignee: name,
			Labels:   c.Labels,
		})
	}

	return tickets
}

// mapClaimStatus maps a claim status string to the board column key.
// The mapping is 1-to-1 with the column definitions in views.DefaultColumns().
func mapClaimStatus(status string) string {
	switch status {
	case teamstate.ClaimStatusPlanned:
		return "todo"
	case teamstate.ClaimStatusInProgress:
		return "in_progress"
	case teamstate.ClaimStatusReview:
		return "review"
	case teamstate.ClaimStatusBlocked:
		return "blocked"
	case teamstate.ClaimStatusDone:
		return "done"
	default:
		return "in_progress"
	}
}

// fetchSubBeadsJSON attempts to get sub-tickets from the beads system.
// Returns nil silently if bd is not installed or fails.
func fetchSubBeadsJSON(parentID string) []struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
} {
	if _, err := exec.LookPath("bd"); err != nil {
		return nil
	}

	out, err := exec.Command("bd", "list", "--parent", parentID, "--json").Output()
	if err != nil || len(out) == 0 {
		return nil
	}

	var raw []struct {
		ID     string `json:"id"`
		Title  string `json:"title"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil
	}
	return raw
}
