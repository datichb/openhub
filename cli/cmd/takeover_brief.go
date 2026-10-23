package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/services/launch"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var takeoverBriefCmd = &cobra.Command{
	Use:     "takeover-brief",
	Aliases: []string{"tb"},
	Short:   "Gestion des briefs de reprise de ticket",
	Long:    i18n.T("cmd.takeover_brief.long"),
}

var takeoverBriefShowCmd = &cobra.Command{
	Use:   "show <ticket-id>",
	Short: "Affiche le brief de reprise d'un ticket",
	Args:  cobra.ExactArgs(1),
	RunE:  runTakeoverBriefShow,
}

var takeoverBriefListCmd = &cobra.Command{
	Use:   "list",
	Short: "Liste les briefs de reprise existants",
	RunE:  runTakeoverBriefList,
}

var takeoverBriefEnrichCmd = &cobra.Command{
	Use:   "enrich <ticket-id>",
	Short: "Enrichit un brief avec l'IA (lecture du code, analyse)",
	Long:  i18n.T("cmd.takeover_brief.enrich.long"),
	Args:  cobra.ExactArgs(1),
	// Hidden from the help; still works (U-02).
	Deprecated: i18n.T("cmd.takeover_brief.enrich.deprecated"),
	RunE:       runTakeoverBriefEnrich,
}

func init() {
	rootCmd.AddCommand(takeoverBriefCmd)
	takeoverBriefCmd.AddCommand(takeoverBriefShowCmd)
	takeoverBriefCmd.AddCommand(takeoverBriefListCmd)
	takeoverBriefCmd.AddCommand(takeoverBriefEnrichCmd)

	takeoverBriefShowCmd.Flags().StringP("project", "p", "", "Project name")
	takeoverBriefListCmd.Flags().StringP("project", "p", "", "Project name")
	takeoverBriefEnrichCmd.Flags().StringP("project", "p", "", "Project name")
}

func runTakeoverBriefShow(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	ticketID := args[0]
	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = detectCurrentProject(ctx, a)
		if project == "" {
			return errors.New(i18n.T("cmd.takeover_brief.cannot_detect_the_current_project_use"))
		}
	}

	content, err := repo.ReadBrief(project, ticketID)
	if err != nil {
		if err == teamstate.ErrBriefNotFound {
			fmt.Fprint(a.IO.Out, i18n.Tf("cmd.takeover_brief.no_brief_found_for", theme.Subtitle.Render(theme.IconInfo), project, ticketID))
			return nil
		}
		return fmt.Errorf(i18n.T("cmd.takeover_brief.reading_brief"), err)
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, content)
	return nil
}

func runTakeoverBriefList(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = detectCurrentProject(ctx, a)
		if project == "" {
			return errors.New(i18n.T("cmd.takeover_brief.cannot_detect_the_current_project_use"))
		}
	}

	metas, err := repo.ListBriefs(project)
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.takeover_brief.listing_briefs"), err)
	}

	fmt.Fprintln(a.IO.Out)
	if len(metas) == 0 {
		fmt.Fprint(a.IO.Out, i18n.Tf("cmd.takeover_brief.no_takeover_brief_for", theme.Subtitle.Render(theme.IconInfo), project))
		return nil
	}

	fmt.Fprintln(a.IO.Out, theme.Title.Render(i18n.Tf("cmd.takeover_brief.takeover_briefs", project)))
	fmt.Fprintln(a.IO.Out)

	for _, m := range metas {
		icon := theme.Subtitle.Render(theme.IconArrow)
		fmt.Fprintf(a.IO.Out, "  %s %s  %s → %s  (%s, %s)\n",
			icon,
			theme.Bold.Render(m.TicketID),
			m.TransferredFrom,
			m.TransferredTo,
			m.Reason,
			m.TransferDate.Format("02 Jan 2006"))
	}
	fmt.Fprintln(a.IO.Out)
	return nil
}

