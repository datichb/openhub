//go:build integration && e2e

package opencodev2

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/datichb/openhub/cli/internal/workflow/hubcat"
)

// N6 (DC6): one checkpoint protocol, with a real model.
//   - a subagent (orchestrator-dev invoked by the orchestrator) passes cp-2
//     itself with workflow_checkpoint: the decision carries its tool
//     session, approving it opens the commit window for the subagent;
//   - a question of the subagent whose header names cp-2 (the former relay
//     « [OrchestratorDev — CP-2 | …] ») is closed by oh;
//   - in a workflow without checkpoints (libre), « Commit ? » asked with the
//     question tool is a plain question, not closed.
func TestE2ESubagentPassesItsCheckpoint(t *testing.T) {
	for _, model := range e2eTicketModels() {
		t.Run(filepath.Base(model), func(t *testing.T) { subagentPassesItsCheckpoint(t, model) })
	}
}

func subagentPassesItsCheckpoint(t *testing.T, model string) {
	tok := bedrockToken(t)
	a := newContractAdapter(t)
	e := newWorkflowEnv(t, a)
	e.env = map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t"}
	wf := sessionspec.WorkflowRuntime{ID: "feature", DefaultMode: "semi-auto",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-2", Label: map[string]string{"": "Commit ou correction"}, Behaviors: map[string]string{"semi-auto": "pause"}, Mandatory: true,
				Unlocks: []string{sessionspec.UnlockCommit}},
		}}
	b := workflowBundle(t, wf)
	b.Agents[0].Body = "You are LEAD, a coordinator. Follow the user's steps exactly: delegate each step to the subagent helper with the task tool, passing the instructions verbatim, then report what helper said. Never run shell commands yourself."
	b.Agents[1].Body = "You are HELPER. Follow the instructions you are given exactly, in order, one tool call at a time. If a command is refused, do not retry it nor work around it."
	shell := func(p string, eff sessionspec.Effect) sessionspec.PermissionRule {
		return sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: eff}
	}
	b.Agents[1].Permissions = []sessionspec.PermissionRule{shell("*", sessionspec.EffectDeny), shell("git *", sessionspec.EffectAllow), shell("echo *", sessionspec.EffectAllow),
		{Action: "question", Resource: "*", Effect: sessionspec.EffectAllow}}
	b.Permissions = append(b.Permissions, bundle.BaseCheckpointRules()...)
	m := sessionspec.ParseModelRef(model)
	b.DefaultModel = &m
	b.Hash = "h_n6_subagent_" + strings.NewReplacer("/", "_", ":", "_", ".", "_").Replace(model)
	e.saveBundle(t, b)
	h, project := e.startGroup(t, a, b, tok)
	gittest.Run(t, project, "commit", "--allow-empty", "-qm", "init")
	c := NewClient(h.URL, h.Password)
	waitMCPConnected(t, c, project, sessionspec.WorkflowMCPServer)
	svc := &sessionsvc.Service{Sessions: e.sessions, Decisions: e.decisions, Servers: e.servers, Adapter: a, BundlesDir: e.bundles}
	svc.UseCheckpoints(e.cp, e.client.WorkflowRefresh)

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	id := sessionspec.NewSessionID()
	require.NoError(t, e.sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: b.Hash, Mode: "semi-auto", State: domain.RunActive, LaunchPath: project}))
	require.NoError(t, a.CreateSession(ctx, h, sessionspec.SessionSpec{SessionID: id, Title: "e2e n6", EntryAgent: "lead", Location: project,
		SessionRules: bundle.SessionRules(b.Workflow, "semi-auto", nil)}))

	// 1. The relay of the old protocol: a question naming cp-2 is closed.
	require.NoError(t, a.SendPrompt(ctx, h, id, "Step 1: delegate to helper: \"Ask the user with the question tool: header \\\"[OrchestratorDev — CP-2 | Ticket #pt-1]\\\", question \\\"Commit ou corriger ?\\\", options Commit and Corriger. Then repeat the answer verbatim.\""))
	require.NoError(t, c.Wait(ctx, id))
	st, err := e.cp.State(ctx, id)
	require.NoError(t, err)
	assert.False(t, st.HasPassed("cp-2"))
	imitated := false
	for _, ev := range st.Timeline {
		imitated = imitated || ev.Kind == domain.CheckpointImitated
	}
	assert.True(t, imitated, "%+v", st.Timeline)

	// 2. The subagent passes cp-2 itself, then commits.
	require.NoError(t, a.SendPrompt(ctx, h, id, "Step 2: delegate to helper: \"1. Call the tool workflow_checkpoint with id cp-2 and summary 'feature.txt ready'. 2. Once it returns, run: echo hello > feature.txt ; then git add feature.txt ; then git commit -m 'feat: hello'.\""))
	var dec domain.Decision
	require.Eventually(t, func() bool {
		l, err := e.decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: id})
		if err != nil {
			return false
		}
		for _, d := range l {
			if d.Kind == domain.DecisionCheckpoint {
				dec = d
				return true
			}
		}
		return false
	}, 4*time.Minute, 300*time.Millisecond, "cp-2 raised by the subagent")
	assert.NotEqual(t, id, dec.ToolSessionID(), "the decision answers the subagent session")
	assert.Equal(t, "init\n", gitLog(t, project), "nothing committed before the approval")
	require.NoError(t, svc.Decide(ctx, sessionsvc.Reply{DecisionID: dec.ID, Decision: checkpoint.ChoiceApprove}))
	require.NoError(t, c.Wait(ctx, id))
	assert.Contains(t, gitLog(t, project), "feat: hello", "committed by the subagent after cp-2: %s", r2text(t, ctx, c, id))
}

