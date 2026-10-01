package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/notify"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

var teamWikiCmd = &cobra.Command{
	Use:   "wiki",
	Short: i18n.T("cmd.team.wiki.short"),
	Long:  i18n.T("cmd.team.wiki.long"),
}

var teamWikiListCmd = &cobra.Command{
	Use:   "list",
	Short: i18n.T("cmd.team.wiki.list.short"),
	RunE:  runTeamWikiList,
}

var teamWikiReadCmd = &cobra.Command{
	Use:   "read <page>",
	Short: i18n.T("cmd.team.wiki.read.short"),
	Args:  cobra.ExactArgs(1),
	RunE:  runTeamWikiRead,
}

var teamWikiReviewCmd = &cobra.Command{
	Use:   "review [id]",
	Short: i18n.T("cmd.team.wiki.review.short"),
	Args:  cobra.MaximumNArgs(1),
	RunE:  runTeamWikiReview,
}

func init() {
	teamCmd.AddCommand(teamWikiCmd)
	teamWikiCmd.AddCommand(teamWikiListCmd)
	teamWikiCmd.AddCommand(teamWikiReadCmd)
	teamWikiCmd.AddCommand(teamWikiReviewCmd)
}

// ── list ─────────────────────────────────────────────────────────────────────

func runTeamWikiList(cmd *cobra.Command, _ []string) error {
	a := MustApp()
	repo := teamRepo

	pages, err := repo.WikiListPages()
	if err != nil {
		return fmt.Errorf("listing wiki pages: %w", err)
	}

	pending, err := repo.WikiListPending()
	if err != nil {
		return fmt.Errorf("listing pending proposals: %w", err)
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, theme.Title.Render(i18n.T("cmd.team.wiki.title")))
	fmt.Fprintln(a.IO.Out)

	// Pages
	if len(pages) == 0 {
		fmt.Fprintf(a.IO.Out, "  %s %s\n",
			theme.Subtitle.Render(theme.IconInfo),
			i18n.T("cmd.team.wiki.no_pages"))
	} else {
		fmt.Fprintf(a.IO.Out, "  %s\n", theme.Bold.Render(i18n.T("cmd.team.wiki.pages_header")))
		for _, p := range pages {
			fmt.Fprintf(a.IO.Out, "    %s %s\n", theme.Subtitle.Render(theme.IconDot), p)
		}
	}

	// Pending proposals
	fmt.Fprintln(a.IO.Out)
	if len(pending) == 0 {
		fmt.Fprintf(a.IO.Out, "  %s %s\n",
			theme.Subtitle.Render(theme.IconInfo),
			i18n.T("cmd.team.wiki.no_pending"))
	} else {
		fmt.Fprintf(a.IO.Out, "  %s\n", theme.Bold.Render(
			i18n.Tf("cmd.team.wiki.pending_header", len(pending))))
		for _, pr := range pending {
			fmt.Fprintf(a.IO.Out, "    %s [%s] %s ← %s (%s, %s)\n",
				theme.Subtitle.Render(theme.IconArrow),
				theme.Bold.Render(pr.ID),
				pr.Page,
				pr.Author,
				pr.Confidence,
				pr.CreatedAt.Local().Format("2006-01-02"))
		}
	}
	fmt.Fprintln(a.IO.Out)

	return nil
}

// ── read ─────────────────────────────────────────────────────────────────────

func runTeamWikiRead(cmd *cobra.Command, args []string) error {
	a := MustApp()
	repo := teamRepo

	name := args[0]
	content, err := repo.WikiReadPage(name)
	if err != nil {
		if errors.Is(err, teamstate.ErrWikiPageNotFound) {
			fmt.Fprintf(a.IO.Out, "\n  %s %s\n\n",
				theme.WarningStyle.Render(theme.IconWarning),
				i18n.Tf("cmd.team.wiki.page_not_found", name))
			return nil
		}
		return fmt.Errorf("reading wiki page: %w", err)
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, theme.Title.Render(fmt.Sprintf("  Wiki: %s  ", name)))
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, content)

	return nil
}

// ── review ───────────────────────────────────────────────────────────────────

