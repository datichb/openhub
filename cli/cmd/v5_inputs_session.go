package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
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
	return map[string]workflowsvc.InputSource{
		workflow.SourceSessionOutput: func(ctx context.Context, c workflowsvc.Context, arg string) (string, error) {
			wfID, out, _ := strings.Cut(arg, ".")
			sess, err := pickSession(ctx, a, c, wfID, func(s *domain.Session) bool { return outputText(s.Outputs[out]) != "" })
			if err != nil {
				return "", err
			}
			v := outputText(sess.Outputs[out])
			note(i18n.Tf("cmd.run.input_from_session", workflow.SourceSessionOutput+"("+arg+")", sess.ID, v))
			return v, nil
		},
		workflow.SourceSessionTrace: func(ctx context.Context, c workflowsvc.Context, wfID string) (string, error) {
			sess, err := pickSession(ctx, a, c, wfID, func(s *domain.Session) bool { return s.State != domain.RunActive })
			if err != nil {
				return "", err
			}
			ssvc, err := newSessionService(ctx, a)
			if err != nil {
				return "", err
			}
			tr, err := ssvc.Trace(ctx, sess.ID, sessionsvc.TraceOptions{Usage: sqlite.NewUsageStore(store)})
			if err != nil {
				return "", err
			}
			note(i18n.Tf("cmd.run.input_from_session", workflow.SourceSessionTrace+"("+wfID+")", sess.ID, ""))
			d := tr.Digest()
			if len(d) > traceDigestMax {
				d = d[:traceDigestMax]
			}
			return d, nil
		},
	}
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
