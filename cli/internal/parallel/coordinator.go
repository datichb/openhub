package parallel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// CoordinatorOpts holds options for launching a parallel run.
type CoordinatorOpts struct {
	ProjectPath     string
	ProjectID       string
	Tickets         []string
	TicketEstimates map[string]int // ticket ID -> estimated_minutes (0 = unknown, uses Config.DefaultTicketWeightMin)
	Priority        string         // priority ticket ID (empty = no priority)
	Agent           string         // agent to use (default: orchestrator-dev)
	BranchPattern   string         // e.g. "feat/%s"; empty = use worktree.BranchName default
	Config          Config
	PromptFunc      func(ticketID string) string // generates the prompt for each ticket
}

// Coordinator orchestrates multiple parallel opencode sessions.
type Coordinator struct {
	opts               CoordinatorOpts
	state              *ParallelState
	servers            []*OpenCodeServer
	context            *SharedContext
	opencodeBin        string
	notifiedConflicts  map[string]bool // "ticketA:ticketB:file" -> true (dedup)
	notifiedCompletion map[string]bool // ticketID -> true (dedup)
}

// NewCoordinator creates a new parallel coordinator.
func NewCoordinator(opts CoordinatorOpts) (*Coordinator, error) {
	opts.Config.Validate()

	if len(opts.Tickets) == 0 {
		return nil, fmt.Errorf("no tickets specified")
	}
	if len(opts.Tickets) > opts.Config.MaxSessions {
		return nil, fmt.Errorf("too many tickets (%d) for max_sessions (%d)", len(opts.Tickets), opts.Config.MaxSessions)
	}
	if opts.Config.MaxBudgetMinutes > 0 {
		if opts.TicketEstimates == nil {
			opts.TicketEstimates = make(map[string]int)
		}
		total := opts.Config.TotalBudget(opts.TicketEstimates, opts.Tickets)
		if total > opts.Config.MaxBudgetMinutes {
			return nil, fmt.Errorf("total ticket weight (%d min) exceeds max_budget (%d min)", total, opts.Config.MaxBudgetMinutes)
		}
	}
	if opts.Agent == "" {
		opts.Agent = "orchestrator-dev"
	}

	bin, err := opencode.FindBinary()
	if err != nil {
		return nil, fmt.Errorf("opencode binary not found: %w", err)
	}

	state := NewState(opts.ProjectPath, opts.Config.MaxSessions)
	return &Coordinator{
		opts:               opts,
		state:              state,
		servers:            make([]*OpenCodeServer, 0, len(opts.Tickets)),
		context:            NewSharedContext(state),
		opencodeBin:        bin,
		notifiedConflicts:  make(map[string]bool),
		notifiedCompletion: make(map[string]bool),
	}, nil
}

// State returns the current state (for TUI consumption).
func (c *Coordinator) State() *ParallelState {
	return c.state
}

// Servers returns the list of servers (for TUI attach).
func (c *Coordinator) Servers() []*OpenCodeServer {
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
	for i, ticket := range c.opts.Tickets {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		branch := worktree.BranchName(c.opts.BranchPattern, ticket)
		wtPath, err := worktree.ResolveOrCreate(c.opts.ProjectPath, branch)
		if err != nil {
			return fmt.Errorf("creating worktree for %s: %w", ticket, err)
		}

		// Link the worktree to the main project's deployed config via relative
		// symlinks. The main project must be deployed before coord.Run() is called
		// (autoDeployIfNeeded in start_parallel_mode.go ensures this).
		if err := worktree.EnsureWorktreeConfig(wtPath, c.opts.ProjectPath); err != nil {
			return fmt.Errorf("linking config for worktree %s: %w", ticket, err)
		}

		port := c.opts.Config.PortRangeStart + i
		isPriority := c.opts.Priority != "" && c.opts.Priority == ticket
		estimate := 0
		if c.opts.TicketEstimates != nil {
			estimate = c.opts.TicketEstimates[ticket]
		}

		c.state.AddSession(SessionInfo{
			TicketID:        ticket,
			Project:         c.opts.ProjectID,
			Branch:          branch,
			WorktreePath:    wtPath,
			Port:            port,
			Status:          StatusPending,
			Priority:        isPriority,
			EstimateMinutes: estimate,
		})
	}
	return nil
}

