package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/llm"
	"github.com/datichb/openhub/cli/internal/parallel"
	"github.com/datichb/openhub/cli/internal/sweep"
	"github.com/datichb/openhub/cli/internal/task"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/layout"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// runSweepMode handles the --sweep flag: decomposes a goal into sub-tasks and
// runs them in parallel. This is the CLI entry point for sweep mode.
func runSweepMode(cmd *cobra.Command, a *app.App, ctx context.Context) error {
	// --- Parse sweep flags ---
	sweepGoal, _ := cmd.Flags().GetString("sweep")
	strategyStr, _ := cmd.Flags().GetString("sweep-strategy")
	manualTasks, _ := cmd.Flags().GetStringSlice("sweep-tasks")
	includeGlobs, _ := cmd.Flags().GetStringSlice("sweep-include")
	excludeGlobs, _ := cmd.Flags().GetStringSlice("sweep-exclude")
	verifyStr, _ := cmd.Flags().GetString("sweep-verify")
	verifyCmd, _ := cmd.Flags().GetString("sweep-verify-cmd")
	dryRun, _ := cmd.Flags().GetBool("sweep-dry-run")
	branchPrefix, _ := cmd.Flags().GetString("sweep-branch-prefix")
	maxSessions, _ := cmd.Flags().GetInt("max-sessions")
	projectFlag, _ := cmd.Flags().GetString("project")

	// --- Validate --sweep-strategy ---
	if strategyStr == "" {
		return fmt.Errorf(`--sweep-strategy est obligatoire. Recommandations :
  by-file     → changements mécaniques (lint, format, renommage)
  by-package  → travail par module (tests, migration, refactoring)
  llm         → objectifs complexes/ambigus (le LLM planifie la décomposition)
  manual      → si vous connaissez la décomposition (--sweep-tasks "a,b,c")`)
	}

	strategy := sweep.Strategy(strategyStr)
	if !sweep.IsValidStrategy(strategy) {
		return fmt.Errorf("stratégie sweep inconnue : %q. Valeurs possibles : manual, by-file, by-package, llm", strategyStr)
	}

	// --- Validate --sweep-verify ---
	verifyStrategy := sweep.VerifyStrategy(verifyStr)
	if !sweep.IsValidVerifyStrategy(verifyStrategy) {
		return fmt.Errorf("stratégie de vérification inconnue : %q. Valeurs possibles : none, tests, lint, build, all, custom", verifyStr)
	}
	if verifyStrategy == sweep.VerifyCustom && verifyCmd == "" {
		return fmt.Errorf("--sweep-verify-cmd est obligatoire avec --sweep-verify=custom")
	}

	// --- Validate manual strategy ---
	if strategy == sweep.StrategyManual && len(manualTasks) == 0 {
		return fmt.Errorf("--sweep-tasks est obligatoire avec --sweep-strategy=manual")
	}

	// --- Resolve project ---
	project, err := resolveProject(ctx, a, projectFlag)
	if err != nil {
		return err
	}

	// --- Auto-deploy ---
	autoDeployIfNeeded(a, project, findHubDir(), "", "", true)

	// --- Build parallel config ---
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
				if teamCfg.Parallel.PortRangeStart > 0 {
					cfg.PortRangeStart = teamCfg.Parallel.PortRangeStart
				}
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

	// --- Build sweep config ---
	sweepCfg := sweep.DefaultSweepConfig()
	sweepCfg.BranchPrefix = branchPrefix
	sweepCfg.VerifyStrategy = string(verifyStrategy)
	sweepCfg.VerifyCmd = verifyCmd

	// --- Build LLM completer (injection point for future direct API calls) ---
	completer := llm.NewOpenCodeCompleter(project.ID)

	// --- Build run options ---
	runOpts := sweep.RunOpts{
		Goal: sweepGoal,
		SplitOpts: sweep.SplitOpts{
			Strategy:     strategy,
			Goal:         sweepGoal,
			ManualTasks:  manualTasks,
			IncludeGlobs: includeGlobs,
			ExcludeGlobs: excludeGlobs,
			MaxTasks:     sweepCfg.MaxSplits,
			ProjectPath:  project.Path,
			BranchPrefix: branchPrefix,
		},
		Config:         sweepCfg,
		ParallelConfig: cfg,
		ProjectPath:    project.Path,
		ProjectID:      project.ID,
		Agent:          "orchestrator-dev",
		DryRun:         dryRun,
		LLM:            completer,
	}

	// --- Header ---
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "%s Mode sweep : %q (stratégie: %s)\n",
		theme.Title.Render("  sweep  "),
		sweepGoal, strategyStr)
	fmt.Fprintln(a.IO.Out)

	// --- Run ---
	result, err := sweep.Run(ctx, runOpts)
	if err != nil {
		return fmt.Errorf("sweep: %w", err)
	}

	// --- Display decomposed tasks ---
	printSweepTasks(a, result.Tasks)

	// --- Dry run: stop here ---
	if result.DryRun {
		fmt.Fprintln(a.IO.Out)
		fmt.Fprintf(a.IO.Out, "%s Mode dry-run : aucune exécution. Relancez sans --sweep-dry-run pour exécuter.\n",
			theme.Subtitle.Render(theme.IconArrow))
		return nil
	}

	// --- Confirm execution ---
	if !dryRun {
		fmt.Fprintf(a.IO.Out, "\n  Lancer %d tâches en parallèle ? [Y/n] ", len(result.Tasks))
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response == "n" || response == "no" {
			fmt.Fprintf(a.IO.Out, "\n%s Sweep annulé.\n",
				theme.Subtitle.Render(theme.IconArrow))
			return nil
		}
	}

	// --- Execute parallel sessions ---
	fmt.Fprintf(a.IO.Out, "\n%s Création des worktrees et lancement des sessions...\n",
		theme.Subtitle.Render(theme.IconArrow))

	coord := result.Coordinator
	defer coord.Cleanup()

	if err := coord.Run(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintf(a.IO.Out, "\n%s Sessions annulées.\n",
				theme.WarningStyle.Render(theme.IconWarning))
			return nil
		}
		if coord.State().RunningCount() == 0 {
			return fmt.Errorf("exécution sweep: %w", err)
		}
	}

	// --- TUI parallel monitor ---
	if coord.State().RunningCount() > 0 || coord.State().AllCompleted() {
		fmt.Fprintf(a.IO.Out, "%s Lancement du moniteur...\n\n",
			theme.Subtitle.Render(theme.IconArrow))

		parallelCfg := views.ParallelConfig{
			Layout: layout.Config{
				ProjectName: a.Config.Name,
				Command:     fmt.Sprintf("sweep: %s", sweepGoal),
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
						return nil // TODO: attach to session
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

	// --- Collect and finalize ---
	finalResult, err := sweep.CollectAndFinalize(
		ctx, result.Tasks, coord.State(),
		project.Path, cfg, sweepCfg,
	)
	if err != nil {
		return fmt.Errorf("sweep finalization: %w", err)
	}

	// --- Print results ---
	printSweepResults(a, finalResult)

	// --- Merge TUI ---
	if finalResult.Merged != nil && finalResult.Merged.MergedCount == 0 {
		// Propose interactive merge via TUI
		branches := toSweepMergeBranches(coord.State(), project.Path, result.Tasks)
		if len(branches) > 0 {
			runSweepMergeView(a, branches, project.Path)
		}
	}

	// --- Verification results ---
	if finalResult.Verified != nil {
		printVerifyResults(a, finalResult.Verified)
	}

	fmt.Fprintln(a.IO.Out)
	return nil
}

// printSweepTasks displays the decomposed task table.
func printSweepTasks(a *app.App, tasks []task.Task) {
	fmt.Fprintf(a.IO.Out, "  Décomposition (%d tâches) :\n\n", len(tasks))
	fmt.Fprintf(a.IO.Out, "  %-4s %-25s %-30s\n",
		theme.Subtitle.Render("#"),
		theme.Subtitle.Render("ID"),
		theme.Subtitle.Render("Scope"))
	fmt.Fprintln(a.IO.Out)

	for i, t := range tasks {
		scope := ""
		if s, ok := t.Metadata["scope"]; ok {
			scope = s
			if len(scope) > 30 {
				scope = scope[:27] + "..."
			}
		}
		fmt.Fprintf(a.IO.Out, "  %-4d %-25s %-30s\n", i+1, t.ID, scope)
	}
}

// printSweepResults displays the post-execution summary.
func printSweepResults(a *app.App, result *sweep.RunResult) {
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, theme.Title.Render("  Résultats Sweep  "))
	fmt.Fprintln(a.IO.Out)

	if result.Collected == nil {
		return
	}

	successCount := 0
	failCount := 0
	totalFiles := 0

	for _, c := range result.Collected {
		icon := theme.SuccessStyle.Render(theme.IconSuccess)
		if c.Status != string(parallel.StatusCompleted) {
			icon = theme.ErrorStyle.Render(theme.IconError)
			failCount++
		} else {
			successCount++
		}
		files := len(c.FilesModified) + len(c.FilesCreated)
		totalFiles += files

		duration := ""
		if c.Duration > 0 {
			duration = fmt.Sprintf(" (%s)", c.Duration.Truncate(time.Second))
		}
		fmt.Fprintf(a.IO.Out, "  %s %-20s — %s%s",
			icon, c.TaskID, c.Status, duration)
		if files > 0 {
			fmt.Fprintf(a.IO.Out, " — %d fichiers", files)
		}
		fmt.Fprintln(a.IO.Out)

		if c.Error != "" {
			fmt.Fprintf(a.IO.Out, "    %s\n", theme.ErrorStyle.Render(c.Error))
		}
	}

	// Summary line
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintf(a.IO.Out, "  %d/%d réussis, %d fichiers modifiés\n",
		successCount, successCount+failCount, totalFiles)
}

// printVerifyResults displays the verification outcome.
func printVerifyResults(a *app.App, v *sweep.VerifyResult) {
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, theme.Title.Render("  Vérification  "))
	fmt.Fprintln(a.IO.Out)

	for _, step := range v.Steps {
		icon := theme.SuccessStyle.Render(theme.IconSuccess)
		if !step.Success {
			icon = theme.ErrorStyle.Render(theme.IconError)
		}
		fmt.Fprintf(a.IO.Out, "  %s %s\n", icon, step.Name)
		if !step.Success && step.Output != "" {
			// Show first 5 lines of output on failure
			lines := strings.Split(step.Output, "\n")
			limit := 5
			if len(lines) < limit {
				limit = len(lines)
			}
			for _, line := range lines[:limit] {
				fmt.Fprintf(a.IO.Out, "    %s\n", line)
			}
			if len(lines) > limit {
				fmt.Fprintf(a.IO.Out, "    ... (%d lignes supplémentaires)\n", len(lines)-limit)
			}
		}
	}

	overallIcon := theme.SuccessStyle.Render(theme.IconSuccess)
	if !v.Success {
		overallIcon = theme.ErrorStyle.Render(theme.IconError)
	}
	fmt.Fprintf(a.IO.Out, "\n  %s Vérification %s : %s\n",
		overallIcon, v.Strategy, boolToStatus(v.Success))
}

