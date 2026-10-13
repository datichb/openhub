//go:build integration && e2e

package opencodev2

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/services/wiki"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// wikiContent is the finding the reviewer subagent is asked to propose.
const wikiContent = "Les fonctions de calc renvoient toujours un int, jamais d'erreur."

const wikiPage = "---\nupdated: 2026-10-01\nconfidence: confirmed\nagents: [onboarder]\n---\n\n# Conventions\n\n## Patterns spécifiques à l'équipe\n" +
	"- Tests table-driven.\n  — `CONFIRMÉ` · onboarder · 2026-09-12 · calc/calc_test.go:1\n"

// wikiBundle is a workflow bundle with the workflow and wiki MCP servers,
// whose subagent (helper, the reviewer) proposes an enrichment.
func wikiBundle(t *testing.T, wf sessionspec.WorkflowRuntime) sessionspec.BundleSpec {
	t.Helper()
	b := workflowBundle(t, wf)
	b.MCP = append(b.MCP, sessionspec.WikiMCPDef())
	b.Agents[0].Body = "You are LEAD, a coordinator. Follow the user's steps exactly, using the tools named. You never propose wiki entries yourself."
	b.Agents[1].Body = "You are HELPER, a reviewer. When asked to propose a wiki entry, call the tool wiki_propose once with exactly the page, section, content, tag and source you are given, then reply with one line: the id the tool returned (wp-…)."
	b.Permissions = append(append(b.Permissions, bundle.BaseCheckpointRules()...), bundle.BaseWikiRules()...)
	m := sessionspec.ParseModelRef(e2eModel)
	b.DefaultModel = &m
	return b
}

func proposePrompt() string {
	return "Delegate to the subagent helper this task, verbatim: \"Propose a wiki entry with wiki_propose: page technical/conventions.md, " +
		"section Patterns spécifiques à l'équipe, content: " + wikiContent + " tag: DÉDUIT, source: calc/calc.go. Then reply with the id returned.\" Then repeat the id helper gave you."
}

func writeProjectWiki(t *testing.T, project string) string {
	t.Helper()
	page := filepath.Join(project, "docs", "wiki", "technical", "conventions.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(page), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(project, "docs", "wiki", "index.md"), []byte("---\nupdated: 2026-10-01\nconfidence: confirmed\nagents: [onboarder]\n---\n\n# Calc\n"), 0o644))
	require.NoError(t, os.WriteFile(page, []byte(wikiPage), 0o644))
	return page
}

// LD-J, review (no commit): the reviewer proposes through the wiki MCP
// server, oh queues the proposal with the author it saw; nothing is
// written in docs/wiki/ and the proposal stays queued (doc-sync).
func TestE2EReviewQueuesWikiProposal(t *testing.T) {
	tok := bedrockToken(t)
	a := newContractAdapter(t)
	e := newWorkflowEnv(t, a)
	b := wikiBundle(t, sessionspec.WorkflowRuntime{ID: "review", DefaultMode: "manuel"})
	b.Hash = "h_review_wiki"
	e.saveBundle(t, b)
	h, project := e.startGroup(t, a, b, tok)
	page := writeProjectWiki(t, project)
	c := NewClient(h.URL, h.Password)
	waitMCPConnected(t, c, project, sessionspec.WikiMCPServer)

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	id := sessionspec.NewSessionID()
	require.NoError(t, e.sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: b.Hash, Mode: "manuel", State: domain.RunActive, LaunchPath: project, EntryAgent: "lead"}))
	require.NoError(t, a.CreateSession(ctx, h, sessionspec.SessionSpec{SessionID: id, Title: "e2e wiki review", EntryAgent: "lead", Location: project,
		SessionRules: bundle.SessionRules(b.Workflow, "manuel", nil)}))
	require.NoError(t, a.SendPrompt(ctx, h, id, proposePrompt()))
	require.NoError(t, c.Wait(ctx, id))

	list, err := e.wiki.Open(ctx, "p1", id)
	require.NoError(t, err)
	require.Len(t, list, 1, "one proposal queued under the oh session: %s", r2text(t, ctx, c, id))
	p := list[0]
	assert.Equal(t, "wp-1", p.ID())
	assert.Equal(t, domain.WikiPending, p.Status, "stays queued: no commit in review")
	assert.Equal(t, "helper", p.Agent, "author seen by oh: the subagent")
	assert.NotEqual(t, id, p.ToolSession, "filed from the subagent session")
	assert.Equal(t, "technical/conventions.md", p.Page)
	assert.Equal(t, wiki.TagDeduced, p.Tag)
	assert.Contains(t, p.Content, "jamais d'erreur")
	data, err := os.ReadFile(page)
	require.NoError(t, err)
	assert.Equal(t, wikiPage, string(data), "nothing written in docs/wiki/")
	assert.Contains(t, r2text(t, ctx, c, id), "wp-1", "the coordinator passes the id on")
}

