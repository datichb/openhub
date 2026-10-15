package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/report"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// Inputs read from a previous session (R5, `from: session.output(…)` and
// `from: session.trace(…)`): the session the launch is chained from
// (--parent) when it fits, else the last session of the project. note
// receives the session used, for the launch output.

// traceDigestMax bounds the trace summary given to an input.
const traceDigestMax = 8000

func sessionInputSources(a *app.App, note func(string)) map[string]workflowsvc.InputSource {
	if note == nil {
		note = func(string) {}
	}
	sources := map[string]workflowsvc.InputSource{
		workflow.SourceSessionOutput: func(ctx context.Context, c workflowsvc.Context, full string) (string, error) {
			arg, equals := workflowsvc.ParseFromArg(full)
			wfID, out, _ := strings.Cut(arg, ".")
			sess, err := pickSession(ctx, a, c, wfID, func(s *domain.Session) bool {
				if outputText(s.Outputs[out]) == "" {
					return false
				}
				// p6-debts: only the sessions whose outputs match (from_equals).
				for k, want := range equals {
					if outputText(s.Outputs[k]) != want {
						return false
					}
				}
				return true
			})
			if err != nil {
				note(i18n.Tf("cmd.run.input_from_session_none", workflow.SourceSessionOutput+"("+full+")"))
				return "", err
			}
			v := outputText(sess.Outputs[out])
			note(i18n.Tf("cmd.run.input_from_session", workflow.SourceSessionOutput+"("+full+")", sess.ID, v))
			// B1: a report goes on as its content (the findings of an audit).
			if sessionOutputType(ctx, c, sess.WorkflowID, out) == workflow.OutputReport {
				return sessionReportBody(ctx, a, sess.ID, v)
			}
			return v, nil
		},
		workflow.SourceSessionTrace: func(ctx context.Context, c workflowsvc.Context, wfID string) (string, error) {
			sess, err := pickSession(ctx, a, c, wfID, func(s *domain.Session) bool { return s.State != domain.RunActive })
			if err != nil {
				return "", err
			}
			d, err := sessionTraceDigest(ctx, a, sess.ID)
			if err != nil {
				return "", err
			}
			note(i18n.Tf("cmd.run.input_from_session", workflow.SourceSessionTrace+"("+wfID+")", sess.ID, ""))
			return clipUTF8(d, traceDigestMax), nil
		},
	}
	// r9b: the sessions named by an input (ids, period, workflows).
	sources[workflow.SourceSessionTraces] = func(ctx context.Context, c workflowsvc.Context, value string) (string, error) {
		if strings.TrimSpace(value) == "" {
			return sources[workflow.SourceSessionTrace](ctx, c, "")
		}
		sel, err := parseTraceSelection(ctx, a, value, time.Now())
		if err != nil {
			return "", err
		}
		list, err := pickTraceSessions(ctx, a, c, sel)
		if err != nil {
			return "", err
		}
		note(i18n.Tf("cmd.run.input_from_session", workflow.SourceSessionTraces+"("+value+")", sessionIDs(list), ""))
		return traceDigests(ctx, a, list)
	}
	return sources
}

// sessionOutputType returns the type of output out of workflow wfID ("" when
// unknown).
var sessionOutputType = func(ctx context.Context, c workflowsvc.Context, wfID, out string) workflow.OutputType {
	res, _ := newWorkflowService(ctx).Resolve(ctx, workflowsvc.Context{ProjectID: c.ProjectID}, wfID, workflowsvc.ResolveOpts{})
	if res == nil {
		return ""
	}
	o, _ := res.Spec.Output(out)
	return o.Type
}

// sessionReportBody returns the report id of a session, cleaned.
var sessionReportBody = func(ctx context.Context, a *app.App, sessionID, id string) (string, error) {
	ssvc, err := newSessionService(ctx, a)
	if err != nil {
		return "", err
	}
	_, body, err := ssvc.Report(ctx, sessionID, id)
	if err != nil {
		return "", fmt.Errorf("report %s of session %s: %w", id, sessionID, err)
	}
	return report.Clean(body), nil
}

// pickSession returns the parent session of the launch when it is a session
// of wfID ("" = any) that fits, else the last session of the project of wfID
// that fits.
func pickSession(ctx context.Context, a *app.App, c workflowsvc.Context, wfID string, fits func(*domain.Session) bool) (*domain.Session, error) {
	if a.Sessions == nil {
		return nil, errors.New("no session store")
	}
	if c.FromSession != "" {
		if s, err := a.Sessions.Get(ctx, c.FromSession); err == nil && (wfID == "" || s.WorkflowID == wfID) && fits(s) {
			return s, nil
		}
	}
	list, err := a.Sessions.List(ctx, c.ProjectID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].StartedAt.After(list[j].StartedAt) })
	for i := range list {
		s := &list[i]
		if s.WorkflowID == "" || (wfID != "" && s.WorkflowID != wfID) || !fits(s) {
			continue
		}
		return s, nil
	}
	if wfID == "" {
		return nil, errors.New(i18n.T("cmd.run.no_previous_session"))
	}
	return nil, errors.New(i18n.Tf("cmd.run.no_previous_session_of", wfID))
}

// outputText is a session output as an input value (lists comma-joined).
func outputText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []string:
		return strings.Join(x, ",")
	case []any:
		parts := make([]string, 0, len(x))
		for _, e := range x {
			parts = append(parts, fmt.Sprint(e))
		}
		return strings.Join(parts, ",")
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

// noteTo prints the notes of a launch.
func noteTo(w io.Writer) func(string) {
	return func(line string) { fmt.Fprintln(w, "  "+line) }
}
