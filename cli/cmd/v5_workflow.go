package cmd

import (
	"context"
	"os"

	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
)

// workflowsDirEnv points the hub layer at another directory of workflow
// documents (workflows under development, tests).
const workflowsDirEnv = "OH_WORKFLOWS_DIR"

// newWorkflowService wires the WorkflowService on the hub content. The
// isolation level of the tool adapter is filled when opencode V2 is present.
func newWorkflowService(ctx context.Context) *workflowsvc.Service {
	svc := &workflowsvc.Service{HubDir: findHubDir(), HubWorkflowsDir: os.Getenv(workflowsDirEnv)}
	if ctx != nil && v5Available(ctx) {
		svc.Isolation = v5Adapter.Capabilities().Isolation
	}
	return svc
}
