package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
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
		return fmt.Errorf(i18n.T("cmd.dashboard_cmd.reading_projects"), err)
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

	// Gather real metrics from platform stats provider
	var totalSessions, todaySessions int
	var tokensUsed int64
	if a.Stats != nil && a.Stats.Available() {
		stats, err := a.Stats.AggregateStats("all")
		if err == nil && stats != nil {
			totalSessions = stats.TotalSessions
			todaySessions = stats.TodaySessions
			tokensUsed = stats.TotalTokensIn + stats.TotalTokensOut
		}
	}

	cfg := views.DashboardConfig{
		Layout: layout.Config{
			ProjectName: a.Config.Name,
			Command:     "dashboard",
			StatusHints: i18n.T("tui.dashboard.quit"),
		},
		Stats: []views.DashboardStat{
			{Label: i18n.T("tui.dashboard.projects"), Value: i18n.Tf("tui.dashboard.projects_value", len(projects), active)},
			{Label: i18n.T("tui.dashboard.top_project"), Value: topProject},
			{Label: i18n.T("tui.dashboard.sessions"), Value: i18n.Tf("tui.dashboard.sessions_value", totalSessions, todaySessions)},
			{Label: i18n.T("tui.dashboard.tokens_used_label"), Value: formatTokens(tokensUsed)},
		},
		TokenUsage: []views.TokenBar{
			{Label: i18n.T("tui.dashboard.today_label"), Current: todaySessions, Max: 50},
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