func TestE2ELibreQuestionIsNotClosed(t *testing.T) {
	tok := bedrockToken(t)
	a := newContractAdapter(t)
	e := newWorkflowEnv(t, a)
	b := workflowBundle(t, sessionspec.WorkflowRuntime{ID: "libre", DefaultMode: "manuel"})
	b.Agents[0].Body = "You are LEAD. Follow the user's steps exactly."
	b.Permissions = append(b.Permissions, bundle.BaseCheckpointRules()...)
	m := sessionspec.ParseModelRef(e2eModel)
	b.DefaultModel = &m
	b.Hash = "h_n6_libre"
	e.saveBundle(t, b)
	h, project := e.startGroup(t, a, b, tok)
	c := NewClient(h.URL, h.Password)
	waitMCPConnected(t, c, project, sessionspec.WorkflowMCPServer)
	svc := &sessionsvc.Service{Sessions: e.sessions, Decisions: e.decisions, Servers: e.servers, Adapter: a, BundlesDir: e.bundles}
	svc.UseCheckpoints(e.cp, e.client.WorkflowRefresh)

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	id := sessionspec.NewSessionID()
	require.NoError(t, e.sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: b.Hash, Mode: "manuel", State: domain.RunActive, LaunchPath: project}))
	require.NoError(t, a.CreateSession(ctx, h, sessionspec.SessionSpec{SessionID: id, Title: "e2e libre", EntryAgent: "lead", Location: project,
		SessionRules: bundle.SessionRules(b.Workflow, "manuel", nil)}))
	require.NoError(t, a.SendPrompt(ctx, h, id, "Ask me with the question tool: header \"Commit ?\", question \"Commit ?\", options Oui and Non. Then reply with the single word I chose."))
	var dec domain.Decision
	require.Eventually(t, func() bool {
		l, err := e.decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: id})
		if err != nil {
			return false
		}
		for _, d := range l {
			if d.Kind == domain.DecisionQuestion {
				dec = d
				return true
			}
		}
		return false
	}, 3*time.Minute, 300*time.Millisecond, "the question reaches oh as a question")
	require.NotEmpty(t, dec.Payload.Fields)
	f := dec.Payload.Fields[0]
	value := "Oui"
	for _, o := range f.Options {
		if strings.EqualFold(o.Label, "oui") {
			value = o.Value
		}
	}
	var answer any = value
	if f.Type == "multiselect" {
		answer = []string{value}
	}
	require.NoError(t, svc.Decide(ctx, sessionsvc.Reply{DecisionID: dec.ID, Answer: map[string]any{f.Key: answer}}))
	require.NoError(t, c.Wait(ctx, id))
	assert.Contains(t, strings.ToLower(r2text(t, ctx, c, id)), "oui")
	st, err := e.cp.State(ctx, id)
	require.NoError(t, err)
	for _, ev := range st.Timeline {
		assert.NotEqual(t, domain.CheckpointImitated, ev.Kind, "not an imitation in a workflow without checkpoints")
	}
}

