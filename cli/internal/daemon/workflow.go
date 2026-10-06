package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/credproxy"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
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

func (d *Daemon) workflowErr(w http.ResponseWriter, err error) {
	switch {
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
	st, err := d.opts.Checkpoints.Status(r.Context(), d.rootSession(id))
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
	root := d.rootSession(req.ToolSession)
	res, err := d.opts.Checkpoints.Reached(r.Context(), root, req.Call)
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
	d.applyRules(r.Context(), req.SessionID)
	w.WriteHeader(http.StatusNoContent)
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
	if err := d.opts.Checkpoints.Declare(r.Context(), d.rootSession(req.ToolSession), req.Output); err != nil {
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
	s, err := d.opts.Sessions.Get(r.Context(), root)
	if err != nil || s.GroupKey != credproxy.HookOwner(r) {
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
	effect, err := d.opts.Checkpoints.Evaluate(r.Context(), root, action, req.Input)
	if err != nil {
		writeJSON(w, http.StatusOK, HookPermissionResponse{})
		return
	}
	writeJSON(w, http.StatusOK, HookPermissionResponse{Effect: effect})
}
