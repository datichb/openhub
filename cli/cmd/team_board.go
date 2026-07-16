package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/common"
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
			common.Subtitle.Render(common.IconInfo),
			common.Bold.Render("oh team init"))
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

// fetchTeamTicketsV2 builds the list of team tickets from claims + members.
func fetchTeamTicketsV2(repo *teamstate.Repo) []views.TeamTicket {
	members, err := repo.ListMembers()
	if err != nil {
		return nil
	}

	claims, err := repo.ListClaims("")
	if err != nil {
		return nil
	}

	// Index claims by member
	claimsByMember := make(map[string][]teamstate.Claim)
	for _, c := range claims {
		claimsByMember[c.ClaimedBy] = append(claimsByMember[c.ClaimedBy], c)
	}

	var tickets []views.TeamTicket

	for _, m := range members {
		memberClaims := claimsByMember[m.ID]

		if len(memberClaims) == 0 {
			tickets = append(tickets, views.TeamTicket{
				ID:       m.ID,
				Title:    m.DisplayName + " (idle)",
				Status:   "todo",
				Assignee: m.DisplayName,
			})
			continue
		}

		for _, c := range memberClaims {
			tickets = append(tickets, views.TeamTicket{
				ID:       c.TicketID,
				Title:    fmt.Sprintf("%s/%s", c.Project, c.TicketID),
				Status:   mapClaimStatusV2(c.Status),
				Assignee: m.DisplayName,
			})
		}
	}

	return tickets
}

// mapClaimStatusV2 maps claim statuses to board column statuses.
func mapClaimStatusV2(status string) string {
	switch status {
	case "in_progress":
		return "in_progress"
	case "review":
		return "done" // map review → done column for now
	case "blocked":
		return "blocked"
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