// N10: with the commit lock on, the commands that merely mention commit or
// push run (anchored patterns), the detours stay refused.
func TestE2ELockPatternsWithoutFalsePositives(t *testing.T) {
	tok := bedrockToken(t)
	a := newContractAdapter(t)
	e := newWorkflowEnv(t, a)
	e.env = map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t"}
	wf := sessionspec.WorkflowRuntime{ID: "ticket", DefaultMode: "manuel",
		Checkpoints: []sessionspec.CheckpointDef{{ID: "cp-2", Label: map[string]string{"": "Commit ou correction"}, Behaviors: map[string]string{"manuel": "pause"},
			Mandatory: true, Unlocks: []string{sessionspec.UnlockCommit, sessionspec.UnlockPush}}}}
	b := workflowBundle(t, wf)
	b.Agents[0].Body = "You are LEAD. You never run shell commands yourself: you delegate them to the subagent helper, verbatim, then report what helper said."
	b.Agents[1].Body = "You are HELPER. Run the shell commands you are given, exactly as written, one per tool call. If one is refused, do not retry it nor work around it: go on with the next one. Then report the full output of each command."
	shell := func(p string, eff sessionspec.Effect) sessionspec.PermissionRule {
		return sessionspec.PermissionRule{Action: sessionspec.ActionShell, Resource: p, Effect: eff}
	}
	b.Agents[1].Permissions = []sessionspec.PermissionRule{shell("*", sessionspec.EffectDeny), shell("git *", sessionspec.EffectAllow), shell("echo *", sessionspec.EffectAllow), shell("sh *", sessionspec.EffectAllow)}
	b.Permissions = append(b.Permissions, bundle.BaseCheckpointRules()...)
	m := sessionspec.ParseModelRef(e2eModel)
	b.DefaultModel = &m
	b.Hash = "h_n10_patterns"
	e.saveBundle(t, b)
	h, project := e.startGroup(t, a, b, tok)
	gittest.Run(t, project, "commit", "--allow-empty", "-qm", "init commit")
	c := NewClient(h.URL, h.Password)
	waitMCPConnected(t, c, project, sessionspec.WorkflowMCPServer)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	id := sessionspec.NewSessionID()
	require.NoError(t, e.sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: b.Hash, Mode: "manuel", State: domain.RunActive, LaunchPath: project}))
	require.NoError(t, a.CreateSession(ctx, h, sessionspec.SessionSpec{SessionID: id, Title: "e2e n10", EntryAgent: "lead", Location: project,
		SessionRules: bundle.SessionRules(b.Workflow, "manuel", nil)}))
	cmds := "git log --grep commit --format=MARKONE%s\n" +
		"git show -s --format=MARKTWO%s HEAD -- src/pushNotifications.ts\n" +
		"git merge-base --is-ancestor HEAD HEAD && echo MARKTHREE\n" +
		"git --no-pager commit --allow-empty -m 'feat: a'\n" +
		"sh -c 'git commit --allow-empty -m feat-b'\n" +
		"git -C . commit --allow-empty -m 'feat: c'"
	require.NoError(t, a.SendPrompt(ctx, h, id, "Delegate to the subagent helper these commands:\n"+cmds+"\nThen report what helper said, including the outputs verbatim."))
	require.NoError(t, c.Wait(ctx, id))
	reply := r2text(t, ctx, c, id)
	for _, mark := range []string{"MARKONE", "MARKTWO", "MARKTHREE"} {
		assert.Contains(t, reply, mark, "allowed command ran: %q", reply)
	}
	assert.Equal(t, "init commit\n", gitLog(t, project), "the detours are refused")
}

