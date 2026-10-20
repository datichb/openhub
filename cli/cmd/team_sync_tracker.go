package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

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
		return errors.New(i18n.Tf("cmd.team.sync_tracker.not_configured", theme.Bold.Render("oh team init")))
	}

	repo := teamstate.NewRepo(tc.StateRepo, tc.StatePath)
	if !repo.IsCloned() {
		return errors.New(i18n.Tf("cmd.team.sync_tracker.not_cloned", theme.Bold.Render("oh team init")))
	}

	teamCfg, err := repo.LoadConfig()
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.team.sync_tracker.config_read_error"), err)
	}

	if teamCfg.Tracker.Type == "" {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.WarningStyle.Render(theme.IconWarning),
			i18n.Tf("cmd.team.sync_tracker.tracker_not_configured",
				theme.Bold.Render("[tracker]\ntype = \"gitlab\""),
				theme.Bold.Render("oh team config")))
		return nil
	}

	// Merge shared team-state config with local hub.toml overrides.
	effTracker := tracker.ResolveTrackerConfig(
		&teamCfg.Tracker,
		a.Config.Tracker,
	)

	if !effTracker.Enabled {
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.WarningStyle.Render(theme.IconWarning),
			i18n.Tf("cmd.team.sync_tracker.disabled_locally",
				theme.Bold.Render("[tracker]\nenabled = false")))
		return nil
	}

	trackerType := tracker.Type(effTracker.Type)
	trackerCfg, err := tracker.ResolveCredentials(ctx, buildCredentialSource(a, effTracker), trackerType)
	if err != nil {
		return fmt.Errorf("%w\n  %s",
			err, i18n.Tf("cmd.team.sync_tracker.credential_hint", envVarForTracker(trackerType), effTracker.Type))
	}

	t, err := tracker.New(trackerCfg)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.team.sync_tracker.init_error"), err)
	}

	// Build the Projects map for the engine from hub projects.
	projects, ticketPatterns := resolveTrackerProjects(ctx, a, effTracker)

	if len(projects) == 0 {
		fmt.Fprint(a.IO.ErrOut, i18n.Tf("cmd.team_sync_tracker.no_project_configured_for_the_tracker", theme.WarningStyle.Render(theme.IconWarning)))
		fmt.Fprintf(a.IO.ErrOut, "  Vérifiez:\n")
		fmt.Fprint(a.IO.ErrOut, i18n.T("cmd.team_sync_tracker.tracker_project_is_set_in_the"))
		fmt.Fprint(a.IO.ErrOut, i18n.T("cmd.team_sync_tracker.or_a_hub_project_overrides_tracker"))
		return nil
	}

	slog.Debug("tracker.sync_cmd.config", "type", effTracker.Type, "projects", len(projects), "autoplan", effTracker.AutoPlanAssigned, "push_labels", effTracker.PushLabels)

	engineCfg := teamstate.TrackerConfig{
		Type:                 effTracker.Type,
		Enabled:              effTracker.Enabled,
		AutoSync:             effTracker.AutoSync,
		SyncIntervalMinutes:  effTracker.SyncIntervalMinutes,
		AutoPlanAssigned:     effTracker.AutoPlanAssigned,
		MaxAutoPlanPerMember: effTracker.MaxAutoPlanPerMember,
		AutoPlanUnassigned:   effTracker.AutoPlanUnassigned,
		UnassignedLabels:     effTracker.UnassignedLabels,
		MaxUnassignedIssues:  effTracker.MaxUnassignedIssues,
		PushLabels:           effTracker.PushLabels,
		TicketPatterns:       ticketPatterns, //nolint:staticcheck // legacy tracker fields still read by the sync engine (backward compatibility)
		Projects:             projects,       //nolint:staticcheck // legacy tracker fields still read by the sync engine (backward compatibility)
		StatusMapping:        effTracker.StatusMapping,
		LabelStatusMapping:   effTracker.LabelStatusMapping,
	}
	engine := tracker.NewEngine(t, repo, engineCfg, config.HubDir())

	// Spinner feedback during sync (disabled in verbose mode to avoid log interference)
	spinDone := make(chan struct{})
	showSpinner := !slog.Default().Enabled(context.Background(), slog.LevelDebug)
	if showSpinner {
		go func() {
			frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			msg := i18n.Tf("cmd.team.sync_tracker.running", teamCfg.Tracker.Type)
			for i := 0; ; i++ {
				select {
				case <-spinDone:
					fmt.Fprintf(a.IO.ErrOut, "\r%s\r", strings.Repeat(" ", len(msg)+4))
					return
				default:
					fmt.Fprintf(a.IO.ErrOut, "\r%s %s", frames[i%len(frames)], msg)
					time.Sleep(80 * time.Millisecond)
				}
			}
		}()
	}

	result, err := engine.Run(ctx)
	close(spinDone)
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.T("cmd.team.sync_tracker.failed"), err)
	}

	out := a.IO.Out

	// Check if any issues were fetched across all projects
	totalIssues := 0
	for _, pr := range result.Projects {
		totalIssues += pr.IssuesFetched
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "%s %s\n\n",
		theme.Title.Render(i18n.T("cmd.team.sync_tracker.title")),
		i18n.Tf("cmd.team.sync_tracker.results_title",
			result.SyncedAt.Local().Format("15:04:05")))

	for _, pr := range result.Projects {
		fmt.Fprintf(out, "  %s %s → %s (%s)\n",
			theme.Bold.Render("•"),
			theme.Bold.Render(pr.ProjectID),
			pr.TrackerProject,
			pr.Duration.Round(1*1e6))
		fmt.Fprintf(out, "%s\n",
			i18n.Tf("cmd.team.sync_tracker.project_stats",
				pr.IssuesFetched, pr.ClaimsCreated, pr.ClaimsUpdated, pr.LabelsPushed))
	}

	if totalIssues == 0 && result.ClaimsCreated == 0 {
		fmt.Fprint(out, i18n.Tf("cmd.team_sync_tracker.no_issue_found_check_that", theme.WarningStyle.Render("ℹ")))
		fmt.Fprint(out, i18n.T("cmd.team_sync_tracker.tracker_project_matches_the_gitlab_project"))
		fmt.Fprint(out, i18n.T("cmd.team_sync_tracker.the_token_can_access_the_project"))
		fmt.Fprint(out, i18n.T("cmd.team_sync_tracker.issues_are_assigned_to_members_gitlab"))
		fmt.Fprint(out, i18n.T("cmd.team_sync_tracker.auto_plan_assigned_is_enabled"))
		fmt.Fprint(out, i18n.T("cmd.team_sync_tracker.tip_run_again_with_verbose_to"))
	}

	if len(result.Warnings) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "  %s %s\n", theme.WarningStyle.Render(theme.IconWarning),
			i18n.T("cmd.team.sync_tracker.warnings"))
		for _, w := range result.Warnings {
			fmt.Fprintf(out, "    %s/%s: %s\n", w.ProjectID, w.TicketID, w.Message)
		}
	}

	if len(result.Errors) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "  %s %s\n", theme.ErrorStyle.Render("✗"),
			i18n.T("cmd.team.sync_tracker.errors"))
		for _, e := range result.Errors {
			fmt.Fprintf(out, "    %s\n", e.Error())
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess),
		i18n.Tf("cmd.team.sync_tracker.total",
			result.ClaimsCreated, result.ClaimsUpdated, result.LabelsPushed))

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
