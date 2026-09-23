package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/beads"
	"github.com/datichb/openhub/cli/internal/opencode"
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
	if teamEnabledForProject(a, project) {
		teamRepo, err := ensureTeamRepoForProject(ctx, a, project)
		if err == nil {
			teamCfg, err := teamRepo.LoadConfig()
			if err == nil {
				if teamCfg.Parallel.MaxSessions > 0 {
					cfg.MaxSessions = teamCfg.Parallel.MaxSessions
				}
				if teamCfg.Parallel.MaxBudgetMinutes > 0 {
					cfg.MaxBudgetMinutes = teamCfg.Parallel.MaxBudgetMinutes
				}
				if teamCfg.Parallel.DefaultTicketWeightMin > 0 {
					cfg.DefaultTicketWeightMin = teamCfg.Parallel.DefaultTicketWeightMin
				}
				if teamCfg.Parallel.PortRangeStart > 0 {
					cfg.PortRangeStart = teamCfg.Parallel.PortRangeStart
				}
				cfg.AutoMergeBeads = teamCfg.Parallel.AutoMergeBeads
				if teamCfg.Parallel.MaxRetries > 0 {
					cfg.MaxRetries = teamCfg.Parallel.MaxRetries
				}
				if teamCfg.Parallel.RetryDelaySeconds > 0 {
					cfg.RetryDelaySeconds = teamCfg.Parallel.RetryDelaySeconds
				}
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

	// Auto-deploy the main project before creating worktrees.
	// Each worktree will symlink to this deployed config instead of deploying independently.
	skipYes, _ := cmd.Flags().GetBool("yes")
	autoDeployIfNeeded(a, project, findHubDir(), "", "", skipYes)

	// Fetch ticket estimates for budget-based admission control.
	ticketEstimates := make(map[string]int, len(tickets))
	for _, tid := range tickets {
		detail, err := beads.Show(project.Path, tid)
		if err == nil && detail.Estimate > 0 {
			ticketEstimates[tid] = detail.Estimate
		}
	}

	coord, err := parallel.NewCoordinator(parallel.CoordinatorOpts{
		ProjectPath:     project.Path,
		ProjectID:       project.ID,
		Tickets:         tickets,
		TicketEstimates: ticketEstimates,
		Priority:        priority,
		Agent:           "orchestrator-dev",
		Config:          cfg,
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

		parallelCfg := views.ParallelConfig{
			Layout: layout.Config{
				ProjectName: a.Config.Name,
				Command:     "parallel",
				StatusHints: "↑↓ navigate · Enter attach · r refresh · q quit",
			},
			Sessions: toParallelSessions(coord.State()),
			RefreshFunc: func() []views.ParallelSession {
				coord.RefreshState()
				return toParallelSessions(coord.State())
			},
			RefreshRate: 5 * time.Second,
			AttachFunc: func(sessionID string) error {
				for _, s := range coord.State().Snapshot().Sessions {
					if s.SessionID == sessionID {
						return opencode.Run(opencode.StartOpts{
							ResumeSessionID: sessionID,
							ProjectPath:     s.WorktreePath,
						})
					}
				}
				return fmt.Errorf("session %s introuvable", sessionID)
			},
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

		isBeads := func(ticketID string) bool {
			return strings.HasPrefix(ticketID, "bd-") || strings.HasPrefix(ticketID, "BD-")
		}

		// Build branches list for TUI merge view
		branches := toMergeBranches(coord.State(), project.Path, isBeads)

		if len(branches) > 0 {
			mergeCfg := views.MergeViewConfig{
				Branches: branches,
				MergeFunc: func(branch views.MergeBranch) error {
					return runGitMerge(project.Path, branch.Branch, branch.TicketID)
				},
			}

			mergeView := views.NewMergeView(mergeCfg)
			shell := layout.Build(layout.Config{
				ProjectName: a.Config.Name,
				Command:     "merge",
				StatusHints: mergeView.StatusHints(),
			})

			content := tview.NewFlex().SetDirection(tview.FlexRow)
			mergeView.Mount(content, shell.App)
			shell.Content.AddItem(content, 0, 1, true)

			shell.Content.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
					shell.App.Stop()
					return nil
				}
				return mergeView.HandleKey(event)
			})

			_ = shell.App.SetRoot(shell.Root, true).EnableMouse(true).Run()

			// Print merge summary
			fmt.Fprintln(a.IO.Out)
			for _, b := range mergeCfg.Branches {
				icon := theme.SuccessStyle.Render(theme.IconSuccess)
				switch b.Status {
				case "skipped":
					icon = theme.Subtitle.Render("→")
				case "conflict":
					icon = theme.WarningStyle.Render(theme.IconWarning)
				case "pending":
					icon = theme.Subtitle.Render("·")
				}
				fmt.Fprintf(a.IO.Out, "  %s %s: %s (%s)\n", icon, b.TicketID, b.Branch, b.Status)
			}
		}
	}

	fmt.Fprintln(a.IO.Out)
	return nil
}

// toParallelSessions converts the domain ParallelState sessions to the v2 views format.
func toParallelSessions(state *parallel.ParallelState) []views.ParallelSession {
	snap := state.Snapshot()

	// Build per-session conflict map
	conflictMap := make(map[string][]views.ConflictDetail) // ticketID -> conflicts
	maxSeverityMap := make(map[string]string)
	for _, c := range snap.Conflicts {
		for _, tid := range c.Sessions {
			others := make([]string, 0, len(c.Sessions)-1)
			for _, other := range c.Sessions {
				if other != tid {
					others = append(others, other)
				}
			}
			conflictMap[tid] = append(conflictMap[tid], views.ConflictDetail{
				File:     c.File,
				Others:   others,
				Severity: c.Severity,
			})
			// Track max severity per session
			if severityRank(c.Severity) > severityRank(maxSeverityMap[tid]) {
				maxSeverityMap[tid] = c.Severity
			}
		}
	}

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

		filesList := make([]string, 0, len(s.FilesModified)+len(s.FilesCreated))
		filesList = append(filesList, s.FilesModified...)
		filesList = append(filesList, s.FilesCreated...)

		sessions = append(sessions, views.ParallelSession{
			ID:              s.SessionID,
			Name:            s.TicketID,
			Status:          string(s.Status),
			Branch:          s.Branch,
			Duration:        duration,
			Agent:           "orchestrator-dev",
			SessionID:       s.SessionID,
			WorktreePath:    s.WorktreePath,
			Priority:        s.Priority,
			EstimateMinutes: s.EstimateMinutes,
			Error:           s.Error,
			FilesModified:   len(s.FilesModified),
			FilesCreated:    len(s.FilesCreated),
			FilesList:       filesList,
			ConflictCount:   len(conflictMap[s.TicketID]),
			MaxSeverity:     maxSeverityMap[s.TicketID],
			Conflicts:       conflictMap[s.TicketID],
			RetryCount:      s.RetryCount,
			RetryErrors:     s.RetryErrors,
		})
	}
	return sessions
}