func runTeamWikiReview(cmd *cobra.Command, args []string) error {
	a := MustApp()
	ctx := cmd.Context()
	repo := teamRepo

	// No ID: list pending proposals only
	if len(args) == 0 {
		pending, err := repo.WikiListPending()
		if err != nil {
			return fmt.Errorf("listing pending proposals: %w", err)
		}

		fmt.Fprintln(a.IO.Out)
		if len(pending) == 0 {
			fmt.Fprintf(a.IO.Out, "  %s %s\n\n",
				theme.Subtitle.Render(theme.IconInfo),
				i18n.T("cmd.team.wiki.no_pending"))
			return nil
		}

		fmt.Fprintln(a.IO.Out, theme.Title.Render(i18n.T("cmd.team.wiki.pending_title")))
		fmt.Fprintln(a.IO.Out)
		for _, pr := range pending {
			fmt.Fprintf(a.IO.Out, "    %s [%s] %s ← %s (%s, %s)\n",
				theme.Subtitle.Render(theme.IconArrow),
				theme.Bold.Render(pr.ID),
				pr.Page,
				pr.Author,
				pr.Confidence,
				pr.CreatedAt.Local().Format("2006-01-02"))
		}
		fmt.Fprintf(a.IO.Out, "\n  %s %s\n\n",
			theme.Subtitle.Render(theme.IconInfo),
			i18n.T("cmd.team.wiki.review_hint"))
		return nil
	}

	// ID provided: interactive review
	id := args[0]
	pending, err := repo.WikiListPending()
	if err != nil {
		return fmt.Errorf("listing pending proposals: %w", err)
	}

	var proposal *teamstate.WikiProposal
	for i := range pending {
		if pending[i].ID == id {
			proposal = &pending[i]
			break
		}
	}
	if proposal == nil {
		fmt.Fprintf(a.IO.Out, "\n  %s %s\n\n",
			theme.WarningStyle.Render(theme.IconWarning),
			i18n.Tf("cmd.team.wiki.proposal_not_found", id))
		return nil
	}

	// Show proposal details
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, theme.Title.Render(i18n.Tf("cmd.team.wiki.proposal_detail_title", id)))
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "  %s %s\n", theme.Bold.Render(i18n.T("cmd.team.wiki.label_page")), proposal.Page)
	fmt.Fprintf(a.IO.Out, "  %s %s\n", theme.Bold.Render(i18n.T("cmd.team.wiki.label_author")), proposal.Author)
	fmt.Fprintf(a.IO.Out, "  %s %s\n", theme.Bold.Render(i18n.T("cmd.team.wiki.label_project")), proposal.Project)
	fmt.Fprintf(a.IO.Out, "  %s %s\n", theme.Bold.Render(i18n.T("cmd.team.wiki.label_confidence")), proposal.Confidence)
	fmt.Fprintf(a.IO.Out, "  %s %s\n", theme.Bold.Render(i18n.T("cmd.team.wiki.label_date")), proposal.CreatedAt.Local().Format("2006-01-02 15:04"))
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, theme.Subtitle.Render(i18n.T("cmd.team.wiki.content_header")))
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, proposal.Content)
	fmt.Fprintln(a.IO.Out)

	// Prompt
	fmt.Fprintf(a.IO.Out, "  %s", theme.Bold.Render(i18n.T("cmd.team.wiki.action_prompt")))
	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.TrimSpace(strings.ToLower(response))

	switch {
	case response == "a" || response == "accept":
		if err := repo.WikiAcceptProposal(ctx, id); err != nil {
			return fmt.Errorf("accepting proposal: %w", err)
		}

		// Emit event AFTER WikiAcceptProposal returns (both acquire withWriteLock)
		event := teamstate.Event{
			Actor:   a.Config.ActiveTeam().MemberID,
			Type:    teamstate.EventWikiAccepted,
			Project: proposal.Project,
			Data:    map[string]interface{}{"page": proposal.Page, "proposal_id": proposal.ID},
		}
		_ = repo.AppendEvent(ctx, event)

		// Dispatch notification
		if teamCfg, cfgErr := repo.LoadConfig(); cfgErr == nil {
			d := notify.NewDispatcher(teamCfg)
			_ = d.Dispatch(ctx, event)
		}

		fmt.Fprintf(a.IO.Out, "\n  %s %s\n\n",
			theme.SuccessStyle.Render(theme.IconSuccess),
			i18n.Tf("cmd.team.wiki.accepted", id, proposal.Page))

	case response == "r" || response == "reject":
		if err := repo.WikiRejectProposal(ctx, id); err != nil {
			return fmt.Errorf("rejecting proposal: %w", err)
		}

		// Emit event AFTER WikiRejectProposal returns (both acquire withWriteLock)
		event := teamstate.Event{
			Actor:   a.Config.ActiveTeam().MemberID,
			Type:    teamstate.EventWikiRejected,
			Project: proposal.Project,
			Data:    map[string]interface{}{"page": proposal.Page, "proposal_id": proposal.ID},
		}
		_ = repo.AppendEvent(ctx, event)

		// Dispatch notification
		if teamCfg, cfgErr := repo.LoadConfig(); cfgErr == nil {
			d := notify.NewDispatcher(teamCfg)
			_ = d.Dispatch(ctx, event)
		}

		fmt.Fprintf(a.IO.Out, "\n  %s %s\n\n",
			theme.WarningStyle.Render(theme.IconWarning),
			i18n.Tf("cmd.team.wiki.rejected", id))

	default:
		fmt.Fprintf(a.IO.Out, "\n  %s %s\n\n",
			theme.Subtitle.Render(theme.IconInfo),
			i18n.T("cmd.team.wiki.skipped"))
	}

	return nil
}
