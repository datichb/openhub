package cmd

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// Source session.traces(<input>) (r9b, retro): the trace summaries of the
// sessions the value of an input names. Words, separated by commas or
// spaces: session ids, a period (7d, 24h, 2w, or a date: since that day),
// workflow ids (only their sessions). Without ids, the most recent finished
// sessions of the project that fit, at most tracesMaxSessions, the launched
// workflow's own sessions aside; an empty value is session.trace() (the
// parent session, else the last finished one).

const (
	tracesMaxSessions = 6
	tracesDigestMax   = 16000
)

var (
	rePeriod = regexp.MustCompile(`^(\d+)([hdw])$`)
	reDay    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	reWfWord = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

// traceSelection is the parsed value of the input of session.traces.
type traceSelection struct {
	ids       []string
	since     time.Time
	workflows []string
}

func parseTraceSelection(ctx context.Context, a *app.App, value string, now time.Time) (traceSelection, error) {
	var sel traceSelection
	for _, w := range strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' }) {
		switch {
		case strings.HasPrefix(w, "ses_"):
			sel.ids = append(sel.ids, w)
		case rePeriod.MatchString(w):
			m := rePeriod.FindStringSubmatch(w)
			n, _ := strconv.Atoi(m[1])
			unit := map[string]time.Duration{"h": time.Hour, "d": 24 * time.Hour, "w": 7 * 24 * time.Hour}[m[2]]
			sel.since = now.Add(-time.Duration(n) * unit)
		case reDay.MatchString(w):
			d, err := time.ParseInLocation("2006-01-02", w, now.Location())
			if err != nil {
				return sel, errors.New(i18n.Tf("cmd.run.traces_word", w))
			}
			sel.since = d
		case reWfWord.MatchString(w):
			sel.workflows = append(sel.workflows, w)
		default:
			if a.Sessions != nil {
				if _, err := a.Sessions.Get(ctx, w); err == nil {
					sel.ids = append(sel.ids, w)
					continue
				}
			}
			return sel, errors.New(i18n.Tf("cmd.run.traces_word", w))
		}
	}
	return sel, nil
}

// pickTraceSessions returns the sessions of a selection, most recent first
// (named ones in the given order).
func pickTraceSessions(ctx context.Context, a *app.App, c workflowsvc.Context, sel traceSelection) ([]*domain.Session, error) {
	if a.Sessions == nil {
		return nil, errors.New("no session store")
	}
	var out []*domain.Session
	for _, id := range sel.ids {
		s, err := a.Sessions.Get(ctx, id)
		if err != nil || s.ProjectID != c.ProjectID {
			return nil, errors.New(i18n.Tf("cmd.run.traces_unknown_session", id))
		}
		out = append(out, s)
	}
	if len(sel.ids) > 0 {
		return out, nil
	}
	list, err := a.Sessions.List(ctx, c.ProjectID)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].StartedAt.After(list[j].StartedAt) })
	for i := range list {
		s := &list[i]
		switch {
		case s.WorkflowID == "" || s.State == domain.RunActive:
			continue
		case len(sel.workflows) > 0 && !containsString(sel.workflows, s.WorkflowID):
			continue
		case len(sel.workflows) == 0 && s.WorkflowID == c.Workflow:
			continue // a retrospective does not look at itself unless asked
		case !sel.since.IsZero() && s.StartedAt.Before(sel.since):
			continue
		}
		out = append(out, s)
		if len(out) == tracesMaxSessions {
			break
		}
	}
	if len(out) == 0 {
		return nil, errors.New(i18n.T("cmd.run.traces_none"))
	}
	return out, nil
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// sessionTraceDigest returns the trace summary of a session.
var sessionTraceDigest = func(ctx context.Context, a *app.App, id string) (string, error) {
	ssvc, err := newSessionService(ctx, a)
	if err != nil {
		return "", err
	}
	tr, err := ssvc.Trace(ctx, id, sessionsvc.TraceOptions{Usage: sqlite.NewUsageStore(store)})
	if err != nil {
		return "", fmt.Errorf("trace of %s: %w", id, err)
	}
	return tr.Digest(), nil
}

// traceDigests joins the trace summaries of sessions, each bounded so that
// the whole stays under tracesDigestMax.
func traceDigests(ctx context.Context, a *app.App, sessions []*domain.Session) (string, error) {
	per := tracesDigestMax/len(sessions) - 64
	var b strings.Builder
	for i, s := range sessions {
		d, err := sessionTraceDigest(ctx, a, s.ID)
		if err != nil {
			return "", err
		}
		if i > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "=== Session %d/%d ===\n%s", i+1, len(sessions), clipUTF8(d, per))
	}
	return b.String(), nil
}

func sessionIDs(list []*domain.Session) string {
	ids := make([]string, len(list))
	for i, s := range list {
		ids[i] = s.ID
	}
	return strings.Join(ids, ", ")
}

// clipUTF8 cuts s to at most n bytes, on a character boundary.
func clipUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}
