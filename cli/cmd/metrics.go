package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Affiche les métriques d'utilisation",
	RunE:  runMetrics,
}

func init() {
	rootCmd.AddCommand(metricsCmd)
}

func runMetrics(cmd *cobra.Command, args []string) error {
	a := MustApp()

	fmt.Fprintln(a.IO.Out, common.Title.Render("  oh metrics  "))
	fmt.Fprintln(a.IO.Out)

	// Open opencode's database for real metrics
	db, err := opencode.OpenStatsDB()
	if err != nil {
		fmt.Fprintf(a.IO.Out, "  %s Impossible d'ouvrir la base opencode: %v\n",
			common.WarningStyle.Render(common.IconWarning), err)
		fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  Aucune donnée de session disponible."))
		return nil
	}
	if db == nil {
		fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  Base de données opencode non trouvée."))
		return nil
	}
	defer db.Close()

	// Get aggregate stats
	stats, err := opencode.TotalStats(db)
	if err != nil {
		return fmt.Errorf("lecture métriques: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "  Sessions totales:    %d\n", stats.TotalSessions)
	fmt.Fprintf(a.IO.Out, "  Sessions aujourd'hui: %d\n", stats.TodaySessions)
	fmt.Fprintf(a.IO.Out, "  Projets actifs:      %d\n", stats.ActiveProjects)
	fmt.Fprintf(a.IO.Out, "  Coût total:          $%.2f\n", stats.TotalCost)
	fmt.Fprintln(a.IO.Out)

	// Per-project stats using oh registered projects
	projects, _ := a.Projects.List("")
	if len(projects) == 0 {
		fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  Aucun projet enregistré dans oh."))
		return nil
	}

	w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  PROJET\tSESSIONS\tTOKENS IN\tTOKENS OUT\tCOÛT")

	for _, p := range projects {
		pStats, err := opencode.ProjectStats(db, p.Path)
		if err != nil || pStats.TotalSessions == 0 {
			continue
		}
		fmt.Fprintf(w, "  %s\t%d\t%s\t%s\t$%.2f\n",
			p.Name, pStats.TotalSessions,
			formatTokenCount(pStats.TotalTokensIn),
			formatTokenCount(pStats.TotalTokensOut),
			pStats.TotalCost)
	}
	w.Flush()

	fmt.Fprintf(a.IO.Out, "\n  Total tokens: %s in / %s out\n",
		formatTokenCount(stats.TotalTokensIn), formatTokenCount(stats.TotalTokensOut))

	return nil
}

func formatTokenCount(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
