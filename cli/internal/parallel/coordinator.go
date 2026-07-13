package parallel

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/opencode"
	"github.com/datichb/openhub/cli/internal/worktree"
)

// CoordinatorOpts holds options for launching a parallel run.
type CoordinatorOpts struct {
	ProjectPath string
	ProjectID   string
	Tickets     []string
	Priority    string   // priority ticket ID (empty = no priority)
	Agent       string   // agent to use (default: orchestrator-dev)
	Config      Config
	PromptFunc  func(ticketID string) string // generates the prompt for each ticket
}

// Coordinator orchestrates multiple parallel opencode sessions.
type Coordinator struct {
	opts       CoordinatorOpts
	state      *ParallelState
	servers    []*OpenCodeServer
	context    *SharedContext
	opencodeBin string
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
	if opts.Agent == "" {
		opts.Agent = "orchestrator-dev"
	}

	bin, err := opencode.FindBinary()
	if err != nil {
		return nil, fmt.Errorf("opencode binary not found: %w", err)
	}

	state := NewState(opts.ProjectPath, opts.Config.MaxSessions)
	return &Coordinator{
		opts:        opts,
		state:       state,
		servers:     make([]*OpenCodeServer, 0, len(opts.Tickets)),
		context:     NewSharedContext(state),
		opencodeBin: bin,
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

		branch := fmt.Sprintf("feat/%s", ticket)
		wtPath, err := worktree.ResolveOrCreate(c.opts.ProjectPath, branch)
		if err != nil {
			return fmt.Errorf("creating worktree for %s: %w", ticket, err)
		}

		port := c.opts.Config.PortRangeStart + i
		isPriority := c.opts.Priority != "" && c.opts.Priority == ticket

		c.state.AddSession(SessionInfo{
			TicketID:     ticket,
			Project:      c.opts.ProjectID,
			Branch:       branch,
			WorktreePath: wtPath,
			Port:         port,
			Status:       StatusPending,
			Priority:     isPriority,
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
		if !ok || sess.Status != StatusRunning {
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
				case "completed", "idle":
					c.state.UpdateSession(srv.TicketID, func(s *SessionInfo) {
						s.Status = StatusCompleted
						s.CompletedAt = time.Now().UTC()
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

func (c *Coordinator) cleanup() {
	for _, srv := range c.servers {
		_ = srv.Dispose()
		srv.Kill()
	}
}
