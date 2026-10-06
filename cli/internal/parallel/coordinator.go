package parallel

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/platform"
	"github.com/datichb/openhub/cli/internal/task"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// CoordinatorOpts holds options for launching a parallel run.
type CoordinatorOpts struct {
	ProjectPath string
	ProjectID   string

	// Tasks is the generalized work-unit list. Use this for new code.
	Tasks []task.Task

	// Deprecated: Tickets is the legacy ticket ID list. If Tasks is nil,
	// Tickets+TicketEstimates+Priority are auto-converted to Tasks via the shim.
	Tickets         []string
	TicketEstimates map[string]int
	Priority        string

	Agent         string // agent to use (default: orchestrator-dev)
	BranchPattern string // e.g. "feat/%s"; empty = use worktree.BranchName default
	Config        Config

	// StateDir is the directory where the parallel state file is persisted
	// for cross-process visibility (e.g. the oh serve dashboard). If empty,
	// state is not persisted to disk.
	StateDir string

	// TaskPromptFunc generates the prompt for a task. Use this for new code.
	TaskPromptFunc func(t task.Task) string

	// Deprecated: PromptFunc generates the prompt from a ticket ID string.
	// Used when TaskPromptFunc is nil.
	PromptFunc func(ticketID string) string

	// SessionBundles means the sessions get their configuration from a
	// session bundle outside the project (v5): worktrees need no linked
	// .opencode/ configuration.
	SessionBundles bool
}

// Coordinator orchestrates multiple parallel coding sessions.
// It manages worktrees, conflict detection, notifications, and state tracking.
// Task lifecycle (start, status, messaging, cleanup) is delegated to a
// platform.ParallelRunner.
type Coordinator struct {
	opts               CoordinatorOpts
	state              *ParallelState
	runner             platform.ParallelRunner
	context            *SharedContext
	mu                 sync.Mutex      // protects notifiedConflicts and notifiedCompletion
	notifiedConflicts  map[string]bool // "ticketA:ticketB:file" -> true (dedup)
	notifiedCompletion map[string]bool // ticketID -> true (dedup)
}

// NewCoordinator creates a new parallel coordinator.
func NewCoordinator(opts CoordinatorOpts, runner platform.ParallelRunner) (*Coordinator, error) {
	opts.Config.Validate()

	// Backward compat shim: convert legacy Tickets to Tasks
	if len(opts.Tasks) == 0 && len(opts.Tickets) > 0 {
		opts.Tasks = task.TicketsToTasks(opts.Tickets, opts.TicketEstimates, opts.Priority)
	}
	// Backward compat shim: wrap legacy PromptFunc into TaskPromptFunc
	if opts.TaskPromptFunc == nil && opts.PromptFunc != nil {
		legacyFn := opts.PromptFunc
		opts.TaskPromptFunc = func(t task.Task) string { return legacyFn(t.ID) }
	}

	if len(opts.Tasks) == 0 {
		return nil, fmt.Errorf("no tasks specified")
	}
	if len(opts.Tasks) > opts.Config.MaxSessions {
		return nil, fmt.Errorf("too many tasks (%d) for max_sessions (%d)", len(opts.Tasks), opts.Config.MaxSessions)
	}
	if opts.Config.MaxBudgetMinutes > 0 {
		total := 0
		for _, t := range opts.Tasks {
			est := t.EstimateMinutes
			if est <= 0 {
				est = opts.Config.DefaultTicketWeightMin
			}
			total += est
		}
		if total > opts.Config.MaxBudgetMinutes {
			return nil, fmt.Errorf("total task weight (%d min) exceeds max_budget (%d min)", total, opts.Config.MaxBudgetMinutes)
		}
	}
	if opts.Agent == "" {
		opts.Agent = "orchestrator-dev"
	}

	if runner == nil {
		return nil, fmt.Errorf("parallel runner is required")
	}

	state := NewState(opts.ProjectPath, opts.Config.MaxSessions)
	return &Coordinator{
		opts:               opts,
		state:              state,
		runner:             runner,
		context:            NewSharedContext(state),
		notifiedConflicts:  make(map[string]bool),
		notifiedCompletion: make(map[string]bool),
	}, nil
}

