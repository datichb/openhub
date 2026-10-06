package cmd

import (
	"context"
	"strings"
	"sync"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// « Enchaîner avec… » in the Sessions view (P1-T27): the WorkflowService
// suggests the workflows taking the outputs of the session; the branch of the
// session directory stands for a `branch` output the session did not declare.

var _ views.SessionChainer = (*tuiSessions)(nil)

var (
	tuiChainMu          sync.Mutex
	tuiChainSuggestions = map[string][]workflowsvc.ChainSuggestion{}
)

// ChainOptions implements views.SessionChainer.
func (t *tuiSessions) ChainOptions(ctx context.Context, sessionID string) ([]views.SelectOption, error) {
	if t.a.Sessions == nil {
		return nil, nil
	}
	sess, err := t.a.Sessions.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	list, err := newWorkflowService(ctx).Chain(ctx, workflowsvc.Context{ProjectID: sess.ProjectID}, sess.WorkflowID, sess.Outputs,
		workflowsvc.SessionFallback(sess.LaunchPath))
	if err != nil {
		return nil, err
	}
	tuiChainMu.Lock()
	tuiChainSuggestions[sessionID] = list
	tuiChainMu.Unlock()
	opts := make([]views.SelectOption, 0, len(list)+1)
	if intent, ok := loadResumeIntent(ctx, t.a, sess.ProjectID, sessionID); ok {
		opts = append(opts, views.SelectOption{Label: i18n.Tf("tui.launch.chain_resume", intent.Workflow), Value: resumeChoice + intent.Workflow})
	}
	for _, s := range list {
		var with []string
		for k, v := range s.Prefill {
			with = append(with, k+"="+v)
		}
		if len(s.Tickets) > 0 {
			with = append(with, strings.Join(s.Tickets, ","))
		}
		label := s.WorkflowID
		if s.Label != "" && s.Label != s.WorkflowID {
			label += " — " + s.Label
		}
		if len(with) > 0 {
			label += "  (" + strings.Join(with, ", ") + ")"
		}
		opts = append(opts, views.SelectOption{Label: label, Value: s.WorkflowID})
	}
	return opts, nil
}

// Chain implements views.SessionChainer: the launch form of the suggestion,
// prefilled and chained to the session.
func (t *tuiSessions) Chain(sessionID, workflowID string) {
	if strings.HasPrefix(workflowID, resumeChoice) {
		t.resume(sessionID)
		return
	}
	tuiChainMu.Lock()
	list := tuiChainSuggestions[sessionID]
	tuiChainMu.Unlock()
	req := tuiLaunchRequest{WorkflowID: workflowID, Parent: sessionID}
	for _, s := range list {
		if s.WorkflowID == workflowID {
			req.Prefill, req.Tickets = s.Prefill, s.Tickets
		}
	}
	if sess, err := t.a.Sessions.Get(context.Background(), sessionID); err == nil {
		req.ProjectID = sess.ProjectID
	}
	if tuiShell != nil {
		tuiShell.ShowToastMsg(i18n.Tf("tui.launch.chained", workflowID), true)
	}
	openLaunchForm(t.a, req)
}

// resumeChoice prefixes the « reprendre » choice of « Enchaîner avec… ».
const resumeChoice = "resume:"

// resume opens the launch form remembered for after a session (failed
// precondition with resume: true) and forgets it.
func (t *tuiSessions) resume(sessionID string) {
	ctx := context.Background()
	sess, err := t.a.Sessions.Get(ctx, sessionID)
	if err != nil {
		return
	}
	intent, ok := loadResumeIntent(ctx, t.a, sess.ProjectID, sessionID)
	if !ok {
		return
	}
	forgetResumeIntent(ctx, t.a, sess.ProjectID, sessionID)
	openLaunchForm(t.a, tuiLaunchRequest{WorkflowID: intent.Workflow, ProjectID: sess.ProjectID, Prefill: intent.Inputs,
		Tickets: intent.Tickets, Parent: sessionID, Mode: intent.Mode, Runtime: intent.Runtime})
}