func (c *Coordinator) startServers(ctx context.Context) error {
	snap := c.state.Snapshot()
	for _, sess := range snap.Sessions {
		srv := NewServer(sess.Port, sess.WorktreePath, sess.TicketID)
		c.servers = append(c.servers, srv)

		c.state.UpdateSession(sess.TicketID, func(s *SessionInfo) {
			s.Status = StatusStarting
		})

		if err := srv.Start(ctx, c.opencodeBin); err != nil {
			// Try next port if busy
			retryPort := sess.Port + 10
			srv = NewServer(retryPort, sess.WorktreePath, sess.TicketID)
			if err := srv.Start(ctx, c.opencodeBin); err != nil {
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
		sess, ok := c.state.GetSession(srv.TicketID)
		if !ok || sess.Status == StatusFailed {
			continue
		}

		// Create session
		title := fmt.Sprintf("parallel: %s", srv.TicketID)
		sessionID, err := srv.CreateSession(title)
		if err != nil {
			c.state.UpdateSession(srv.TicketID, func(s *SessionInfo) {
				s.Status = StatusFailed
				s.Error = fmt.Sprintf("failed to create session: %v", err)
			})
			continue
		}

		c.state.UpdateSession(srv.TicketID, func(s *SessionInfo) {
			s.SessionID = sessionID
		})

		// Generate and send prompt
		prompt := c.opts.PromptFunc(srv.TicketID)
		if err := srv.SendPromptAsync(sessionID, prompt, c.opts.Agent); err != nil {
			c.state.UpdateSession(srv.TicketID, func(s *SessionInfo) {
				s.Status = StatusFailed
				s.Error = fmt.Sprintf("failed to send prompt: %v", err)
			})
			continue
		}

		c.state.UpdateSession(srv.TicketID, func(s *SessionInfo) {
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
		sess, ok := c.state.GetSession(srv.TicketID)
		if !ok || (sess.Status != StatusRunning && sess.Status != StatusIdle) {
			continue
		}

		wg.Add(1)
		go func(srv *OpenCodeServer) {
			defer wg.Done()

			if !srv.IsAlive() {
				c.state.UpdateSession(srv.TicketID, func(s *SessionInfo) {
					s.Status = StatusFailed
					s.Error = "server process died"
					s.CompletedAt = time.Now().UTC()
				})
				return
			}

			statuses, err := srv.GetSessionStatus()
			if err != nil {
				return
			}

			// Check if the session is done
			sessionInfo, _ := c.state.GetSession(srv.TicketID)
			if sessionInfo.SessionID == "" {
				return
			}

			if status, ok := statuses[sessionInfo.SessionID]; ok {
				switch status {
				case "completed":
					c.state.UpdateSession(srv.TicketID, func(s *SessionInfo) {
						s.Status = StatusCompleted
						s.CompletedAt = time.Now().UTC()
					})
				case "idle":
					// Idle means the agent finished its turn. If no notification
					// is pending, promote to completed. Otherwise keep as idle
					// so the monitor loop can send the notification first.
					c.state.UpdateSession(srv.TicketID, func(s *SessionInfo) {
						if s.Status == StatusRunning {
							s.Status = StatusIdle
						}
					})
				case "error", "failed":
					c.state.UpdateSession(srv.TicketID, func(s *SessionInfo) {
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
				if srv.TicketID == ticketID && sess.SessionID != "" {
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
					_ = srv.SendNotification(sess.SessionID, msg)
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
			other, ok := c.state.GetSession(srv.TicketID)
			if !ok || srv.TicketID == sess.TicketID {
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
			_ = srv.SendNotification(other.SessionID, msg)
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

// findServer returns the server for a given ticket ID, or nil.
func (c *Coordinator) findServer(ticketID string) *OpenCodeServer {
	for _, srv := range c.servers {
		if srv.TicketID == ticketID {
			return srv
		}
	}
	return nil
}

// replaceServer swaps the server for a ticket ID in the servers slice.
func (c *Coordinator) replaceServer(ticketID string, newSrv *OpenCodeServer) {
	for i, srv := range c.servers {
		if srv.TicketID == ticketID {
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
	// Failed sessions are intentionally preserved for post-mortem inspection.
	snap := c.state.Snapshot()
	for _, sess := range snap.Sessions {
		if sess.Status != StatusCompleted || sess.WorktreePath == "" {
			continue
		}
		_ = worktree.Remove(c.opts.ProjectPath, sess.WorktreePath, false)
	}
}
