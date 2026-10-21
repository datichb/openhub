package cmd

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// R5: session.output(<workflow>.<id>) reads the parent session when it is a
// session of that workflow with the output, else the last one of the
// project.
func TestSessionOutputSource(t *testing.T) {
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1").Project("p2", "/p2")
	sessions := sqlite.NewSessionStore(st)
	t0 := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	add := func(id, project, wf string, at time.Duration, outputs map[string]any) {
		require.NoError(t, sessions.Create(ctx, &domain.Session{ID: id, ProjectID: project, WorkflowID: wf, StartedAt: t0.Add(at),
			Status: domain.SessionStatusCompleted, State: domain.RunCompleted, Outputs: outputs}))
	}
	add("ses_old", "p1", "spec", 0, map[string]any{"tickets": []string{"ev-1"}})
	add("ses_new", "p1", "spec", time.Hour, map[string]any{"tickets": []string{"ev-2", "ev-3"}, "spec": "docs/wiki/specs/a.md"})
	add("ses_empty", "p1", "spec", 2*time.Hour, nil)
	add("ses_other", "p1", "ticket", 3*time.Hour, map[string]any{"tickets": []string{"ev-9"}})
	add("ses_p2", "p2", "spec", 4*time.Hour, map[string]any{"tickets": []string{"ev-7"}})

	var notes bytes.Buffer
	src := inputSources(&app.App{Sessions: sessions}, noteTo(&notes))[workflow.SourceSessionOutput]
	got, err := src(ctx, workflowsvc.Context{ProjectID: "p1"}, "spec.tickets")
	require.NoError(t, err)
	assert.Equal(t, "ev-2,ev-3", got, "the last spec session of the project with tickets")
	assert.Contains(t, notes.String(), "ses_new")

	got, err = src(ctx, workflowsvc.Context{ProjectID: "p1", FromSession: "ses_old"}, "spec.tickets")
	require.NoError(t, err)
	assert.Equal(t, "ev-1", got, "the parent session first")
	got, _ = src(ctx, workflowsvc.Context{ProjectID: "p1", FromSession: "ses_other"}, "spec.tickets")
	assert.Equal(t, "ev-2,ev-3", got, "a parent of another workflow is not read")

	got, err = src(ctx, workflowsvc.Context{ProjectID: "p1"}, "spec.spec")
	require.NoError(t, err)
	assert.Equal(t, "docs/wiki/specs/a.md", got)
	_, err = src(ctx, workflowsvc.Context{ProjectID: "p1"}, "review.report")
	assert.Error(t, err, "no such session")
}

func TestNextCommand(t *testing.T) {
	got := nextCommand(workflowsvc.ChainSuggestion{WorkflowID: "implement-spec", Tickets: []string{"ev-1", "ev-2"},
		Prefill: map[string]string{"spec": "docs/wiki/specs/a b.md"}}, "ses_1")
	assert.Equal(t, "oh run implement-spec --parent ses_1 --tickets ev-1,ev-2 -i spec='docs/wiki/specs/a b.md'", got)
}

// B1: an output of type report reaches the input as the content of the
// report (the findings of an audit for security-fix), not its id.
func TestSessionOutputSourceReadsReports(t *testing.T) {
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	sessions := sqlite.NewSessionStore(st)
	require.NoError(t, sessions.Create(ctx, &domain.Session{ID: "ses_audit", ProjectID: "p1", WorkflowID: "audit", StartedAt: time.Now(),
		Status: domain.SessionStatusCompleted, State: domain.RunCompleted, Outputs: map[string]any{"findings": "rp-2", "type": "security"}}))

	oldType, oldBody := sessionOutputType, sessionReportBody
	t.Cleanup(func() { sessionOutputType, sessionReportBody = oldType, oldBody })
	sessionOutputType = func(_ context.Context, _ workflowsvc.Context, wfID, out string) workflow.OutputType {
		if wfID == "audit" && out == "findings" {
			return workflow.OutputReport
		}
		return workflow.OutputText
	}
	var asked []string
	sessionReportBody = func(_ context.Context, _ *app.App, sessionID, id string) (string, error) {
		asked = append(asked, sessionID+"/"+id)
		return "## Audit security\n- F1 🔴 injection SQL — db.go:12", nil
	}
	var notes bytes.Buffer
	src := inputSources(&app.App{Sessions: sessions}, noteTo(&notes))[workflow.SourceSessionOutput]
	got, err := src(ctx, workflowsvc.Context{ProjectID: "p1", FromSession: "ses_audit"}, "audit.findings")
	require.NoError(t, err)
	assert.Equal(t, "## Audit security\n- F1 🔴 injection SQL — db.go:12", got)
	assert.Equal(t, []string{"ses_audit/rp-2"}, asked)
	got, err = src(ctx, workflowsvc.Context{ProjectID: "p1"}, "audit.type")
	require.NoError(t, err)
	assert.Equal(t, "security", got, "other outputs as they are")
}

