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
	"os"
	"os/exec"
	"runtime"
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
	// Guard refuses a ticket operation for the workflow state of the
	// session (nil = none); its error message is shown to the agent.
	Guard func(ctx context.Context, g Grant, op BeadsOp) error
	// Done is told the ticket operations that ran successfully (nil = none).
	Done func(ctx context.Context, g Grant, op BeadsOp)
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
	view := View{}
	var err error
	if b.View != nil {
		if view, err = b.View(ctx, g.GroupKey); err != nil {
			slog.Warn("gateway: runtime view unavailable", "group", g.GroupKey, "error", err)
			return refusalResp(i18n.Tf("cmd.gateway.beads.view_failed", err)), http.StatusServiceUnavailable
		}
	}
	if len(view.Locations) == 0 {
		view.Locations = []string{g.Location}
	}
	hookName := ""
	if req.GitHook {
		hookName = GitHookName(req.Argv)
	}
	if hookName != "" && (len(view.Paths) > 0 || journalMode()) {
		// Another runtime: the hooks bd chains (the project's own hooks)
		// would run on the machine, out of the container; a remote job
		// replays its journal later. Skipped: nothing runs.
		slog.Debug("gateway: Beads git hook skipped (not a local session)", "session", g.SessionID, "hook", hookName)
		return beadswire.ExecResponse{}, http.StatusOK
	}
	allow := g.BeadsAllow
	hook := false
	if hookName != "" {
		if b.Store == nil || !b.Store.consumeHookToken(req.HookToken, g, hookName) {
			slog.Warn("gateway: Beads git hook without a valid hook token", "session", g.SessionID, "hook", hookName)
			return refusalResp(i18n.T("cmd.gateway.beads.hook_unverified")), http.StatusForbidden
		}
		hook, allow = true, gitHookAllow // run by git, whatever the workflow allows (A19)
	}
	c, err := checkBeads(req.Argv, allow)
	if err != nil {
		return refusalResp(err.Error()), http.StatusForbidden
	}
	dir := view.hostDir(req.Cwd, g.Location)
	argv, err := view.translateArgs(c, dir)
	if err != nil {
		return refusalResp(err.Error()), http.StatusForbidden
	}
	if !exists(dir) {
		return refusalResp(i18n.Tf("cmd.gateway.beads.path_outside", dir)), http.StatusForbidden
	}
	beadsDir := ""
	if c.Name() != "" && !c.Help() {
		base := b.base(ctx, g)
		if base == nil {
			return refusalResp(i18n.T("cmd.gateway.beads.base_unknown")), http.StatusServiceUnavailable
		}
		if err := base.check(); err != nil {
			slog.Warn("gateway: Beads workspace of the session changed", "session", g.SessionID, "error", err)
			return refusalResp(err.Error()), http.StatusForbidden
		}
		if base.Dir != journalBase {
			beadsDir = base.Dir
		}
	}
	bin, err := b.binary()
	if err != nil {
		return refusalResp(i18n.T("cmd.gateway.beads.bd_missing")), http.StatusServiceUnavailable
	}
	op, isOp := ticketOp(c)
	if !isOp {
		op, isOp = createOp(c)
	}
	if isOp && op.Status != "" && !b.closingStatus(ctx, bin, dir, beadsDir, op.Status) {
		isOp = false
	}
	if isOp && b.Guard != nil {
		if err := b.Guard(ctx, g, op); err != nil {
			slog.Debug("gateway: bd refused by the workflow", "session", g.SessionID, "command", c.Key(), "error", err)
			return refusalResp(err.Error()), http.StatusForbidden
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
	cmd.Env = beadsEnv(os.Environ(), beadsDir, hook)
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
	slog.Debug("gateway: bd", "session", g.SessionID, "command", c.Key(), "exit", resp.ExitCode)
	if isOp && b.Done != nil && resp.ExitCode == 0 && resp.Error == "" {
		b.Done(ctx, g, op)
	}
	return resp, http.StatusOK
}

func (b *Beads) binary() (string, error) {
	if b.Binary != "" {
		return b.Binary, nil
	}
	return exec.LookPath("bd")
}

// closingStatus asks bd whether a status of the project's configuration
// is a done status (bd statuses); unknown answers count as closing.
func (b *Beads) closingStatus(ctx context.Context, bin, dir, beadsDir, status string) bool {
	ctx, cancel := context.WithTimeout(ctx, baseTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "statuses", "--json")
	cmd.Dir = dir
	cmd.Env = beadsEnv(os.Environ(), beadsDir, false)
	out, err := cmd.Output()
	if err != nil {
		return true
	}
	return ClosingStatus(out, status)
}

// ClosingStatus reads the output of `bd statuses --json`: whether status is
// a done status (unknown or unreadable: closing).
func ClosingStatus(out []byte, status string) bool {
	var st struct {
		Custom []struct {
			Name     string `json:"name"`
			Category string `json:"category"`
		} `json:"custom_statuses"`
	}
	if json.Unmarshal(out, &st) != nil {
		return true
	}
	for _, s := range st.Custom {
		if strings.EqualFold(s.Name, status) {
			return s.Category == "done"
		}
	}
	return true // bd refuses an unknown status; counted as closing
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

// beadsEnv is the environment of the real bd (audit v5, S-14): only what
// bd needs from the daemon's (PATH, HOME, user, locale, temporary
// directory, XDG directories, the Beads settings BEADS_*, the journal mode
// of a remote job), with the workspace fixed by oh (BEADS_DIR) and the git
// hook marker set only when git runs the command. The daemon's secrets and
// other variables are not passed.
func beadsEnv(base []string, beadsDir string, hook bool) []string {
	env := make([]string, 0, 16)
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if keepBeadsVar(k) {
			env = append(env, kv)
		}
	}
	if beadsDir != "" {
		env = append(env, "BEADS_DIR="+beadsDir)
	}
	if hook {
		env = append(env, beadswire.EnvGitHook+"=1")
	}
	return env
}

// beadsVars are the variables of the daemon passed to bd.
var beadsVars = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "LOGNAME": true, "SHELL": true, "LANG": true, "TZ": true,
	"TMPDIR": true, "TEMP": true, "TMP": true, "NO_COLOR": true,
	"XDG_CONFIG_HOME": true, "XDG_DATA_HOME": true, "XDG_CACHE_HOME": true, "XDG_STATE_HOME": true, "XDG_RUNTIME_DIR": true,
	// Windows
	"SYSTEMROOT": true, "SYSTEMDRIVE": true, "WINDIR": true, "COMSPEC": true, "PATHEXT": true, "USERPROFILE": true,
	"APPDATA": true, "LOCALAPPDATA": true, "PROGRAMDATA": true, "HOMEDRIVE": true, "HOMEPATH": true, "USERNAME": true,
	// journal mode of a remote job (cmd/runner.go)
	beadswire.EnvMode: true, beadswire.EnvSnapshot: true, beadswire.EnvJournal: true,
}

func keepBeadsVar(k string) bool {
	switch {
	case k == "BEADS_DIR" || k == beadswire.EnvGitHook:
		return false
	case beadsVars[k], beadsVars[strings.ToUpper(k)] && runtime.GOOS == "windows":
		return true
	case strings.HasPrefix(k, "LC_"), strings.HasPrefix(k, "BEADS_"):
		return true
	}
	return false
}
