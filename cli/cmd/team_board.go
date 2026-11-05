package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/i18n"
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
	addBoardFlags(teamBoardCmd)
}

// runTeamBoard opens the TUI on the board of the team ResolveTeam gives (--team,
// else the project, else the default team); its team state must be set up and
// cloned.
func runTeamBoard(cmd *cobra.Command, args []string) error {
	a := MustApp()
	team, err := ResolveTeam(cmd.Context(), a)
	if err != nil {
		return err
	}
	if _, err := ensureTeamRepoFor(team); err != nil {
		return err
	}
	if !canLaunchTUI() {
		return withExitCode(ExitUsage, errors.New(i18n.T("cmd.team.board.needs_terminal")))
	}
	name := team.TeamID
	if t := a.Config.FindTeam(team.TeamID); t != nil {
		name = t.DisplayName()
	}
	return runTUI(tuiLaunch{Team: &views.ActiveTeam{ID: team.TeamID, Name: name}, View: "team.board"})
}

// fetchSubBeadsJSON attempts to get sub-tickets from the beads system.
// Returns nil silently if bd is not installed or fails.
func fetchSubBeadsJSON(parentID string) []struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
} {
	if err := beads.Available(); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := beads.BdCommand(ctx, "list", "--parent", parentID, "--json").Output()
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