// r9b (retro): session.traces(<input>) reads the sessions the input names
// (ids, a period, workflows), the launched workflow's own sessions aside,
// at most tracesMaxSessions; empty, the parent session or the last one.
func TestSessionTracesSource(t *testing.T) {
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1").Project("p2", "/p2")
	sessions := sqlite.NewSessionStore(st)
	now := time.Now()
	add := func(id, project, wf string, ago time.Duration, state domain.RunState) {
		require.NoError(t, sessions.Create(ctx, &domain.Session{ID: id, ProjectID: project, WorkflowID: wf, StartedAt: now.Add(-ago),
			Status: domain.SessionStatusCompleted, State: state}))
	}
	add("ses_t1", "p1", "ticket", 10*24*time.Hour, domain.RunCompleted)
	add("ses_t2", "p1", "ticket", 2*24*time.Hour, domain.RunCompleted)
	add("ses_s1", "p1", "spec", 24*time.Hour, domain.RunCompleted)
	add("ses_r1", "p1", "retro", 12*time.Hour, domain.RunCompleted)
	add("ses_live", "p1", "ticket", time.Hour, domain.RunActive)
	add("ses_p2", "p2", "ticket", time.Hour, domain.RunCompleted)

	old := sessionTraceDigest
	t.Cleanup(func() { sessionTraceDigest = old })
	sessionTraceDigest = func(_ context.Context, _ *app.App, id string) (string, error) { return "Session " + id, nil }
	var notes bytes.Buffer
	src := inputSources(&app.App{Sessions: sessions}, noteTo(&notes))[workflow.SourceSessionTraces]
	c := workflowsvc.Context{ProjectID: "p1", Workflow: "retro"}

	got, err := src(ctx, c, "7d")
	require.NoError(t, err)
	assert.Equal(t, "=== Session 1/2 ===\nSession ses_s1\n\n=== Session 2/2 ===\nSession ses_t2", got,
		"the last 7 days, most recent first, without the running session, the retro itself or another project")
	got, err = src(ctx, c, "ticket")
	require.NoError(t, err)
	assert.Equal(t, "=== Session 1/2 ===\nSession ses_t2\n\n=== Session 2/2 ===\nSession ses_t1", got)
	got, err = src(ctx, c, "ses_t1, ses_r1")
	require.NoError(t, err)
	assert.Contains(t, got, "Session ses_r1", "a session named is read, whatever its workflow")
	assert.Contains(t, notes.String(), "ses_t1, ses_r1")
	got, err = src(ctx, c, "retro,"+now.Add(-5*24*time.Hour).Format("2006-01-02"))
	require.NoError(t, err)
	assert.Equal(t, "=== Session 1/1 ===\nSession ses_r1", got, "asked for: a workflow named, since a day")
	got, err = src(ctx, workflowsvc.Context{ProjectID: "p1", FromSession: "ses_t1"}, "")
	require.NoError(t, err)
	assert.Equal(t, "Session ses_t1", got, "empty: session.trace(), the parent session first")

	_, err = src(ctx, c, "ses_p2")
	assert.ErrorContains(t, err, "ses_p2", "a session of another project")
	_, err = src(ctx, c, "hier!")
	assert.Error(t, err)
	_, err = src(ctx, c, "audit")
	assert.Error(t, err, "nothing fits")
}

// p6-debts: security-fix without --parent reads the last audit of type
// security only (from_equals), never the report of another audit; without
// one the input stays empty and the launch says so.
func TestSessionOutputSourceFromEquals(t *testing.T) {
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	sessions := sqlite.NewSessionStore(st)
	t0 := time.Date(2026, 10, 14, 9, 0, 0, 0, time.UTC)
	add := func(id, typ string, at time.Duration) {
		require.NoError(t, sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", WorkflowID: "audit", StartedAt: t0.Add(at),
			Status: domain.SessionStatusCompleted, State: domain.RunCompleted, Outputs: map[string]any{"findings": "rp-" + id, "type": typ}}))
	}
	add("sec", "security", 0)
	add("perf", "performance", time.Hour)

	oldType := sessionOutputType
	t.Cleanup(func() { sessionOutputType = oldType })
	sessionOutputType = func(context.Context, workflowsvc.Context, string, string) workflow.OutputType {
		return workflow.OutputText
	}
	var notes bytes.Buffer
	src := inputSources(&app.App{Sessions: sessions}, noteTo(&notes))[workflow.SourceSessionOutput]
	arg := "audit.findings" + workflowsvc.FromEqualsSuffix(map[string]string{"type": "security"})
	got, err := src(ctx, workflowsvc.Context{ProjectID: "p1"}, arg)
	require.NoError(t, err)
	assert.Equal(t, "rp-sec", got, "the last security audit, not the last audit")
	assert.Contains(t, notes.String(), "sec")
	got, err = src(ctx, workflowsvc.Context{ProjectID: "p1", FromSession: "perf"}, arg)
	require.NoError(t, err)
	assert.Equal(t, "rp-sec", got, "a parent audit of another type is not read")
	got, _ = src(ctx, workflowsvc.Context{ProjectID: "p1"}, "audit.findings")
	assert.Equal(t, "rp-perf", got, "without from_equals: the last audit")

	notes.Reset()
	_, err = src(ctx, workflowsvc.Context{ProjectID: "p1"}, "audit.findings"+workflowsvc.FromEqualsSuffix(map[string]string{"type": "privacy"}))
	assert.Error(t, err)
	assert.Contains(t, notes.String(), "type=privacy", "the launch says no session fits")
}
