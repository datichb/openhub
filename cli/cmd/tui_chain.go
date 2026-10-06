package cmd

import (
	"context"
	"strings"
	"sync"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/worktree"
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
	fallback := map[workflow.OutputType]any{}
	if sess.LaunchPath != "" {
		if b, err := worktree.CurrentBranch(sess.LaunchPath); err == nil && b != "" && b != "main" && b != "master" {
			fallback[workflow.OutputBranch] = b
		}
	}
	list, err := newWorkflowService(ctx).Chain(ctx, workflowsvc.Context{ProjectID: sess.ProjectID}, sess.WorkflowID, sess.Outputs, fallback)
	if err != nil {
		return nil, err
	}
	tuiChainMu.Lock()
	tuiChainSuggestions[sessionID] = list
	tuiChainMu.Unlock()
	opts := make([]views.SelectOption, 0, len(list))
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
