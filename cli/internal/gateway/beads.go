// Package gateway is the GatewayService of the oh daemon (03 §11, P4-T07):
// it serves, to tool servers running in another runtime (container), the
// machine resources that must stay on the machine (D9, D10). The Beads
// gateway receives the commands of the fake bd, checks them against the
// workflow allow-list and runs the real bd in the session directory.
package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/i18n"
)

// Beads is the Beads gateway.
type Beads struct {
	Store *Store
	// View returns how the runtime of a server group sees the machine
	// (nil, or a nil PathMap: same paths; no locations: the grant location).
	View func(ctx context.Context, group string) (View, error)
	// Alive reports whether a session may still use Beads (nil = always).
	Alive func(ctx context.Context, sessionID string) bool
	// Binary is the real bd ("" = looked up in PATH at each call).
	Binary    string
	Timeout   time.Duration // default 2 minutes
	MaxOutput int           // bytes per stream, default 8 MiB
}

const (
	defaultTimeout   = 2 * time.Minute
	defaultMaxOutput = 8 << 20
)

// ServeHTTP serves POST <gateway>/beads/v1/exec (the gateway prefix is
// stripped by the caller).
func (b *Beads) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != beadswire.ExecPath || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	tok, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	g, ok := b.Store.Lookup(strings.TrimSpace(tok))
	if !ok {
		writeResp(w, http.StatusUnauthorized, refusalResp(i18n.T("cmd.gateway.beads.unauthorized")))
		return
	}
	if b.Alive != nil && !b.Alive(r.Context(), g.SessionID) {
		writeResp(w, http.StatusUnauthorized, refusalResp(i18n.Tf("cmd.gateway.beads.session_closed", g.SessionID)))
		return
	}
	var req beadswire.ExecRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 2*beadswire.MaxStdin)).Decode(&req); err != nil || len(req.Stdin) > beadswire.MaxStdin {
		writeResp(w, http.StatusBadRequest, refusalResp(i18n.T("cmd.gateway.beads.bad_request")))
		return
	}
	resp, status := b.Exec(r.Context(), g, req)
	writeResp(w, status, resp)
}

func refusalResp(msg string) beadswire.ExecResponse {
	return beadswire.ExecResponse{Error: msg, ExitCode: 1}
}

func writeResp(w http.ResponseWriter, status int, resp beadswire.ExecResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

// Exec checks and runs one command for a grant. The HTTP status is 200 when
// bd ran (whatever its exit code), 403 when the command is refused.
func (b *Beads) Exec(ctx context.Context, g Grant, req beadswire.ExecRequest) (resp beadswire.ExecResponse, status int) {
	c, err := checkBeads(req.Argv, g.BeadsAllow)
	if err != nil {
		return refusalResp(err.Error()), http.StatusForbidden
	}
	view := View{}
	if b.View != nil {
		if view, err = b.View(ctx, g.GroupKey); err != nil {
			slog.Warn("gateway: runtime view unavailable", "group", g.GroupKey, "error", err)
			return refusalResp(i18n.Tf("cmd.gateway.beads.view_failed", err)), http.StatusServiceUnavailable
		}
	}
	if len(view.Locations) == 0 {
		view.Locations = []string{g.Location}
	}
	dir := view.hostDir(req.Cwd, g.Location)
	argv, err := view.translateArgs(req.Argv, c, dir)
	if err != nil {
		return refusalResp(err.Error()), http.StatusForbidden
	}
	if !exists(dir) {
		return refusalResp(i18n.Tf("cmd.gateway.beads.path_outside", dir)), http.StatusForbidden
	}
	bin := b.Binary
	if bin == "" {
		if bin, err = exec.LookPath("bd"); err != nil {
			return refusalResp(i18n.T("cmd.gateway.beads.bd_missing")), http.StatusServiceUnavailable
		}
	}
	timeout := b.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	limit := b.MaxOutput
	if limit <= 0 {
		limit = defaultMaxOutput
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, bin, argv...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(req.Stdin)
	stdout, stderr := &capped{max: limit}, &capped{max: limit}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	runErr := cmd.Run()
	resp = beadswire.ExecResponse{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}
	var exit *exec.ExitError
	switch {
	case runErr == nil:
	case errors.Is(cctx.Err(), context.DeadlineExceeded):
		resp.ExitCode, resp.Error = 1, i18n.Tf("cmd.gateway.beads.timeout", timeout)
	case errors.As(runErr, &exit):
		resp.ExitCode = exit.ExitCode()
	default:
		resp.ExitCode, resp.Error = 1, runErr.Error()
	}
	if stdout.cut || stderr.cut {
		resp.Stderr = append(resp.Stderr, []byte(fmt.Sprintf("bd: %s\n", i18n.Tf("cmd.gateway.beads.output_truncated", limit)))...)
	}
	slog.Debug("gateway: bd", "session", g.SessionID, "command", c.Name, "exit", resp.ExitCode)
	return resp, http.StatusOK
}

// capped keeps the first max bytes written (not an embedded bytes.Buffer:
// its ReadFrom would bypass the limit in io.Copy).
type capped struct {
	buf bytes.Buffer
	max int
	cut bool
}

func (c *capped) Write(p []byte) (int, error) {
	if room := c.max - c.buf.Len(); room < len(p) {
		c.cut = true
		if room > 0 {
			c.buf.Write(p[:room])
		}
		return len(p), nil
	}
	return c.buf.Write(p)
}

func (c *capped) Bytes() []byte { return c.buf.Bytes() }