// LD-J, ticket: the proposal of the reviewer is on the card of the commit
// checkpoint; validated there, oh applies it when the checkpoint is passed,
// before the call returns (committed with the code).
func TestE2ETicketAppliesValidatedWikiProposal(t *testing.T) {
	tok := bedrockToken(t)
	a := newContractAdapter(t)
	e := newWorkflowEnv(t, a)
	wf := sessionspec.WorkflowRuntime{ID: "ticket", DefaultMode: "manuel", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-2", Label: map[string]string{"": "Commit ou correction"}, Behaviors: map[string]string{"manuel": "pause"}, Mandatory: true,
			Unlocks: []string{sessionspec.UnlockCommit}},
	}}
	b := wikiBundle(t, wf)
	b.Hash = "h_ticket_wiki"
	e.saveBundle(t, b)
	h, project := e.startGroup(t, a, b, tok)
	page := writeProjectWiki(t, project)
	c := NewClient(h.URL, h.Password)
	waitMCPConnected(t, c, project, sessionspec.WikiMCPServer)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	id := sessionspec.NewSessionID()
	require.NoError(t, e.sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: b.Hash, Mode: "manuel", State: domain.RunActive, LaunchPath: project, EntryAgent: "lead"}))
	require.NoError(t, a.CreateSession(ctx, h, sessionspec.SessionSpec{SessionID: id, Title: "e2e wiki ticket", EntryAgent: "lead", Location: project,
		SessionRules: bundle.SessionRules(b.Workflow, "manuel", nil)}))
	require.NoError(t, a.SendPrompt(ctx, h, id, "Step 1: "+proposePrompt()+
		"\nStep 2: call workflow_checkpoint with id cp-2 and a one-line summary. Wait for its answer, then repeat its answer verbatim and stop."))

	svc := &sessionsvc.Service{Sessions: e.sessions, Decisions: e.decisions, Servers: e.servers, Adapter: a, BundlesDir: e.bundles, Wiki: e.wiki}
	svc.UseCheckpoints(e.cp, e.client.WorkflowRefresh)
	var dec domain.Decision
	require.Eventually(t, func() bool {
		l, err := e.decisions.ListOpen(ctx, domain.DecisionFilter{SessionID: id})
		if err != nil || len(l) == 0 {
			return false
		}
		dec = l[0]
		return dec.Kind == domain.DecisionCheckpoint
	}, 5*time.Minute, 300*time.Millisecond, "cp-2 raised by the daemon")

	card, err := svc.CheckpointCard(ctx, dec.ID)
	require.NoError(t, err)
	require.True(t, card.WikiCommit)
	require.Len(t, card.Wiki, 1, "the proposal of the session is on the commit card")
	p := card.Wiki[0]
	assert.Equal(t, "helper", p.Agent)
	data, _ := os.ReadFile(page)
	assert.Equal(t, wikiPage, string(data), "not applied before the validation")

	// The user validates it on the card, then the checkpoint.
	_, err = svc.WikiDecide(ctx, id, p.ID(), wiki.ChoiceAccept, "")
	require.NoError(t, err)
	require.NoError(t, svc.Decide(ctx, sessionsvc.Reply{DecisionID: dec.ID, Decision: checkpoint.ChoiceApprove}))
	require.NoError(t, c.Wait(ctx, id))

	st, err := e.cp.State(ctx, id)
	require.NoError(t, err)
	assert.True(t, st.HasPassed("cp-2"))
	data, err = os.ReadFile(page)
	require.NoError(t, err)
	text := string(data)
	assert.Contains(t, text, "- "+wikiContent+"\n  — `DÉDUIT` · helper · ", "applied at the end of its section, author set by oh")
	assert.Contains(t, text, "agents: [onboarder, helper]")
	assert.Contains(t, text, "confidence: mixed")
	got, err := e.wiki.Get(ctx, "p1", p.ID())
	require.NoError(t, err)
	assert.Equal(t, domain.WikiApplied, got.Status)
	assert.Equal(t, wiki.AppliedByOh, got.AppliedBy)
	assert.Contains(t, checkpoint.TimelineText(checkpoint.Timeline(b.Workflow, "manuel", "fr", st), 0), "✎ "+p.ID())
	assert.True(t, strings.Contains(r2text(t, ctx, c, id), "docs/wiki/technical/conventions.md"), "the agent is told what to commit: %s", r2text(t, ctx, c, id))
}