// State returns the current state (for TUI consumption).
func (c *Coordinator) State() *ParallelState {
	return c.state
}

// AttachTask gives the user interactive access to a running task.
func (c *Coordinator) AttachTask(ctx context.Context, taskID string) error {
	return c.runner.AttachTask(ctx, taskID)
}

// Run executes the full parallel workflow.
// This is the main entry point — blocks until all sessions complete or ctx is cancelled.
func (c *Coordinator) Run(ctx context.Context) error {
	slog.Info("parallel: starting run", "tickets", len(c.opts.Tickets))

	// Phase 1: Setup worktrees (sequential to avoid index.lock)
	c.state.SetPhase("setup")
	if err := c.createWorktrees(ctx); err != nil {
		return fmt.Errorf("creating worktrees: %w", err)
	}

	// Phase 2: Launch all tasks via the runner
	c.state.SetPhase("running")
	if err := c.launchTasks(ctx); err != nil {
		c.cleanup(ctx)
		return fmt.Errorf("launching tasks: %w", err)
	}
	slog.Info("parallel: all tasks launched, entering monitor phase")

	// Phase 3: Monitor until all complete (or context cancelled)
	if err := c.monitor(ctx); err != nil {
		c.cleanup(ctx)
		return err
	}

	// Phase 4: Done (merge is handled by caller)
	c.state.SetPhase("done")
	slog.Info("parallel: run completed")
	return nil
}

// RefreshState polls all task statuses and updates the shared context.
// Exposed for the TUI to call on refresh.
func (c *Coordinator) RefreshState() {
	ctx := context.Background()
	c.updateStatuses(ctx)
}

// Cleanup shuts down all tasks and optionally removes worktrees.
func (c *Coordinator) Cleanup() {
	c.cleanup(context.Background())
}

// --- Internal methods ---

func (c *Coordinator) createWorktrees(ctx context.Context) error {
	for _, t := range c.opts.Tasks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		branchName := t.BranchName
		if branchName == "" {
			branchName = worktree.BranchName(c.opts.BranchPattern, t.ID)
		}
		wtPath, err := worktree.ResolveOrCreate(c.opts.ProjectPath, branchName)
		if err != nil {
			return fmt.Errorf("creating worktree for %s: %w", t.ID, err)
		}

		if !c.opts.SessionBundles {
			if err := worktree.EnsureWorktreeConfig(wtPath, c.opts.ProjectPath); err != nil {
				return fmt.Errorf("linking config for worktree %s: %w", t.ID, err)
			}
		}

		c.state.AddSession(SessionInfo{
			TicketID:        t.ID,
			Project:         c.opts.ProjectID,
			Branch:          branchName,
			WorktreePath:    wtPath,
			Status:          StatusPending,
			Priority:        t.Priority,
			EstimateMinutes: t.EstimateMinutes,
		})
	}
	return nil
}

func (c *Coordinator) launchTasks(ctx context.Context) error {
	snap := c.state.Snapshot()
	for _, sess := range snap.Sessions {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		c.state.UpdateSession(sess.TicketID, func(s *SessionInfo) {
			s.Status = StatusStarting
		})

		prompt := c.promptForTask(sess.TicketID)
		handle, err := c.runner.LaunchTask(ctx, platform.TaskOpts{
			TaskID:       sess.TicketID,
			Title:        fmt.Sprintf("parallel: %s", sess.TicketID),
			WorktreePath: sess.WorktreePath,
			Prompt:       prompt,
			Agent:        c.opts.Agent,
		})
		if err != nil {
			c.state.UpdateSession(sess.TicketID, func(s *SessionInfo) {
				s.Status = StatusFailed
				s.Error = fmt.Sprintf("failed to launch: %v", err)
			})
			continue
		}

		c.state.UpdateSession(sess.TicketID, func(s *SessionInfo) {
			s.SessionID = handle.SessionID
			s.Status = StatusRunning
			s.StartedAt = time.Now().UTC()
		})
	}
	return nil
}

