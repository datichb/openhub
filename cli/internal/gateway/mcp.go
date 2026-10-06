package gateway

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/datichb/openhub/cli/internal/i18n"
)

// MCPCommand is the machine command of an oh MCP server (stdio).
type MCPCommand struct {
	Argv []string
	Env  map[string]string // added to the daemon environment
	Dir  string
}

// MCP serves the oh MCP servers of a server group over HTTP (P4-T08), for
// tool servers running outside the machine: each server runs on the machine
// as its usual stdio command (`oh mcp serve <name>`, tokens read from the
// machine secret store) and the gateway relays JSON-RPC messages between the
// HTTP requests of the tool (MCP streamable HTTP, JSON answers) and its
// stdio. One process per (group, server), started at the first request and
// stopped with the group.
type MCP struct {
	// Resolve returns the command of server name in a group.
	Resolve func(ctx context.Context, group, name string) (MCPCommand, error)
	// Check refuses a call (403) before it reaches the server (session of
	// another group…). Optional.
	Check   func(ctx context.Context, group string, msg map[string]json.RawMessage) error
	Timeout time.Duration // per request, default 10 minutes

	mu    sync.Mutex
	procs map[string]*mcpProc
}

const defaultMCPTimeout = 10 * time.Minute

// ErrMCPDenied is returned by Check to refuse a call.
var ErrMCPDenied = errors.New("gateway: MCP call refused")

func procKey(group, name string) string { return group + "\x00" + name }

// ServeHTTP handles one MCP message of server name for group.
func (m *MCP) ServeHTTP(w http.ResponseWriter, r *http.Request, group, name string) {
	switch r.Method {
	case http.MethodPost:
	case http.MethodDelete:
		w.WriteHeader(http.StatusNoContent)
		return
	default: // no server-initiated stream (GET): allowed by the transport
		w.Header().Set("Allow", "POST, DELETE")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "gateway: reading request", http.StatusBadRequest)
		return
	}
	var msg map[string]json.RawMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		writeRPCError(w, nil, -32700, "Parse error")
		return
	}
	if m.Check != nil {
		if err := m.Check(r.Context(), group, msg); err != nil {
			http.Error(w, i18n.Tf("cmd.gateway.mcp.refused", name, err), http.StatusForbidden)
			return
		}
	}
	p, err := m.proc(r.Context(), group, name)
	if err != nil {
		slog.Warn("gateway: MCP server not started", "group", group, "server", name, "error", err)
		writeRPCError(w, msg["id"], -32603, i18n.Tf("cmd.gateway.mcp.unavailable", name, err))
		return
	}
	id, isRequest := msg["id"]
	if !isRequest || string(id) == "null" {
		if err := p.send(msg); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		return
	}
	timeout := m.Timeout
	if timeout <= 0 {
		timeout = defaultMCPTimeout
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	resp, err := p.call(ctx, msg)
	if err != nil {
		writeRPCError(w, id, -32603, i18n.Tf("cmd.gateway.mcp.failed", name, err))
		return
	}
	resp["id"] = id
	writeJSONMsg(w, resp)
}

// StopGroup stops the MCP servers of a group.
func (m *MCP) StopGroup(group string) {
	m.mu.Lock()
	var stop []*mcpProc
	for k, p := range m.procs {
		if p.group == group {
			stop = append(stop, p)
			delete(m.procs, k)
		}
	}
	m.mu.Unlock()
	for _, p := range stop {
		p.stop()
	}
}

// Close stops every MCP server.
func (m *MCP) Close() {
	m.mu.Lock()
	procs := m.procs
	m.procs = nil
	m.mu.Unlock()
	for _, p := range procs {
		p.stop()
	}
}

// Running lists the running servers of a group.
func (m *MCP) Running(group string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for _, p := range m.procs {
		if p.group == group && p.alive() {
			out = append(out, p.name)
		}
	}
	return out
}

func (m *MCP) proc(ctx context.Context, group, name string) (*mcpProc, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p := m.procs[procKey(group, name)]; p != nil && p.alive() {
		return p, nil
	}
	if m.Resolve == nil {
		return nil, errors.New("no MCP resolver")
	}
	c, err := m.Resolve(ctx, group, name)
	if err != nil {
		return nil, err
	}
	p, err := startMCP(group, name, c)
	if err != nil {
		return nil, err
	}
	if m.procs == nil {
		m.procs = map[string]*mcpProc{}
	}
	m.procs[procKey(group, name)] = p
	return p, nil
}

