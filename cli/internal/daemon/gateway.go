package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gateway"
	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// startGateway restores the gateway grants (hashes only) and serves the
// Beads gateway on the proxy listeners, which containers already reach
// (HostAddress, Linux bridge listener). Grants of groups that are no longer
// running are dropped.
func (d *Daemon) startGateway(ctx context.Context) {
	var key []byte
	if d.opts.Capability != "" {
		key = []byte("oh-gateway-grants:" + d.opts.Capability) // the file is signed by the daemon (S-14)
	}
	st, err := gateway.OpenSignedStore(d.opts.Paths.Gateway(), key)
	if err != nil {
		slog.Warn("ohd: gateway grants not restored", "error", err)
	}
	if d.opts.Servers != nil && st.Len() > 0 {
		live := map[string]bool{}
		if servers, err := d.opts.Servers.List(ctx); err == nil {
			for _, s := range servers {
				if s.Status != domain.ServerStopped && s.Status != domain.ServerSleeping {
					live[s.GroupKey] = true
				}
			}
			st.Retain(func(g gateway.Grant) bool { return live[g.GroupKey] })
		}
	}
	d.gateway = st
	d.mcp = &gateway.MCP{Resolve: d.resolveMCP, Check: d.checkMCP}
	beads := &gateway.Beads{
		Store:  st,
		View:   d.opts.GatewayView,
		Alive:  d.sessionAlive,
		Binary: d.opts.BeadsBinary,
		Guard:  d.guardTickets,
		Done:   d.ticketsDone,
		Agent:  d.beadsAgent,
	}
	st.ResolveBase = beads.ResolveBase // the Beads workspace is fixed at the grant (S-08)
	st.ResolveHuman = gateway.ResolveHuman
	d.beadsGW = beads
	d.proxy.Mount(beadswire.Prefix, beads)
}

// guardTickets refuses to close a ticket before the checkpoint that
// unlocks it, or before the work is committed (A17).
func (d *Daemon) guardTickets(ctx context.Context, g gateway.Grant, op gateway.BeadsOp) error {
	cp := d.opts.Checkpoints
	if cp != nil && op.Kind == gateway.OpCreate {
		return d.guardCreate(ctx, g)
	}
	if cp != nil && op.Kind != gateway.OpClose {
		return d.guardUpdate(ctx, g, false) // a change or a claim (r9b)
	}
	if cp == nil {
		return nil
	}
	if err := d.guardUpdate(ctx, g, true); err != nil {
		return err // duplicate, supersede… in a workflow that does not lock closing
	}
	err := cp.GuardClose(ctx, g.SessionID, g.Location)
	var lock *checkpoint.LockError
	if err != nil && !errors.As(err, &lock) {
		// The guard fails closed (S-09): a session without workflow is
		// answered nil by GuardClose, any other error refuses.
		slog.Warn("ohd: ticket guard failed, closing refused", "session", g.SessionID, "error", err)
		return errors.New(i18n.Tf("cmd.gateway.beads.guard_failed", err))
	}
	return err
}

// ticketsDone records the tickets a session claimed or closed; a ticket
// closed locks the operations of its checkpoint again.
func (d *Daemon) ticketsDone(ctx context.Context, g gateway.Grant, op gateway.BeadsOp) {
	d.recordProvenance(ctx, g, op)
	d.trackLease(g, op)
	cp := d.opts.Checkpoints
	if cp == nil {
		return
	}
	d.recordTickets(ctx, g, op)
	if op.Kind == gateway.OpCreate || op.Kind == gateway.OpUpdate {
		return
	}
	kind := checkpoint.BeadsClaim
	if op.Kind == gateway.OpClose {
		kind = checkpoint.BeadsClose
	}
	changed, err := cp.BeadsDone(ctx, g.SessionID, kind, op.IDs)
	if err != nil {
		slog.Debug("ohd: tickets of a session", "session", g.SessionID, "error", err)
		return
	}
	if changed {
		d.applyRules(ctx, g.SessionID)
	}
}

// recordTickets records the tickets created, closed or changed in the
// session outputs declared with `record: created`, `record: closed` (R5)
// or `record: updated` (r9b). A failure is only logged.
func (d *Daemon) recordTickets(ctx context.Context, g gateway.Grant, op gateway.BeadsOp) {
	kind := ""
	switch op.Kind {
	case gateway.OpCreate:
		kind = checkpoint.RecordCreated
	case gateway.OpClose:
		kind = checkpoint.RecordClosed
	case gateway.OpUpdate:
		kind = checkpoint.RecordUpdated
	default:
		return
	}
	if _, err := d.opts.Checkpoints.Record(ctx, g.SessionID, kind, op.IDs...); err != nil {
		slog.Debug("ohd: session output not recorded", "session", g.SessionID, "kind", kind, "error", err)
	}
}

