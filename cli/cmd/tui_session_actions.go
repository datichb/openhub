package cmd

import (
	"context"

	"github.com/datichb/openhub/cli/internal/i18n"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// freeWorkflow is the workflow of the free sessions (entry selectable).
const freeWorkflow = "libre"

// actionFreeSession opens the launch form of the `libre` workflow (free
// session; the form offers the entry agents of the workflow, its default
// one first: `oh run libre --agent <id>`). Used by the "coder" omnibar
// command and the « Démarrer » fallback.
func actionFreeSession() {
	openFreeSession("")
}

// openFreeSession opens the form of `libre` with agent preselected as the
// entry agent ("" = the default one): « session libre avec cet agent ».
func openFreeSession(agent string) {
	if tuiShell == nil {
		return
	}
	openLaunchForm(MustApp(), tuiLaunchRequest{WorkflowID: freeWorkflow, Agent: agent})
}

// actionFreeSessionWithAgent asks the entry agent first (the agents of
// `libre`), then opens its form with that agent.
func actionFreeSessionWithAgent() {
	sh := tuiShell
	if sh == nil {
		return
	}
	scope := activeScope()
	go func() {
		ctx := context.Background()
		res, err := newWorkflowService(ctx).Resolve(ctx, workflowsvc.Context{ProjectID: scope.ProjectID}, freeWorkflow, workflowsvc.ResolveOpts{})
		sh.App().QueueUpdateDraw(func() {
			if err != nil {
				sh.ShowToast(err.Error(), shell.ToastError)
				return
			}
			agents := res.EntryChoices()
			if len(agents) == 0 {
				openFreeSession("")
				return
			}
			opts := make([]views.SelectOption, len(agents))
			for i, id := range agents {
				opts[i] = views.SelectOption{Label: id, Value: id}
			}
			sh.ShowSelectModal(i18n.T("tui.launch.agent_pick"), opts, agents[0], openFreeSession)
		})
	}()
}
