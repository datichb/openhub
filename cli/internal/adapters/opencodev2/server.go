package opencodev2

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/datichb/openhub/cli/internal/envfilter"
	"github.com/datichb/openhub/cli/internal/logrotate"
	ohruntime "github.com/datichb/openhub/cli/internal/runtime"
	"github.com/datichb/openhub/cli/internal/safego"
)

// ServerOptions configures a dedicated `opencode serve` process.
type ServerOptions struct {
	Binary        string            // opencode binary (default: looked up in PATH)
	WorkDir       string            // process working directory (required)
	DataDir       string            // isolated tool data directory → XDG_DATA_HOME (required)
	ConfigHome    string            // optional isolated config home → XDG_CONFIG_HOME (strict isolation)
	ConfigContent string            // inline config → OPENCODE_CONFIG_CONTENT
	Env           map[string]string // extra environment (e.g. provider session token)
	EnvAllow      []string          // variables of the user's environment inherited besides the base list (names or PREFIX_*)
	Port          int               // 0 = pick a free port
	Password      string            // "" = random
	LogPath       string            // default: <DataDir>/oh-server.log
	LogMaxBytes   int64             // log rotated above it (default logrotate.DefaultMaxBytes)
	LogKeep       int               // rotated copies kept (default logrotate.DefaultKeep)
	ReadyDir      string            // location probed for readiness (default: WorkDir)
	ReadyAgent    string            // agent that must be listed before the server is considered ready
	ReadyTimeout  time.Duration     // default: 30s (60s with Run)

	// Run, when set, runs the server in another environment (container).
	// It returns the machine command running argv with env in dir, all
	// expressed in the environment's paths (translated with Paths); the
	// server listens on every interface inside and port is published on the
	// machine loopback. The machine environment is never inherited.
	Run   func(argv []string, env map[string]string, dir string, port int) (*exec.Cmd, error)
	Paths ohruntime.PathMap
}

// Server is a running (or re-attached) opencode server.
type Server struct {
	URL      string
	Port     int
	Password string
	PID      int
	Started  time.Time
	Client   *Client

	exited  chan struct{}
	exitErr error
	mu      sync.Mutex
}

// buildEnv returns the server environment: the part of the parent
// environment that internal/envfilter lets through (an allow-list: a short
// base list, plus ServerOptions.EnvAllow; the credentials oh manages never
// pass), without any OPENCODE_* / tool-isolation variable, plus oh's
// settings. Credentials reach the tool only through the session token of the
// oh credential proxy.
func buildEnv(parent []string, opts ServerOptions, password string) []string {
	env := make([]string, 0, len(parent)+8)
	allowed, _ := envfilter.Split(parent, opts.EnvAllow)
	for _, kv := range allowed {
		k, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(k, "OPENCODE_") || k == "XDG_DATA_HOME" {
			continue
		}
		if opts.ConfigHome != "" && k == "XDG_CONFIG_HOME" {
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"OPENCODE_SERVER_PASSWORD="+password,
		"OPENCODE_DISABLE_PROJECT_CONFIG=1",
		"OPENCODE_DISABLE_AUTOUPDATE=true",
		"XDG_DATA_HOME="+opts.DataDir,
	)
	if opts.ConfigHome != "" {
		env = append(env, "XDG_CONFIG_HOME="+opts.ConfigHome)
	}
	if opts.ConfigContent != "" {
		env = append(env, "OPENCODE_CONFIG_CONTENT="+opts.ConfigContent)
	}
	for k, v := range opts.Env {
		env = append(env, k+"="+v)
	}
	return env
}

// runtimeEnv is the complete server environment in another runtime: only
// oh's settings (no machine variable is inherited).
func runtimeEnv(opts ServerOptions, password, dataDir string) map[string]string {
	env := map[string]string{
		"OPENCODE_SERVER_PASSWORD":        password,
		"OPENCODE_DISABLE_PROJECT_CONFIG": "1",
		"OPENCODE_DISABLE_AUTOUPDATE":     "true",
		"XDG_DATA_HOME":                   dataDir,
	}
	if opts.ConfigContent != "" {
		// Container env files hold one variable per line.
		var buf bytes.Buffer
		if err := json.Compact(&buf, []byte(opts.ConfigContent)); err == nil {
			env["OPENCODE_CONFIG_CONTENT"] = buf.String()
		} else {
			env["OPENCODE_CONFIG_CONTENT"] = opts.ConfigContent
		}
	}
	for k, v := range opts.Env {
		env[k] = v
	}
	return env
}

// innerPaths translates machine paths to the runtime's view.
func innerPaths(m ohruntime.PathMap, paths ...string) ([]string, error) {
	out := make([]string, len(paths))
	for i, p := range paths {
		in, ok := m.ToInner(p)
		if !ok {
			return nil, fmt.Errorf("%s is not visible in the server runtime", p)
		}
		out[i] = in
	}
	return out, nil
}

// RandomPassword returns a 32-byte hex password.
func RandomPassword() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("opencodev2: crypto/rand failed: %v", err))
	}
	return hex.EncodeToString(b)
}