// recordProvenance binds a claimed ticket to the session, and a closed
// ticket to the commit it was closed on (bd provenance record). A failure
// is only logged: it never refuses the operation, which already ran.
func (d *Daemon) recordProvenance(ctx context.Context, g gateway.Grant, op gateway.BeadsOp) {
	gw := d.beadsGW
	if gw == nil || len(op.IDs) == 0 {
		return
	}
	actor := gw.ActorOf(ctx, g)
	var evs []gateway.Provenance
	switch op.Kind {
	case gateway.OpClaim:
		for _, id := range op.IDs {
			evs = append(evs, gateway.Provenance{Issue: id, Kind: gateway.ProvClaim, RefKind: gateway.RefWorkID, Ref: g.SessionID,
				Actor: actor, Payload: provenancePayload("workflow", g.WorkflowID)})
		}
	case gateway.OpClose:
		gctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		head, _ := gitutil.Command(gctx, g.Location, "rev-parse", "HEAD").Output()
		branch, _ := gitutil.Command(gctx, g.Location, "rev-parse", "--abbrev-ref", "HEAD").Output()
		cancel()
		sha := strings.TrimSpace(string(head))
		if sha == "" {
			return
		}
		for _, id := range op.IDs {
			evs = append(evs, gateway.Provenance{Issue: id, Kind: gateway.ProvCommit, RefKind: gateway.RefGitSHA, Ref: sha,
				Actor: actor, Payload: provenancePayload("session", g.SessionID, "branch", strings.TrimSpace(string(branch)))})
		}
	}
	for _, ev := range evs {
		if err := gw.RecordProvenance(ctx, g, ev); err != nil {
			slog.Warn("ohd: provenance not recorded", "session", g.SessionID, "ticket", ev.Issue, "kind", ev.Kind, "error", err)
		}
	}
}

// provenancePayload keeps the non-empty values of key/value pairs.
func provenancePayload(kv ...string) map[string]string {
	out := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] != "" {
			out[kv[i]] = kv[i+1]
		}
	}
	return out
}

