package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/report"
)

// Reports of workflow_report (modernisation L4): routes of the workflow MCP
// server, authenticated like the other workflow routes. The author of a
// report is the agent the daemon saw in the calling tool session: a
// request carries no author.

// reportRequestMax bounds the body of POST /v1/workflow/report (the report
// itself is bounded by the ReportService, after cleaning).
const reportRequestMax = 1 << 20

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

func reportView(r domain.Report) WorkflowReport { return WorkflowReport{ID: r.ID(), Report: r} }

// callerAgent returns the agent oh saw in a tool session of root: the
// subagent of a child session, else the agent of the session's own
// messages, else its entry agent.
func (d *Daemon) callerAgent(ctx context.Context, root, toolSession string) string {
	d.wmu.Lock()
	ws := make([]*watcher, 0, len(d.watchers))
	for _, w := range d.watchers {
		ws = append(ws, w)
	}
	d.wmu.Unlock()
	for _, w := range ws {
		w.mu.Lock()
		agent := w.childAgent[toolSession]
		if agent == "" && toolSession == root {
			if t := w.tracks[root]; t != nil {
				agent = t.own
			}
		}
		if agent == "" {
			if r := w.runs[toolSession]; r != nil {
				agent = r.agent
			}
		}
		w.mu.Unlock()
		if agent != "" {
			return agent
		}
	}
	if toolSession == root && d.opts.Sessions != nil {
		if s, err := d.opts.Sessions.Get(ctx, root); err == nil {
			return s.EntryAgent
		}
	}
	return ""
}

func (d *Daemon) reportErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, report.ErrEmpty), errors.Is(err, report.ErrTooLarge), errors.Is(err, report.ErrBadID):
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, report.ErrFull):
		writeErr(w, http.StatusInsufficientStorage, err.Error())
	case errors.Is(err, domain.ErrNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}

// reportsOff answers when the daemon keeps no report.
func (d *Daemon) reportsOff(w http.ResponseWriter) bool {
	if d.opts.Reports == nil {
		writeErr(w, http.StatusServiceUnavailable, i18n.T("cmd.daemon.workflow.reports_off"))
		return true
	}
	return false
}

func (d *Daemon) handleWorkflowReport(w http.ResponseWriter, r *http.Request) {
	if d.reportsOff(w) {
		return
	}
	var req WorkflowReportRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, reportRequestMax)).Decode(&req); err != nil || req.ToolSession == "" {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeErr(w, http.StatusRequestEntityTooLarge, report.ErrTooLarge.Error())
			return
		}
		writeErr(w, http.StatusBadRequest, "tool_session and body are required")
		return
	}
	root, ok := d.authorizeWorkflow(w, r, req.ToolSession)
	if !ok {
		return
	}
	agent := d.callerAgent(r.Context(), root, req.ToolSession)
	rp, err := d.opts.Reports.Add(r.Context(), root, agent, req.ToolSession, req.Title, req.Body)
	if err != nil {
		d.reportErr(w, err)
		return
	}
	d.reported(r.Context(), root, rp)
	writeJSON(w, http.StatusCreated, reportView(rp))
}

func (d *Daemon) handleWorkflowReports(w http.ResponseWriter, r *http.Request) {
	if d.reportsOff(w) {
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
	list, err := d.opts.Reports.List(r.Context(), root)
	if err != nil {
		d.reportErr(w, err)
		return
	}
	out := make([]WorkflowReport, 0, len(list))
	for _, rp := range list {
		out = append(out, reportView(rp))
	}
	writeJSON(w, http.StatusOK, out)
}

func (d *Daemon) handleWorkflowReportGet(w http.ResponseWriter, r *http.Request) {
	if d.reportsOff(w) {
		return
	}
	q := r.URL.Query()
	id, rid := q.Get("tool_session"), q.Get("id")
	if id == "" || rid == "" {
		writeErr(w, http.StatusBadRequest, "tool_session and id are required")
		return
	}
	root, ok := d.authorizeWorkflow(w, r, id)
	if !ok {
		return
	}
	rp, body, err := d.opts.Reports.Get(r.Context(), root, rid)
	if err != nil {
		d.reportErr(w, err)
		return
	}
	out := reportView(rp)
	out.Body = body
	if sec := strings.TrimSpace(q.Get("section")); sec != "" {
		part, found := report.Section(body, sec)
		if !found {
			writeErr(w, http.StatusNotFound, i18n.Tf("cmd.daemon.workflow.no_section", sec, rp.ID(), strings.Join(rp.Sections, " · ")))
			return
		}
		out.Body, out.Section = part, sec
	}
	writeJSON(w, http.StatusOK, out)
}

// WorkflowReport stores a report filed by workflow_report.
func (c *Client) WorkflowReport(ctx context.Context, toolSession, title, body string) (WorkflowReport, error) {
	var out WorkflowReport
	err := c.do(ctx, http.MethodPost, "/workflow/report", WorkflowReportRequest{ToolSession: toolSession, Title: title, Body: body}, &out)
	return out, err
}

// WorkflowReports lists the reports of the oh session of a tool session.
func (c *Client) WorkflowReports(ctx context.Context, toolSession string) ([]WorkflowReport, error) {
	var out []WorkflowReport
	err := c.do(ctx, http.MethodGet, "/workflow/reports?tool_session="+url.QueryEscape(toolSession), nil, &out)
	return out, err
}

// WorkflowReportGet returns a report (or one of its sections).
func (c *Client) WorkflowReportGet(ctx context.Context, toolSession, id, section string) (WorkflowReport, error) {
	q := url.Values{"tool_session": {toolSession}, "id": {id}}
	if section != "" {
		q.Set("section", section)
	}
	var out WorkflowReport
	err := c.do(ctx, http.MethodGet, "/workflow/report?"+q.Encode(), nil, &out)
	return out, err
}

// reported is called once a report is stored.
func (d *Daemon) reported(_ context.Context, root string, rp domain.Report) {
	slog.Debug("ohd: report stored", "session", root, "report", rp.ID(), "agent", rp.Agent)
}
