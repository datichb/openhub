package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

var teamBoardCmd = &cobra.Command{
	Use:   "board",
	Short: i18n.T("cmd.team.board.short"),
	Long:  i18n.T("cmd.team.board.long"),
	RunE:  runTeamBoard,
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

	tickets := views.FetchTeamTickets(repo)

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
			return views.FetchTeamTickets(r)
		},
	}

	return views.RunTeamBoard(cfg)
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
