package cmd

import (
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/tui/views/dashboard"
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

	// Gather project stats from oh DB
	projects, _ := a.Projects.List("")
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

	cfg := dashboard.Config{
		Title: "oh dashboard — Vue d'ensemble",
		Stats: dashboard.Stats{
			TotalProjects:  len(projects),
			ActiveProjects: active,
			TotalSessions:  totalSessions,
			TodaySessions:  todaySessions,
			TokensUsed:     tokensUsed,
			TokensSaved:    0, // TODO: from RTK metrics
			TopProject:     topProject,
		},
	}

	return dashboard.Run(cfg)
}
