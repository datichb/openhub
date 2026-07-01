package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/domain"
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
	a := GetApp()

	fmt.Fprintln(a.IO.Out, common.Title.Render("  oh metrics  "))
	fmt.Fprintln(a.IO.Out)

	sessions, _ := a.Sessions.List("")

	// Per-project stats
	type projectStats struct {
		sessions  int
		tokensIn  int64
		tokensOut int64
	}
	stats := make(map[string]*projectStats)

	for _, s := range sessions {
		ps, ok := stats[s.ProjectID]
		if !ok {
			ps = &projectStats{}
			stats[s.ProjectID] = ps
		}
		ps.sessions++
		ps.tokensIn += s.TokensIn
		ps.tokensOut += s.TokensOut
	}

	// Display
	fmt.Fprintf(a.IO.Out, "  Sessions totales: %d\n\n", len(sessions))

	if len(stats) == 0 {
		fmt.Fprintln(a.IO.Out, common.Subtitle.Render("  Aucune session enregistrée."))
		return nil
	}

	// Get project names
	projects, _ := a.Projects.List("")
	nameMap := make(map[string]string)
	for _, p := range projects {
		nameMap[p.ID] = p.Name
	}

	w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  PROJET\tSESSIONS\tTOKENS IN\tTOKENS OUT")
	for id, ps := range stats {
		name := nameMap[id]
		if name == "" {
			name = id
		}
		fmt.Fprintf(w, "  %s\t%d\t%s\t%s\n",
			name, ps.sessions,
			formatTokenCount(ps.tokensIn), formatTokenCount(ps.tokensOut))
	}
	w.Flush()

	// Totals
	var totalIn, totalOut int64
	for _, ps := range stats {
		totalIn += ps.tokensIn
		totalOut += ps.tokensOut
	}
	fmt.Fprintf(a.IO.Out, "\n  Total tokens: %s in / %s out\n",
		formatTokenCount(totalIn), formatTokenCount(totalOut))

	// Status breakdown
	running := 0
	completed := 0
	failed := 0
	for _, s := range sessions {
		switch s.Status {
		case domain.SessionStatusRunning:
			running++
		case domain.SessionStatusCompleted:
			completed++
		case domain.SessionStatusFailed:
			failed++
		}
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "  %s %d complétées  %s %d en cours  %s %d échouées\n",
		common.SuccessStyle.Render(common.IconSuccess), completed,
		common.WarningStyle.Render(common.IconInfo), running,
		common.ErrorStyle.Render(common.IconError), failed)

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
