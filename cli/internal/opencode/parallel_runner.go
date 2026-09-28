package opencode

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/platform"
)

// serverProvider is the internal interface for a single serve-mode server.
// The real implementation is serverAdapter (wraps parallel.OpenCodeServer).
// Tests inject a fake implementation via serverProviderFactory.
type serverProvider interface {
	start(ctx context.Context) error
	waitReady(ctx context.Context, timeout time.Duration) error
	isAlive() bool
	dispose() error
	kill()
	port() int
	dir() string
	createSession(title string) (string, error)
	sendPrompt(sessionID, prompt, agent string) error
	sendNotification(sessionID, message string) error
	getStatus() (map[string]string, error)
	getModifiedFiles() ([]platform.FileChange, error)
	abortSession(sessionID string) error
}

// serverProviderFactory creates a serverProvider for a given port, dir, id, binary, and extra env.
// The default factory creates a real serverAdapter. Tests override this.
type serverProviderFactory func(port int, dir, id, bin string, extraEnv []string) serverProvider

// defaultServerFactory returns a factory that creates real serverAdapters
// with logging to the given hubDir.
func defaultServerFactory(hubDir string) serverProviderFactory {
	return func(port int, dir, id, bin string, extraEnv []string) serverProvider {
		return newServerAdapter(port, dir, id, bin, hubDir, extraEnv)
	}
}

// maxPortScanAttempts is the number of ports to try before giving up.
const maxPortScanAttempts = 20

// OpenCodeParallelRunner implements platform.ParallelRunner using opencode serve.
type OpenCodeParallelRunner struct {
	mu            sync.Mutex
	servers       map[string]serverProvider
	sessionIDs    map[string]string // taskID -> backend session ID
	opts          platform.ParallelRunnerOpts
	portBase      int
	portNext      int
	bin           string
	agent         string
	credentialEnv []string // pre-computed credential env vars for all tasks
	serverFactory serverProviderFactory
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

	// Pre-compute credential env vars from provider/credentials (same as buildEnv).
	var credEnv []string
	if opts.Provider != "" || opts.Credentials != (platform.Credentials{}) {
		credEnv = buildEnv(opts.Provider, Credentials(opts.Credentials))
	}

	return &OpenCodeParallelRunner{
		servers:       make(map[string]serverProvider),
		sessionIDs:    make(map[string]string),
		opts:          opts,
		portBase:      portBase,
		portNext:      portBase,
		bin:           bin,
		credentialEnv: credEnv,
		serverFactory: defaultServerFactory(opts.HubDir),
	}, nil
}

func (r *OpenCodeParallelRunner) LaunchTask(ctx context.Context, opts platform.TaskOpts) (platform.TaskHandle, error) {
	// Allocate an available port (pre-checked with net.Listen).
	port, err := r.allocatePort()
	if err != nil {
		return platform.TaskHandle{}, fmt.Errorf("port allocation for %s: %w", opts.TaskID, err)
	}

	slog.Info("parallel: launching task", "task", opts.TaskID, "port", port)

	// Build per-task env: credentials + isolated OPENCODE_DATA_HOME in the worktree.
	dataHome := filepath.Join(opts.WorktreePath, ".opencode-data")
	extraEnv := append(r.credentialEnv, "OPENCODE_DATA_HOME="+dataHome)

	srv := r.serverFactory(port, opts.WorktreePath, opts.TaskID, r.bin, extraEnv)

	// Start the server
	if err := srv.start(ctx); err != nil {
		return platform.TaskHandle{}, fmt.Errorf("failed to start server for %s on port %d: %w", opts.TaskID, port, err)
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
	r.sessionIDs[opts.TaskID] = sessionID
	r.mu.Unlock()

	return platform.TaskHandle{
		TaskID:    opts.TaskID,
		SessionID: sessionID,
	}, nil
}

// allocatePort scans ports starting from portNext, pre-checking availability
// with net.Listen. Returns the first available port or an error if the scan
// range is exhausted.
func (r *OpenCodeParallelRunner) allocatePort() (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	startPort := r.portNext
	for i := 0; i < maxPortScanAttempts; i++ {
		port := r.portNext
		r.portNext++
		if isPortAvailable(port) {
			return port, nil
		}
		slog.Debug("parallel: port unavailable, skipping", "port", port)
	}
	return 0, fmt.Errorf("no available port in range %d-%d", startPort, r.portNext-1)
}

// isPortAvailable checks if a TCP port is free on localhost by attempting
// to bind to it. The listener is closed immediately after the check.
func isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

func (r *OpenCodeParallelRunner) GetAllStatuses(ctx context.Context) (map[string]platform.TaskStatus, error) {
	r.mu.Lock()
	snapshot := make(map[string]serverProvider, len(r.servers))
	sessionSnapshot := make(map[string]string, len(r.sessionIDs))
	for k, v := range r.servers {
		snapshot[k] = v
	}
	for k, v := range r.sessionIDs {
		sessionSnapshot[k] = v
	}
	r.mu.Unlock()

	result := make(map[string]platform.TaskStatus, len(snapshot))
	var wg sync.WaitGroup
	var mu sync.Mutex

	for taskID, srv := range snapshot {
		wg.Add(1)
		go func(taskID string, srv serverProvider) {
			defer wg.Done()

			sid := sessionSnapshot[taskID]
			ts := platform.TaskStatus{
				SessionID: sid,
			}

			if !srv.isAlive() {
				slog.Warn("parallel: server process died", "task", taskID)
				ts.Status = "failed"
				ts.Error = "server process died"
				mu.Lock()
				result[taskID] = ts
				mu.Unlock()
				return
			}

			statuses, err := srv.getStatus()
			if err != nil {
				ts.Status = "running"
				mu.Lock()
				result[taskID] = ts
				mu.Unlock()
				return
			}

			if status, ok := statuses[sid]; ok {
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
	sid := r.sessionIDs[taskID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if sid == "" {
		return fmt.Errorf("task %s has no active session", taskID)
	}
	return srv.sendNotification(sid, message)
}

func (r *OpenCodeParallelRunner) AbortTask(ctx context.Context, taskID string) error {
	r.mu.Lock()
	srv, ok := r.servers[taskID]
	sid := r.sessionIDs[taskID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if sid != "" {
		_ = srv.abortSession(sid)
	}
	_ = srv.dispose()
	srv.kill()

	r.mu.Lock()
	delete(r.servers, taskID)
	delete(r.sessionIDs, taskID)
	r.mu.Unlock()

	return nil
}

func (r *OpenCodeParallelRunner) AttachTask(ctx context.Context, taskID string) error {
	r.mu.Lock()
	srv, ok := r.servers[taskID]
	sid := r.sessionIDs[taskID]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}
	if sid == "" {
		return fmt.Errorf("task %s has no active session to attach to", taskID)
	}

	p := NewPlatform()
	_, err := p.RunInteractive(ctx, platform.RunOpts{
		ProjectPath: srv.dir(),
		ResumeID:    sid,
	})
	return err
}

func (r *OpenCodeParallelRunner) Cleanup(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	slog.Info("parallel: cleaning up servers", "count", len(r.servers))
	for taskID, srv := range r.servers {
		if err := srv.dispose(); err != nil {
			slog.Debug("parallel runner: dispose failed", "task", taskID, "error", err)
		}
		srv.kill()
	}
	r.servers = make(map[string]serverProvider)
	r.sessionIDs = make(map[string]string)
}
