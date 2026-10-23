package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/i18n"
	teamsvc "github.com/datichb/openhub/cli/internal/services/team"
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

	result, err := teamService(a).SyncTracker(ctx, teamsvc.SyncOptions{
		OnRun: func(trackerType string) func() { return startSyncSpinner(a, trackerType) },
	})
	if err != nil {
		return syncTrackerError(a, err)
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
			fmt.Fprintf(out, "    %s\n", w.String())
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

// syncTrackerError translates the typed errors of the team service into the
// messages of the command line. The cases that are not failures (nothing to
// sync, tracker disabled in hub.toml) print a notice and return nil.
func syncTrackerError(a *app.App, err error) error {
	var (
		cfgErr  *teamsvc.ConfigError
		credErr *teamsvc.CredentialsError
		initErr *teamsvc.TrackerInitError
	)
	switch {
	case errors.Is(err, teamsvc.ErrTeamNotConfigured):
		return errors.New(i18n.Tf("cmd.team.sync_tracker.not_configured", theme.Bold.Render("oh team init")))
	case errors.Is(err, teamsvc.ErrNotCloned):
		return errors.New(i18n.Tf("cmd.team.sync_tracker.not_cloned", theme.Bold.Render("oh team init")))
	case errors.As(err, &cfgErr):
		return fmt.Errorf("%s: %w", i18n.T("cmd.team.sync_tracker.config_read_error"), cfgErr.Err)
	case errors.Is(err, teamsvc.ErrTrackerNotConfigured):
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.WarningStyle.Render(theme.IconWarning),
			i18n.Tf("cmd.team.sync_tracker.tracker_not_configured",
				theme.Bold.Render("[tracker]\ntype = \"gitlab\""),
				theme.Bold.Render("oh team config")))
		return nil
	case errors.Is(err, teamsvc.ErrTrackerDisabled):
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.WarningStyle.Render(theme.IconWarning),
			i18n.Tf("cmd.team.sync_tracker.disabled_locally",
				theme.Bold.Render("[tracker]\nenabled = false")))
		return nil
	case errors.As(err, &credErr) && tracker.IsMissingToken(credErr.Err):
		// the error names the server, the sources read and what to declare
		return credErr.Err
	case errors.As(err, &credErr):
		return fmt.Errorf("%w\n  %s",
			credErr.Err, i18n.Tf("cmd.team.sync_tracker.credential_hint", credErr.EnvVar, credErr.TrackerType))
	case errors.As(err, &initErr):
		return fmt.Errorf("%s: %w", i18n.T("cmd.team.sync_tracker.init_error"), initErr.Err)
	case errors.Is(err, teamsvc.ErrNoProjects):
		fmt.Fprint(a.IO.ErrOut, i18n.Tf("cmd.team_sync_tracker.no_project_configured_for_the_tracker", theme.WarningStyle.Render(theme.IconWarning)))
		fmt.Fprintf(a.IO.ErrOut, "  Vérifiez:\n")
		fmt.Fprint(a.IO.ErrOut, i18n.T("cmd.team_sync_tracker.tracker_project_is_set_in_the"))
		fmt.Fprint(a.IO.ErrOut, i18n.T("cmd.team_sync_tracker.or_a_hub_project_overrides_tracker"))
		return nil
	}
	return fmt.Errorf("%s: %w", i18n.T("cmd.team.sync_tracker.failed"), err)
}

// startSyncSpinner shows a spinner on stderr during the sync (not in verbose
// mode, where it would mix with the logs) and returns the function that
// stops it and erases the line.
func startSyncSpinner(a *app.App, trackerType string) func() {
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		return nil
	}
	stop := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		msg := i18n.Tf("cmd.team.sync_tracker.running", trackerType)
		for i := 0; ; i++ {
			select {
			case <-stop:
				fmt.Fprintf(a.IO.ErrOut, "\r%s\r", strings.Repeat(" ", len(msg)+4))
				return
			default:
				fmt.Fprintf(a.IO.ErrOut, "\r%s %s", frames[i%len(frames)], msg)
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
	return func() {
		close(stop)
		<-stopped
	}
}
