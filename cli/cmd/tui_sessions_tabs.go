package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/filelock"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/services/fanout"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// Tabs of the session detail in the TUI (modernisation tui-sessions): the
// outputs recorded by oh and the `next:` steps (`oh session next`), the
// trace (`oh session trace`), the reports (`oh session report`) and the
// fan-out (`oh fanout status|drive|clean`). Same services as the CLI.

var (
	_ views.SessionNexter       = (*tuiSessions)(nil)
	_ views.SessionTracer       = (*tuiSessions)(nil)
	_ views.SessionReportLister = (*tuiSessions)(nil)
	_ views.SessionFanouter     = (*tuiSessions)(nil)
)

// sessionOutputs are the outputs of a session, sorted by name.
func sessionOutputs(out map[string]any) []views.SessionOutput {
	if len(out) == 0 {
		return nil
	}
	list := make([]views.SessionOutput, 0, len(out))
	for k, v := range out {
		list = append(list, views.SessionOutput{Name: k, Value: outputText(v)})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
	return list
}

// fanoutLine is the fan-out line of the detail of a session.
func fanoutLine(s *domain.Session) string {
	f := s.Fanout
	if f == nil {
		return ""
	}
	if f.Role != domain.RoleParent {
		return i18n.Tf("tui.sessions.fanout.line_child", f.Parent, firstNonEmpty(f.Ticket, "—"), firstNonEmpty(f.Branch, "—"))
	}
	merged := 0
	for _, c := range f.Children {
		if c.State == domain.ChildMerged {
			merged++
		}
	}
	return i18n.Tf("tui.sessions.fanout.line_parent", merged, len(f.Children), i18n.T("tui.sessions.fanout.status."+string(f.Status)))
}

// NextSteps are the `next:` steps of a session whose conditions hold.
func (t *tuiSessions) NextSteps(ctx context.Context, sessionID string) ([]views.SessionNextStep, error) {
	if t.a.Sessions == nil {
		return nil, nil
	}
	sess, err := t.a.Sessions.Get(ctx, sessionID)
	if err != nil || sess.WorkflowID == "" {
		return nil, err
	}
	steps, err := newWorkflowService(ctx).NextSteps(ctx, workflowsvc.Context{ProjectID: sess.ProjectID}, sess.WorkflowID, sess.Outputs)
	if err != nil {
		return nil, err
	}
	out := make([]views.SessionNextStep, 0, len(steps))
	for _, s := range steps {
		out = append(out, views.SessionNextStep{Workflow: s.WorkflowID, Label: s.Label, Inputs: stepInputs(s)})
	}
	return out, nil
}

// stepInputs are the inputs of a suggestion, "k=v" separated by spaces.
func stepInputs(s workflowsvc.ChainSuggestion) string {
	var in []string
	if len(s.Tickets) > 0 {
		in = append(in, "tickets="+strings.Join(s.Tickets, ","))
	}
	for _, k := range sortedStringKeys(s.Prefill) {
		in = append(in, k+"="+s.Prefill[k])
	}
	return strings.Join(in, " ")
}

// SessionTrace is the trace of a session as `oh session trace` prints it.
func (t *tuiSessions) SessionTrace(ctx context.Context, sessionID string) (string, error) {
	svc, err := t.service(ctx)
	if err != nil {
		return "", err
	}
	opts := sessionsvc.TraceOptions{}
	if store != nil {
		opts.Usage = sqlite.NewUsageStore(store)
	}
	tr, err := svc.Trace(ctx, sessionID, opts)
	if err != nil {
		return "", err
	}
	appendNextEvents(ctx, t.a, tr, sessionID)
	var b bytes.Buffer
	printTrace(&b, tr)
	return b.String(), nil
}

// SessionReports lists the reports of a session, oldest first. On the
// parent of a fan-out it adds the reports of each child that has a session,
// with the child that filed them (a child that cannot be read is left out).
func (t *tuiSessions) SessionReports(ctx context.Context, sessionID string) ([]views.CheckpointReport, error) {
	svc, err := t.service(ctx)
	if err != nil {
		return nil, err
	}
	list, err := svc.SessionReports(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	out := make([]views.CheckpointReport, 0, len(list))
	for _, r := range list {
		out = append(out, views.CheckpointReport{ID: r.ID(), Agent: r.Agent, Title: r.Title, Bytes: r.Bytes, At: r.CreatedAt})
	}
	if t.a.Sessions == nil {
		return out, nil
	}
	sess, err := t.a.Sessions.Get(ctx, sessionID)
	if err != nil {
		return out, nil //nolint:nilerr // the own reports are still shown
	}
	if sess.Fanout == nil || sess.Fanout.Role != domain.RoleParent {
		return out, nil
	}
	for _, c := range sess.Fanout.Children {
		if c.Session == "" {
			continue
		}
		kids, err := svc.SessionReports(ctx, c.Session)
		if err != nil {
			continue
		}
		for _, r := range kids {
			out = append(out, views.CheckpointReport{ID: r.ID(), Agent: r.Agent, Title: r.Title, Bytes: r.Bytes, At: r.CreatedAt,
				Session: c.Session, Child: fanout.ChildLabel(c)})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

// fanoutParent is the parent session of the fan-out of a session.
func (t *tuiSessions) fanoutParent(ctx context.Context, sessionID string) (*domain.Session, error) {
	if t.a.Sessions == nil {
		return nil, errors.New(i18n.Tf("cmd.fanout.none", sessionID))
	}
	sess, err := t.a.Sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if sess.Fanout != nil && sess.Fanout.Role != domain.RoleParent && sess.Fanout.Parent != "" {
		if sess, err = t.a.Sessions.Get(ctx, sess.Fanout.Parent); err != nil {
			return nil, err
		}
	}
	if sess.Fanout == nil || sess.Fanout.Role != domain.RoleParent {
		return nil, errors.New(i18n.Tf("cmd.fanout.none", sessionID))
	}
	return sess, nil
}

// fanoutDriverRunning reports whether the driver of a fan-out holds its
// lock.
func fanoutDriverRunning(parent string) bool {
	unlock, err := filelock.TryLock(filepath.Join(ohSessionsDir(), parent, "fanout.lock"))
	if err != nil {
		return errors.Is(err, filelock.ErrLocked)
	}
	unlock()
	return false
}

// Fanout is the fan-out of a session (its parent's for a child).
func (t *tuiSessions) Fanout(ctx context.Context, sessionID string) (views.FanoutView, error) {
	sess, err := t.fanoutParent(ctx, sessionID)
	if err != nil {
		return views.FanoutView{}, err
	}
	f := sess.Fanout
	v := views.FanoutView{Parent: sess.ID, Experimental: f.Experimental, Status: string(f.Status), Parallel: f.Parallel,
		Driver: fanoutDriverRunning(sess.ID), BudgetUSD: f.BudgetUSD, SpentUSD: f.SpentUSD, FixRounds: len(f.Fixes),
		FixMax: fanout.DefaultFixRounds, Cleanable: fanout.Cleanable(sess), Cleaned: f.Cleaned, Commits: -1}
	if svc, err := t.service(ctx); err == nil && svc.Checkpoints != nil {
		if wf, _, err := svc.Checkpoints.Workflow(ctx, sess.ID); err == nil && wf != nil {
			v.FixMax = fanout.FixRounds(wf)
		}
	}
	if x, ok := f.LastFix(); ok {
		v.LastRefusal = x.Request
	}
	if ig := f.Integration; ig != nil {
		v.Integration = ig.Branch
		if !f.Cleaned && ig.Path != "" && ig.Base != "" {
			if n, err := fanout.CommitsAhead(ctx, ig.Path, ig.Base, ig.Branch); err == nil {
				v.Commits = n
			}
		}
	}
	ends := childEnds(f)
	for _, c := range f.Children {
		cv := views.FanoutChildView{Ticket: fanout.ChildLabel(c), Session: c.Session, State: string(c.State), Branch: c.Branch,
			CostUSD: c.CostUSD, End: ends[c.Ticket], Detail: c.Error}
		if cv.Detail == "" && len(c.Conflicts) > 0 {
			cv.Detail = strings.Join(c.Conflicts, ", ")
		}
		v.Children = append(v.Children, cv)
	}
	return v, nil
}

// childEnds is why each child ended (last child_done event, by ticket).
func childEnds(f *domain.SessionFanout) map[string]string {
	out := map[string]string{}
	for _, e := range f.Events {
		if e.Kind != domain.FanoutEventChildDone || e.Ticket == "" {
			continue
		}
		why, _, _ := strings.Cut(e.Message, ", ")
		out[e.Ticket] = strings.TrimSpace(why)
	}
	return out
}

// fanoutDriverNeeded reports whether the driver of a parent has work left
// (as the oh daemon decides it, p6-debts).
func fanoutDriverNeeded(s *domain.Session) bool {
	f := s.Fanout
	if f == nil || f.Role != domain.RoleParent || !f.Experimental || s.State == domain.RunStopped || s.State == domain.RunFailed {
		return false
	}
	switch f.Status {
	case domain.FanoutWaiting, domain.FanoutRunning, domain.FanoutReady, domain.FanoutFixing:
		return true
	}
	return false
}

// RelaunchFanout starts the driver of a fan-out again (`oh fanout drive`
// in the background), when it has work left and does not run.
func (t *tuiSessions) RelaunchFanout(ctx context.Context, parentID string) (string, error) {
	if !fanoutEnabled(t.a) {
		return "", errors.New(i18n.T("cmd.fanout.disabled"))
	}
	sess, err := t.fanoutParent(ctx, parentID)
	if err != nil {
		return "", err
	}
	if !fanoutDriverNeeded(sess) {
		return "", errors.New(i18n.Tf("tui.sessions.fanout.driver_done", string(sess.Fanout.Status)))
	}
	if fanoutDriverRunning(sess.ID) {
		return "", errors.New(i18n.T("tui.sessions.fanout.driver_running"))
	}
	log, err := spawnDriver(sess.ID, relaunchByUserArg)
	if err != nil {
		return "", err
	}
	return i18n.Tf("tui.sessions.fanout.relaunched", log), nil
}

// CleanFanout cleans up (or plans, dry) the fan-out of a parent, as `oh
// fanout clean [--discard] [--dry-run]`.
func (t *tuiSessions) CleanFanout(ctx context.Context, parentID string, discard, dry bool) (views.FanoutCleanView, error) {
	sess, err := t.fanoutParent(ctx, parentID)
	if err != nil {
		return views.FanoutCleanView{}, err
	}
	if !fanout.Cleanable(sess) {
		return views.FanoutCleanView{}, errors.New(i18n.Tf("cmd.fanout.clean.not_done", sess.ID, string(sess.Fanout.Status)))
	}
	if store == nil {
		return views.FanoutCleanView{}, fmt.Errorf("fan-out %s: no session store", sess.ID)
	}
	res, err := fanout.Clean(ctx, sqlite.NewSessionStore(store), sess, fanout.CleanOptions{DryRun: dry, Discard: discard})
	if err != nil {
		return views.FanoutCleanView{}, err
	}
	return views.FanoutCleanView{Worktrees: res.Worktrees, Branches: res.Branches, Discarded: res.Discarded, Kept: res.Kept}, nil
}
