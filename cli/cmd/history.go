package cmd

import (
	"context"
	"errors"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: i18n.T("cmd.history.short"),
	Long:  i18n.T("cmd.history.long"),
	RunE:  runHistory,
}

var historyExportCmd = &cobra.Command{
	Use:   "export",
	Short: i18n.T("cmd.history.export.short"),
	RunE:  runHistoryExport,
}

var historyImportCmd = &cobra.Command{
	Use:   "import [file]",
	Short: i18n.T("cmd.history.import.short"),
	Args:  cobra.ExactArgs(1),
	RunE:  runHistoryImport,
}

func init() {
	rootCmd.AddCommand(historyCmd)
	historyCmd.AddCommand(historyExportCmd)
	historyCmd.AddCommand(historyImportCmd)

	historyCmd.Flags().Bool("team", false, i18n.T("cmd.history.flags.team"))
	historyCmd.Flags().Int("limit", 20, "Maximum number of entries")

	historyExportCmd.Flags().String("output", "", i18n.T("cmd.history.export.flags.output"))
	historyExportCmd.Flags().String("member-id", "", i18n.T("cmd.history.export.flags.member_id"))
	historyImportCmd.Flags().String("member-id", "", i18n.T("cmd.history.import.flags.member_id"))
}

// activeMemberID is the member of the active team ("" without a team).
func activeMemberID(a *app.App) string {
	if activeTeam := a.Config.ActiveTeam(); activeTeam.Enabled {
		return activeTeam.MemberID
	}
	return ""
}

// historyError words the failure of an export or an import for the command
// line.
func historyError(err error) error {
	var he *sessionsvc.HistoryError
	if !errors.As(err, &he) {
		return err
	}
	key := ""
	switch he.Stage {
	case sessionsvc.HistoryList:
		key = "cmd.history.listing_sessions"
	case sessionsvc.HistoryEncode:
		key = "cmd.history.encoding_export"
	case sessionsvc.HistoryWrite:
		key = "cmd.history.writing_export"
	case sessionsvc.HistoryRead:
		key = "cmd.history.reading_import_file"
	case sessionsvc.HistoryParse:
		key = "cmd.history.parsing_import_file"
	default:
		return err
	}
	return fmt.Errorf(i18n.T(key), he.Err)
}

func runHistory(cmd *cobra.Command, _ []string) error {
	teamMode, _ := cmd.Flags().GetBool("team")
	limit, _ := cmd.Flags().GetInt("limit")

	if teamMode {
		return runHistoryTeam(cmd.Context(), limit)
	}

	// Default: show local sessions
	a := MustApp()
	ctx := cmd.Context()
	sessions, err := a.Sessions.List(ctx, "")
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.history.listing_sessions"), err)
	}

	if len(sessions) == 0 {
		fmt.Fprintln(a.IO.Out, i18n.T("cmd.history.no_sessions_found"))
		return nil
	}

	w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tProject\tStatus\tStarted\tDuration\tMember")
	for i, s := range sessions {
		if limit > 0 && i >= limit {
			break
		}
		dur := "—"
		if s.EndedAt != nil {
			dur = s.EndedAt.Sub(s.StartedAt).Truncate(time.Second).String()
		}
		member := "—"
		if s.MemberID != nil {
			member = *s.MemberID
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			s.ID[:8], s.ProjectID[:8], s.Status,
			s.StartedAt.Format("2006-01-02 15:04"), dur, member)
	}
	return w.Flush()
}

func runHistoryTeam(ctx context.Context, limit int) error {
	a := MustApp()
	activeTeam := a.Config.ActiveTeam()
	if !activeTeam.Enabled {
		return errors.New(i18n.T("cmd.history.no_team_configured_use_oh_team"))
	}

	statePath := activeTeam.StatePath
	if statePath == "" {
		statePath = config.TeamStatePath(activeTeam.StateRepo)
	}
	repo := teamstate.NewRepo(activeTeam.StateRepo, statePath)
	if !repo.IsCloned() {
		return errors.New(i18n.T("cmd.history.team_state_repository_not_cloned_run"))
	}

	events, err := repo.ListEventsLimited("", limit)
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.history.listing_team_events"), err)
	}

	// Filter to session.complete events for the current member
	var filtered []teamstate.Event
	for _, e := range events {
		if e.Actor == activeTeam.MemberID && e.Type == teamstate.EventSessionComplete {
			filtered = append(filtered, e)
		}
	}

	if len(filtered) == 0 {
		fmt.Fprintln(a.IO.Out, i18n.T("cmd.history.no_team_session_history_found_for"), activeTeam.MemberID)
		return nil
	}

	w := tabwriter.NewWriter(a.IO.Out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Date\tProject\tDuration\tStatus")
	for _, e := range filtered {
		dur := "—"
		if d, ok := e.Data["duration_s"].(float64); ok {
			dur = time.Duration(int64(d) * int64(time.Second)).String()
		}
		status := "—"
		if s, ok := e.Data["status"].(string); ok {
			status = s
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			e.Timestamp.Format("2006-01-02 15:04"), e.Project, dur, status)
	}
	return w.Flush()
}

func runHistoryExport(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	output, _ := cmd.Flags().GetString("output")
	memberID, _ := cmd.Flags().GetString("member-id")
	if memberID == "" {
		memberID = activeMemberID(a)
	}
	if output == "" {
		output = fmt.Sprintf("oh-history-%s.json", time.Now().Format("20060102-150405"))
	}

	n, err := sessionsvc.ExportHistory(cmd.Context(), a.Sessions, output, sessionsvc.ExportOptions{MemberID: memberID})
	if err != nil {
		return historyError(err)
	}

	fmt.Fprint(a.IO.Out, i18n.Tf("cmd.history.exported_sessions_to", theme.SuccessStyle.Render(theme.IconSuccess), n, output))
	return nil
}

func runHistoryImport(cmd *cobra.Command, args []string) error {
	a := MustApp()
	memberIDOverride, _ := cmd.Flags().GetString("member-id")

	res, err := sessionsvc.ImportHistoryFile(cmd.Context(), a.Sessions, args[0], sessionsvc.ImportOptions{
		MemberID: memberIDOverride, DefaultMemberID: activeMemberID(a),
	})
	if err != nil {
		return historyError(err)
	}

	fmt.Fprintf(a.IO.Out, "%s Imported %d sessions (skipped %d existing)\n",
		theme.SuccessStyle.Render(theme.IconSuccess), res.Imported, res.Skipped)
	return nil
}