// FreePort returns a free TCP port on 127.0.0.1. The port is free when it
// returns: another process may take it before the server binds it
// (StartServer then tries another one).
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// StartServer launches a detached `opencode serve` and waits until it lists
// agents for ReadyDir (agents and skills load asynchronously after startup).
// The process is placed in its own process group so it survives the oh
// process and terminal closing; Stop terminates the whole group.
func StartServer(ctx context.Context, opts ServerOptions) (*Server, error) {
	if opts.WorkDir == "" || opts.DataDir == "" {
		return nil, errors.New("opencodev2: WorkDir and DataDir are required")
	}
	bin := opts.Binary
	if bin == "" {
		var err error
		if bin, err = exec.LookPath("opencode"); err != nil {
			return nil, fmt.Errorf("opencode binary not found: %w", err)
		}
	}
	if err := os.MkdirAll(opts.DataDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating data dir: %w", err)
	}
	if opts.Port != 0 {
		return startServer(ctx, opts, bin, opts.Port)
	}
	for attempt := 1; ; attempt++ {
		port, err := FreePort()
		if err != nil {
			return nil, fmt.Errorf("allocating port: %w", err)
		}
		s, err := startServer(ctx, opts, bin, port)
		if !errors.Is(err, errPortTaken) || attempt == portAttempts {
			return s, err
		}
		slog.Warn("opencode serve: port taken before the server bound it, trying another one", "port", port)
	}
}

// portAttempts bounds the ports StartServer tries when it picks them.
const portAttempts = 3

// errPortTaken: the server exited during startup, its port being in use.
var errPortTaken = errors.New("port already in use")

// portTaken reports whether a startup log says the port was in use (Bun:
// EADDRINUSE, « Is port … in use? »; Docker: « port is already allocated »).
func portTaken(log string) bool {
	l := strings.ToLower(log)
	return strings.Contains(l, "eaddrinuse") || strings.Contains(l, "address already in use") ||
		(strings.Contains(l, "port") && strings.Contains(l, "in use")) || strings.Contains(l, "port is already allocated")
}