func runTakeoverBriefEnrich(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()

	repo, err := ensureTeamRepo(ctx, a)
	if err != nil {
		return err
	}

	ticketID := args[0]
	project, _ := cmd.Flags().GetString("project")
	if project == "" {
		project = detectCurrentProject(ctx, a)
		if project == "" {
			return errors.New(i18n.T("cmd.takeover_brief.cannot_detect_the_current_project_use"))
		}
	}

	// Find the project path for the headless run
	p, err := a.Projects.GetByName(ctx, project)
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.takeover_brief.project_not_found_in_the_hub"), project, err)
	}

	fmt.Fprint(a.IO.Out, i18n.Tf("cmd.takeover_brief.enriching_the_brief_with_ai", theme.Subtitle.Render(theme.IconArrow)))

	enriched, err := enrichBrief(cmd, a, p, ticketID)
	if err != nil {
		return fmt.Errorf(i18n.T("cmd.takeover_brief.enrichment_failed"), err)
	}

	// Save the enriched version
	enrichedContent := fmt.Sprintf("# Takeover Brief (enrichi): %s\n\n%s", ticketID, enriched)
	enrichedPath := filepath.Join(repo.Path(), "projects", project, "takeover-briefs")

	// Find the latest brief file to derive the enriched filename
	entries, _ := readDirSafe(enrichedPath)
	var latestBase string
	for _, e := range entries {
		name := e.Name()
		if len(name) > len(ticketID)+1 && name[:len(ticketID)] == ticketID && hasSuffix(name, ".md") && !hasSuffix(name, ".enriched.md") {
			latestBase = name[:len(name)-3] // strip .md
		}
	}

	if latestBase == "" {
		return errors.New(i18n.T("cmd.takeover_brief.cannot_find_the_base_brief_file"))
	}

	enrichedFile := filepath.Join(enrichedPath, latestBase+".enriched.md")
	if err := writeFile(enrichedFile, []byte(enrichedContent)); err != nil {
		return fmt.Errorf(i18n.T("cmd.takeover_brief.writing_the_enriched_brief"), err)
	}

	// Commit and push
	relPath := filepath.Join("projects", project, "takeover-briefs", latestBase+".enriched.md")
	if err := repo.CommitAndPush(ctx, fmt.Sprintf("takeover: enriched brief for %s/%s", project, ticketID), relPath); err != nil {
		slog.Warn("enriched brief saved locally but sync failed", "ticket", ticketID, "error", err)
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.WarningStyle.Render(theme.IconWarning),
			i18n.T("cmd.claim.sync_pending"))
	}

	fmt.Fprint(a.IO.Out, i18n.Tf("cmd.takeover_brief.enriched_brief_saved", theme.SuccessStyle.Render(theme.IconSuccess), theme.Subtitle.Render("oh takeover-brief show "+ticketID)))
	return nil
}

// helpers for the enrich command
func readDirSafe(path string) ([]dirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	result := make([]dirEntry, len(entries))
	for i, e := range entries {
		result[i] = dirEntry{name: e.Name()}
	}
	return result, nil
}

type dirEntry struct {
	name string
}

func (d dirEntry) Name() string { return d.name }

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

// enrichBrief enriches a takeover brief with the brief-enrich workflow
// (`oh run brief-enrich --headless`); the former `oh takeover-brief enrich`
// command is its alias.
func enrichBrief(cmd *cobra.Command, a *app.App, p *domain.Project, ticketID string) (string, error) {
	warnDeprecatedAlias(cmd.ErrOrStderr(), "oh takeover-brief enrich", "oh run brief-enrich --headless")
	return runBriefEnrich(cmd.Context(), a, p, ticketID, cmd.ErrOrStderr())
}

// runBriefEnrich runs the brief-enrich workflow without interface and
// returns the enriched brief (CLI and TUI); the workflow reads the brief
// itself (input `brief`, from: ticket.brief).
func runBriefEnrich(ctx context.Context, a *app.App, p *domain.Project, ticketID string, errOut io.Writer) (string, error) {
	opts := launch.Options{Workflow: "brief-enrich", Project: p, Inputs: map[string]string{"ticket": ticketID}}
	opts.Progress = launchProgress(nil, errOut)
	run, err := launch.Prepare(ctx, launchDeps(a), opts)
	if err != nil {
		return "", err
	}
	results, err := runHeadless(ctx, a, run, launcher.NewCLIUI(errOut), 30*time.Minute)
	if err != nil {
		return "", err
	}
	if len(results) == 0 {
		return "", errors.New(i18n.T("cmd.takeover_brief.brief_enrich_no_answer"))
	}
	return results[0].Text, nil
}