// severityRank returns a numeric rank for conflict severity comparison.
func severityRank(severity string) int {
	switch severity {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

// toMergeBranches builds a MergeBranch list from completed sessions.
func toMergeBranches(state *parallel.ParallelState, projectPath string, isBeads func(string) bool) []views.MergeBranch {
	snap := state.Snapshot()
	var branches []views.MergeBranch

	baseBranch, _ := parallel.DetectBaseBranch(projectPath)
	if baseBranch == "" {
		baseBranch = "main"
	}

	for _, sess := range snap.Sessions {
		if sess.Status != parallel.StatusCompleted {
			continue
		}
		var duration time.Duration
		if !sess.StartedAt.IsZero() && !sess.CompletedAt.IsZero() {
			duration = sess.CompletedAt.Sub(sess.StartedAt)
		}

		// Get commit count and diff stat
		commitCount := gitCommitCount(projectPath, baseBranch, sess.Branch)
		diffStat := gitDiffStat(projectPath, baseBranch, sess.Branch)

		branches = append(branches, views.MergeBranch{
			TicketID:    sess.TicketID,
			Branch:      sess.Branch,
			IsBeads:     isBeads(sess.TicketID),
			DiffStat:    diffStat,
			CommitCount: commitCount,
			Duration:    duration,
			Status:      "pending",
		})
	}
	return branches
}

// runGitMerge executes a git merge --no-ff in the terminal (runs inside app.Suspend).
func runGitMerge(projectPath, branch, ticketID string) error {
	cmd := exec.Command("git", "merge", "--no-ff", branch, "-m",
		fmt.Sprintf("merge: parallel session %s", ticketID))
	cmd.Dir = projectPath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Printf("\n  → git merge --no-ff %s\n\n", branch)
	if err := cmd.Run(); err != nil {
		// Abort on conflict
		abortCmd := exec.Command("git", "merge", "--abort")
		abortCmd.Dir = projectPath
		_ = abortCmd.Run()
		return err
	}
	fmt.Printf("\n  ✓ Merge réussi\n")
	fmt.Printf("  Appuyez sur Entrée pour continuer...")
	_, _ = fmt.Scanln()
	return nil
}

func gitCommitCount(projectPath, base, branch string) int {
	cmd := exec.Command("git", "rev-list", "--count", fmt.Sprintf("%s..%s", base, branch))
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	n := 0
	_, _ = fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n)
	return n
}

func gitDiffStat(projectPath, base, branch string) string {
	cmd := exec.Command("git", "diff", "--stat", fmt.Sprintf("%s...%s", base, branch))
	cmd.Dir = projectPath
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
