package parallel

import (
	"context"
	"fmt"
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

	// TaskPromptFunc generates the prompt for a task. Use this for new code.
	TaskPromptFunc func(t task.Task) string

	// Deprecated: PromptFunc generates the prompt from a ticket ID string.
	// Used when TaskPromptFunc is nil.
	PromptFunc func(ticketID string) string
}

// Coordinator orchestrates multiple parallel coding sessions.
type Coordinator struct {
	opts               CoordinatorOpts
	state              *ParallelState
	servers            []platform.SessionServer
	context            *SharedContext
	platform           platform.SessionPlatform
	notifiedConflicts  map[string]bool // "ticketA:ticketB:file" -> true (dedup)
	notifiedCompletion map[string]bool // ticketID -> true (dedup)
}

// NewCoordinator creates a new parallel coordinator.
func NewCoordinator(opts CoordinatorOpts, p platform.SessionPlatform) (*Coordinator, error) {
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

	if p == nil || !p.Available() {
		return nil, fmt.Errorf("platform backend not available")
	}
	if !p.SupportsServeMode() {
		return nil, fmt.Errorf("platform %s does not support serve mode (required for parallel)", p.Name())
	}

	state := NewState(opts.ProjectPath, opts.Config.MaxSessions)
	return &Coordinator{
		opts:               opts,
		state:              state,
		servers:            make([]platform.SessionServer, 0, len(opts.Tasks)),
		context:            NewSharedContext(state),
		platform:           p,
		notifiedConflicts:  make(map[string]bool),
		notifiedCompletion: make(map[string]bool),
	}, nil
}

// State returns the current state (for TUI consumption).
func (c *Coordinator) State() *ParallelState {
	return c.state
}

// Servers returns the list of servers (for TUI attach).
func (c *Coordinator) Servers() []platform.SessionServer {
	return c.servers
}

// Run executes the full parallel workflow.
// This is the main entry point — blocks until all sessions complete or ctx is cancelled.
// If useTUI is true, a BubbleTea TUI is displayed for monitoring.
func (c *Coordinator) Run(ctx context.Context) error {
	// Phase 1: Setup worktrees (sequential to avoid index.lock)
	c.state.SetPhase("setup")
	if err := c.createWorktrees(ctx); err != nil {
		return fmt.Errorf("creating worktrees: %w", err)
	}

	// Phase 2: Start servers
	if err := c.startServers(ctx); err != nil {
		c.cleanup()
		return fmt.Errorf("starting servers: %w", err)
	}

	// Phase 3: Create sessions and send prompts
	c.state.SetPhase("running")
	if err := c.startSessions(ctx); err != nil {
		c.cleanup()
		return fmt.Errorf("starting sessions: %w", err)
	}

	// Phase 4: Monitor until all complete (or context cancelled)
	if err := c.monitor(ctx); err != nil {
		c.cleanup()
		return err
	}

	// Phase 5: Done (merge is handled by caller)
	c.state.SetPhase("done")
	return nil
}

// RefreshState polls all servers and updates the shared context.
// Exposed for the TUI to call on refresh.
func (c *Coordinator) RefreshState() {
	c.pollStatus()
	c.context.UpdateFromServers(c.servers)
}

// Cleanup shuts down all servers and optionally removes worktrees.
func (c *Coordinator) Cleanup() {
	c.cleanup()
}

// --- Internal methods ---

func (c *Coordinator) createWorktrees(ctx context.Context) error {
	for i, t := range c.opts.Tasks {
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

		// Link the worktree to the main project's deployed config via relative
		// symlinks. The main project must be deployed before coord.Run() is called
		// (autoDeployIfNeeded in start_parallel_mode.go ensures this).
		if err := worktree.EnsureWorktreeConfig(wtPath, c.opts.ProjectPath); err != nil {
			return fmt.Errorf("linking config for worktree %s: %w", t.ID, err)
		}

		port := c.opts.Config.PortRangeStart + i

		c.state.AddSession(SessionInfo{
			TicketID:        t.ID,
			Project:         c.opts.ProjectID,
			Branch:          branchName,
			WorktreePath:    wtPath,
			Port:            port,
			Status:          StatusPending,
			Priority:        t.Priority,
			EstimateMinutes: t.EstimateMinutes,
		})
	}
	return nil
}

