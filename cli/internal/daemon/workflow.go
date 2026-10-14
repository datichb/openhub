package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Workflow API (P3-T01): backend of the oh MCP server `workflow` run by the
// tool for each location. The MCP server identifies the calling tool session
// (a subagent runs in a child session); the daemon files it under the oh
// session that delegated it.

// WorkflowCheckpointRequest is the body of POST /v1/workflow/checkpoint.
type WorkflowCheckpointRequest struct {
	ToolSession string `json:"tool_session"`
	checkpoint.Call
}

// WorkflowOutputRequest is the body of POST /v1/workflow/outputs.
type WorkflowOutputRequest struct {
	ToolSession string `json:"tool_session"`
	checkpoint.Output
}

// rootSession returns the oh session a tool session belongs to (itself, or
// the root of a subagent session).
func (d *Daemon) rootSession(id string) string {
	d.wmu.Lock()
	defer d.wmu.Unlock()
	for _, w := range d.watchers {
		if r, ok := w.rootOf(id); ok {
			return r
		}
	}
	return id
}

// Workflow routes are authenticated by the token of the server group (N1):
// the workflow MCP server of the tool server sends it (EnvGroupToken), and
// the session must belong to that group. Without a capability configured
// (embedded uses, tests, a runner job), they are not guarded.

// wfTokenPrefix marks the tokens the daemon hands to its own workflow MCP
// servers.
const wfTokenPrefix = "ohw_"

// workflowToken returns a token of group for a workflow MCP server run by
// the daemon (valid while the daemon lives).
func (d *Daemon) workflowToken(group string) string {
	tok := wfTokenPrefix + strings.TrimPrefix(credproxy.NewToken(), credproxy.TokenPrefix)
	d.mu.Lock()
	if d.wfTokens == nil {
		d.wfTokens = map[string]string{}
	}
	d.wfTokens[credproxy.TokenHash(tok)] = group
	d.mu.Unlock()
	return tok
}

// workflowOwner returns the server group of the token of a request.
func (d *Daemon) workflowOwner(r *http.Request) (string, bool) {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	tok = strings.TrimSpace(tok)
	if !ok || tok == "" {
		return "", false
	}
	if strings.HasPrefix(tok, wfTokenPrefix) {
		d.mu.Lock()
		g, ok := d.wfTokens[credproxy.TokenHash(tok)]
		d.mu.Unlock()
		return g, ok
	}
	if d.proxy == nil {
		return "", false
	}
	return d.proxy.Owner(r.Context(), tok)
}

// authorizeWorkflow returns the oh session of a tool session when the
// request may act on it (written error otherwise).
func (d *Daemon) authorizeWorkflow(w http.ResponseWriter, r *http.Request, toolSession string) (string, bool) {
	root := d.rootSession(toolSession)
	if d.opts.Capability == "" && r.Header.Get("Authorization") == "" {
		return root, true
	}
	group, ok := d.workflowOwner(r)
	if !ok {
		writeErr(w, http.StatusUnauthorized, i18n.T("cmd.daemon.workflow.unauthorized"))
		return "", false
	}
	if d.opts.Sessions != nil {
		if s, err := d.opts.Sessions.Get(r.Context(), root); err == nil && s.GroupKey != group {
			writeErr(w, http.StatusForbidden, i18n.T("cmd.daemon.workflow.forbidden"))
			return "", false
		}
	}
	return root, true
}

func (d *Daemon) workflowErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, checkpoint.ErrNotApproved), errors.Is(err, checkpoint.ErrVerifyFailed), errors.Is(err, checkpoint.ErrVerifyPending),
		errors.Is(err, checkpoint.ErrVerifyUnavailable), errors.Is(err, checkpoint.ErrOutOfOrder), errors.Is(err, checkpoint.ErrTasksInvalid):
		writeErr(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, checkpoint.ErrNoWorkflow):
		writeErr(w, http.StatusNotFound, err.Error())
	case errors.Is(err, checkpoint.ErrUnknownCP):
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}

