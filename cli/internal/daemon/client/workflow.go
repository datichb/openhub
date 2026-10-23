package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
)

// WorkflowRefresh asks the daemon to re-apply the session rules of a session.
func (c *Client) WorkflowRefresh(ctx context.Context, sessionID string) error {
	return c.do(ctx, http.MethodPost, api.RouteWorkflowRules, api.WorkflowRulesRequest{SessionID: sessionID}, nil)
}

// WorkflowStatus returns the workflow status of the oh session a tool session belongs to.
func (c *Client) WorkflowStatus(ctx context.Context, toolSession string) (checkpoint.Status, error) {
	var out checkpoint.Status
	err := c.do(ctx, http.MethodGet, api.RouteWorkflowStatus+"?tool_session="+url.QueryEscape(toolSession), nil, &out)
	return out, err
}

// WorkflowCheckpoint records a workflow_checkpoint call the tool let run.
func (c *Client) WorkflowCheckpoint(ctx context.Context, toolSession string, call checkpoint.Call) (checkpoint.Result, error) {
	var out checkpoint.Result
	err := c.do(ctx, http.MethodPost, api.RouteWorkflowCheckpoint, api.WorkflowCheckpointRequest{ToolSession: toolSession, Call: call}, &out)
	return out, err
}

// WorkflowOutput records an output declared by workflow_outputs.
func (c *Client) WorkflowOutput(ctx context.Context, toolSession string, o checkpoint.Output) error {
	return c.do(ctx, http.MethodPost, api.RouteWorkflowOutputs, api.WorkflowOutputRequest{ToolSession: toolSession, Output: o}, nil)
}
