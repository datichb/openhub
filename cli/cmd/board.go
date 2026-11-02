package cmd

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/cliout"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// `oh board`, `oh team board` and `oh dashboard` open the TUI shell on the
// view that shows the same thing (board, team.board, metrics), over the home
// view of the mode: Esc goes back to it. Without a terminal there is no
// board to show: the command ends with ExitUsage and names what works
// without one.

var boardCmd = &cobra.Command{
	Use:  "board",
	RunE: runBoard,
}

func init() {
	rootCmd.AddCommand(boardCmd)
	addBoardFlags(boardCmd)
	boardCmd.Flags().String("project", "", "")
}

// addBoardFlags adds --watch: the board of the TUI refreshes every 5 s on
// its own (and on r), so the option stays accepted for the scripts and the
// habits, without effect.
func addBoardFlags(c *cobra.Command) {
	c.Flags().Bool("watch", false, "")
	_ = c.Flags().MarkHidden("watch")
}

func runBoard(cmd *cobra.Command, args []string) error {
	a := MustApp()

	// Without bd there is nothing to list, with or without a terminal.
	if !bdInstalled() {
		cliout.Warn(cmd.OutOrStdout(), i18n.T("tui.board.bd_not_installed"))
		return nil
	}
	if !canLaunchTUI() {
		return withExitCode(ExitUsage, errors.New(i18n.T("cmd.board.needs_terminal")))
	}

	name, _ := cmd.Flags().GetString("project")
	project, err := boardProject(cmd.Context(), a.Projects, name)
	if err != nil {
		return err
	}
	return runTUI(tuiLaunch{Project: project, View: "board"})
}

// bdInstalled reports whether bd (Beads) is in the PATH.
func bdInstalled() bool { return beads.Available() == nil }

// boardProject is the project of `oh board`: the one that is named, else the
// one that holds the current directory, else the only one.
func boardProject(ctx context.Context, projects domain.ProjectStore, name string) (string, error) {
	if name != "" {
		return name, nil
	}
	if projects == nil {
		return "", withExitCode(ExitUsage, errors.New(i18n.T("cmd.board.no_project")))
	}
	active, err := projects.List(ctx, domain.ProjectStatusActive)
	if err != nil {
		return "", err
	}
	if p := projectAtCwd(active); p != nil {
		return p.Name, nil
	}
	if len(active) == 1 {
		return active[0].Name, nil
	}
	return "", withExitCode(ExitUsage, errors.New(i18n.T("cmd.board.no_project")))
}
