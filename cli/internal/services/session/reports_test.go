package session

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/services/report"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
	"github.com/datichb/openhub/cli/internal/storage/sqlite/sqlitetest"
)

// The card of a checkpoint carries the reports filed since the previous
// checkpoint passed, and the older ones the agent cited.
func TestCheckpointCardReports(t *testing.T) {
	st := sqlitetest.New(t)
	ctx := context.Background()
	sqlitetest.Seed(t, st).Project("p1", "/p1")
	bundles := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bundles, "h1"), 0o755))
	data, _ := json.Marshal(sessionspec.BundleSpec{Hash: "h1", Workflow: &sessionspec.WorkflowRuntime{ID: "ticket",
		Checkpoints: []sessionspec.CheckpointDef{{ID: "cp-1", Behaviors: map[string]string{"manuel": "pause"}}, {ID: "cp-2", Behaviors: map[string]string{"manuel": "pause"}}}}})
	require.NoError(t, os.WriteFile(filepath.Join(bundles, "h1", "bundle.json"), data, 0o644))

	at := time.Date(2026, 10, 13, 10, 0, 0, 0, time.UTC)
	clock := at
	rs := &report.Service{Store: sqlite.NewReportStore(st), SessionsDir: t.TempDir(), Now: func() time.Time { return clock }}
	svc := &Service{Sessions: sqlite.NewSessionStore(st), Decisions: sqlite.NewDecisionStore(st), BundlesDir: bundles, Reports: rs}
	cp := &checkpoint.Service{Sessions: svc.Sessions, States: sqlite.NewCheckpointStore(st), BundlesDir: bundles}
	svc.UseCheckpoints(cp, nil)
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: "h1", Mode: "manuel"}))

	plan, err := rs.Add(ctx, "ses_a", "planner", "", "Plan", "## Plan\n1. corriger Sub")
	require.NoError(t, err)
	clock = at.Add(time.Minute)
	_, err = cp.States.UpdateCheckpointState(ctx, "ses_a", func(s *domain.CheckpointState) error {
		s.Timeline = []domain.CheckpointEvent{{At: at.Add(2 * time.Minute), Kind: domain.CheckpointPassed, ID: "cp-1"}}
		return nil
	})
	require.NoError(t, err)
	clock = at.Add(3 * time.Minute)
	dev, err := rs.Add(ctx, "ses_a", "developer", "", "Dev", "## Changements par fichier\n- calc.go")
	require.NoError(t, err)
	clock = at.Add(4 * time.Minute)
	rev, err := rs.Add(ctx, "ses_a", "reviewer", "", "Review", "## Verdict\nvalidé")
	require.NoError(t, err)

	d := &domain.Decision{SessionID: "ses_a", Kind: domain.DecisionCheckpoint, ToolRef: "per_2", CreatedAt: at.Add(5 * time.Minute),
		Payload: domain.DecisionPayload{Title: "Commit", Data: map[string]any{checkpoint.DataCheckpoint: "cp-2"}}}
	require.NoError(t, svc.Raise(ctx, d))
	card, err := svc.CheckpointCard(ctx, d.ID)
	require.NoError(t, err)
	var ids []string
	for _, r := range card.Reports {
		ids = append(ids, r.ID()+":"+r.Agent)
	}
	assert.Equal(t, []string{dev.ID() + ":developer", rev.ID() + ":reviewer"}, ids, "the plan was before cp-1")

	// Cited by the agent: attached although older.
	d2 := &domain.Decision{SessionID: "ses_a", Kind: domain.DecisionCheckpoint, ToolRef: "per_3", CreatedAt: at.Add(5 * time.Minute),
		Payload: domain.DecisionPayload{Data: map[string]any{checkpoint.DataCheckpoint: "cp-2", domain.DataReports: []any{"RP-1"}}}}
	require.NoError(t, svc.Raise(ctx, d2))
	card, err = svc.CheckpointCard(ctx, d2.ID)
	require.NoError(t, err)
	require.Len(t, card.Reports, 3)
	assert.Equal(t, plan.ID(), card.Reports[0].ID())

	_, body, err := svc.Report(ctx, "ses_a", rev.ID())
	require.NoError(t, err)
	assert.Equal(t, "## Verdict\nvalidé", body)

	// Without the report store: no report, no error.
	svc.Reports = nil
	card, err = svc.CheckpointCard(ctx, d.ID)
	require.NoError(t, err)
	assert.Empty(t, card.Reports)
}
