package cmd

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/prompt"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// handleDevMode orchestrates the --dev workflow:
// 1. Verify bd is available
// 2. Sync tracker
// 3. Query epics and orphan tickets
// 4. Present picker
// 5. Resolve selected tickets
// 6. Build prompt for orchestrator-dev
// Returns the agent name and constructed prompt.
func handleDevMode(cmd *cobra.Command, a *app.App, project *domain.Project, launchPath string) (agentName, devPrompt string, err error) {
	if err := beads.Available(); err != nil {
		return "", "", fmt.Errorf("%s", i18n.T("cmd.start.dev_no_bd"))
	}

	labelFilter, _ := cmd.Flags().GetString("label")
	assigneeFilter, _ := cmd.Flags().GetString("assignee")
	ticketFlag, _ := cmd.Flags().GetString("ticket")

	// Pull team-state for claim awareness
	var teamRepo *teamstate.Repo
	if teamEnabledForProject(a, project) {
		tc := resolvedTeamConfig(a, project)
		statePath := tc.StatePath
		if statePath == "" {
			statePath = defaultTeamStatePath(a)
		}
		teamRepo = teamstate.NewRepo(tc.StateRepo, statePath)
		if teamRepo.IsCloned() {
			_ = teamRepo.Pull(cmd.Context())
		}
	}

	// Direct ticket mode (skip picker)
	if ticketFlag != "" {
		if teamRepo != nil && teamRepo.IsCloned() {
			autoClaimTicket(cmd.Context(), a, teamRepo, project, ticketFlag)
		}
		_ = beads.RememberGitLabContext(launchPath, ticketFlag, ticketFlag, "")
		directPrompt := fmt.Sprintf("Travaille sur le ticket %s. Utilise `bd prime` pour le contexte et `bd ready` pour les tâches disponibles.", ticketFlag)
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.SuccessStyle.Render(theme.IconArrow), i18n.T("cmd.start.dev_launching"))
		return "orchestrator-dev", directPrompt, nil
	}

	// Query tickets
	epics, err := beads.ListEpicsWithReadyChildren(launchPath)
	if err != nil {
		slog.Warn("failed to list epics", "error", err)
	}

	withLabel, withoutLabel, err := beads.OrphanTickets(launchPath, labelFilter)
	if err != nil {
		return "", "", fmt.Errorf("querying tickets: %w", err)
	}

	// Apply assignee filter
	if assigneeFilter != "" {
		readyOpts := beads.ReadyOpts{Assignee: assigneeFilter}
		filtered, err := beads.ListReady(launchPath, readyOpts)
		if err != nil {
			return "", "", fmt.Errorf("querying tickets by assignee: %w", err)
		}
		withLabel = nil
		withoutLabel = nil
		for _, t := range filtered {
			if t.Parent != "" || t.Type == "epic" {
				continue
			}
			if beads.HasLabelExported(t, "ai-delegated") {
				withLabel = append(withLabel, t)
			} else {
				withoutLabel = append(withoutLabel, t)
			}
		}
	}

	totalOptions := len(epics) + len(withLabel) + len(withoutLabel)
	if totalOptions == 0 {
		label := "ai-delegated"
		if labelFilter != "" {
			label = labelFilter
		}
		return "", "", fmt.Errorf("%s", i18n.Tf("cmd.start.dev_no_tickets", label))
	}

	// Build picker
	type pickerItem struct {
		label  string
		isEpic bool
		epicID string
		ticket beads.Ticket
	}

	var items []pickerItem
	for _, e := range epics {
		items = append(items, pickerItem{
			label:  fmt.Sprintf("[Epic] %s (%d tickets)", e.Ticket.Title, e.ReadyCount),
			isEpic: true,
			epicID: e.Ticket.ID,
			ticket: e.Ticket,
		})
	}
	for _, t := range withLabel {
		items = append(items, pickerItem{
			label:  fmt.Sprintf("[ai-delegated] %s — %s", t.ID, t.Title),
			isEpic: false,
			ticket: t,
		})
	}
	for _, t := range withoutLabel {
		items = append(items, pickerItem{
			label:  fmt.Sprintf("%s — %s", t.ID, t.Title),
			isEpic: false,
			ticket: t,
		})
	}

	options := make([]huh.Option[int], len(items))
	for i, item := range items {
		options[i] = huh.NewOption(item.label, i)
	}

	var selectedIdx int
	form := theme.NewForm(
		huh.NewGroup(
			huh.NewSelect[int]().
				Title(i18n.T("cmd.start.dev_picker_title")).
				Options(options...).
				Value(&selectedIdx),
		),
	)
	if err := form.Run(); err != nil {
		return "", "", err
	}

	selected := items[selectedIdx]

	// Resolve tickets for selected item
	var tickets []beads.Ticket
	if selected.isEpic {
		children, err := beads.ReadyChildren(launchPath, selected.epicID)
		if err != nil {
			return "", "", fmt.Errorf("querying epic children: %w", err)
		}
		tickets = children
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.SuccessStyle.Render(theme.IconSuccess),
			i18n.Tf("cmd.start.dev_selected_epic", selected.ticket.Title, len(tickets)))
	} else {
		tickets = []beads.Ticket{selected.ticket}
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.SuccessStyle.Render(theme.IconSuccess),
			i18n.Tf("cmd.start.dev_selected_ticket", selected.ticket.ID, selected.ticket.Title))
	}

	// Auto-claim selected ticket
	if teamRepo != nil && teamRepo.IsCloned() && a.Config.Team.MemberID != "" {
		claimID := selected.ticket.ID
		if selected.isEpic {
			claimID = selected.epicID
		}
		autoClaimTicket(cmd.Context(), a, teamRepo, project, claimID)
	}

	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconArrow), i18n.T("cmd.start.dev_launching"))

	devPrompt = prompt.BuildDevPrompt(tickets)
	return "orchestrator-dev", devPrompt, nil
}

