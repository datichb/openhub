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

	"github.com/datichb/openhub/cli/internal/daemon"
	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/services/checkpoint"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

func TestCheckpointCardAndResolver(t *testing.T) {
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	ctx := context.Background()
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	bundles := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bundles, "h1"), 0o755))
	data, _ := json.Marshal(sessionspec.BundleSpec{Hash: "h1", Workflow: &sessionspec.WorkflowRuntime{ID: "ticket",
		Checkpoints: []sessionspec.CheckpointDef{{ID: "cp-1", Behaviors: map[string]string{"manuel": "pause"}}, {ID: "cp-2", Behaviors: map[string]string{"manuel": "pause"}}}}})
	require.NoError(t, os.WriteFile(filepath.Join(bundles, "h1", "bundle.json"), data, 0o644))

	svc := &Service{Sessions: sqlite.NewSessionStore(st), Decisions: sqlite.NewDecisionStore(st), Servers: sqlite.NewServerStore(st), BundlesDir: bundles, SessionsDir: t.TempDir(),
		Live: func(context.Context, string) (<-chan daemon.StreamEvent, error) {
			ch := make(chan daemon.StreamEvent, 4)
			for _, txt := range []string{"a", "b", "c", "d"} {
				ch <- daemon.StreamEvent{Feed: &domain.FeedItem{SessionID: "ses_a", Kind: domain.FeedText, Agent: "reviewer", Text: txt}}
			}
			return ch, nil
		}}
	cp := &checkpoint.Service{Sessions: svc.Sessions, States: sqlite.NewCheckpointStore(st), BundlesDir: bundles}
	refreshed := 0
	svc.UseCheckpoints(cp, func(context.Context, string) error { refreshed++; return nil })
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_a", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: "h1", Mode: "manuel", State: domain.RunWaiting}))
	_, err = cp.States.UpdateCheckpointState(ctx, "ses_a", func(s *domain.CheckpointState) error {
		s.Passed = map[string]time.Time{"cp-1": time.Now()}
		s.Waiting = "cp-2"
		s.Timeline = []domain.CheckpointEvent{{Kind: domain.CheckpointPassed, ID: "cp-1"}, {Kind: domain.CheckpointWaiting, ID: "cp-2"}}
		return nil
	})
	require.NoError(t, err)
	d := &domain.Decision{SessionID: "ses_a", Kind: domain.DecisionCheckpoint, ToolRef: "per_2",
		Payload: domain.DecisionPayload{Title: "Commit", Message: "prêt", Data: map[string]any{checkpoint.DataCheckpoint: "cp-2"}}}
	require.NoError(t, svc.Raise(ctx, d))

	card, err := svc.CheckpointCard(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, "cp-2", card.Checkpoint)
	assert.Equal(t, "prêt", card.Summary)
	require.Len(t, card.Messages, 3, "last three texts")
	assert.Equal(t, "b", card.Messages[0].Text)
	assert.Contains(t, checkpoint.TimelineText(card.Timeline, 0), "⏸ cp-2")
	assert.Equal(t, "cp-2 « Commit »", DecisionSummary(*d))

	// The circuit breaker is dismissed through the resolver.
	c := &domain.Decision{SessionID: "ses_a", Kind: domain.DecisionCircuit, ToolRef: "1"}
	require.NoError(t, svc.Raise(ctx, c))
	require.NoError(t, svc.Decide(ctx, Reply{DecisionID: c.ID, Decision: "dismiss"}))
	assert.Equal(t, 1, refreshed)
	_, err = svc.CheckpointCard(ctx, c.ID)
	assert.Error(t, err, "not a checkpoint")
}
