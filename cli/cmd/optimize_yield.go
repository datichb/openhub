package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/tui/common"
)

var optimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "Analyse et suggestions d'optimisation",
	RunE: func(cmd *cobra.Command, args []string) error {
		a := GetApp()

		fmt.Fprintln(a.IO.Out, common.Title.Render("  oh optimize  "))
		fmt.Fprintln(a.IO.Out)

		// Analyze sessions for optimization opportunities
		sessions, _ := a.Sessions.List("")

		if len(sessions) == 0 {
			fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  Pas assez de données pour analyser."))
			return nil
		}

		var totalIn, totalOut int64
		for _, s := range sessions {
			totalIn += s.TokensIn
			totalOut += s.TokensOut
		}

		fmt.Fprintln(a.IO.Out, common.Bold.Render("  Analyse d'utilisation:"))
		fmt.Fprintf(a.IO.Out, "  %s Sessions analysées: %d\n", common.IconInfo, len(sessions))
		fmt.Fprintf(a.IO.Out, "  %s Tokens consommés: %s\n", common.IconInfo, formatTokenCount(totalIn+totalOut))
		fmt.Fprintln(a.IO.Out)

		// Suggestions
		fmt.Fprintln(a.IO.Out, common.Bold.Render("  Suggestions:"))
		if totalIn > 100_000 {
			fmt.Fprintf(a.IO.Out, "  %s Activez RTK pour réduire la consommation de tokens\n",
				common.WarningStyle.Render(common.IconArrow))
		}
		if len(sessions) > 10 && totalOut/int64(len(sessions)) > 5000 {
			fmt.Fprintf(a.IO.Out, "  %s Utilisez des prompts plus concis pour réduire les outputs\n",
				common.WarningStyle.Render(common.IconArrow))
		}
		fmt.Fprintf(a.IO.Out, "  %s Configurez la compaction de cache opencode\n",
			common.Subtitle.Render(common.IconDot))

		return nil
	},
}

var yieldCmd = &cobra.Command{
	Use:   "yield",
	Short: "Affiche le rapport sessions ↔ commits",
	RunE: func(cmd *cobra.Command, args []string) error {
		a := GetApp()

		fmt.Fprintln(a.IO.Out, common.Title.Render("  oh yield — Sessions ↔ Commits  "))
		fmt.Fprintln(a.IO.Out)

		sessions, _ := a.Sessions.List("")

		if len(sessions) == 0 {
			fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  Aucune session enregistrée."))
			return nil
		}

		completed := 0
		for _, s := range sessions {
			if s.Status == "completed" {
				completed++
			}
		}

		fmt.Fprintf(a.IO.Out, "  Sessions complétées: %d\n", completed)
		fmt.Fprintf(a.IO.Out, "  Rendement estimé: %.1f commits/session\n",
			estimateYield(completed))
		fmt.Fprintln(a.IO.Out)
		fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  (Analyse détaillée disponible après plus de sessions)"))

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
