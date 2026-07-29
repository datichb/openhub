package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var teamSyncTrackerCmd = &cobra.Command{
	Use:   "sync-tracker",
	Short: i18n.T("cmd.team.sync_tracker.short"),
	RunE:  runSyncTracker,
}


func init() {
	teamCmd.AddCommand(teamSyncTrackerCmd)
}

func runSyncTracker(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	ctx := cmd.Context()

	// Resolve team config.
	tc := a.Config.ActiveTeam()
	if !tc.Enabled {
		return fmt.Errorf("team non configurée. Lance %s", theme.Bold.Render("oh team init"))
	}

	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		return fmt.Errorf("team-state repo non cloné. Lance %s", theme.Bold.Render("oh team init"))
	}

	teamCfg, err := repo.LoadConfig()
	if err != nil {
		return fmt.Errorf("lecture de la config équipe: %w", err)
	}

	if teamCfg.Tracker.Type == "" {
		fmt.Fprintf(a.IO.Out, "%s Tracker sync non configuré. Ajoute %s dans config.toml du team-state ou lance %s.\n",
			theme.WarningStyle.Render(theme.IconWarning),
			theme.Bold.Render("[tracker]\ntype = \"gitlab\""),
			theme.Bold.Render("oh team config"))
		return nil
	}

	// Merge shared team-state config with local hub.toml overrides.
	effTracker := tracker.ResolveTrackerConfig(
		&teamCfg.Tracker,
		a.Config.Tracker,
		resolveWriteEnabledForTracker(a, teamCfg),
	)

	if !effTracker.Enabled {
		fmt.Fprintf(a.IO.Out, "%s Tracker sync désactivé localement. Retire la ligne %s de hub.toml pour réactiver.\n",
			theme.WarningStyle.Render(theme.IconWarning),
			theme.Bold.Render("[tracker]\nenabled = false"))
		return nil
	}

	trackerType := tracker.Type(effTracker.Type)
	trackerCfg, err := tracker.ResolveCredentials(ctx, buildCredentialSource(a, teamCfg.MCP), trackerType)
	if err != nil {
		return fmt.Errorf("%w\n  Conseil: exporte %s ou configure [mcp.%s] dans hub.toml",
			err, envVarForTracker(trackerType), effTracker.Type)
	}

	t, err := tracker.New(trackerCfg)
	if err != nil {
		return fmt.Errorf("initialisation du tracker: %w", err)
	}

	engineCfg := teamstate.TrackerConfig{
		Type:                 effTracker.Type,
		Enabled:              effTracker.Enabled,
		AutoSync:             effTracker.AutoSync,
		SyncIntervalMinutes:  effTracker.SyncIntervalMinutes,
		AutoPlanAssigned:     effTracker.AutoPlanAssigned,
		MaxAutoPlanPerMember: effTracker.MaxAutoPlanPerMember,
		PushLabels:           effTracker.PushLabels,
		TicketPatterns:       effTracker.TicketPatterns,
		Projects:             effTracker.Projects,
	}
	engine := tracker.NewEngine(t, repo, engineCfg, config.HubDir())

	fmt.Fprintf(a.IO.Out, "%s Sync tracker en cours (%s)...\n",
		theme.Subtitle.Render(theme.IconArrow), teamCfg.Tracker.Type)

	result, err := engine.Run(ctx)
	if err != nil {
		return fmt.Errorf("sync échoué: %w", err)
	}

	// ── Display results ───────────────────────────────────────────────────────

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "%s Résultats du sync (%s)\n\n",
		theme.Title.Render("Tracker Sync"),
		result.SyncedAt.Local().Format("15:04:05"))

	for _, pr := range result.Projects {
		fmt.Fprintf(a.IO.Out, "  %s %s → %s (%s)\n",
			theme.Bold.Render("•"),
			theme.Bold.Render(pr.ProjectID),
			pr.TrackerProject,
			pr.Duration.Round(1*1e6))
		fmt.Fprintf(a.IO.Out, "    Issues récupérées: %d  |  Claims créés: %d  |  Mis à jour: %d  |  Labels pushés: %d\n",
			pr.IssuesFetched, pr.ClaimsCreated, pr.ClaimsUpdated, pr.LabelsPushed)
	}

	if len(result.Warnings) > 0 {
		fmt.Fprintln(a.IO.Out)
		fmt.Fprintf(a.IO.Out, "  %s Warnings:\n", theme.WarningStyle.Render(theme.IconWarning))
		for _, w := range result.Warnings {
			fmt.Fprintf(a.IO.Out, "    %s/%s: %s\n", w.ProjectID, w.TicketID, w.Message)
		}
	}

	if len(result.Errors) > 0 {
		fmt.Fprintln(a.IO.Out)
		fmt.Fprintf(a.IO.Out, "  %s Erreurs:\n", theme.ErrorStyle.Render("✗"))
		for _, e := range result.Errors {
			fmt.Fprintf(a.IO.Out, "    %s\n", e.Error())
		}
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "%s Total: %d créés, %d mis à jour, %d labels pushés\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		result.ClaimsCreated, result.ClaimsUpdated, result.LabelsPushed)

	return nil
}

// envVarForTracker returns the environment variable name for a tracker token.
func envVarForTracker(t tracker.Type) string {
	switch t {
	case tracker.TypeGitLab:
		return "GITLAB_TOKEN"
	case tracker.TypeJira:
		return "JIRA_TOKEN"
	}
	return "TOKEN"
}
