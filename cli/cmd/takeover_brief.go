package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/launcher"
	"github.com/datichb/openhub/cli/internal/services/launch"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/textfmt"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var takeoverBriefCmd = &cobra.Command{
	Use:     "takeover-brief",
	Aliases: []string{"tb"},
	Long:    i18n.T("cmd.takeover_brief.long"),
}

var takeoverBriefShowCmd = &cobra.Command{
	Use:  "show <ticket-id>",
	Args: cobra.ExactArgs(1),
	RunE: runTakeoverBriefShow,
}

var takeoverBriefListCmd = &cobra.Command{
	Use:  "list",
	RunE: runTakeoverBriefList,
}

func init() {
	rootCmd.AddCommand(takeoverBriefCmd)
	takeoverBriefCmd.AddCommand(takeoverBriefShowCmd)
	takeoverBriefCmd.AddCommand(takeoverBriefListCmd)

	takeoverBriefShowCmd.Flags().StringP("project", "p", "", "")
	takeoverBriefListCmd.Flags().StringP("project", "p", "", "")
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
			textfmt.Date(m.TransferDate))
	}
	fmt.Fprintln(a.IO.Out)
	return nil
}

// runBriefEnrich runs the brief-enrich workflow without interface and
// returns the enriched brief (the TUI); the workflow reads the brief
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
