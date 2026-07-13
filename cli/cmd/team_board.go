package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/common"
	"github.com/datichb/openhub/cli/internal/tui/views/teamboard"
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

	tickets := fetchTeamTickets(repo)

	if len(tickets) == 0 {
		fmt.Fprintf(a.IO.Out, "%s Aucun membre dans l'équipe. Lance %s\n",
			common.Subtitle.Render(common.IconInfo),
			common.Bold.Render("oh team init"))
		return nil
	}

	cfg := teamboard.Config{
		Title:   "oh team board — Équipe",
		Tickets: tickets,
		RefreshFunc: func() []teamboard.TeamTicket {
			// Pull latest and rebuild
			r, err := ensureTeamRepo(ctx, a)
			if err != nil {
				return tickets // return stale on error
			}
			return fetchTeamTickets(r)
		},
	}

	return teamboard.Run(cfg)
}

// fetchTeamTickets builds the list of team tickets from claims + members.
func fetchTeamTickets(repo *teamstate.Repo) []teamboard.TeamTicket {
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

	var tickets []teamboard.TeamTicket

	for _, m := range members {
		memberClaims := claimsByMember[m.ID]

		if len(memberClaims) == 0 {
			// Idle member
			tickets = append(tickets, teamboard.TeamTicket{
				Member: m.DisplayName,
				Status: "idle",
				Since:  time.Now(),
			})
			continue
		}

		for _, c := range memberClaims {
			since := c.ClaimedAt
			if !c.LastActivity.IsZero() {
				since = c.LastActivity
			}

			tt := teamboard.TeamTicket{
				Member:   m.DisplayName,
				TicketID: c.TicketID,
				Project:  c.Project,
				Status:   mapClaimStatus(c.Status),
				Since:    since,
			}

			// Try to get sub-beads (optional, silent failure)
			tt.SubBeads = fetchSubBeads(c.TicketID)

			tickets = append(tickets, tt)
		}
	}

	return tickets
}

// mapClaimStatus maps claim statuses to board column statuses.
func mapClaimStatus(status string) string {
	switch status {
	case "in_progress":
		return "in_progress"
	case "review":
		return "review"
	case "blocked":
		return "blocked"
	default:
		return "in_progress" // default unknown statuses to in_progress
	}
}

// fetchSubBeads attempts to get sub-tickets from the beads system.
// Returns nil silently if bd is not installed or fails.
func fetchSubBeads(parentID string) []teamboard.SubBead {
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

	beads := make([]teamboard.SubBead, 0, len(raw))
	for _, r := range raw {
		beads = append(beads, teamboard.SubBead{
			ID:     r.ID,
			Title:  r.Title,
			Status: r.Status,
		})
	}
	return beads
}