func (c *Coordinator) startServers(ctx context.Context) error {
	snap := c.state.Snapshot()
	for _, sess := range snap.Sessions {
		srv, err := c.platform.NewServer(sess.Port, sess.WorktreePath, sess.TicketID)
		if err != nil {
			c.state.UpdateSession(sess.TicketID, func(s *SessionInfo) {
				s.Status = StatusFailed
				s.Error = fmt.Sprintf("failed to create server: %v", err)
			})
			continue
		}
		c.servers = append(c.servers, srv)

		c.state.UpdateSession(sess.TicketID, func(s *SessionInfo) {
			s.Status = StatusStarting
		})

		if err := srv.Start(ctx); err != nil {
			// Try next port if busy
			retryPort := sess.Port + 10
			srv, err = c.platform.NewServer(retryPort, sess.WorktreePath, sess.TicketID)
			if err != nil {
				c.state.UpdateSession(sess.TicketID, func(s *SessionInfo) {
					s.Status = StatusFailed
					s.Error = fmt.Sprintf("failed to create retry server: %v", err)
				})
				continue
			}
			if err := srv.Start(ctx); err != nil {
				c.state.UpdateSession(sess.TicketID, func(s *SessionInfo) {
					s.Status = StatusFailed
					s.Error = fmt.Sprintf("failed to start server: %v", err)
				})
				continue
			}
			c.servers[len(c.servers)-1] = srv
			c.state.UpdateSession(sess.TicketID, func(s *SessionInfo) {
				s.Port = retryPort
			})
		}

		// Wait for server to be ready
		if err := srv.WaitReady(ctx, 30*time.Second); err != nil {
			c.state.UpdateSession(sess.TicketID, func(s *SessionInfo) {
				s.Status = StatusFailed
				s.Error = fmt.Sprintf("server did not start: %v", err)
			})
			srv.Kill()
			continue
		}
	}
	return nil
}

func (c *Coordinator) startSessions(ctx context.Context) error {
	for _, srv := range c.servers {
		sess, ok := c.state.GetSession(srv.TicketID())
		if !ok || sess.Status == StatusFailed {
			continue
		}

		// Create session
		title := fmt.Sprintf("parallel: %s", srv.TicketID())
		sessionID, err := srv.CreateSession(title)
		if err != nil {
			c.state.UpdateSession(srv.TicketID(), func(s *SessionInfo) {
				s.Status = StatusFailed
				s.Error = fmt.Sprintf("failed to create session: %v", err)
			})
			continue
		}

		c.state.UpdateSession(srv.TicketID(), func(s *SessionInfo) {
			s.SessionID = sessionID
		})

		// Generate and send prompt
		prompt := c.promptForTask(srv.TicketID())
		if err := srv.SendPrompt(sessionID, prompt, c.opts.Agent); err != nil {
			c.state.UpdateSession(srv.TicketID(), func(s *SessionInfo) {
				s.Status = StatusFailed
				s.Error = fmt.Sprintf("failed to send prompt: %v", err)
			})
			continue
		}

		c.state.UpdateSession(srv.TicketID(), func(s *SessionInfo) {
			s.Status = StatusRunning
			s.StartedAt = time.Now().UTC()
		})
	}
	return nil
}

func (c *Coordinator) monitor(ctx context.Context) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			c.pollStatus()
			c.context.UpdateFromServers(c.servers)

			// Attempt recovery of failed sessions (before notifications)
			c.attemptRecovery(ctx)

			// Send notifications for new conflicts and completions
			c.sendConflictNotifications()
			c.sendCompletionNotifications()

			// Promote idle sessions with no pending work to completed
			c.promoteIdleSessions()

			if c.state.AllCompleted() {
				return nil
			}
		}
	}
}

func (c *Coordinator) pollStatus() {
	var wg sync.WaitGroup
	for _, srv := range c.servers {
		sess, ok := c.state.GetSession(srv.TicketID())
		if !ok || (sess.Status != StatusRunning && sess.Status != StatusIdle) {
			continue
		}

		wg.Add(1)
		go func(srv platform.SessionServer) {
			defer wg.Done()

			if !srv.IsAlive() {
				c.state.UpdateSession(srv.TicketID(), func(s *SessionInfo) {
					s.Status = StatusFailed
					s.Error = "server process died"
					s.CompletedAt = time.Now().UTC()
				})
				return
			}

			statuses, err := srv.GetStatus()
			if err != nil {
				return
			}

			// Check if the session is done
			sessionInfo, _ := c.state.GetSession(srv.TicketID())
			if sessionInfo.SessionID == "" {
				return
			}

			if status, ok := statuses[sessionInfo.SessionID]; ok {
				switch status {
				case "completed":
					c.state.UpdateSession(srv.TicketID(), func(s *SessionInfo) {
						s.Status = StatusCompleted
						s.CompletedAt = time.Now().UTC()
					})
				case "idle":
					c.state.UpdateSession(srv.TicketID(), func(s *SessionInfo) {
						if s.Status == StatusRunning {
							s.Status = StatusIdle
						}
					})
				case "error", "failed":
					c.state.UpdateSession(srv.TicketID(), func(s *SessionInfo) {
						s.Status = StatusFailed
						s.CompletedAt = time.Now().UTC()
					})
				}
			}
		}(srv)
	}
	wg.Wait()
}

