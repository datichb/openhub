package cmd

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/app"
	"github.com/datichb/openhub/cli/internal/domain"
	workflowsvc "github.com/datichb/openhub/cli/internal/services/workflow"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// R5: session.output(<workflow>.<id>) reads the parent session when it is a
// session of that workflow with the output, else the last one of the
// project.
func TestSessionOutputSource(t *testing.T) {
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	for _, p := range []string{"p1", "p2"} {
		_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES (?, ?, ?)`, p, p, "/"+p)
		require.NoError(t, err)
	}
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