// beadsAgent names the oh agent of a gateway grant: the entry agent of the
// session, or the subagent of a sub-session ("" = not known yet).
func (d *Daemon) beadsAgent(ctx context.Context, g gateway.Grant) string {
	if d.opts.Sessions == nil {
		return ""
	}
	s, err := d.opts.Sessions.Get(ctx, g.SessionID)
	if err != nil || s == nil {
		return ""
	}
	if g.Holder == "" {
		return s.EntryAgent
	}
	d.wmu.Lock()
	w := d.watchers[s.GroupKey]
	d.wmu.Unlock()
	if w == nil {
		return ""
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.childAgent[g.Holder]
}

// sessionAlive: Beads is reachable while the session is open and awake.
func (d *Daemon) sessionAlive(ctx context.Context, id string) bool {
	if d.opts.Sessions == nil {
		return true
	}
	s, err := d.opts.Sessions.Get(ctx, id)
	if err != nil || s == nil {
		return false
	}
	switch s.State {
	case domain.RunStopped, domain.RunCompleted, domain.RunFailed, domain.RunSleeping:
		return false
	}
	return true
}

func (d *Daemon) handleGatewayGrant(w http.ResponseWriter, r *http.Request) {
	var req GatewayGrantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid body")
		return
	}
	tok, err := d.gateway.Issue(gateway.Grant{
		SessionID: req.SessionID, GroupKey: req.GroupKey, ProjectID: req.ProjectID,
		WorkflowID: req.WorkflowID, Location: req.Location, BeadsAllow: req.BeadsAllow, GatewayURL: req.GatewayURL,
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, GatewayGrantResponse{Token: tok})
}

// envSessionID mirrors runsvc.EnvSessionID (runsvc depends on this package).
const envSessionID = "OH_SESSION_ID"

// childEnv is the environment of a subagent sub-session: the tool does not
// pass the session environment to the sub-sessions started by the task
// tool, so the daemon applies the one of the root session (static
// variables, OH_SESSION_ID) with a gateway token of its own.
func (d *Daemon) childEnv(root, child string) (map[string]string, error) {
	env := map[string]string{}
	if d.opts.SessionsDir != "" {
		data, err := os.ReadFile(filepath.Join(d.opts.SessionsDir, root, "env.json"))
		switch {
		case err == nil:
			if err := json.Unmarshal(data, &env); err != nil {
				return nil, fmt.Errorf("reading the environment of %s: %w", root, err)
			}
		case !errors.Is(err, os.ErrNotExist):
			return nil, err
		}
	}
	env[envSessionID] = root
	if d.gateway != nil {
		if g, ok := d.gateway.Root(root); ok {
			g.Holder = child
			g.CreatedAt = time.Time{}
			tok, err := d.gateway.Issue(g)
			if err != nil {
				return nil, err
			}
			env[beadswire.EnvURL], env[beadswire.EnvToken] = g.GatewayURL, tok
		}
	}
	return env, nil
}

// applyChildEnv gives a newly linked sub-session the environment of its
// root session. Its first shell command may start before (event latency).
func (w *watcher) applyChildEnv(child, root string) {
	setter, ok := w.ad.(adapters.SessionEnvSetter)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	env, err := w.d.childEnv(root, child)
	if err == nil {
		err = setter.SetSessionEnv(ctx, w.handle(), child, env)
	}
	if err != nil {
		slog.Warn("ohd: environment of a subagent session not applied", "session", root, "child", child, "error", err)
	}
}

// Gateway tokens of subagents (audit v5, S-14) live while the sub-session
// runs: revoked when it ends, given again (with the session environment)
// when the delegating agent resumes it.

func (w *watcher) endChildGrant(root, child string) {
	if w.d.gateway == nil || !w.d.gateway.RevokeHolder(root, child) {
		return
	}
	w.mu.Lock()
	if w.childGrantEnded == nil {
		w.childGrantEnded = map[string]bool{}
	}
	w.childGrantEnded[child] = true
	w.mu.Unlock()
}

func (w *watcher) resumeChildGrant(root, child string) {
	w.mu.Lock()
	ended := w.childGrantEnded[child]
	delete(w.childGrantEnded, child)
	w.mu.Unlock()
	if ended {
		go w.applyChildEnv(child, root)
	}
}

// handleHookMCP relays a message of an oh MCP server to its process on the
// machine (P4-T08). The proxy has checked the session token of the group.
func (d *Daemon) handleHookMCP(w http.ResponseWriter, r *http.Request) {
	if d.mcp == nil || d.opts.MCPCommand == nil {
		http.NotFound(w, r)
		return
	}
	d.mcp.ServeHTTP(w, r, credproxy.HookOwner(r), r.PathValue("name"))
}

func (d *Daemon) resolveMCP(ctx context.Context, group, name string) (gateway.MCPCommand, error) {
	if d.opts.Servers == nil {
		return gateway.MCPCommand{}, errors.New("no server registry")
	}
	srv, err := d.opts.Servers.Get(ctx, group)
	if err != nil {
		return gateway.MCPCommand{}, err
	}
	c, err := d.opts.MCPCommand(ctx, *srv, name)
	if err != nil || !sessionspec.OhDaemonMCPServer(name) {
		return c, err
	}
	env := maps.Clone(c.Env)
	if env == nil {
		env = map[string]string{}
	}
	env[sessionspec.EnvGroupToken] = d.workflowToken(group)
	c.Env = env
	return c, nil
}

// checkMCP is the gate of the MCP gateway (audit v5, S-13): it fails
// closed. The workflow and wiki servers trust the session the tool names in
// the `_meta` of tools/call, so a call is relayed only when `_meta` names
// one session, which belongs to the group of the token (the group of the
// tool server). A missing, unreadable or unknown `_meta`, several sessions,
// or the session of another group, refuse the call.
//
// The token is the one of the server group (one per tool server): sessions of
// a group cannot be told apart by it, but `_meta` is written by the tool,
// which does not give its token to the shell (serverSecretEnv).
func (d *Daemon) checkMCP(ctx context.Context, group string, msg map[string]json.RawMessage) error {
	var method string
	if err := json.Unmarshal(msg["method"], &method); err != nil || method == "" {
		return gateway.Denied(i18n.T("cmd.gateway.mcp.no_method"))
	}
	ids, err := mcpMetaSessions(msg["params"])
	if err != nil {
		return gateway.Denied(err.Error())
	}
	switch {
	case len(ids) == 0 && mcpSessionless(method):
		return nil
	case len(ids) == 0:
		return gateway.Denied(i18n.Tf("cmd.gateway.mcp.no_session", method))
	case len(ids) > 1:
		return gateway.Denied(i18n.T("cmd.gateway.mcp.many_sessions"))
	}
	return d.checkMCPSession(ctx, group, ids[0])
}

// mcpSessionless are the messages that carry no session and act on nothing:
// opening the connection, discovery, notifications.
func mcpSessionless(method string) bool {
	switch method {
	case "initialize", "ping", "tools/list", "prompts/list", "resources/list", "resources/templates/list":
		return true
	}
	return strings.HasPrefix(method, "notifications/")
}

// mcpMetaSessions returns the distinct sessions named in the `_meta` of
// params (the tool names them "<vendor>/sessionID": any key ending with
// "sessionid" is read, as mcp/workflow.ToolSession does).
func mcpMetaSessions(params json.RawMessage) ([]string, error) {
	if len(params) == 0 {
		return nil, nil
	}
	var p struct {
		Meta map[string]json.RawMessage `json:"_meta"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, errors.New(i18n.T("cmd.gateway.mcp.bad_meta"))
	}
	var ids []string
	for k, raw := range p.Meta {
		if !strings.HasSuffix(strings.ToLower(k), "sessionid") {
			continue
		}
		var id string
		if err := json.Unmarshal(raw, &id); err != nil || id == "" {
			return nil, errors.New(i18n.Tf("cmd.gateway.mcp.bad_session", k))
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// checkMCPSession refuses a session that is not one of the group: a root
// session, or a subagent session of one (the tool may name it before the
// daemon has linked it: it is looked up in the tool, as for the hooks).
func (d *Daemon) checkMCPSession(ctx context.Context, group, id string) error {
	if d.opts.Sessions == nil {
		return gateway.Denied(i18n.Tf("cmd.gateway.mcp.session_unverified", id))
	}
	s, err := d.opts.Sessions.Get(ctx, d.rootSession(id))
	if errors.Is(err, domain.ErrNotFound) {
		s, err = d.opts.Sessions.Get(ctx, d.adoptForHook(ctx, id, group))
	}
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return gateway.Denied(i18n.Tf("cmd.gateway.mcp.foreign_session", id))
	case err != nil || s == nil:
		slog.Warn("ohd: session of an MCP call not verified", "session", id, "error", err)
		return gateway.Denied(i18n.Tf("cmd.gateway.mcp.session_unverified", id))
	case s.GroupKey != group:
		return gateway.Denied(i18n.Tf("cmd.gateway.mcp.foreign_session", id))
	}
	return nil
}

// Shell of local sessions (A16): oh's start-up files keep the fake bd first
// on the PATH; the daemon checks it once per session and raises an alert
// (✗, not cleared by the agent loop) when another bd comes first.

// dataOhAlert marks the alerts oh raises about a session that only the user
// dismisses.
const dataOhAlert = "oh_alert"

// alertShellBD is the alert of a session whose shell finds another bd first.
const alertShellBD = "shell_bd"

func isOhAlert(d domain.Decision) bool {
	_, ok := d.Payload.Data[dataOhAlert]
	return ok
}

// checkShellOnce checks the shell of a local session, once per daemon.
func (w *watcher) checkShellOnce(ctx context.Context, root string) {
	w.mu.Lock()
	if w.shellChecked == nil {
		w.shellChecked = map[string]bool{}
	}
	done := w.shellChecked[root]
	w.shellChecked[root] = true
	w.mu.Unlock()
	if done || w.d.opts.SessionsDir == "" || w.d.opts.Decisions == nil {
		return
	}
	sess, err := w.d.opts.Sessions.Get(ctx, root)
	if err != nil || (sess.Runtime != "" && sess.Runtime != "local") || isTerminal(sess.State) {
		return
	}
	data, err := os.ReadFile(filepath.Join(w.d.opts.SessionsDir, root, "env.json"))
	if err != nil {
		return
	}
	env := map[string]string{}
	if json.Unmarshal(data, &env) != nil || env[gateway.EnvPathFirst] == "" {
		return // started without oh's shell start-up (older oh)
	}
	first := strings.Split(env[gateway.EnvPathFirst], string(os.PathListSeparator))[0]
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	got, err := gateway.CheckShellBD(cctx, env, filepath.Join(first, "bd"))
	if !errors.Is(err, gateway.ErrShellBD) {
		return
	}
	if got == "" {
		got = "—"
	}
	d := domain.Decision{SessionID: root, GroupKey: sess.GroupKey, Kind: domain.DecisionError, ToolRef: alertShellBD,
		Payload: domain.DecisionPayload{Title: i18n.T("cmd.gateway.beads.shell_bd_title"),
			Message: i18n.Tf("cmd.gateway.beads.shell_bd_detail", got, first),
			Data:    map[string]any{dataOhAlert: alertShellBD}}}
	d.ID = domain.DecisionID(d.Kind, root, d.ToolRef)
	if prev, err := w.d.opts.Decisions.Get(ctx, d.ID); err == nil && prev != nil {
		return // already raised (and maybe dismissed)
	}
	if err := w.d.opts.Decisions.Upsert(ctx, &d); err != nil {
		slog.Warn("ohd: shell alert not recorded", "session", root, "error", err)
		return
	}
	slog.Warn("ohd: the shell of a local session finds another bd first", "session", root, "bd", got)
	w.refreshAlerts(ctx, root)
}
