package sweep

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/datichb/openhub/cli/internal/llm"
	"github.com/datichb/openhub/cli/internal/parallel"
	"github.com/datichb/openhub/cli/internal/platform"
	"github.com/datichb/openhub/cli/internal/task"
)

// RunOpts configures a full sweep run.
type RunOpts struct {
	// Goal is the high-level sweep objective (required).
	Goal string

	// SplitOpts configures the decomposition strategy.
	SplitOpts SplitOpts

	// Config holds sweep-specific settings.
	Config SweepConfig

	// ParallelConfig holds coordinator settings (MaxSessions, budget, etc.).
	ParallelConfig parallel.Config

	// ProjectPath is the root directory of the project.
	ProjectPath string

	// ProjectID is the hub project identifier.
	ProjectID string

	// HubDir is the path to the .oh directory (for log files, state).
	HubDir string

	// Provider is the LLM provider name for credential injection in parallel tasks.
	Provider string

	// Credentials holds provider-specific authentication tokens for parallel tasks.
	Credentials platform.Credentials

	// Agent is the opencode agent to use for sweep sub-tasks.
	Agent string

	// DryRun stops after decomposition and returns the planned tasks.
	DryRun bool

	// LLM is the completer used by the LLM splitting strategy.
	// Injected by the CLI entry point — today it's PlatformCompleter,
	// tomorrow it can be a direct API call.
	LLM llm.Completer

	// Platform is the session platform backend (ADR-036).
	// Required for parallel execution (to create a ParallelRunner).
	Platform platform.SessionPlatform

	// Runner is the parallel runner for task orchestration.
	// If nil, one is created from Platform.NewParallelRunner().
	Runner platform.ParallelRunner
}

// RunResult holds the outcome of a complete sweep run.
type RunResult struct {
	// Tasks is the decomposed task list.
	Tasks []task.Task

	// Collected holds per-task results (nil if DryRun).
	Collected []CollectResult

	// Merged holds the merge summary (nil if DryRun or no completed tasks).
	Merged *MergeSummary

	// Verified holds the verification outcome (nil if DryRun or verify=none).
	Verified *VerifyResult

	// DryRun indicates whether this was a dry run (decomposition only).
	DryRun bool

	// State gives access to the raw parallel state for TUI consumption.
	// Nil if DryRun.
	State *parallel.ParallelState

	// Coordinator gives access to the coordinator for TUI monitoring.
	// Nil if DryRun. The caller must call Coordinator.Cleanup() when done.
	Coordinator *parallel.Coordinator
}

// Run executes the full sweep workflow:
//
//  1. Split the goal into sub-tasks via the configured strategy
//  2. If DryRun, return the tasks for preview
//  3. Run the tasks in parallel via the coordinator
//  4. Collect results
//  5. Merge completed branches
//  6. Verify the merged result
//
// The caller is responsible for:
//   - Displaying progress (TUI) during step 3 (access via result.Coordinator)
//   - Calling result.Coordinator.Cleanup() when done
//   - Displaying the final summary
func Run(ctx context.Context, opts RunOpts) (*RunResult, error) {
	if opts.Goal == "" {
		return nil, fmt.Errorf("sweep goal is required")
	}
	if opts.Agent == "" {
		opts.Agent = "orchestrator-dev"
	}

	// --- Step 1: Split ---
	splitter := NewSplitter(opts.LLM)
	splitOpts := opts.SplitOpts
	splitOpts.Goal = opts.Goal
	splitOpts.ProjectPath = opts.ProjectPath
	if splitOpts.BranchPrefix == "" {
		splitOpts.BranchPrefix = opts.Config.BranchPrefix
	}
	if splitOpts.MaxTasks <= 0 {
		splitOpts.MaxTasks = opts.Config.MaxSplits
	}

	tasks, err := splitter.Split(ctx, splitOpts)
	if err != nil {
		return nil, fmt.Errorf("sweep decomposition failed: %w", err)
	}

	// --- Step 2: Dry run? ---
	if opts.DryRun {
		return &RunResult{
			Tasks:  tasks,
			DryRun: true,
		}, nil
	}

	// --- Step 2b: Single task fast path ---
	// When the decomposition produces exactly one task, skip the full parallel
	// infrastructure (HTTP server, coordinator, TUI monitor) and run directly
	// via RunHeadless. This is faster (~5s saved) and simpler.
	if len(tasks) == 1 {
		return runSingleTask(ctx, tasks[0], opts)
	}

	// --- Step 3: Run in parallel ---
	runner := opts.Runner
	if runner == nil {
		var err error
		runner, err = opts.Platform.NewParallelRunner(platform.ParallelRunnerOpts{
			ProjectPath: opts.ProjectPath,
			ProjectID:   opts.ProjectID,
			HubDir:      opts.HubDir,
			Provider:    opts.Provider,
			Credentials: opts.Credentials,
		})
		if err != nil {
			return nil, fmt.Errorf("creating parallel runner: %w", err)
		}
	}

	goal := opts.Goal
	coord, err := parallel.NewCoordinator(parallel.CoordinatorOpts{
		ProjectPath: opts.ProjectPath,
		ProjectID:   opts.ProjectID,
		Tasks:       tasks,
		Agent:       opts.Agent,
		BranchPattern: opts.Config.BranchPrefix + "%s",
		Config:      opts.ParallelConfig,
		StateDir:    filepath.Join(opts.HubDir, "parallel"),
		TaskPromptFunc: func(t task.Task) string {
			return BuildSweepPrompt(t, goal)
		},
	}, runner)
	if err != nil {
		return nil, fmt.Errorf("sweep coordinator init: %w", err)
	}

	// Return the coordinator in the result so the caller can:
	// - Launch the TUI monitor (coord.State(), coord.RefreshState())
	// - Cleanup when done (coord.Cleanup())
	//
	// The caller is responsible for calling coord.Run(ctx) and then
	// proceeding with collection/merge/verification.
	return &RunResult{
		Tasks:       tasks,
		DryRun:      false,
		State:       coord.State(),
		Coordinator: coord,
	}, nil
}

// CollectAndFinalize performs the post-execution phases of a sweep run:
// collection, merge, and verification. Call this after the coordinator has
// completed (all sessions in terminal state).
func CollectAndFinalize(
	ctx context.Context,
	tasks []task.Task,
	state *parallel.ParallelState,
	projectPath string,
	parallelCfg parallel.Config,
	sweepCfg SweepConfig,
) (*RunResult, error) {
	// --- Collect ---
	collected := Collect(tasks, state)

	result := &RunResult{
		Tasks:     tasks,
		Collected: collected,
	}

	// --- Merge ---
	completedCount := 0
	for _, c := range collected {
		if c.Status == string(parallel.StatusCompleted) {
			completedCount++
		}
	}

	if completedCount > 0 && parallelCfg.AutoMergeSweep {
		merged, err := MergeAll(tasks, state, projectPath, parallelCfg)
		if err != nil {
			return nil, fmt.Errorf("sweep merge failed: %w", err)
		}
		result.Merged = merged
	}

	// --- Verify ---
	verifyStrategy := VerifyStrategy(sweepCfg.VerifyStrategy)
	if verifyStrategy != VerifyNone && verifyStrategy != "" {
		verified, err := Verify(ctx, VerifyOpts{
			Strategy:    verifyStrategy,
			CustomCmd:   sweepCfg.VerifyCmd,
			ProjectPath: projectPath,
		})
		if err != nil {
			return nil, fmt.Errorf("sweep verification failed: %w", err)
		}
		result.Verified = verified
	}

	return result, nil
}
