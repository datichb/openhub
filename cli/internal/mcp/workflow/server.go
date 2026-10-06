// Package workflow is the oh MCP server `workflow` (P3-T01, 03 §10), run by
// the tool for each session location (`oh mcp serve workflow`). Agents call
// it at each checkpoint of their workflow:
//
//   - workflow_status      current step, checkpoints and their behavior in
//     the session mode, locked agents, outputs;
//   - workflow_checkpoint  {id, summary}: the tool asks oh (or the user)
//     before running it when the checkpoint pauses in this mode; once it
//     runs, the checkpoint is validated and the answer carries the
//     instruction given with the validation;
//   - workflow_outputs     {type, value[, id]}: declares a typed output (O7).
//
// The server holds no state: everything goes to the oh daemon
// (CheckpointService), for the session that calls the tool.
package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/mcp/protocol"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Backend is the CheckpointService as seen from the MCP server (the oh
// daemon client).
type Backend interface {
	WorkflowStatus(ctx context.Context, toolSession string) (checkpoint.Status, error)
	WorkflowCheckpoint(ctx context.Context, toolSession string, call checkpoint.Call) (checkpoint.Result, error)
	WorkflowOutput(ctx context.Context, toolSession string, o checkpoint.Output) error
}

// Serve runs the server on stdio against the oh daemon.
func Serve() error {
	return New(daemon.NewClient(daemon.DefaultPaths())).Serve()
}

// New returns the MCP server backed by b.
func New(b Backend) *protocol.Server {
	s := protocol.NewServer("oh-workflow", "1.0.0")
	h := handlers{b: b}
	s.RegisterTool(protocol.Tool{
		Name:        sessionspec.WorkflowToolStatus,
		Description: i18n.T("cmd.mcp.workflow.status_desc"),
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}, h.status)
	s.RegisterTool(protocol.Tool{
		Name:        sessionspec.WorkflowToolCheckpoint,
		Description: i18n.T("cmd.mcp.workflow.checkpoint_desc"),
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":      map[string]any{"type": "string", "description": i18n.T("cmd.mcp.workflow.checkpoint_id")},
				"summary": map[string]any{"type": "string", "description": i18n.T("cmd.mcp.workflow.checkpoint_summary")},
			},
			"required": []string{"id", "summary"},
		},
	}, h.checkpoint)
	s.RegisterTool(protocol.Tool{
		Name:        sessionspec.WorkflowToolOutputs,
		Description: i18n.T("cmd.mcp.workflow.outputs_desc"),
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"type":  map[string]any{"type": "string", "description": i18n.T("cmd.mcp.workflow.output_type")},
				"value": map[string]any{"type": "string", "description": i18n.T("cmd.mcp.workflow.output_value")},
				"id":    map[string]any{"type": "string", "description": i18n.T("cmd.mcp.workflow.output_id")},
			},
			"required": []string{"type", "value"},
		},
	}, h.outputs)
	return s
}

type handlers struct{ b Backend }

// ToolSession returns the tool session of the call, read from the request
// `_meta` (opencode: "ai.opencode/sessionID"; any key ending with
// "sessionid" is accepted).
func ToolSession(ctx context.Context) string {
	for k, v := range protocol.Meta(ctx) {
		if s, ok := v.(string); ok && s != "" && strings.HasSuffix(strings.ToLower(k), "sessionid") {
			return s
		}
	}
	return ""
}

func text(s string) *protocol.ToolResult {
	return &protocol.ToolResult{Content: []protocol.ContentBlock{{Type: "text", Text: s}}}
}

func session(ctx context.Context) (string, error) {
	id := ToolSession(ctx)
	if id == "" {
		return "", errors.New(i18n.T("cmd.mcp.workflow.no_session"))
	}
	return id, nil
}

func (h handlers) status(ctx context.Context, _ json.RawMessage) (*protocol.ToolResult, error) {
	id, err := session(ctx)
	if err != nil {
		return nil, err
	}
	st, err := h.b.WorkflowStatus(ctx, id)
	if err != nil {
		return nil, unavailable(err)
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return nil, err
	}
	return text(string(data)), nil
}

func (h handlers) checkpoint(ctx context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var call checkpoint.Call
	if err := json.Unmarshal(params, &call); err != nil || strings.TrimSpace(call.ID) == "" {
		return nil, errors.New(i18n.T("cmd.mcp.workflow.checkpoint_id_required"))
	}
	call.ID = strings.TrimSpace(call.ID)
	id, err := session(ctx)
	if err != nil {
		return nil, err
	}
	res, err := h.b.WorkflowCheckpoint(ctx, id, call)
	if errors.Is(err, daemon.ErrNotRunning) {
		// The tool already let the call run (validated, or not paused in
		// this mode): only the bookkeeping is lost.
		return text(i18n.Tf("cmd.mcp.workflow.checkpoint_untracked", call.ID)), nil
	}
	if err != nil {
		return nil, unavailable(err)
	}
	var b strings.Builder
	b.WriteString(i18n.Tf("cmd.mcp.workflow.checkpoint_ok", res.ID, res.Label))
	if res.Message != "" {
		b.WriteString("\n")
		b.WriteString(i18n.Tf("cmd.mcp.workflow.checkpoint_message", res.Message))
	}
	if res.Next != "" {
		b.WriteString("\n")
		b.WriteString(i18n.Tf("cmd.mcp.workflow.checkpoint_next", res.Next))
	}
	return text(b.String()), nil
}

func (h handlers) outputs(ctx context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var o checkpoint.Output
	if err := json.Unmarshal(params, &o); err != nil || strings.TrimSpace(o.Type) == "" || strings.TrimSpace(o.Value) == "" {
		return nil, errors.New(i18n.T("cmd.mcp.workflow.output_required"))
	}
	id, err := session(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.b.WorkflowOutput(ctx, id, o); err != nil {
		return nil, unavailable(err)
	}
	return text(i18n.Tf("cmd.mcp.workflow.output_ok", o.Type)), nil
}

func unavailable(err error) error {
	if errors.Is(err, daemon.ErrNotRunning) {
		return errors.New(i18n.T("cmd.mcp.workflow.unavailable"))
	}
	var api *daemon.APIError
	if errors.As(err, &api) && api.Message != "" {
		return errors.New(api.Message)
	}
	return err
}
