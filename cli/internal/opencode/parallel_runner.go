package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/platform"
)

// OpenCodeParallelRunner implements platform.ParallelRunner using opencode serve.
// It manages N serverAdapter instances internally, one per task.
//
// The coordinator delegates task lifecycle to this runner. The runner handles:
//   - Server creation (port allocation, binary resolution)
//   - Server startup and readiness
//   - Session creation and prompt dispatch
//   - Status polling (maps per-server status to per-task status)
//   - File change tracking
//   - Message delivery (notifications)
//   - Graceful shutdown
//
// Recovery (retry) is managed by the coordinator, which calls AbortTask +
// LaunchTask to restart a failed task. The runner allocates a new port for
// the retry.
type OpenCodeParallelRunner struct {
	mu       sync.Mutex
	servers  map[string]*serverAdapter // taskID -> server
	opts     platform.ParallelRunnerOpts
	portBase int
	portNext int
	bin      string
	agent    string
}

// Compile-time check.
var _ platform.ParallelRunner = (*OpenCodeParallelRunner)(nil)

// openCodeRunnerConfig is the backend-specific config for OpenCode parallel mode.
type openCodeRunnerConfig struct {
	PortRangeStart int `json:"port_range_start"`
}

// NewOpenCodeParallelRunner creates a runner backed by opencode serve instances.
func NewOpenCodeParallelRunner(opts platform.ParallelRunnerOpts) (*OpenCodeParallelRunner, error) {
	bin, err := FindBinary()
	if err != nil {
		return nil, fmt.Errorf("opencode binary not found: %w", err)
	}

	portBase := 4100 // default
	if opts.Config != nil {
		var cfg openCodeRunnerConfig
		if err := json.Unmarshal(opts.Config, &cfg); err == nil && cfg.PortRangeStart > 0 {
			portBase = cfg.PortRangeStart
		}
	}

	return &OpenCodeParallelRunner{
		servers:  make(map[string]*serverAdapter),
		opts:     opts,
		portBase: portBase,
		portNext: portBase,
		bin:      bin,
	}, nil
}

func (r *OpenCodeParallelRunner) LaunchTask(ctx context.Context, opts platform.TaskOpts) (platform.TaskHandle, error) {
	r.mu.Lock()
	port := r.portNext
	r.portNext++
	r.mu.Unlock()

	srv := newServerAdapter(port, opts.WorktreePath, opts.TaskID, r.bin)

	// Start the server
	if err := srv.start(ctx); err != nil {
		// Retry on next port
		r.mu.Lock()
		port = r.portNext
		r.portNext++
		r.mu.Unlock()
		srv = newServerAdapter(port, opts.WorktreePath, opts.TaskID, r.bin)
		if err := srv.start(ctx); err != nil {
			return platform.TaskHandle{}, fmt.Errorf("failed to start server for %s: %w", opts.TaskID, err)
		}
	}

	// Wait ready
	if err := srv.waitReady(ctx, 30*time.Second); err != nil {
		srv.kill()
		return platform.TaskHandle{}, fmt.Errorf("server for %s did not become ready: %w", opts.TaskID, err)
	}

	// Create session
	title := opts.Title
	if title == "" {
		title = fmt.Sprintf("parallel: %s", opts.TaskID)
	}
	sessionID, err := srv.createSession(title)
	if err != nil {
		srv.kill()
		return platform.TaskHandle{}, fmt.Errorf("failed to create session for %s: %w", opts.TaskID, err)
	}
	srv.sessionID = sessionID

	// Send prompt
	agent := opts.Agent
	if agent == "" {
		agent = r.agent
	}
	if err := srv.sendPrompt(sessionID, opts.Prompt, agent); err != nil {
		srv.kill()
		return platform.TaskHandle{}, fmt.Errorf("failed to send prompt for %s: %w", opts.TaskID, err)
	}

	// Register
	r.mu.Lock()
	r.servers[opts.TaskID] = srv
	r.mu.Unlock()

	return platform.TaskHandle{
		TaskID:    opts.TaskID,
		SessionID: sessionID,
	}, nil
}

func (r *OpenCodeParallelRunner) GetAllStatuses(ctx context.Context) (map[string]platform.TaskStatus, error) {
	r.mu.Lock()
	// Take a snapshot of servers to avoid holding the lock during HTTP calls
	snapshot := make(map[string]*serverAdapter, len(r.servers))
	for k, v := range r.servers {
		snapshot[k] = v
	}
	r.mu.Unlock()

	result := make(map[string]platform.TaskStatus, len(snapshot))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for taskID, srv := range snapshot {
		wg.Add(1)
		go func(taskID string, srv *serverAdapter) {
			defer wg.Done()

			ts := platform.TaskStatus{
				SessionID: srv.sessionID,
			}

			if !srv.isAlive() {
				ts.Status = "failed"
				ts.Error = "server process died"
				mu.Lock()
				result[taskID] = ts
				mu.Unlock()
				return
			}

			// Poll session status
			statuses, err := srv.getStatus()
			if err != nil {
				ts.Status = "running" // assume running if poll fails
				mu.Lock()
				result[taskID] = ts
				mu.Unlock()
				return
			}

			if status, ok := statuses[srv.sessionID]; ok {
				switch status {
				case "completed":
					ts.Status = "completed"
				case "idle":
					ts.Status = "idle"
				case "error", "failed":
					ts.Status = "failed"
				default:
					ts.Status = "running"
				}
			} else {
				ts.Status = "running"
			}

			// Poll modified files
			files, err := srv.getModifiedFiles()
			if err == nil {
				for _, f := range files {
					ts.FilesModified = append(ts.FilesModified, f.Path)
				}
			}

			mu.Lock()
			result[taskID] = ts
			mu.Unlock()
		}(taskID, srv)
	}
	wg.Wait()

	return result, nil
}

func (r *OpenCodeParallelRunner) GetModifiedFiles(ctx context.Context, taskID string) ([]platform.FileChange, error) {
	r.mu.Lock()
	srv, ok := r.servers[taskID]
	r.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	return srv.getModifiedFiles()
}

func (r *OpenCodeParallelRunner) SendMessage(ctx context.Context, taskID, message string) error {
	r.mu.Lock()
	srv, ok := r.servers[taskID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if srv.sessionID == "" {
		return fmt.Errorf("task %s has no active session", taskID)
	}
	return srv.sendNotification(srv.sessionID, message)
}

func (r *OpenCodeParallelRunner) AbortTask(ctx context.Context, taskID string) error {
	r.mu.Lock()
	srv, ok := r.servers[taskID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if srv.sessionID != "" {
		_ = srv.abortSession(srv.sessionID)
	}
	_ = srv.dispose()
	srv.kill()

	r.mu.Lock()
	delete(r.servers, taskID)
	r.mu.Unlock()

	return nil
}

func (r *OpenCodeParallelRunner) AttachTask(ctx context.Context, taskID string) error {
	r.mu.Lock()
	srv, ok := r.servers[taskID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if srv.sessionID == "" {
		return fmt.Errorf("task %s has no active session to attach to", taskID)
	}

	// Use the platform's RunInteractive with ResumeID to attach
	p := NewPlatform()
	_, err := p.RunInteractive(ctx, platform.RunOpts{
		ProjectPath: srv.dir(),
		ResumeID:    srv.sessionID,
	})
	return err
}

func (r *OpenCodeParallelRunner) Cleanup(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for taskID, srv := range r.servers {
		if err := srv.dispose(); err != nil {
			slog.Debug("parallel runner: dispose failed", "task", taskID, "error", err)
		}
		srv.kill()
	}
	r.servers = make(map[string]*serverAdapter)
}
