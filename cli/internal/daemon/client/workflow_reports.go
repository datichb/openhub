package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/datichb/openhub/cli/internal/daemon/api"
)

// WorkflowReport stores a report filed by workflow_report.
func (c *Client) WorkflowReport(ctx context.Context, toolSession, title, body string) (api.WorkflowReport, error) {
	var out api.WorkflowReport
	err := c.do(ctx, http.MethodPost, api.RouteWorkflowReport, api.WorkflowReportRequest{ToolSession: toolSession, Title: title, Body: body}, &out)
	return out, err
}

// WorkflowReports lists the reports of the oh session of a tool session.
func (c *Client) WorkflowReports(ctx context.Context, toolSession string) ([]api.WorkflowReport, error) {
	var out []api.WorkflowReport
	err := c.do(ctx, http.MethodGet, api.RouteWorkflowReports+"?tool_session="+url.QueryEscape(toolSession), nil, &out)
	return out, err
}

// WorkflowReportGet returns a report (or one of its sections).
func (c *Client) WorkflowReportGet(ctx context.Context, toolSession, id, section string) (api.WorkflowReport, error) {
	q := url.Values{"tool_session": {toolSession}, "id": {id}}
	if section != "" {
		q.Set("section", section)
	}
	var out api.WorkflowReport
	err := c.do(ctx, http.MethodGet, api.RouteWorkflowReport+"?"+q.Encode(), nil, &out)
	return out, err
}