func (c *Coordinator) monitor(ctx context.Context) error {
	// Check if the runner supports event-driven mode
	if es, ok := c.runner.(platform.EventSource); ok {
		return c.monitorEvents(ctx, es)
	}
	return c.monitorPolling(ctx)
}

func (c *Coordinator) monitorEvents(ctx context.Context, es platform.EventSource) error {
	ch, err := es.Subscribe(ctx)
	if err != nil {
		// Fallback to polling if subscription fails
		return c.monitorPolling(ctx)
	}

	// Recovery ticker: periodically check for failed sessions eligible for retry.
	// In polling mode this happens on every tick (5s). In event-driven mode we use
	// a slightly longer interval (10s) since status changes are already captured by
	// events — the ticker is a safety net for failures that don't emit events.
	recoveryTicker := time.NewTicker(10 * time.Second)
	defer recoveryTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-ch:
			if !ok {
				// Channel closed, check if we're done
				if c.state.AllCompleted() {
					return nil
				}
				// Fallback to polling
				return c.monitorPolling(ctx)
			}
			c.handleEvent(event)

			c.sendConflictNotifications(ctx)
			c.sendCompletionNotifications(ctx)
			c.promoteIdleSessions()
			c.persistState()

			if c.state.AllCompleted() {
				return nil
			}
		case <-recoveryTicker.C:
			c.attemptRecovery(ctx)
			c.persistState()
			if c.state.AllCompleted() {
				return nil
			}
		}
	}
}

func (c *Coordinator) monitorPolling(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			c.updateStatuses(ctx)
			c.attemptRecovery(ctx)
			c.sendConflictNotifications(ctx)
			c.sendCompletionNotifications(ctx)
			c.promoteIdleSessions()
			c.persistState()

			if c.state.AllCompleted() {
				return nil
			}
		}
	}
}

func (c *Coordinator) handleEvent(event platform.Event) {
	if event.TaskID == "" {
		return // global event, ignore for now
	}
	// Update state based on event type
	switch event.Type {
	case "status_change":
		// Re-poll to get the full status (events may not carry all fields)
		c.updateStatuses(context.Background())
	case "file_change":
		c.updateStatuses(context.Background())
	}
}

// persistState writes the current state to disk for cross-process visibility.
// Best-effort — errors are logged but do not affect the coordinator.
func (c *Coordinator) persistState() {
	if c.opts.StateDir == "" {
		return
	}
	if err := c.state.Save(c.opts.StateDir); err != nil {
		slog.Debug("parallel: failed to persist state", "error", err)
	}
}

func (c *Coordinator) updateStatuses(ctx context.Context) {
	statuses, err := c.runner.GetAllStatuses(ctx)
	if err != nil {
		return
	}

	for taskID, ts := range statuses {
		c.state.UpdateSession(taskID, func(s *SessionInfo) {
			switch ts.Status {
			case "completed":
				if s.Status != StatusCompleted {
					s.Status = StatusCompleted
					s.CompletedAt = time.Now().UTC()
				}
			case "idle":
				if s.Status == StatusRunning {
					s.Status = StatusIdle
				}
			case "failed":
				if s.Status != StatusFailed {
					s.Status = StatusFailed
					s.Error = ts.Error
					s.CompletedAt = time.Now().UTC()
				}
			case "running":
				// Don't overwrite a more specific status
			}
			if ts.SessionID != "" {
				s.SessionID = ts.SessionID
			}
			if len(ts.FilesModified) > 0 {
				s.FilesModified = ts.FilesModified
			}
		})
	}

	// Re-detect conflicts after file updates
	c.context.DetectConflicts()
}

