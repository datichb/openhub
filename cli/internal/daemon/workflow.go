package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

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
	res, err := d.opts.Checkpoints.Reached(r.Context(), d.rootSession(req.ToolSession), req.Call)
	if err != nil {
		d.workflowErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
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
