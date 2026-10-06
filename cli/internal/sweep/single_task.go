package sweep

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/headlesstrack"
	"github.com/datichb/openhub/cli/internal/parallel"
	"github.com/datichb/openhub/cli/internal/platform"
	"github.com/datichb/openhub/cli/internal/task"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// runSingleTask is the fast path for sweeps that decompose into exactly one task.
// Instead of spawning an opencode serve instance with HTTP polling and a full
// coordinator, it runs the task directly via RunHeadless. This eliminates:
//   - HTTP server startup (~5s health polling)
//   - Port allocation
//   - The coordinator's monitoring loop
//   - The TUI parallel monitor (pointless for 1 task)
//
// The result is a RunResult with a synthetic ParallelState that is compatible
// with CollectAndFinalize — the downstream merge and verification pipelines
// work unchanged.
func runSingleTask(ctx context.Context, t task.Task, opts RunOpts) (*RunResult, error) {
	slog.Info("sweep: single task detected, using headless fast path", "task", t.ID)

	// 1. Create worktree (same as coordinator.createWorktrees).
	branchPrefix := opts.Config.BranchPrefix
	if branchPrefix == "" {
		branchPrefix = "sweep/"
	}
	branchName := worktree.BranchName(branchPrefix+"%s", t.ID)
	wtPath, err := worktree.ResolveOrCreate(opts.ProjectPath, branchName)
	if err != nil {
		return nil, fmt.Errorf("creating worktree for %s: %w", t.ID, err)
	}
	if opts.Platform == nil || opts.Platform.RequiresDeploy() {
		if err := worktree.EnsureWorktreeConfig(wtPath, opts.ProjectPath); err != nil {
			slog.Warn("sweep: worktree config setup failed (non-fatal)", "error", err)
		}
	}

	// 2. Build prompt and run headless.
	prompt := BuildSweepPrompt(t, opts.Goal)
	startedAt := time.Now()

	headlessOpts := platform.HeadlessOpts{
		ProjectPath: wtPath,
		ProjectID:   opts.ProjectID,
		Agent:       opts.Agent,
		Prompt:      prompt,
		Provider:    opts.Provider,
		Credentials: opts.Credentials,
	}

	// Wrap with headless tracking if a session store is available via the platform.
	result, runErr := headlesstrack.Track(ctx, headlesstrack.Opts{
		PlatformName:  string(opts.Platform.Name()),
		ProjectID:     opts.ProjectID,
		ProjectPath:   wtPath,
		Provider:      opts.Provider,
		Label:         "sweep-single-task",
	}, func(ctx context.Context) (*platform.HeadlessResult, error) {
		return opts.Platform.RunHeadless(ctx, headlessOpts)
	})

	completedAt := time.Now()

	// 3. Determine status and collect file changes from git diff.
	status := parallel.StatusCompleted
	var errMsg string
	if runErr != nil {
		status = parallel.StatusFailed
		errMsg = runErr.Error()
	}

	filesModified, filesCreated := gitDiffFiles(wtPath)

	// 4. Build synthetic ParallelState for CollectAndFinalize compatibility.
	state := parallel.NewState("", 1)
	state.AddSession(parallel.SessionInfo{
		TicketID:      t.ID,
		Branch:        branchName,
		WorktreePath:  wtPath,
		Status:        status,
		StartedAt:     startedAt,
		CompletedAt:   completedAt,
		Error:         errMsg,
		FilesModified: filesModified,
		FilesCreated:  filesCreated,
	})

	_ = result // content is already tracked via headlesstrack

	return &RunResult{
		Tasks:       []task.Task{t},
		DryRun:      false,
		State:       state,
		Coordinator: nil, // signals caller to skip TUI monitor
	}, nil
}

// gitDiffFiles runs git diff in the worktree to detect files modified and created
// by the headless run. Returns two slices: modified files and new (untracked) files.
func gitDiffFiles(wtPath string) (modified, created []string) {
	// Modified/staged files relative to HEAD.
	out, err := exec.Command("git", "-C", wtPath, "diff", "--name-only", "HEAD").Output()
	if err == nil {
		for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if f != "" {
				modified = append(modified, f)
			}
		}
	}

	// Untracked (new) files.
	out, err = exec.Command("git", "-C", wtPath, "ls-files", "--others", "--exclude-standard").Output()
	if err == nil {
		for _, f := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if f != "" {
				created = append(created, f)
			}
		}
	}

	return modified, created
}
