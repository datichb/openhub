//go:build integration && e2e

package opencodev2

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/bundle"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// reportBody is the report the reviewer subagent is asked to file.
const reportBody = "## Verdict\ncorriger\n\n## Corrections requises\n- calc.Sub additionne au lieu de soustraire (TestSub échoue)"

// reportBundle is a workflow bundle whose subagent (helper, the reviewer)
// files a report with workflow_report.
func reportBundle(t *testing.T, e *workflowEnv, wf sessionspec.WorkflowRuntime) sessionspec.BundleSpec {
	t.Helper()
	b := workflowBundle(t, wf)
	b.Agents[0].Body = "You are LEAD, a coordinator. Follow the user's steps exactly, using the tools named. You never write reports yourself."
	b.Agents[1].Body = "You are HELPER, a reviewer. When asked to file a report, call the tool workflow_report once with the exact title and body you are given, then reply with one line: the id the tool returned (rp-…)."
	b.Permissions = append(b.Permissions, bundle.BaseCheckpointRules()...)
	m := sessionspec.ParseModelRef(e2eModel)
	b.DefaultModel = &m
	return b
}

func fileReportPrompt() string {
	return "Delegate to the subagent helper this task, verbatim: \"File a report with workflow_report, title: Review, body:\n" + reportBody +
		"\nThen reply with the id returned.\" Then repeat the id helper gave you."
}

// L4 with the whole chain (subagent session, workflow MCP server, group
// token, daemon): the reviewer files its report, oh stores it under the oh
// session with the author it saw, and the coordinator gets only its id.
func TestE2EReviewStoresReport(t *testing.T) {
	tok := bedrockToken(t)
	a := newContractAdapter(t)
	e := newWorkflowEnv(t, a)
	b := reportBundle(t, e, sessionspec.WorkflowRuntime{ID: "review", DefaultMode: "manuel"})
	b.Hash = "h_review_report"
	e.saveBundle(t, b)
	h, project := e.startGroup(t, a, b, tok)
	c := NewClient(h.URL, h.Password)
	waitMCPConnected(t, c, project, sessionspec.WorkflowMCPServer)

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	id := sessionspec.NewSessionID()
	require.NoError(t, e.sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: b.Hash, Mode: "manuel", State: domain.RunActive, LaunchPath: project, EntryAgent: "lead"}))
	require.NoError(t, a.CreateSession(ctx, h, sessionspec.SessionSpec{SessionID: id, Title: "e2e report", EntryAgent: "lead", Location: project,
		SessionRules: bundle.SessionRules(b.Workflow, "manuel", nil)}))
	require.NoError(t, a.SendPrompt(ctx, h, id, fileReportPrompt()))
	require.NoError(t, c.Wait(ctx, id))

	list, err := e.reports.List(ctx, id)
	require.NoError(t, err)
	require.Len(t, list, 1, "one report filed under the oh session: %s", r2text(t, ctx, c, id))
	rp := list[0]
	assert.Equal(t, "rp-1", rp.ID())
	assert.Equal(t, "helper", rp.Agent, "author seen by oh: the subagent")
	assert.NotEqual(t, id, rp.ToolSession, "filed from the subagent session")
	assert.Equal(t, "Review", rp.Title)
	_, body, err := e.reports.Get(ctx, id, "rp-1")
	require.NoError(t, err)
	assert.Contains(t, body, "TestSub échoue")
	assert.Contains(t, r2text(t, ctx, c, id), "rp-1", "the coordinator passes the id on")

	st, err := e.cp.State(ctx, id)
	require.NoError(t, err)
	assert.Contains(t, checkpoint.TimelineText(checkpoint.Timeline(b.Workflow, "manuel", "fr", st), 0), "▤ rp-1 (helper)")
}

// L4: the card of the next checkpoint shows the report filed since the
// previous one, readable in full.
func TestE2ETicketCardShowsReport(t *testing.T) {
	tok := bedrockToken(t)
	a := newContractAdapter(t)
	e := newWorkflowEnv(t, a)
	wf := sessionspec.WorkflowRuntime{ID: "ticket", DefaultMode: "manuel", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-2", Label: map[string]string{"": "Commit ou correction"}, Behaviors: map[string]string{"manuel": "pause"}, Mandatory: true},
	}}
	b := reportBundle(t, e, wf)
	b.Hash = "h_ticket_report"
	e.saveBundle(t, b)
	h, project := e.startGroup(t, a, b, tok)
	c := NewClient(h.URL, h.Password)
	waitMCPConnected(t, c, project, sessionspec.WorkflowMCPServer)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	id := sessionspec.NewSessionID()
	require.NoError(t, e.sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: b.Hash, Mode: "manuel", State: domain.RunActive, LaunchPath: project, EntryAgent: "lead"}))
	require.NoError(t, a.CreateSession(ctx, h, sessionspec.SessionSpec{SessionID: id, Title: "e2e card", EntryAgent: "lead", Location: project,
		SessionRules: bundle.SessionRules(b.Workflow, "manuel", nil)}))
	require.NoError(t, a.SendPrompt(ctx, h, id, "Step 1: "+fileReportPrompt()+
		"\nStep 2: call workflow_checkpoint with id cp-2, a one-line summary, and reports set to the id helper gave you. Wait for its answer, then stop."))

	svc := &sessionsvc.Service{Sessions: e.sessions, Decisions: e.decisions, Servers: e.servers, Adapter: a, BundlesDir: e.bundles, Reports: e.reports}
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
	require.Len(t, card.Reports, 1, "the report filed since the start is on the card")
	assert.Equal(t, "rp-1", card.Reports[0].ID())
	assert.Equal(t, "helper", card.Reports[0].Agent)
	_, body, err := svc.Report(ctx, id, card.Reports[0].ID())
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(body, "## Verdict"), body)
	if cited, ok := dec.Payload.Data[domain.DataReports]; ok {
		assert.Contains(t, cited, "rp-1", "cited by the agent")
	}

	require.NoError(t, svc.Decide(ctx, sessionsvc.Reply{DecisionID: dec.ID, Decision: checkpoint.ChoiceApprove}))
	require.NoError(t, c.Wait(ctx, id))
	st, err := e.cp.State(ctx, id)
	require.NoError(t, err)
	assert.True(t, st.HasPassed("cp-2"))
}