// sendConflictNotifications notifies running sessions about file conflicts.
func (c *Coordinator) sendConflictNotifications(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	snap := c.state.Snapshot()
	for _, conflict := range snap.Conflicts {
		if conflict.Severity == "low" {
			continue
		}
		for _, ticketID := range conflict.Sessions {
			key := ticketID + ":" + conflict.File
			if c.notifiedConflicts[key] {
				continue
			}
			c.notifiedConflicts[key] = true

			sess, ok := c.state.GetSession(ticketID)
			if !ok || (sess.Status != StatusRunning && sess.Status != StatusIdle) {
				continue
			}

			otherSessions := make([]string, 0)
			for _, other := range conflict.Sessions {
				if other != ticketID {
					otherSessions = append(otherSessions, other)
				}
			}
			msg := fmt.Sprintf(
				"[PARALLEL-NOTIFICATION] Conflit de fichier détecté : %s est aussi modifié par %s (sévérité: %s). Minimise les changements sur ce fichier si possible.",
				conflict.File, strings.Join(otherSessions, ", "), conflict.Severity,
			)
			_ = c.runner.SendMessage(ctx, ticketID, msg)
		}
	}
}

// sendCompletionNotifications notifies running sessions when another completes.
func (c *Coordinator) sendCompletionNotifications(ctx context.Context) {
	c.mu.Lock()
	defer c.mu.Unlock()

	snap := c.state.Snapshot()
	for _, sess := range snap.Sessions {
		if sess.Status != StatusCompleted {
			continue
		}
		if c.notifiedCompletion[sess.TicketID] {
			continue
		}
		c.notifiedCompletion[sess.TicketID] = true

		for _, other := range snap.Sessions {
			if other.TicketID == sess.TicketID {
				continue
			}
			if other.Status != StatusRunning && other.Status != StatusIdle {
				continue
			}

			filesInfo := ""
			if len(sess.FilesModified) > 0 {
				filesInfo = fmt.Sprintf(" Fichiers modifiés : %s.", strings.Join(sess.FilesModified, ", "))
			}
			msg := fmt.Sprintf(
				"[PARALLEL-NOTIFICATION] La session %s est terminée.%s",
				sess.TicketID, filesInfo,
			)
			_ = c.runner.SendMessage(ctx, other.TicketID, msg)
		}
	}
}

func (c *Coordinator) promoteIdleSessions() {
	snap := c.state.Snapshot()
	for _, sess := range snap.Sessions {
		if sess.Status != StatusIdle {
			continue
		}
		c.state.UpdateSession(sess.TicketID, func(s *SessionInfo) {
			s.Status = StatusCompleted
			s.CompletedAt = time.Now().UTC()
		})
	}
}

// promptForTask generates the prompt for a task by ID.
func (c *Coordinator) promptForTask(taskID string) string {
	if c.opts.TaskPromptFunc != nil {
		for _, t := range c.opts.Tasks {
			if t.ID == taskID {
				return c.opts.TaskPromptFunc(t)
			}
		}
		return c.opts.TaskPromptFunc(task.Task{ID: taskID, Kind: task.KindTicket})
	}
	return ""
}

func (c *Coordinator) cleanup(ctx context.Context) {
	c.runner.Cleanup(ctx)

	// Remove the state file so the dashboard no longer shows an active run.
	if c.opts.StateDir != "" {
		RemoveStateFile(c.opts.StateDir)
	}

	if !c.opts.Config.CleanupCompletedWorktrees {
		return
	}

	snap := c.state.Snapshot()
	for _, sess := range snap.Sessions {
		if sess.Status != StatusCompleted || sess.WorktreePath == "" {
			continue
		}
		_ = worktree.Remove(c.opts.ProjectPath, sess.WorktreePath, false)
	}
}
