package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/common"
)

var metricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Affiche les métriques d'utilisation",
	Long: `Affiche les métriques d'utilisation d'opencode : sessions, tokens, coûts, et économies AI.

Utilisez --period pour filtrer par période :
  --period 7d   : 7 derniers jours
  --period 30d  : 30 derniers jours
  --period all  : toutes les données (défaut)`,
	RunE: runMetrics,
}

func init() {
	rootCmd.AddCommand(metricsCmd)
	metricsCmd.Flags().StringP("period", "p", "all", "Période d'analyse (7d, 30d, all)")
}

func runMetrics(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	period, _ := cmd.Flags().GetString("period")

	// Validate period
	switch period {
	case "7d", "30d", "all":
		// valid
	default:
		return fmt.Errorf("%s", i18n.Tf("cmd.metrics.period_invalid", period))
	}

	fmt.Fprintln(a.IO.Out, common.Title.Render("  oh metrics  "))
	fmt.Fprintln(a.IO.Out)

	if period != "all" {
		fmt.Fprintf(a.IO.Out, "  %s\n\n", i18n.Tf("cmd.metrics.period_label", period))
	}

	// Open opencode's database for real metrics
	db, err := opencode.OpenStatsDB()
	if err != nil {
		fmt.Fprintf(a.IO.Out, "  %s %s\n",
			common.WarningStyle.Render(common.IconWarning), i18n.Tf("cmd.metrics.no_db", err))
		fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  "+i18n.T("cmd.metrics.no_db_file")))
		return nil
	}
	if db == nil {
		fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  "+i18n.T("cmd.metrics.no_db_file")))
		return nil
	}
	defer db.Close()

	// Get aggregate stats for the period
	stats, err := opencode.PeriodStats(db, period)
	if err != nil {
		return fmt.Errorf("%s", i18n.Tf("cmd.metrics.read_error", err))
	}

	if stats.TotalSessions == 0 {
		fmt.Fprintf(a.IO.Out, "  %s %s\n",
			common.WarningStyle.Render(common.IconWarning), i18n.Tf("cmd.metrics.no_data", period))
		return nil
	}

	// --- Usage summary ---
	fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  "+i18n.T("cmd.metrics.usage")))
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.metrics.sessions", stats.TotalSessions))
	if period == "all" {
		fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.metrics.today", stats.TodaySessions))
	}
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.metrics.active_projects", stats.ActiveProjects))
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.metrics.total_cost", stats.TotalCost))
	fmt.Fprintln(a.IO.Out)

	// --- Token breakdown ---
	fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  "+i18n.T("cmd.metrics.tokens")))
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.metrics.tokens_in", formatTokenCount(stats.TotalTokensIn)))
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.metrics.tokens_out", formatTokenCount(stats.TotalTokensOut)))
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.metrics.tokens_reasoning", formatTokenCount(stats.ReasoningTokens)))
	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.metrics.tokens_cache", formatTokenCount(stats.CacheReadTokens)))
	fmt.Fprintln(a.IO.Out)

	// --- AI Savings ---
	fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  "+i18n.T("cmd.metrics.savings")))
	fmt.Fprintln(a.IO.Out)
	displayAISavings(a, stats)
	fmt.Fprintln(a.IO.Out)

	// --- Per-project breakdown ---
	projects, _ := a.Projects.List(ctx, "")
	if len(projects) == 0 {
		return nil
	}

	fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  "+i18n.T("cmd.metrics.per_project")))
	fmt.Fprintln(a.IO.Out)

	w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, i18n.T("cmd.metrics.table.header"))

	for _, p := range projects {
		pStats, err := opencode.ProjectPeriodStats(db, p.Path, period)
		if err != nil || pStats.TotalSessions == 0 {
			continue
		}
		cacheRatio := ""
		if pStats.TotalTokensIn > 0 {
			ratio := float64(pStats.CacheReadTokens) / float64(pStats.TotalTokensIn) * 100
			cacheRatio = fmt.Sprintf("%.0f%%", ratio)
		}
		fmt.Fprintf(w, "  %s\t%d\t%s\t%s\t%s\t$%.2f\n",
			p.Name, pStats.TotalSessions,
			formatTokenCount(pStats.TotalTokensIn),
			formatTokenCount(pStats.TotalTokensOut),
			cacheRatio,
			pStats.TotalCost)
	}
	w.Flush()

	return nil
}

// displayAISavings computes and shows AI cost savings from caching.
func displayAISavings(a *app.App, stats *opencode.AggregateStats) {
	// Cache hit ratio: tokens read from cache vs total input tokens
	if stats.TotalTokensIn == 0 {
		fmt.Fprintf(a.IO.Out, "  %s\n", i18n.T("cmd.metrics.savings_nodata"))
		return
	}

	cacheRatio := float64(stats.CacheReadTokens) / float64(stats.TotalTokensIn) * 100

	// Savings estimation:
	// Cache reads cost ~90% less than fresh input tokens on most providers.
	// Conservative estimate: cache tokens save 75% of their cost.
	const cacheSavingsFactor = 0.75
	estimatedSavings := float64(stats.CacheReadTokens) / float64(stats.TotalTokensIn) * stats.TotalCost * cacheSavingsFactor

	fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.metrics.cache_ratio", cacheRatio))
	fmt.Fprintf(a.IO.Out, "  %s\n",
		i18n.Tf("cmd.metrics.cache_tokens", formatTokenCount(stats.CacheReadTokens), formatTokenCount(stats.TotalTokensIn)))

	if estimatedSavings > 0.01 {
		fmt.Fprintf(a.IO.Out, "  %s\n",
			i18n.Tf("cmd.metrics.estimated_savings", estimatedSavings, cacheRatio*cacheSavingsFactor))
	}

	// Reasoning efficiency
	if stats.ReasoningTokens > 0 && stats.TotalTokensOut > 0 {
		reasoningRatio := float64(stats.ReasoningTokens) / float64(stats.TotalTokensOut) * 100
		fmt.Fprintf(a.IO.Out, "  %s\n", i18n.Tf("cmd.metrics.reasoning_ratio", reasoningRatio))
	}
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
