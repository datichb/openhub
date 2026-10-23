package api

import (
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

// WorkflowRulesRequest is the body of POST /v1/workflow/rules.
type WorkflowRulesRequest struct {
	SessionID string `json:"session_id"`
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

// WorkflowReportRequest is the body of POST /v1/workflow/report.
type WorkflowReportRequest struct {
	ToolSession string `json:"tool_session"`
	Title       string `json:"title,omitempty"`
	Body        string `json:"body"`
}

// WorkflowReport is a stored report, with its body when one is asked.
type WorkflowReport struct {
	ID string `json:"id"`
	domain.Report
	// Body is the report, or the section asked (report_get).
	Body    string `json:"body,omitempty"`
	Section string `json:"section,omitempty"`
}
