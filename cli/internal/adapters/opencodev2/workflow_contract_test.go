//go:build integration

package opencodev2

import (
	"context"
	"testing"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// The workflow MCP server of a bundle (`{{oh.bin}} mcp serve workflow`) is
// started by the tool for the session location (P3-T01).
func TestContractWorkflowMCPConnected(t *testing.T) {
	a := newContractAdapter(t)
	e := newWorkflowEnv(t, a)
	b := workflowBundle(t, sessionspec.WorkflowRuntime{ID: "contract"})
	h, project := e.startServer(t, a, b, sessionspec.ProviderSpec{})
	waitMCPConnected(t, NewClient(h.URL, h.Password), project, sessionspec.WorkflowMCPServer)

	rep, err := a.Attest(context.Background(), h, b.WithOhBinary(e.ohBin), project)
	if err != nil {
		t.Fatalf("attest: %v (%+v)", err, rep)
	}
}
