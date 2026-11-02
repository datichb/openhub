package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

var boardCmd = &cobra.Command{
	Use:  "board",
	RunE: runBoard,
}

func init() {
	rootCmd.AddCommand(boardCmd)
	boardCmd.Flags().Bool("watch", false, "")
}

func runBoard(cmd *cobra.Command, args []string) error {
	a := MustApp()
	tickets := fetchTickets()

	if len(tickets) == 0 {
		// Distinguish "bd not installed" from "0 tickets"
		if err := beads.Available(); err != nil {
			cliout.Warn(a.IO.Out, i18n.T("tui.board.bd_not_installed"))
		} else {
			fmt.Fprintln(a.IO.Out, theme.Subtitle.Render(i18n.T("tui.board.no_tickets_hint")))
		}
		return nil
	}

	watch, _ := cmd.Flags().GetBool("watch")

	v2Tickets := boardViewTickets(tickets)
	cfg := views.BoardConfig{
		Layout: layout.Config{
			ProjectName: a.Config.Name,
			Command:     "board",
			StatusHints: i18n.T("cmd.board.status_hints"),
		},
		Tickets: v2Tickets,
	}
	if watch {
		cfg.RefreshFunc = func() []views.BoardTicket {
			return boardViewTickets(fetchTickets())
		}
	}
	return views.RunBoard(cfg)
}

// boardTicket is a local struct for the CLI board display.
type boardTicket struct {
	ID       string
	Title    string
	Status   string
	Priority string
	Type     string
	Labels   []string
}

// boardViewTickets converts the CLI board tickets to the board view ones.
func boardViewTickets(raw []boardTicket) []views.BoardTicket {
	out := make([]views.BoardTicket, len(raw))
	for i, t := range raw {
		out[i] = views.BoardTicket{
			ID:       t.ID,
			Title:    t.Title,
			Status:   t.Status,
			Priority: t.Priority,
			Type:     t.Type,
			Labels:   t.Labels,
		}
	}
	return out
}

// fetchTickets attempts to get tickets from the beads system via beads.ListCWD.
// Returns nil if bd is not installed or returns no data.
func fetchTickets() []boardTicket {
	if err := beads.Available(); err != nil {
		return nil
	}
	raw, err := beads.ListCWD()
	if err != nil || len(raw) == 0 {
		return nil
	}
	return boardTickets(raw)
}

// boardTickets maps Beads tickets to the CLI board ones, the status being
// the board column (an in-progress ticket labelled ready-for-review is in
// REVIEW).
func boardTickets(raw []beads.Ticket) []boardTicket {
	tickets := make([]boardTicket, len(raw))
	for i, r := range raw {
		tickets[i] = boardTicket{
			ID:       r.ID,
			Title:    r.Title,
			Status:   beads.BoardColumn(r.Status, r.Labels),
			Priority: r.Priority,
			Type:     r.Type,
			Labels:   r.Labels,
		}
	}
	return tickets
}
