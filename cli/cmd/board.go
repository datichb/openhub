package cmd

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
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
	a := MustApp()
	tickets := fetchTickets()

	if len(tickets) == 0 {
		// Distinguish "bd not installed" from "0 tickets"
		if _, err := exec.LookPath("bd"); err != nil {
			fmt.Fprintln(a.IO.Out, common.WarningStyle.Render(common.IconWarning)+" "+i18n.T("tui.board.bd_not_installed"))
		} else {
			fmt.Fprintln(a.IO.Out, common.Subtitle.Render(i18n.T("tui.board.no_tickets_hint")))
		}
		return nil
	}

	watch, _ := cmd.Flags().GetBool("watch")

	v2Tickets := make([]views.BoardTicket, len(tickets))
	for i, t := range tickets {
		v2Tickets[i] = views.BoardTicket{
			ID:       t.ID,
			Title:    t.Title,
			Status:   t.Status,
			Priority: t.Priority,
			Type:     t.Type,
		}
	}
	cfg := views.BoardConfig{
		Layout: layout.Config{
			ProjectName: a.Config.Name,
			Command:     "board",
			StatusHints: "←→ columns · ↑↓ scroll · r refresh · q quit",
		},
		Tickets: v2Tickets,
	}
	if watch {
		cfg.RefreshFunc = func() []views.BoardTicket {
			raw := fetchTickets()
			out := make([]views.BoardTicket, len(raw))
			for i, t := range raw {
				out[i] = views.BoardTicket{
					ID:       t.ID,
					Title:    t.Title,
					Status:   t.Status,
					Priority: t.Priority,
					Type:     t.Type,
				}
			}
			return out
		}
	}
	return views.RunBoard(cfg)
}

// boardTicket is a local struct for JSON deserialization from bd list.
type boardTicket struct {
	ID       string
	Title    string
	Status   string
	Priority string
	Type     string
}

// fetchTickets attempts to get tickets from the beads system (bd list).
// Returns nil if bd is not installed or returns no data.
func fetchTickets() []boardTicket {
	if _, err := exec.LookPath("bd"); err != nil {
		return nil // bd not installed — caller handles the message
	}

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

	tickets := make([]boardTicket, len(raw))
	for i, r := range raw {
		tickets[i] = boardTicket{
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