// autoClaimTicket attempts to claim a ticket in team-state, printing status.
// If the ticket is already claimed by this member in "planned" status, it
// automatically transitions it to "in_progress" (the session is starting).
func autoClaimTicket(ctx context.Context, a *app.App, repo *teamstate.Repo, project *domain.Project, ticketID string) {
	existing, claimErr := repo.CreateClaim(ctx, teamstate.Claim{
		TicketID:  ticketID,
		Project:   project.ID,
		ClaimedBy: a.Config.Team.MemberID,
		Status:    teamstate.ClaimStatusInProgress,
	})
	if claimErr == teamstate.ErrClaimExists && existing != nil {
		if existing.ClaimedBy != a.Config.Team.MemberID {
			fmt.Fprintf(a.IO.Out, "  %s %s déjà pris par %s\n",
				theme.WarningStyle.Render(theme.IconWarning), ticketID, existing.ClaimedBy)
			return
		}
		// The ticket belongs to this member. If it was planned, start it now.
		if existing.Status == teamstate.ClaimStatusPlanned {
			if err := repo.UpdateClaimStatus(ctx, project.ID, ticketID, teamstate.ClaimStatusInProgress); err == nil {
				fmt.Fprintf(a.IO.Out, "  %s %s/%s: planned → in_progress\n",
					theme.SuccessStyle.Render(theme.IconSuccess), project.ID, ticketID)
			}
		}
	} else if claimErr == nil {
		fmt.Fprintf(a.IO.Out, "  %s Claim %s/%s\n",
			theme.SuccessStyle.Render(theme.IconSuccess), project.ID, ticketID)
	}
}

// defaultTeamStatePath returns the team state path from config or default.
func defaultTeamStatePath(a *app.App) string {
	if a.Config.Team.StatePath != "" {
		return a.Config.Team.StatePath
	}
	return config.DefaultTeamStatePath()
}