func boolToStatus(b bool) string {
	if b {
		return "OK"
	}
	return "FAILED"
}

// toSweepMergeBranches builds a MergeBranch list for sweep tasks.
func toSweepMergeBranches(state *parallel.ParallelState, projectPath string, tasks []task.Task) []views.MergeBranch {
	// Build task lookup
	taskMap := make(map[string]bool, len(tasks))
	for _, t := range tasks {
		taskMap[t.ID] = true
	}

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

		commitCount := gitCommitCount(projectPath, baseBranch, sess.Branch)
		diffStat := gitDiffStat(projectPath, baseBranch, sess.Branch)

		branches = append(branches, views.MergeBranch{
			TicketID:    sess.TicketID,
			Branch:      sess.Branch,
			IsMergeable: taskMap[sess.TicketID], // sweep tasks are mergeable
			DiffStat:    diffStat,
			CommitCount: commitCount,
			Duration:    duration,
			Status:      "pending",
		})
	}
	return branches
}

// runSweepMergeView launches the TUI merge view for sweep branches.
func runSweepMergeView(a *app.App, branches []views.MergeBranch, projectPath string) {
	fmt.Fprintln(a.IO.Out)
	fmt.Fprintln(a.IO.Out, theme.Title.Render("  Merge  "))

	mergeCfg := views.MergeViewConfig{
		Branches: branches,
		MergeFunc: func(branch views.MergeBranch) error {
			return runGitMerge(projectPath, branch.Branch, branch.TicketID)
		},
	}

	mergeView := views.NewMergeView(mergeCfg)
	shell := layout.Build(layout.Config{
		ProjectName: a.Config.Name,
		Command:     "sweep merge",
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
}
