package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/tui/common"
	"github.com/datichb/openhub/cli/internal/tui/views/board"
)

var boardCmd = &cobra.Command{
	Use:   "board",
	Short: "Affiche le tableau kanban des tickets",
	Long:  "Lance un kanban board interactif en plein écran avec rafraîchissement automatique.",
	RunE:  runBoard,
}

func init() {
	rootCmd.AddCommand(boardCmd)
	boardCmd.Flags().Bool("watch", false, "Rafraîchissement automatique toutes les 5s")
}

func runBoard(cmd *cobra.Command, args []string) error {
	tickets := fetchTickets()

	if len(tickets) == 0 {
		a := MustApp()
		fmt.Fprintln(a.IO.Out, common.Subtitle.Render("Aucun ticket trouvé. Configurez un tracker avec `oh project add --tracker`."))
		return nil
	}

	watch, _ := cmd.Flags().GetBool("watch")

	cfg := board.Config{
		Title:   "oh board — Kanban",
		Tickets: tickets,
	}

	if watch {
		cfg.RefreshFunc = fetchTickets
	}

	return board.Run(cfg)
}

// fetchTickets attempts to get tickets from the beads system (bd list).
func fetchTickets() []board.Ticket {
	out, err := exec.Command("bd", "list", "--json").Output()
	if err != nil {
		return nil
	}

	var raw []struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Status   string `json:"status"`
		Priority string `json:"priority"`
		Type     string `json:"type"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil
	}

	tickets := make([]board.Ticket, len(raw))
	for i, r := range raw {
		tickets[i] = board.Ticket{
			ID:       r.ID,
			Title:    r.Title,
			Status:   normalizeStatus(r.Status),
			Priority: r.Priority,
			Type:     r.Type,
		}
	}
	return tickets
}

func normalizeStatus(s string) string {
	s = strings.ToLower(s)
	switch s {
	case "todo", "to_do", "backlog":
		return "todo"
	case "in_progress", "in-progress", "doing", "wip":
		return "in_progress"
	case "done", "completed", "closed":
		return "done"
	case "blocked", "stuck":
		return "blocked"
	default:
		return "todo"
	}
}