// startServer starts the server on port.
func startServer(ctx context.Context, opts ServerOptions, bin string, port int) (*Server, error) {
	password := opts.Password
	if password == "" {
		password = RandomPassword()
	}
	logPath := opts.LogPath
	if logPath == "" {
		logPath = filepath.Join(opts.DataDir, "oh-server.log")
	}
	// The previous server of this data directory is gone: its log is
	// rotated before the new one opens it.
	if _, err := logrotate.Rotate(logPath, logMaxBytes(opts.LogMaxBytes), logKeep(opts.LogKeep)); err != nil {
		slog.Warn("rotating the server log", "path", logPath, "error", err)
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening server log: %w", err)
	}
	var logStart int64 // what this start writes to the log comes after
	if fi, err := logFile.Stat(); err == nil {
		logStart = fi.Size()
	}

	readyDir := opts.ReadyDir
	if readyDir == "" {
		readyDir = opts.WorkDir
	}
	timeout := opts.ReadyTimeout
	var cmd *exec.Cmd
	if opts.Run != nil {
		inner, err := innerPaths(opts.Paths, opts.WorkDir, opts.DataDir, readyDir)
		if err != nil {
			logFile.Close()
			return nil, err
		}
		readyDir = inner[2]
		if timeout == 0 {
			timeout = 60 * time.Second
		}
		if cmd, err = opts.Run([]string{"opencode", "serve", "--hostname", "0.0.0.0", "--port", fmt.Sprint(port)},
			runtimeEnv(opts, password, inner[1]), inner[0], port); err != nil {
			logFile.Close()
			return nil, fmt.Errorf("preparing the server command: %w", err)
		}
	} else {
		cmd = exec.Command(bin, "serve", "--hostname", "127.0.0.1", "--port", fmt.Sprint(port))
		cmd.Dir = opts.WorkDir
		cmd.Env = buildEnv(os.Environ(), opts, password)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil
	setDetached(cmd)
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("starting opencode serve: %w", err)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	s := &Server{
		URL:      url,
		Port:     port,
		Password: password,
		PID:      cmd.Process.Pid,
		Started:  time.Now(),
		Client:   NewClient(url, password),
		exited:   make(chan struct{}),
	}
	safego.Go(func() {
		defer close(s.exited)
		err := cmd.Wait()
		logFile.Close()
		s.mu.Lock()
		s.exitErr = err
		s.mu.Unlock()
	})

	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if err := s.waitReady(ctx, readyDir, opts.ReadyAgent, timeout); err != nil {
		_ = s.Stop(context.Background(), 3*time.Second)
		if errors.Is(err, errServerExited) && portTaken(logSince(logPath, logStart)) {
			err = fmt.Errorf("%w (%d): %w", errPortTaken, port, err)
		}
		return nil, fmt.Errorf("%w%s", err, logTail(logPath))
	}
	return s, nil
}

// logSince returns what the log at path holds after offset.
func logSince(path string, offset int64) string {
	data, err := os.ReadFile(path)
	if err != nil || offset > int64(len(data)) {
		return ""
	}
	return string(data[offset:])
}

// errServerExited: the server process ended before it was ready.
var errServerExited = errors.New("opencode serve exited during startup")

// AttachServer reconnects to a server started earlier (e.g. by a previous oh process).
func AttachServer(url, password string, pid int) *Server {
	return &Server{URL: url, Password: password, PID: pid, Client: NewClient(url, password)}
}

func (s *Server) waitReady(ctx context.Context, dir, agent string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var lastErr error
	for {
		agents, err := s.Client.Agents(ctx, dir)
		switch {
		case err != nil:
			lastErr = err
		case len(agents) == 0:
			lastErr = errors.New("agents not loaded yet")
		case agent != "" && !hasAgent(agents, agent):
			lastErr = fmt.Errorf("agent %q not listed", agent)
		default:
			return nil
		}
		select {
		case <-s.exited:
			return fmt.Errorf("%w: %v", errServerExited, s.exitError())
		case <-ctx.Done():
			// Whatever the last answer was (a slow start gives none), the
			// error says what the server was waited for.
			if agent != "" {
				return fmt.Errorf("opencode serve not ready after %s (waiting for agent %q): %w", timeout, agent, lastErr)
			}
			return fmt.Errorf("opencode serve not ready after %s: %w", timeout, lastErr)
		case <-time.After(150 * time.Millisecond):
		}
	}
}

func hasAgent(agents []Agent, id string) bool {
	for _, a := range agents {
		if a.ID == id {
			return true
		}
	}
	return false
}

func (s *Server) exitError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exitErr
}

// Alive reports whether the server process is still running.
func (s *Server) Alive() bool {
	if s.exited != nil {
		select {
		case <-s.exited:
			return false
		default:
			return true
		}
	}
	return s.PID > 0 && processAlive(s.PID)
}

// Healthy reports whether the server answers authenticated API calls.
func (s *Server) Healthy(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err := s.Client.Info(ctx)
	return err == nil
}

// Stop terminates the server process group: graceful signal first, then a
// forced kill after timeout.
func (s *Server) Stop(ctx context.Context, timeout time.Duration) error {
	if s.PID <= 0 || !s.Alive() {
		return nil
	}
	terminateGroup(s.PID)
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for {
		if !s.Alive() {
			return nil
		}
		select {
		case <-ctx.Done():
			killGroup(s.PID)
			return ctx.Err()
		case <-deadline.C:
			killGroup(s.PID)
			return nil
		case <-tick.C:
		}
	}
}

func logTail(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	const maxTail = 2000
	if len(data) > maxTail {
		data = data[len(data)-maxTail:]
	}
	return "\n--- opencode serve log ---\n" + string(bytes.TrimSpace(data))
}

func logMaxBytes(n int64) int64 {
	if n > 0 {
		return n
	}
	return logrotate.DefaultMaxBytes
}

func logKeep(n int) int {
	if n > 0 {
		return n
	}
	return logrotate.DefaultKeep
}