func (d *Daemon) handleWorkflowStatus(w http.ResponseWriter, r *http.Request) {
	if d.opts.Checkpoints == nil {
		writeErr(w, http.StatusServiceUnavailable, "checkpoints are not available")
		return
	}
	id := r.URL.Query().Get("tool_session")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "tool_session is required")
		return
	}
	root, ok := d.authorizeWorkflow(w, r, id)
	if !ok {
		return
	}
	st, err := d.opts.Checkpoints.Status(r.Context(), root)
	if err != nil {
		d.workflowErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (d *Daemon) handleWorkflowCheckpoint(w http.ResponseWriter, r *http.Request) {
	if d.opts.Checkpoints == nil {
		writeErr(w, http.StatusServiceUnavailable, "checkpoints are not available")
		return
	}
	var req WorkflowCheckpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ToolSession == "" || req.ID == "" {
		writeErr(w, http.StatusBadRequest, "tool_session and id are required")
		return
	}
	root, ok := d.authorizeWorkflow(w, r, req.ToolSession)
	if !ok {
		return
	}
	req.Agent = d.callerAgent(r.Context(), root, req.ToolSession) // never from the request
	res, err := d.opts.Checkpoints.Reached(r.Context(), root, req.Call)
	if errors.Is(err, checkpoint.ErrNotApproved) {
		writeErr(w, http.StatusConflict, i18n.Tf("cmd.daemon.workflow.not_approved", req.ID))
		return
	}
	if errors.Is(err, gitutil.ErrConfigChanged) {
		d.raiseGitChanged(r.Context(), root, req.ID, err)
		writeErr(w, http.StatusConflict, i18n.Tf("cmd.daemon.workflow.git_changed", req.ID))
		return
	}
	if err != nil {
		d.workflowErr(w, err)
		return
	}
	d.applyRules(r.Context(), root) // locks released by the checkpoint
	writeJSON(w, http.StatusOK, res)
}

// WorkflowRulesRequest is the body of POST /v1/workflow/rules.
type WorkflowRulesRequest struct {
	SessionID string `json:"session_id"`
}

// handleWorkflowRules re-applies the session rules of an oh session (its
// workflow state changed in another process: circuit breaker dismissed).
func (d *Daemon) handleWorkflowRules(w http.ResponseWriter, r *http.Request) {
	var req WorkflowRulesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == "" {
		writeErr(w, http.StatusBadRequest, "session_id is required")
		return
	}
	d.adoptAll(r.Context(), req.SessionID) // an imported session brings its subagent sessions
	d.applyRules(r.Context(), req.SessionID)
	w.WriteHeader(http.StatusNoContent)
}

// adoptAll links the subagent sessions of the group of an oh session that
// are not linked yet (sessions imported into a running server).
func (d *Daemon) adoptAll(ctx context.Context, root string) {
	if d.opts.Sessions == nil {
		return
	}
	s, err := d.opts.Sessions.Get(ctx, root)
	if err != nil {
		return
	}
	d.wmu.Lock()
	w := d.watchers[s.GroupKey]
	d.wmu.Unlock()
	if w != nil {
		w.adoptChildren(ctx)
	}
}

// WorkflowRefresh asks the daemon to re-apply the session rules of a session.
func (c *Client) WorkflowRefresh(ctx context.Context, sessionID string) error {
	return c.do(ctx, http.MethodPost, "/workflow/rules", WorkflowRulesRequest{SessionID: sessionID}, nil)
}

func (d *Daemon) handleWorkflowOutputs(w http.ResponseWriter, r *http.Request) {
	if d.opts.Checkpoints == nil {
		writeErr(w, http.StatusServiceUnavailable, "checkpoints are not available")
		return
	}
	var req WorkflowOutputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ToolSession == "" {
		writeErr(w, http.StatusBadRequest, "tool_session is required")
		return
	}
	root, ok := d.authorizeWorkflow(w, r, req.ToolSession)
	if !ok {
		return
	}
	if err := d.opts.Checkpoints.Declare(r.Context(), root, req.Output); err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, checkpoint.ErrNoWorkflow) {
			d.workflowErr(w, err)
			return
		}
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// WorkflowStatus returns the workflow status of the oh session a tool session belongs to.
func (c *Client) WorkflowStatus(ctx context.Context, toolSession string) (checkpoint.Status, error) {
	var out checkpoint.Status
	err := c.do(ctx, http.MethodGet, "/workflow/status?tool_session="+url.QueryEscape(toolSession), nil, &out)
	return out, err
}

// WorkflowCheckpoint records a workflow_checkpoint call the tool let run.
func (c *Client) WorkflowCheckpoint(ctx context.Context, toolSession string, call checkpoint.Call) (checkpoint.Result, error) {
	var out checkpoint.Result
	err := c.do(ctx, http.MethodPost, "/workflow/checkpoint", WorkflowCheckpointRequest{ToolSession: toolSession, Call: call}, &out)
	return out, err
}

// WorkflowOutput records an output declared by workflow_outputs.
func (c *Client) WorkflowOutput(ctx context.Context, toolSession string, o checkpoint.Output) error {
	return c.do(ctx, http.MethodPost, "/workflow/outputs", WorkflowOutputRequest{ToolSession: toolSession, Output: o}, nil)
}

// HookPermissionRequest is sent by the oh plugin before a permission is
// asked (POST /oh/v1/hooks/permission on the proxy listeners).
type HookPermissionRequest struct {
	SessionID string         `json:"sessionID"`
	Agent     string         `json:"agent,omitempty"`
	Action    string         `json:"action"`
	Resources []string       `json:"resources,omitempty"`
	CallID    string         `json:"callID,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	// Pending marks a request the plugin holds until oh confirms the
	// session rules (bundle.PendingGuard): oh answers it from the session
	// rules of the oh session, a refusal when it cannot.
	Pending bool `json:"pending,omitempty"`
}

// HookPermissionResponse: effect "" keeps the rendered decision.
type HookPermissionResponse struct {
	Effect  string `json:"effect,omitempty"`
	Message string `json:"message,omitempty"`
}

// hooksHandler serves the plugin hooks (P3-T05). The proxy has checked the
// session token; a group may only ask about its own sessions.
func (d *Daemon) hooksHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+credproxy.HooksPrefix+"permission", d.handleHookPermission)
	mux.HandleFunc(credproxy.HooksPrefix+"mcp/{name}", d.handleHookMCP)
	return mux
}

func (d *Daemon) handleHookPermission(w http.ResponseWriter, r *http.Request) {
	var req HookPermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.SessionID == "" || req.Action == "" {
		writeErr(w, http.StatusBadRequest, "sessionID and action are required")
		return
	}
	if d.opts.Checkpoints == nil || d.opts.Sessions == nil {
		writeJSON(w, http.StatusOK, HookPermissionResponse{})
		return
	}
	root := d.rootSession(req.SessionID)
	if req.Pending && root == req.SessionID {
		root = d.adoptForHook(r.Context(), req.SessionID, credproxy.HookOwner(r))
	}
	s, err := d.opts.Sessions.Get(r.Context(), root)
	if err != nil || s.GroupKey != credproxy.HookOwner(r) {
		if req.Pending {
			// p6-debts: a session oh cannot link to one of its sessions has
			// no checkpoint locks: what they lock is refused.
			writeJSON(w, http.StatusOK, HookPermissionResponse{Effect: string(sessionspec.EffectDeny), Message: i18n.T("cmd.daemon.workflow.pending_unknown")})
			return
		}
		writeErr(w, http.StatusForbidden, "unknown session for this token")
		return
	}
	d.wmu.Lock()
	wt := d.watchers[s.GroupKey]
	d.wmu.Unlock()
	action := req.Action
	if wt != nil {
		if n, ok := wt.ad.(adapters.ActionNamer); ok {
			action = n.NeutralAction(action)
		}
	}
	if req.Pending {
		// The tool decided on the rules it held: the session rules may not
		// be set yet (subagent just adopted, session imported). oh decides
		// from the rules it sets.
		rules, err := d.opts.Checkpoints.Rules(r.Context(), root)
		if err != nil {
			writeJSON(w, http.StatusOK, HookPermissionResponse{Effect: string(sessionspec.EffectDeny), Message: i18n.T("cmd.daemon.workflow.pending_unconfirmed")})
			return
		}
		if eff, ok := bundle.SessionRulesEffect(rules, action, req.Resources); ok && eff == sessionspec.EffectDeny {
			writeJSON(w, http.StatusOK, HookPermissionResponse{Effect: string(eff), Message: i18n.Tf("cmd.daemon.workflow.pending_locked", strings.Join(req.Resources, " ; "))})
			return
		}
	}
	// r5-fixes: the edit of a `writes:` path held until a checkpoint is
	// refused with the reason.
	if effect, msg, err := d.opts.Checkpoints.EvaluateEdit(r.Context(), root, action, req.Resources); err == nil && effect != "" {
		writeJSON(w, http.StatusOK, HookPermissionResponse{Effect: effect, Message: msg})
		return
	}
	effect, err := d.opts.Checkpoints.Evaluate(r.Context(), root, action, req.Input)
	if err != nil {
		writeJSON(w, http.StatusOK, HookPermissionResponse{})
		return
	}
	writeJSON(w, http.StatusOK, HookPermissionResponse{Effect: effect})
}
