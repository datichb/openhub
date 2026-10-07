package cmd

import (
	"context"
	"errors"
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
// devSelection is the outcome of the dev mode picker.
type devSelection struct {
	Agent   string
	Prompt  string
	Tickets []string // selected ticket, or the children of the selected epic
	Epic    string   // selected epic ("" for a single ticket)
}

func handleDevMode(cmd *cobra.Command, a *app.App, project *domain.Project, launchPath string) (devSelection, error) {
	if err := beads.Available(); err != nil {
		return devSelection{}, fmt.Errorf("%s", i18n.T("cmd.start.dev_no_bd"))
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
		_ = beads.RememberGitLabContext(launchPath, ticketFlag, ticketFlag, "")
		directPrompt := fmt.Sprintf("Travaille sur le ticket %s. Utilise `bd prime` pour le contexte et `bd ready` pour les tâches disponibles.", ticketFlag)
		fmt.Fprintf(a.IO.Out, "%s %s\n",
			theme.SuccessStyle.Render(theme.IconArrow), i18n.T("cmd.start.dev_launching"))
		return devSelection{Agent: "orchestrator-dev", Prompt: directPrompt, Tickets: []string{ticketFlag}}, nil
	}

	// Query tickets — include both ready (todo) and in-progress (resumable)
	epics, err := beads.ListEpicsWithDevPickableChildren(launchPath)
	if err != nil {
		slog.Warn("failed to list epics", "error", err)
	}

	withLabel, withoutLabel, err := beads.DevPickableOrphanTickets(launchPath, labelFilter)
	if err != nil {
		return devSelection{}, fmt.Errorf("querying tickets: %w", err)
	}

	// Apply assignee filter
	if assigneeFilter != "" {
		readyOpts := beads.ReadyOpts{Assignee: assigneeFilter}
		filtered, err := beads.ListReady(launchPath, readyOpts)
		if err != nil {
			return devSelection{}, fmt.Errorf("querying tickets by assignee: %w", err)
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
		return devSelection{}, fmt.Errorf("aucun ticket disponible (todo ou en cours) avec le label %q", label)
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
			label:  fmt.Sprintf("[ai-delegated]%s %s — %s", devStatusTag(t.Status), t.ID, t.Title),
			isEpic: false,
			ticket: t,
		})
	}
	for _, t := range withoutLabel {
		items = append(items, pickerItem{
			label:  fmt.Sprintf("%s%s — %s", t.ID, devStatusTag(t.Status), t.Title),
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
		return devSelection{}, err
	}

	selected := items[selectedIdx]

	// Resolve tickets for selected item
	var tickets []beads.Ticket
	if selected.isEpic {
		children, err := beads.DevPickableChildren(launchPath, selected.epicID)
		if err != nil {
			return devSelection{}, fmt.Errorf("querying epic children: %w", err)
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

	// The team claim is taken when the session starts (startTicketClaims).
	fmt.Fprintf(a.IO.Out, "%s %s\n",
		theme.SuccessStyle.Render(theme.IconArrow), i18n.T("cmd.start.dev_launching"))

	sel := devSelection{Agent: "orchestrator-dev", Prompt: prompt.BuildDevPrompt(tickets)}
	for _, t := range tickets {
		sel.Tickets = append(sel.Tickets, t.ID)
	}
	if selected.isEpic {
		sel.Epic = selected.epicID
	}
	return sel, nil
}

// devStatusTag returns a visual tag for the ticket status in the dev picker.
// Ready tickets get no tag (default), in-progress tickets get " ▸EN COURS".
func devStatusTag(status string) string {
	if beads.IsInProgressStatus(status) {
		return " ▸EN COURS"
	}
	return ""
}

// startTicketClaims returns the hook run when a session starts on Beads
// tickets (any workflow, QB2; formerly the --dev alias only): each ticket is
// claimed in the team-state for the active member, or its claim moves from
// the initial / planned status to the work status. Nil without a team.
func startTicketClaims(a *app.App, project *domain.Project) func(ctx context.Context, projectID string, tickets []string) []string {
	if project == nil || !teamEnabledForProject(a, project) || a.Config.ActiveTeam().MemberID == "" {
		return nil
	}
	tc := resolvedTeamConfig(a, project)
	statePath := tc.StatePath
	if statePath == "" {
		statePath = defaultTeamStatePath(a)
	}
	repo := teamstate.NewRepo(tc.StateRepo, statePath)
	member := a.Config.ActiveTeam().MemberID
	return func(ctx context.Context, projectID string, tickets []string) []string {
		if !repo.IsCloned() {
			return nil
		}
		var notes []string
		for _, t := range tickets {
			if n := startTicketClaim(ctx, repo, projectID, member, t); n != "" {
				notes = append(notes, n)
			}
		}
		return notes
	}
}

// startTicketClaim claims a ticket for member, or moves its claim from the
// initial / planned status to the work status; it returns a note ("" when
// nothing changed).
func startTicketClaim(ctx context.Context, repo *teamstate.Repo, projectID, member, ticketID string) string {
	var boardCfg teamstate.BoardConfig
	if cfg, err := repo.LoadConfig(); err == nil && cfg != nil {
		boardCfg = cfg.Board
	}
	workStatus := boardCfg.DefaultWorkStatus()
	existing, err := repo.CreateClaim(ctx, teamstate.Claim{TicketID: ticketID, Project: projectID, ClaimedBy: member, Status: workStatus})
	switch {
	case err == nil:
		return i18n.Tf("cmd.run.claim.taken", projectID, ticketID)
	case errors.Is(err, teamstate.ErrClaimExists) && existing != nil:
		if existing.ClaimedBy != member {
			return i18n.Tf("cmd.run.claim.other", ticketID, existing.ClaimedBy)
		}
		if existing.Status == boardCfg.InitialStatus() || existing.Status == teamstate.ClaimStatusPlanned {
			if err := repo.UpdateClaimStatus(ctx, projectID, ticketID, workStatus); err == nil {
				return i18n.Tf("cmd.run.claim.started", projectID, ticketID, existing.Status, workStatus)
			}
		}
	}
	return ""
}

// defaultTeamStatePath returns the team state path from config or default.
func defaultTeamStatePath(a *app.App) string {
	if a.Config.ActiveTeam().StatePath != "" {
		return a.Config.ActiveTeam().StatePath
	}
	return config.DefaultTeamStatePath()
}
