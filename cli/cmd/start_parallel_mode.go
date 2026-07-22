package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/parallel"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// runParallelMode handles the --parallel flag: launches N sessions concurrently.
func runParallelMode(cmd *cobra.Command, a *app.App, ctx context.Context) error {
	tickets, _ := cmd.Flags().GetStringSlice("tickets")
	maxSessions, _ := cmd.Flags().GetInt("max-sessions")
	priority, _ := cmd.Flags().GetString("priority")
	projectFlag, _ := cmd.Flags().GetString("project")

	if len(tickets) == 0 {
		return fmt.Errorf("--parallel nécessite --tickets (ex: --tickets bd-42,bd-43,bd-44)")
	}

	project, err := resolveProject(ctx, a, projectFlag)
	if err != nil {
		return err
	}

	cfg := parallel.DefaultConfig()
	if a.Config.Team.Enabled {
		teamRepo, err := ensureTeamRepo(ctx, a)
		if err == nil {
			teamCfg, err := teamRepo.LoadConfig()
			if err == nil {
				if teamCfg.Parallel.MaxSessions > 0 {
					cfg.MaxSessions = teamCfg.Parallel.MaxSessions
				}
				if teamCfg.Parallel.PortRangeStart > 0 {
					cfg.PortRangeStart = teamCfg.Parallel.PortRangeStart
				}
				cfg.AutoMergeBeads = teamCfg.Parallel.AutoMergeBeads
			}
		}
	}

	if maxSessions > 0 {
		cfg.MaxSessions = maxSessions
	}

	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "%s Mode parallèle : %d tickets, max %d sessions\n",
		theme.Title.Render("  parallel  "),
		len(tickets), cfg.MaxSessions)
	fmt.Fprintln(a.IO.Out)

	for _, t := range tickets {
		icon := theme.IconDot
		if t == priority {
			icon = theme.IconSuccess
		}
		fmt.Fprintf(a.IO.Out, "  %s %s", icon, t)
		if t == priority {
			fmt.Fprintf(a.IO.Out, " (priority)")
		}
		fmt.Fprintln(a.IO.Out)
	}
	fmt.Fprintln(a.IO.Out)

	coord, err := parallel.NewCoordinator(parallel.CoordinatorOpts{
		ProjectPath: project.Path,
		ProjectID:   project.ID,
		Tickets:     tickets,
		Priority:    priority,
		Agent:       "orchestrator-dev",
		Config:      cfg,
		PromptFunc: func(ticketID string) string {
			return fmt.Sprintf("Travaille sur le ticket %s. Analyse, implémente et teste.", ticketID)
		},
	})
	if err != nil {
		return fmt.Errorf("initialisation parallèle: %w", err)
	}
	defer coord.Cleanup()

	fmt.Fprintf(a.IO.Out, "%s Création des worktrees et lancement des sessions...\n",
		theme.Subtitle.Render(theme.IconArrow))

	if err := coord.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintf(a.IO.Out, "\n%s Sessions annulées.\n",
				theme.WarningStyle.Render(theme.IconWarning))
			return nil
		}
		if coord.State().RunningCount() == 0 {
			return fmt.Errorf("exécution parallèle: %w", err)
		}
	}

	if coord.State().RunningCount() > 0 || coord.State().AllCompleted() {
		fmt.Fprintf(a.IO.Out, "%s Lancement du moniteur parallèle...\n\n",
			theme.Subtitle.Render(theme.IconArrow))

		// TODO: v2 parallel view does not support attach functionality yet.
		parallelCfg := views.ParallelConfig{
			Layout: layout.Config{
				ProjectName: a.Config.Name,
				Command:     "parallel",
				StatusHints: "↑↓ navigate · r refresh · q quit",
			},
			Sessions:    toParallelSessions(coord.State()),
			RefreshFunc: func() []views.ParallelSession {
				coord.RefreshState()
				return toParallelSessions(coord.State())
			},
			RefreshRate: 5 * time.Second,
		}

		if err := views.RunParallel(parallelCfg); err != nil {
			fmt.Fprintf(a.IO.Out, "%s TUI error: %v\n",
				theme.WarningStyle.Render(theme.IconWarning), err)
		}
	}

	// Print results
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, theme.Title.Render("  Résultats  "))
	fmt.Fprintln(a.IO.Out)

	snap := coord.State().Snapshot()
	for _, sess := range snap.Sessions {
		icon := theme.SuccessStyle.Render(theme.IconSuccess)
		if sess.Status == parallel.StatusFailed {
			icon = theme.ErrorStyle.Render(theme.IconError)
		}
		duration := ""
		if !sess.StartedAt.IsZero() && !sess.CompletedAt.IsZero() {
			duration = fmt.Sprintf(" (%s)", sess.CompletedAt.Sub(sess.StartedAt).Truncate(time.Second))
		}
		fmt.Fprintf(a.IO.Out, "  %s %s — %s%s\n", icon, sess.TicketID, sess.Status, duration)
		if sess.Error != "" {
			fmt.Fprintf(a.IO.Out, "    %s\n", theme.ErrorStyle.Render(sess.Error))
		}
		if len(sess.FilesModified) > 0 {
			fmt.Fprintf(a.IO.Out, "    fichiers: %d modifiés\n", len(sess.FilesModified))
		}
	}

	if len(snap.Conflicts) > 0 {
		fmt.Fprintln(a.IO.Out)
		fmt.Fprintf(a.IO.Out, "  %s %d conflit(s) potentiel(s) détecté(s):\n",
			theme.WarningStyle.Render(theme.IconWarning), len(snap.Conflicts))
		for _, c := range snap.Conflicts {
			fmt.Fprintf(a.IO.Out, "    %s — %s [%s]\n", c.File, strings.Join(c.Sessions, " ↔ "), c.Severity)
		}
	}

	// Phase merge
	completedCount := 0
	for _, sess := range snap.Sessions {
		if sess.Status == parallel.StatusCompleted {
			completedCount++
		}
	}

	if completedCount > 0 {
		fmt.Fprintln(a.IO.Out)
		fmt.Fprintln(a.IO.Out, theme.Title.Render("  Merge  "))

		merger := parallel.NewMerger(coord.State(), project.Path, cfg)
		merger.SetOutput(a.IO.Out)
		isBeads := func(ticketID string) bool {
			return strings.HasPrefix(ticketID, "bd-") || strings.HasPrefix(ticketID, "BD-")
		}

		results, err := merger.ProposeMerge(isBeads)
		if err != nil {
			fmt.Fprintf(a.IO.Out, "  %s Merge error: %v\n",
				theme.WarningStyle.Render(theme.IconWarning), err)
		}

		fmt.Fprintln(a.IO.Out)
		for _, r := range results {
			icon := theme.SuccessStyle.Render(theme.IconSuccess)
			if !r.Success {
				icon = theme.ErrorStyle.Render(theme.IconError)
			}
			if r.Conflict {
				icon = theme.WarningStyle.Render(theme.IconWarning)
			}
			fmt.Fprintf(a.IO.Out, "  %s %s: %s\n", icon, r.TicketID, r.Message)
		}
	}

	fmt.Fprintln(a.IO.Out)
	return nil
}

// toParallelSessions converts the domain ParallelState sessions to the v2 views format.
func toParallelSessions(state *parallel.ParallelState) []views.ParallelSession {
	snap := state.Snapshot()
	sessions := make([]views.ParallelSession, 0, len(snap.Sessions))
	for _, s := range snap.Sessions {
		var duration time.Duration
		if !s.StartedAt.IsZero() {
			if !s.CompletedAt.IsZero() {
				duration = s.CompletedAt.Sub(s.StartedAt)
			} else {
				duration = time.Since(s.StartedAt)
			}
		}
		sessions = append(sessions, views.ParallelSession{
			ID:       s.SessionID,
			Name:     s.TicketID,
			Status:   string(s.Status),
			Branch:   s.Branch,
			Duration: duration,
			Agent:    "orchestrator-dev",
		})
	}
	return sessions
}