// sendConflictNotifications notifies running sessions about file conflicts
// detected by the SharedContext. Each conflict is notified at most once.
func (c *Coordinator) sendConflictNotifications() {
	snap := c.state.Snapshot()
	for _, conflict := range snap.Conflicts {
		if conflict.Severity == "low" {
			continue // don't spam for lock files
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

			// Find the server for this session
			for _, srv := range c.servers {
				if srv.TicketID() == ticketID && sess.SessionID != "" {
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
					_ = sendNotification(srv, sess.SessionID, msg)
					break
				}
			}
		}
	}
}

// sendCompletionNotifications notifies running sessions when another session completes.
// Each completion is notified at most once.
func (c *Coordinator) sendCompletionNotifications() {
	snap := c.state.Snapshot()
	for _, sess := range snap.Sessions {
		if sess.Status != StatusCompleted {
			continue
		}
		if c.notifiedCompletion[sess.TicketID] {
			continue
		}
		c.notifiedCompletion[sess.TicketID] = true

		// Notify all still-running/idle sessions
		for _, srv := range c.servers {
			other, ok := c.state.GetSession(srv.TicketID())
			if !ok || srv.TicketID() == sess.TicketID {
				continue
			}
			if other.Status != StatusRunning && other.Status != StatusIdle {
				continue
			}
			if other.SessionID == "" {
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
			_ = sendNotification(srv, other.SessionID, msg)
		}
	}
}

// promoteIdleSessions promotes sessions that are idle (agent finished its turn)
// to completed, unless they have just been sent a notification that hasn't been
// processed yet (give one tick of grace period).
func (c *Coordinator) promoteIdleSessions() {
	snap := c.state.Snapshot()
	for _, sess := range snap.Sessions {
		if sess.Status != StatusIdle {
			continue
		}
		// Promote to completed -- the idle state has lasted at least one monitor tick
		// which gives enough time for any pending notification to be delivered.
		c.state.UpdateSession(sess.TicketID, func(s *SessionInfo) {
			s.Status = StatusCompleted
			s.CompletedAt = time.Now().UTC()
		})
	}
}

// promptForTask generates the prompt for a task by ID, using TaskPromptFunc.
func (c *Coordinator) promptForTask(taskID string) string {
	if c.opts.TaskPromptFunc != nil {
		// Find the task by ID
		for _, t := range c.opts.Tasks {
			if t.ID == taskID {
				return c.opts.TaskPromptFunc(t)
			}
		}
		// Fallback: create a minimal task
		return c.opts.TaskPromptFunc(task.Task{ID: taskID, Kind: task.KindTicket})
	}
	return ""
}

// findServer returns the server for a given ticket ID, or nil.
func (c *Coordinator) findServer(ticketID string) platform.SessionServer {
	for _, srv := range c.servers {
		if srv.TicketID() == ticketID {
			return srv
		}
	}
	return nil
}

// replaceServer swaps the server for a ticket ID in the servers slice.
func (c *Coordinator) replaceServer(ticketID string, newSrv platform.SessionServer) {
	for i, srv := range c.servers {
		if srv.TicketID() == ticketID {
			c.servers[i] = newSrv
			return
		}
	}
	c.servers = append(c.servers, newSrv)
}

func (c *Coordinator) cleanup() {
	for _, srv := range c.servers {
		_ = srv.Dispose()
		srv.Kill()
	}

	if !c.opts.Config.CleanupCompletedWorktrees {
		return
	}

	// Remove worktrees of successfully completed sessions only.
	snap := c.state.Snapshot()
	for _, sess := range snap.Sessions {
		if sess.Status != StatusCompleted || sess.WorktreePath == "" {
			continue
		}
		_ = worktree.Remove(c.opts.ProjectPath, sess.WorktreePath, false)
	}
}

// sendNotification sends a message to a session via the server.
// It tries to use the ServerAdapter extension method if available,
// otherwise falls back to SendPrompt.
func sendNotification(srv platform.SessionServer, sessionID, message string) error {
	if adapter, ok := srv.(*ServerAdapter); ok {
		return adapter.SendNotification(sessionID, message)
	}
	// Fallback: send as a regular prompt (the notification prefix is in the message)
	return srv.SendPrompt(sessionID, message, "")
}
