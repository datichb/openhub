package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Affiche le tableau de bord du hub",
	Long:  "Lance un dashboard interactif affichant les métriques projets, sessions et tokens.",
	RunE:  runDashboard,
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
}

func runDashboard(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	// Gather project stats from oh DB
	projects, err := a.Projects.List(ctx, "")
	if err != nil {
		return fmt.Errorf("lecture projets: %w", err)
	}
	active := 0
	for _, p := range projects {
		if p.Status == domain.ProjectStatusActive {
			active++
		}
	}

	topProject := "—"
	if len(projects) > 0 {
		topProject = projects[0].Name
	}

	// Gather real metrics from opencode DB
	var totalSessions, todaySessions int
	var tokensUsed int64
	db, err := opencode.OpenStatsDB()
	if err == nil && db != nil {
		defer db.Close()
		stats, err := opencode.TotalStats(db)
		if err == nil {
			totalSessions = stats.TotalSessions
			todaySessions = stats.TodaySessions
			tokensUsed = stats.TotalTokensIn + stats.TotalTokensOut
		}
	}

	cfg := views.DashboardConfig{
		Layout: layout.Config{
			ProjectName: a.Config.Name,
			Command:     "dashboard",
			StatusHints: "q quit",
		},
		Stats: []views.DashboardStat{
			{Label: "Projects", Value: fmt.Sprintf("%d (%d active)", len(projects), active)},
			{Label: "Top Project", Value: topProject},
			{Label: "Sessions", Value: fmt.Sprintf("%d total / %d today", totalSessions, todaySessions)},
			{Label: "Tokens Used", Value: formatTokens(tokensUsed)},
		},
		TokenUsage: []views.TokenBar{
			{Label: "Today", Current: todaySessions, Max: 50},
		},
		RecentItems: []string{},
	}

	return views.RunDashboard(cfg)
}

func formatTokens(n int64) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	}
	return strconv.FormatInt(n, 10)
}