// DC26 (N11): the shipped quick workflow, its real developer: no commit
// before cp-commit; the developer asks it, and commits once approved.
func TestE2EQuickCommitGate(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	hub := filepath.Join(filepath.Dir(file), "..", "..", "..", "..")
	if _, err := os.Stat(filepath.Join(hub, "workflows", "quick.yaml")); err != nil {
		t.Skip("hub content not found")
	}
	tok := bedrockToken(t)
	a := newContractAdapter(t)
	e := newWorkflowEnv(t, a)
	e.env = map[string]string{"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@t", "GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@t"}
	cat, diags := hubcat.LoadWorkflows(hub)
	require.Empty(t, diags)
	hc, err := hubcat.New(hub)
	require.NoError(t, err)
	r, diags := workflow.ResolveSpec(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "quick"}, &workflow.SessionOptions{Mode: "semi-auto",
		Inputs: map[string]any{"request": "Crée le fichier hello.txt contenant le mot hello, puis commite-le avec le message 'feat: hello'."}})
	require.False(t, diags.HasErrors(), "%v", diags)
	prompt, err := workflow.RenderPrompt(r, hc, workflow.PromptContext{Project: "demo", Mode: "semi-auto", Runtime: "local", Lang: "fr", Workflow: "quick"})
	require.NoError(t, err)
	built, err := bundle.Build(bundle.Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec, Provider: "bedrock"})
	require.NoError(t, err)
	b := built.Spec
	m := sessionspec.ParseModelRef(e2eModel)
	b.DefaultModel = &m
	for i := range b.Agents {
		b.Agents[i].Model = &m
	}
	// The bundle is changed after the build (test model): saved under a
	// test name, not under its content hash (bundles named by their hash are
	// verified at load, N23).
	b.Hash = "h_quick_e2e"
	e.saveBundle(t, b)
	h, project := e.startGroup(t, a, b, tok)
	gittest.Run(t, project, "commit", "--allow-empty", "-qm", "init")
	c := NewClient(h.URL, h.Password)
	waitMCPConnected(t, c, project, sessionspec.WorkflowMCPServer)
	svc := &sessionsvc.Service{Sessions: e.sessions, Decisions: e.decisions, Servers: e.servers, Adapter: a, BundlesDir: e.bundles}
	svc.UseCheckpoints(e.cp, e.client.WorkflowRefresh)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	id := sessionspec.NewSessionID()
	require.NoError(t, e.sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: b.Hash, Mode: "semi-auto", State: domain.RunActive, LaunchPath: project}))
	require.NoError(t, a.CreateSession(ctx, h, sessionspec.SessionSpec{SessionID: id, Title: "e2e quick", EntryAgent: b.EntryAgent, Location: project,
		SessionRules: bundle.SessionRules(b.Workflow, "semi-auto", nil)}))
	require.NoError(t, a.SendPrompt(ctx, h, id, prompt))
	var dec domain.Decision
	require.Eventually(t, func() bool {
		l, err := e.decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: id})
		if err != nil {
			return false
		}
		for _, d := range l {
			if d.Kind == domain.DecisionCheckpoint {
				dec = d
				return true
			}
		}
		return false
	}, 6*time.Minute, 500*time.Millisecond, "the developer asks cp-commit")
	assert.Equal(t, "init\n", gitLog(t, project), "nothing committed before cp-commit")
	require.NoError(t, svc.Decide(ctx, sessionsvc.Reply{DecisionID: dec.ID, Decision: checkpoint.ChoiceApprove}))
	require.NoError(t, c.Wait(ctx, id))
	assert.Contains(t, gitLog(t, project), "hello", "committed after cp-commit: %s", r2text(t, ctx, c, id))
}
