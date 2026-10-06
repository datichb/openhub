//go:build integration && e2e

package opencodev2

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

func e2eWorkflowRuntime() sessionspec.WorkflowRuntime {
	return sessionspec.WorkflowRuntime{ID: "e2e", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-1", Label: map[string]string{"": "Feu vert"}, Behaviors: map[string]string{"manuel": "pause"}},
		{ID: "cp-2", Label: map[string]string{"": "Fin"}, Behaviors: map[string]string{"manuel": "pause"}},
	}}
}

// workflow_checkpoint asked through the permission API (S3, P3-T01/T02):
// the action name is `<server>_<tool>`, the MCP server identifies the
// calling session from the request `_meta` and gets its answer from the
// oh daemon.
func TestE2EWorkflowCheckpointAsked(t *testing.T) {
	tok := bedrockToken(t)
	a := newContractAdapter(t)
	e := newWorkflowEnv(t)
	b := workflowBundle(t, e2eWorkflowRuntime())
	b.Agents[0].Body = "You are LEAD. When asked to reach a checkpoint, call the tool workflow_checkpoint with that id and a one-line summary, then reply with the tool result verbatim."
	m := sessionspec.ParseModelRef(e2eModel)
	b.DefaultModel = &m
	e.saveBundle(t, b)
	h, project := e.startServer(t, a, b, sessionspec.ProviderSpec{ID: "amazon-bedrock", Region: "eu-west-1", SessionToken: tok})
	c := NewClient(h.URL, h.Password)
	waitMCPConnected(t, c, project, sessionspec.WorkflowMCPServer)

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	id := sessionspec.NewSessionID()
	require.NoError(t, e.sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: b.Hash, Mode: "manuel", State: domain.RunActive}))
	require.NoError(t, a.CreateSession(ctx, h, sessionspec.SessionSpec{SessionID: id, Title: "e2e", EntryAgent: "lead", Location: project,
		SessionRules: []sessionspec.PermissionRule{{Action: sessionspec.MCPToolAction(sessionspec.WorkflowMCPServer, sessionspec.WorkflowToolCheckpoint), Resource: "*", Effect: sessionspec.EffectAsk}}}))
	require.NoError(t, a.SendPrompt(ctx, h, id, "Reach the checkpoint cp-1."))

	r := &e2eRun{adapter: a, handle: h, project: project}
	p := r.pendingOf(t, ctx, id, adapters.DecisionPermission)
	assert.Equal(t, "workflow_workflow_checkpoint", p.Action, "opencode names MCP tools <server>_<tool>")
	require.NoError(t, a.Reply(ctx, h, adapters.DecisionReply{SessionID: id, ID: p.ID, Kind: adapters.DecisionPermission, Decision: "once"}))
	require.NoError(t, c.Wait(ctx, id))
	reply := r.text(t, ctx, id)
	assert.Contains(t, reply, "Feu vert", "label read by the daemon for the calling session: %q", reply)
	assert.Contains(t, reply, "cp-2", "next checkpoint: %q", reply)
}
