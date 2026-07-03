package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var optimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "Analyse et suggestions d'optimisation",
	RunE: func(cmd *cobra.Command, args []string) error {
		a := MustApp()
		ctx := cmd.Context()

		fmt.Fprintln(a.IO.Out, common.Title.Render("  oh optimize  "))
		fmt.Fprintln(a.IO.Out)

		// Analyze sessions for optimization opportunities
		sessions, err := a.Sessions.List(ctx, "")
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf("cmd.optimize.read_sessions", err))
		}

		if len(sessions) == 0 {
			fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  "+i18n.T("cmd.optimize.no_data")))
			return nil
		}

		var totalIn, totalOut int64
		for _, s := range sessions {
			totalIn += s.TokensIn
			totalOut += s.TokensOut
		}

		fmt.Fprintln(a.IO.Out, common.Bold.Render("  "+i18n.T("cmd.optimize.analysis_title")))
		fmt.Fprintf(a.IO.Out, "  %s %s\n", common.IconInfo, i18n.Tf("cmd.optimize.sessions_analyzed", len(sessions)))
		fmt.Fprintf(a.IO.Out, "  %s %s\n", common.IconInfo, i18n.Tf("cmd.optimize.tokens_consumed", formatTokenCount(totalIn+totalOut)))
		fmt.Fprintln(a.IO.Out)

		// Suggestions
		fmt.Fprintln(a.IO.Out, common.Bold.Render("  "+i18n.T("cmd.optimize.suggestions_title")))
		if totalIn > 100_000 {
			fmt.Fprintf(a.IO.Out, "  %s %s\n",
				common.WarningStyle.Render(common.IconArrow), i18n.T("cmd.optimize.suggest_rtk"))
		}
		if len(sessions) > 10 && totalOut/int64(len(sessions)) > 5000 {
			fmt.Fprintf(a.IO.Out, "  %s %s\n",
				common.WarningStyle.Render(common.IconArrow), i18n.T("cmd.optimize.suggest_concise"))
		}
		fmt.Fprintf(a.IO.Out, "  %s %s\n",
			common.Subtitle.Render(common.IconDot), i18n.T("cmd.optimize.suggest_compaction"))

		return nil
	},
}

var yieldCmd = &cobra.Command{
	Use:   "yield",
	Short: "Affiche le rapport sessions ↔ commits",
	RunE: func(cmd *cobra.Command, args []string) error {
		a := MustApp()
		ctx := cmd.Context()

		fmt.Fprintln(a.IO.Out, common.Title.Render("  oh yield — Sessions ↔ Commits  "))
		fmt.Fprintln(a.IO.Out)

		sessions, err := a.Sessions.List(ctx, "")
		if err != nil {
			return fmt.Errorf("%s", i18n.Tf("cmd.optimize.read_sessions", err))
		}

		if len(sessions) == 0 {
			fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  "+i18n.T("cmd.yield.no_sessions")))
			return nil
		}

		completed := 0
		for _, s := range sessions {
			if s.Status == "completed" {
				completed++
			}
		}

		fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.yield.completed", completed))
		fmt.Fprintf(a.IO.Out, "  %s\n",
			i18n.Tf("cmd.yield.estimated", estimateYield(completed)))
		fmt.Fprintln(a.IO.Out)
		fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  "+i18n.T("cmd.yield.more_data")))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(optimizeCmd)
	rootCmd.AddCommand(yieldCmd)
}

func estimateYield(sessions int) float64 {
	if sessions == 0 {
		return 0
	}
	// Placeholder — in real implementation, count git commits during session windows
	return 1.2
}
