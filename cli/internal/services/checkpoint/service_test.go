package checkpoint

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/storage/sqlite"
)

// TestWorkflow is the runtime of the test bundles (cp-1 auto in semi-auto,
// cp-2 paused in every mode).
var TestWorkflow = sessionspec.WorkflowRuntime{
	ID: "ticket",
	Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-1", Label: map[string]string{"fr": "Démarrer", "en": "Start"}, Behaviors: map[string]string{"manuel": "pause", "semi-auto": "auto", "auto": "skip"}},
		{ID: "cp-2", Label: map[string]string{"": "Commit"}, Behaviors: map[string]string{"manuel": "pause", "semi-auto": "pause", "auto": "pause"}, Mandatory: true},
	},
	Gates:   []sessionspec.AgentGate{{Agent: "developer", After: "cp-1"}},
	Outputs: []sessionspec.OutputDef{{ID: "branch", Type: "branch"}, {ID: "mr", Type: "merge_request"}},
}

func writeBundle(t *testing.T, dir, hash string, wf *sessionspec.WorkflowRuntime) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, hash), 0o755))
	data, err := json.Marshal(sessionspec.BundleSpec{Hash: hash, EntryAgent: "orchestrator-dev", Workflow: wf})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, hash, "bundle.json"), data, 0o644))
}

func newTestService(t *testing.T) (*Service, context.Context) {
	t.Helper()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "oh.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	_, err = st.DB().Exec(`INSERT INTO projects (id, name, path) VALUES ('p1','p1','/p1')`)
	require.NoError(t, err)
	svc := &Service{Sessions: sqlite.NewSessionStore(st), BundlesDir: t.TempDir(), SessionsDir: t.TempDir()}
	writeBundle(t, svc.BundlesDir, "h_wf", &TestWorkflow)
	writeBundle(t, svc.BundlesDir, "h_plain", nil)
	ctx := context.Background()
	for id, h := range map[string]string{"ses_a": "h_wf", "ses_plain": "h_plain"} {
		require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: id, ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: h, Mode: "semi-auto", State: domain.RunActive}))
	}
	return svc, ctx
}

func TestStatus(t *testing.T) {
	svc, ctx := newTestService(t)
	st, err := svc.Status(ctx, "ses_a")
	require.NoError(t, err)
	assert.Equal(t, "ticket", st.Workflow)
	assert.Equal(t, "semi-auto", st.Mode)
	require.Len(t, st.Checkpoints, 2)
	assert.Equal(t, "auto", st.Checkpoints[0].Behavior)
	assert.Equal(t, StateTodo, st.Checkpoints[0].State)
	assert.Equal(t, "cp-1", st.Next)
	assert.Equal(t, TestWorkflow.Gates, st.Locked)

	_, err = svc.Status(ctx, "ses_plain")
	assert.ErrorIs(t, err, ErrNoWorkflow)
	_, err = svc.Status(ctx, "ses_missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestReached(t *testing.T) {
	svc, ctx := newTestService(t)
	res, err := svc.Reached(ctx, "ses_a", Call{ID: "cp-1", Summary: "ok"})
	require.NoError(t, err)
	assert.Equal(t, "auto", res.Behavior)
	assert.Equal(t, "cp-2", res.Next)

	_, err = svc.Reached(ctx, "ses_a", Call{ID: "cp-9"})
	assert.ErrorIs(t, err, ErrUnknownCP)
	assert.ErrorContains(t, err, "cp-1, cp-2", "known checkpoints listed for the agent")
}

func TestDeclareOutputs(t *testing.T) {
	svc, ctx := newTestService(t)
	require.NoError(t, svc.Declare(ctx, "ses_a", Output{Type: "branch", Value: "feat/a"}))
	require.NoError(t, svc.Declare(ctx, "ses_a", Output{Type: "merge_request", Value: "!12"}))
	require.NoError(t, svc.Declare(ctx, "ses_a", Output{Type: "branch", Value: "feat/b"}))
	assert.ErrorContains(t, svc.Declare(ctx, "ses_a", Output{Type: "path", Value: "x"}), "branch:branch, mr:merge_request")
	assert.Error(t, svc.Declare(ctx, "ses_a", Output{Type: "branch"}))

	out, err := svc.Outputs("ses_a")
	require.NoError(t, err)
	require.Len(t, out, 2, "same output replaced")
	assert.Equal(t, "mr", out[0].ID)
	assert.Equal(t, "branch", out[1].ID, "id taken from the workflow")
	assert.Equal(t, "feat/b", out[1].Value)

	info, err := os.Stat(filepath.Join(svc.SessionsDir, "ses_a", "outputs.json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
