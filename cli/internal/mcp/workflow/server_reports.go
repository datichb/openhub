package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/datichb/openhub/cli/internal/daemon/api"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/mcp/protocol"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// Reports (modernisation L4): the agent files its whole report, oh keeps it
// (rp-<n>) and the coordinators only pass the id on.

func registerReports(s *protocol.Server, h handlers) {
	s.RegisterTool(protocol.Tool{
		Name:        sessionspec.WorkflowToolReport,
		Description: i18n.T("cmd.mcp.workflow.report_desc"),
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{"type": "string", "description": i18n.T("cmd.mcp.workflow.report_title")},
				"body":  map[string]any{"type": "string", "description": i18n.T("cmd.mcp.workflow.report_body")},
			},
			"required": []string{"body"},
		},
	}, h.report)
	s.RegisterTool(protocol.Tool{
		Name:        sessionspec.WorkflowToolReportGet,
		Description: i18n.T("cmd.mcp.workflow.report_get_desc"),
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":      map[string]any{"type": "string", "description": i18n.T("cmd.mcp.workflow.report_id")},
				"section": map[string]any{"type": "string", "description": i18n.T("cmd.mcp.workflow.report_section")},
			},
			"required": []string{"id"},
		},
	}, h.reportGet)
	s.RegisterTool(protocol.Tool{
		Name:        sessionspec.WorkflowToolReports,
		Description: i18n.T("cmd.mcp.workflow.reports_desc"),
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}, h.reports)
}

// notRecorded reports that the daemon cannot keep reports (stopped, reports
// turned off, a daemon without the route): the agent falls back on its
// whole report.
func notRecorded(err error) bool {
	if errors.Is(err, api.ErrNotRunning) {
		return true
	}
	var ae *api.APIError
	return errors.As(err, &ae) && (ae.Status == http.StatusServiceUnavailable || ae.Status == http.StatusNotFound && strings.HasPrefix(ae.Message, "404 "))
}

func (h handlers) report(ctx context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	// The author is never read from the call: oh knows who calls.
	var in struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.Unmarshal(params, &in); err != nil || strings.TrimSpace(in.Body) == "" {
		return nil, errors.New(i18n.T("cmd.mcp.workflow.report_body_required"))
	}
	id, err := session(ctx)
	if err != nil {
		return nil, err
	}
	rp, err := h.b.WorkflowReport(ctx, id, in.Title, in.Body)
	var unknown *api.APIError
	if notRecorded(err) || errors.As(err, &unknown) && unknown.Status == http.StatusNotFound { // session unknown to oh
		return text(i18n.T("cmd.mcp.workflow.report_not_recorded")), nil
	}
	var ae *api.APIError
	if errors.As(err, &ae) && (ae.Status == http.StatusUnprocessableEntity || ae.Status == http.StatusInsufficientStorage || ae.Status == http.StatusRequestEntityTooLarge) {
		return nil, errors.New(i18n.Tf("cmd.mcp.workflow.report_refused", ae.Message))
	}
	if err != nil {
		return nil, unavailable(err)
	}
	return text(i18n.Tf("cmd.mcp.workflow.report_ok", rp.ID, firstText(rp.Agent, "?"), size(rp.Bytes), rp.ID)), nil
}

func (h handlers) reportGet(ctx context.Context, params json.RawMessage) (*protocol.ToolResult, error) {
	var in struct {
		ID      string `json:"id"`
		Section string `json:"section"`
	}
	if err := json.Unmarshal(params, &in); err != nil || strings.TrimSpace(in.ID) == "" {
		return nil, errors.New(i18n.T("cmd.mcp.workflow.report_id_required"))
	}
	id, err := session(ctx)
	if err != nil {
		return nil, err
	}
	rp, err := h.b.WorkflowReportGet(ctx, id, strings.TrimSpace(in.ID), strings.TrimSpace(in.Section))
	if notRecorded(err) {
		return nil, errors.New(i18n.T("cmd.mcp.workflow.reports_unavailable"))
	}
	if err != nil {
		return nil, unavailable(err)
	}
	return text(i18n.Tf("cmd.mcp.workflow.report_header", rp.ID, firstText(rp.Agent, "?"), rp.Title) + "\n\n" + rp.Body), nil
}

func (h handlers) reports(ctx context.Context, _ json.RawMessage) (*protocol.ToolResult, error) {
	id, err := session(ctx)
	if err != nil {
		return nil, err
	}
	list, err := h.b.WorkflowReports(ctx, id)
	if notRecorded(err) {
		return nil, errors.New(i18n.T("cmd.mcp.workflow.reports_unavailable"))
	}
	if err != nil {
		return nil, unavailable(err)
	}
	if len(list) == 0 {
		return text(i18n.T("cmd.mcp.workflow.reports_none")), nil
	}
	var b strings.Builder
	for _, rp := range list {
		fmt.Fprintf(&b, "%s · %s · « %s » · %s · %s", rp.ID, firstText(rp.Agent, "?"), rp.Title, size(rp.Bytes), rp.CreatedAt.Local().Format("15:04"))
		if len(rp.Sections) > 0 {
			fmt.Fprintf(&b, " · %s", strings.Join(rp.Sections, " | "))
		}
		b.WriteString("\n")
	}
	return text(strings.TrimRight(b.String(), "\n")), nil
}

func firstText(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}

// size renders a size in bytes ("820 o", "2,1 Ko").
func size(n int) string {
	if n < 1024 {
		return i18n.Tf("cmd.mcp.workflow.size_bytes", n)
	}
	return i18n.Tf("cmd.mcp.workflow.size_kib", float64(n)/1024)
}
