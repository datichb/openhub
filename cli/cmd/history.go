package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
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
	historyImportCmd.Flags().String("member-id", "", i18n.T("cmd.history.import.flags.member_id"))
}

// historyExportData is the JSON structure for history export.
type historyExportData struct {
	ExportedAt time.Time        `json:"exported_at"`
	MemberID   string           `json:"member_id,omitempty"`
	Sessions   []domain.Session `json:"sessions"`
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
		return fmt.Errorf("listing sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Fprintln(a.IO.Out, "No sessions found.")
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
		return fmt.Errorf("no team configured — use 'oh team init' or 'oh team rejoin'")
	}

	statePath := activeTeam.StatePath
	if statePath == "" {
		statePath = config.TeamStatePath(activeTeam.StateRepo)
	}
	repo := teamstate.NewRepo(activeTeam.StateRepo, statePath)
	if !repo.IsCloned() {
		return fmt.Errorf("team-state repository not cloned — run 'oh team rejoin --repo <url>'")
	}

	events, err := repo.ListEventsLimited("", limit)
	if err != nil {
		return fmt.Errorf("listing team events: %w", err)
	}

	// Filter to session.complete events for the current member
	var filtered []teamstate.Event
	for _, e := range events {
		if e.Actor == activeTeam.MemberID && e.Type == teamstate.EventSessionComplete {
			filtered = append(filtered, e)
		}
	}

	if len(filtered) == 0 {
		fmt.Fprintln(a.IO.Out, "No team session history found for member:", activeTeam.MemberID)
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
	ctx := cmd.Context()
	output, _ := cmd.Flags().GetString("output")

	sessions, err := a.Sessions.List(ctx, "")
	if err != nil {
		return fmt.Errorf("listing sessions: %w", err)
	}

	memberID := ""
	if activeTeam := a.Config.ActiveTeam(); activeTeam.Enabled {
		memberID = activeTeam.MemberID
	}

	data := historyExportData{
		ExportedAt: time.Now().UTC(),
		MemberID:   memberID,
		Sessions:   sessions,
	}

	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding export: %w", err)
	}

	if output == "" {
		output = fmt.Sprintf("oh-history-%s.json", time.Now().Format("20060102-150405"))
	}

	if err := os.WriteFile(output, encoded, 0o644); err != nil {
		return fmt.Errorf("writing export: %w", err)
	}

	fmt.Fprintf(a.IO.Out, "%s Exported %d sessions to %s\n",
		theme.SuccessStyle.Render(theme.IconSuccess), len(sessions), output)
	return nil
}

func runHistoryImport(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	memberIDOverride, _ := cmd.Flags().GetString("member-id")

	raw, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("reading import file: %w", err)
	}

	var data historyExportData
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("parsing import file: %w", err)
	}

	// Determine member_id: override > export data > current config
	memberID := memberIDOverride
	if memberID == "" {
		memberID = data.MemberID
	}
	if memberID == "" {
		if activeTeam := a.Config.ActiveTeam(); activeTeam.Enabled {
			memberID = activeTeam.MemberID
		}
	}

	imported := 0
	for _, s := range data.Sessions {
		if memberID != "" {
			s.MemberID = &memberID
		}
		// Try to get existing session — skip if it exists
		if _, err := a.Sessions.Get(ctx, s.ID); err == nil {
			continue // already exists
		}
		if err := a.Sessions.Create(ctx, &s); err != nil {
			continue // skip on error
		}
		imported++
	}

	fmt.Fprintf(a.IO.Out, "%s Imported %d sessions (skipped %d existing)\n",
		theme.SuccessStyle.Render(theme.IconSuccess), imported, len(data.Sessions)-imported)
	return nil
}
