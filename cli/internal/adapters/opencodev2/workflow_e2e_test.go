//go:build integration && e2e

package opencodev2

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
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
	e := newWorkflowEnv(t, a)
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

// subagentError returns the error type of the last subagent call of a
// session ("" when it succeeded, "none" without a call). Messages are listed
// newest first.
func subagentError(t *testing.T, ctx context.Context, c *Client, id string) string {
	t.Helper()
	var msgs struct {
		Data []struct {
			Content []struct {
				Type  string `json:"type"`
				Name  string `json:"name"`
				State struct {
					Status string `json:"status"`
					Error  struct {
						Type string `json:"type"`
					} `json:"error"`
				} `json:"state"`
			} `json:"content"`
		} `json:"data"`
	}
	require.NoError(t, c.do(ctx, "GET", "/api/session/"+id+"/message", nil, nil, &msgs))
	for _, m := range msgs.Data {
		for _, p := range m.Content {
			if p.Type == "tool" && p.Name == "subagent" {
				return p.State.Error.Type
			}
		}
	}
	return "none"
}

// Acceptance (07 §3, criterion 3) with the whole chain: oh daemon
// (CheckpointService, session rules), oh plugin (level 3 hooks through the
// credential proxy), workflow MCP server, SessionService resolver.
//   - before cp-1, delegating to the locked agent is refused;
//   - cp-1 (pause) goes to the inbox, validated from oh with an instruction
//     that reaches the agent; the agent is then allowed;
//   - cp-2 (auto in the mode) is let through by the plugin: never asked.
func TestE2ECheckpointGating(t *testing.T) {
	tok := bedrockToken(t)
	a := newContractAdapter(t)
	e := newWorkflowEnv(t, a)
	b := workflowBundle(t, sessionspec.WorkflowRuntime{ID: "gating", DefaultMode: "manuel",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-1", Label: map[string]string{"": "Feu vert"}, Behaviors: map[string]string{"manuel": "pause"}},
			{ID: "cp-2", Label: map[string]string{"": "Fin"}, Behaviors: map[string]string{"manuel": "auto"}},
		},
		Gates: []sessionspec.AgentGate{{Agent: "helper", After: "cp-1"}},
	})
	b.Agents[0].Body = "You are LEAD. Follow the user's steps exactly, one tool call at a time. Never retry a failed tool call."
	b.Agents[1].Body = "You are HELPER. Reply with the single word PONG."
	b.Permissions = append(b.Permissions, bundle.BaseCheckpointRules()...)
	m := sessionspec.ParseModelRef(e2eModel)
	b.DefaultModel = &m
	e.saveBundle(t, b)
	h, project := e.startGroup(t, a, b, tok)
	c := NewClient(h.URL, h.Password)
	waitMCPConnected(t, c, project, sessionspec.WorkflowMCPServer)

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	evs, err := a.Events(ctx, h)
	require.NoError(t, err)
	var asked atomic.Int32
	go func() {
		for ev := range evs {
			if ev.Type == "permission.asked" {
				asked.Add(1)
			}
		}
	}()

	id := sessionspec.NewSessionID()
	require.NoError(t, e.sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: b.Hash, Mode: "manuel", State: domain.RunActive}))
	require.NoError(t, a.CreateSession(ctx, h, sessionspec.SessionSpec{SessionID: id, Title: "e2e", EntryAgent: "lead", Location: project,
		SessionRules: bundle.SessionRules(b.Workflow, "manuel", nil)}))

	// 1. Before cp-1: the delegation is refused.
	require.NoError(t, a.SendPrompt(ctx, h, id, "Step 1: delegate to the subagent helper the task 'say PONG'. If it fails, say FAILED and stop."))
	require.NoError(t, c.Wait(ctx, id))
	assert.Equal(t, "permission.rejected", subagentError(t, ctx, c, id), "helper locked before cp-1")

	// 2. cp-1 pauses: ⏸ in the inbox, validated from oh with an instruction.
	require.NoError(t, a.SendPrompt(ctx, h, id, "Step 2: call workflow_checkpoint with id cp-1 and a one-line summary. Then follow the instruction in its result, then delegate to the subagent helper the task 'say PONG' and reply with its answer."))
	svc := &sessionsvc.Service{Sessions: e.sessions, Decisions: e.decisions, Servers: e.servers, Adapter: a, BundlesDir: e.bundles}
	svc.UseCheckpoints(e.cp, e.client.WorkflowRefresh)
	var dec domain.Decision
	require.Eventually(t, func() bool {
		l, err := e.decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: id})
		if err != nil || len(l) == 0 {
			return false
		}
		dec = l[0]
		return dec.Kind == domain.DecisionCheckpoint
	}, 2*time.Minute, 300*time.Millisecond, "checkpoint decision raised by the daemon")
	assert.Equal(t, "Feu vert", dec.Payload.Title)
	require.NoError(t, svc.Decide(ctx, sessionsvc.Reply{DecisionID: dec.ID, Decision: checkpoint.ChoiceApprove, Message: "commence ta réponse finale par OK-OH"}))
	require.NoError(t, c.Wait(ctx, id))
	reply := r2text(t, ctx, c, id)
	assert.Contains(t, reply, "PONG", "helper allowed after cp-1: %q", reply)
	assert.Contains(t, reply, "OK-OH", "instruction given with the validation reached the agent: %q", reply)
	assert.Equal(t, "", subagentError(t, ctx, c, id))
	st, err := e.cp.State(ctx, id)
	require.NoError(t, err)
	assert.True(t, st.HasPassed("cp-1"))
	assert.True(t, st.HasRun("helper"))

	// 3. cp-2 is automatic in this mode: let through by the plugin.
	before := asked.Load()
	require.NoError(t, a.SendPrompt(ctx, h, id, "Step 3: call workflow_checkpoint with id cp-2 and a one-line summary, then say DONE."))
	require.NoError(t, c.Wait(ctx, id))
	st, _ = e.cp.State(ctx, id)
	assert.True(t, st.HasPassed("cp-2"))
	assert.Equal(t, before, asked.Load(), "no permission asked for an automatic checkpoint (level 3)")

	// 4. A refusal: the instruction (not forwarded by the tool) is sent to the agent.
	require.NoError(t, a.SendPrompt(ctx, h, id, "Step 4: call workflow_checkpoint with id cp-1 and a one-line summary. If it fails, wait for my instruction."))
	require.Eventually(t, func() bool {
		l, err := e.decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: id})
		if err != nil || len(l) == 0 {
			return false
		}
		dec = l[0]
		return dec.Kind == domain.DecisionCheckpoint
	}, 2*time.Minute, 300*time.Millisecond)
	require.NoError(t, svc.Decide(ctx, sessionsvc.Reply{DecisionID: dec.ID, Decision: checkpoint.ChoiceFix, Message: "réponds uniquement REFUSED-OK"}))
	require.Eventually(t, func() bool {
		txt, err := c.AssistantText(ctx, id)
		return err == nil && strings.Contains(txt, "REFUSED-OK")
	}, 2*time.Minute, time.Second, "refusal instruction reached the agent")
	st, _ = e.cp.State(ctx, id)
	refused := false
	for _, ev := range st.Timeline {
		refused = refused || (ev.Kind == domain.CheckpointRefused && ev.ID == "cp-1")
	}
	assert.True(t, refused, "%+v", st.Timeline)
}

func r2text(t *testing.T, ctx context.Context, c *Client, id string) string {
	t.Helper()
	txt, err := c.AssistantText(ctx, id)
	require.NoError(t, err)
	return txt
}