// mcpProc is one stdio MCP server. Request ids are rewritten so that the
// clients sharing the process (one per tool location) never collide.
type mcpProc struct {
	group, name string
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	wmu         sync.Mutex
	next        atomic.Int64

	pmu     sync.Mutex
	pending map[string]chan map[string]json.RawMessage
	done    chan struct{}
	err     error
}

func startMCP(group, name string, c MCPCommand) (*mcpProc, error) {
	if len(c.Argv) == 0 {
		return nil, errors.New("empty MCP command")
	}
	cmd := exec.Command(c.Argv[0], c.Argv[1:]...)
	cmd.Dir = c.Dir
	env := os.Environ()
	for _, k := range slices.Sorted(maps.Keys(c.Env)) {
		env = append(env, k+"="+c.Env[k])
	}
	cmd.Env = env
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &limitedWriter{w: &stderr, n: 64 << 10}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &mcpProc{group: group, name: name, cmd: cmd, stdin: stdin, pending: map[string]chan map[string]json.RawMessage{}, done: make(chan struct{})}
	go p.read(stdout)
	go func() {
		err := cmd.Wait()
		p.pmu.Lock()
		p.err = fmt.Errorf("server exited: %v %s", err, bytes.TrimSpace(stderr.Bytes()))
		for _, ch := range p.pending {
			close(ch)
		}
		p.pending = map[string]chan map[string]json.RawMessage{}
		close(p.done)
		p.pmu.Unlock()
		slog.Debug("gateway: MCP server stopped", "group", group, "server", name, "error", err)
	}()
	return p, nil
}

func (p *mcpProc) alive() bool {
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

func (p *mcpProc) read(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 64<<20)
	for sc.Scan() {
		var msg map[string]json.RawMessage
		if json.Unmarshal(sc.Bytes(), &msg) != nil {
			continue
		}
		id, ok := msg["id"]
		if !ok {
			continue // server notifications: no stream to carry them
		}
		p.pmu.Lock()
		ch := p.pending[string(id)]
		delete(p.pending, string(id))
		p.pmu.Unlock()
		if ch != nil {
			ch <- msg
		}
	}
}

func (p *mcpProc) send(msg map[string]json.RawMessage) error {
	line, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	p.wmu.Lock()
	defer p.wmu.Unlock()
	_, err = p.stdin.Write(append(line, '\n'))
	return err
}

func (p *mcpProc) call(ctx context.Context, msg map[string]json.RawMessage) (map[string]json.RawMessage, error) {
	id := json.RawMessage(strconv.FormatInt(p.next.Add(1), 10))
	ch := make(chan map[string]json.RawMessage, 1)
	p.pmu.Lock()
	if !p.alive() {
		err := p.err
		p.pmu.Unlock()
		return nil, err
	}
	p.pending[string(id)] = ch
	p.pmu.Unlock()
	out := maps.Clone(msg)
	out["id"] = id
	if err := p.send(out); err != nil {
		p.forget(id)
		return nil, err
	}
	select {
	case resp, ok := <-ch:
		if !ok {
			p.pmu.Lock()
			err := p.err
			p.pmu.Unlock()
			return nil, err
		}
		return resp, nil
	case <-ctx.Done():
		p.forget(id)
		return nil, ctx.Err()
	}
}

func (p *mcpProc) forget(id json.RawMessage) {
	p.pmu.Lock()
	delete(p.pending, string(id))
	p.pmu.Unlock()
}

// stop closes stdin (stdio servers exit on EOF), then kills after a delay.
func (p *mcpProc) stop() {
	_ = p.stdin.Close()
	select {
	case <-p.done:
	case <-time.After(2 * time.Second):
		_ = p.cmd.Process.Kill()
		<-p.done
	}
}

func writeJSONMsg(w http.ResponseWriter, msg any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(msg)
}

func writeRPCError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	if id == nil {
		id = json.RawMessage("null")
	}
	writeJSONMsg(w, map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}})
}

type limitedWriter struct {
	w io.Writer
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n > 0 {
		k := min(len(p), l.n)
		_, _ = l.w.Write(p[:k])
		l.n -= k
	}
	return len(p), nil
}
