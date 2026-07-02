package cmd

import (
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/domain"
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

	// Gather stats
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

	sessions, _ := a.Sessions.List("")
	var totalTokensIn, totalTokensOut int64
	for _, s := range sessions {
		totalTokensIn += s.TokensIn
		totalTokensOut += s.TokensOut
	}

	cfg := dashboard.Config{
		Title: "oh dashboard — Vue d'ensemble",
		Stats: dashboard.Stats{
			TotalProjects:  len(projects),
			ActiveProjects: active,
			TotalSessions:  len(sessions),
			TodaySessions:  0, // TODO: filter by today
			TokensUsed:     totalTokensIn + totalTokensOut,
			TokensSaved:    0, // TODO: from RTK metrics
			TopProject:     topProject,
		},
	}

	return dashboard.Run(cfg)
}
