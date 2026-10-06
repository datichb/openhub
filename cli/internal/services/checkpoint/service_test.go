package checkpoint

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/adapters"
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
	svc := &Service{Sessions: sqlite.NewSessionStore(st), States: sqlite.NewCheckpointStore(st), BundlesDir: t.TempDir(), SessionsDir: t.TempDir()}
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

type fakeTool struct {
	replies  []string
	steers   []string
	refresh  int
	replyErr error
}

func (f *fakeTool) ReplyPermission(_ context.Context, _ domain.Decision, decision string) error {
	f.replies = append(f.replies, decision)
	return f.replyErr
}
func (f *fakeTool) Steer(_ context.Context, _ string, text string) error {
	f.steers = append(f.steers, text)
	return nil
}
func (f *fakeTool) Refresh(context.Context, string) error { f.refresh++; return nil }

func cpDecision(t *testing.T, svc *Service, ctx context.Context, id string) *domain.Decision {
	t.Helper()
	d, auto, err := svc.Asked(ctx, "ses_a", "ses_a", adapters.PendingDecision{ID: "per_" + id, Call: &adapters.ToolCall{Input: map[string]any{"id": id, "summary": "done"}}})
	require.NoError(t, err)
	require.False(t, auto)
	return d
}

func TestCheckpointLifecycle(t *testing.T) {
	svc, ctx := newTestService(t)
	tool := &fakeTool{}

	// semi-auto: cp-1 automatic (let through), cp-2 paused.
	_, auto, err := svc.Asked(ctx, "ses_a", "ses_a", adapters.PendingDecision{ID: "per_1", Call: &adapters.ToolCall{Input: map[string]any{"id": "cp-1"}}})
	require.NoError(t, err)
	assert.True(t, auto)
	_, auto, _ = svc.Asked(ctx, "ses_a", "ses_a", adapters.PendingDecision{ID: "per_x", Call: &adapters.ToolCall{Input: map[string]any{"id": "cp-9"}}})
	assert.True(t, auto, "unknown checkpoint: the call fails with the list")

	rules, err := svc.Rules(ctx, "ses_a")
	require.NoError(t, err)
	assert.Contains(t, rules, sessionspec.PermissionRule{Action: sessionspec.ActionSubagent, Resource: "developer", Effect: sessionspec.EffectDeny})
	_, err = svc.Reached(ctx, "ses_a", Call{ID: "cp-1"})
	require.NoError(t, err)
	rules, _ = svc.Rules(ctx, "ses_a")
	assert.NotContains(t, rules, sessionspec.PermissionRule{Action: sessionspec.ActionSubagent, Resource: "developer", Effect: sessionspec.EffectDeny}, "cp-1 releases developer")

	d := cpDecision(t, svc, ctx, "cp-2")
	assert.Equal(t, domain.DecisionCheckpoint, d.Kind)
	assert.Equal(t, "Commit", d.Payload.Title)
	assert.Equal(t, "done", d.Payload.Message)
	st, _ := svc.Status(ctx, "ses_a")
	assert.Equal(t, StatePassed, st.Checkpoints[0].State)
	assert.Equal(t, StateWaiting, st.Checkpoints[1].State)
	assert.Equal(t, "cp-2", st.Next)
	assert.Empty(t, st.Locked)

	// Refused with an instruction: reject, then the instruction is sent.
	require.Error(t, svc.Resolve(ctx, tool, *d, ChoiceFix, ""), "fix needs an instruction")
	require.NoError(t, svc.Resolve(ctx, tool, *d, ChoiceFix, "renomme la fonction"))
	assert.Equal(t, []string{"reject"}, tool.replies)
	require.Len(t, tool.steers, 1)
	assert.Contains(t, tool.steers[0], "renomme la fonction")
	cs, _ := svc.State(ctx, "ses_a")
	assert.Empty(t, cs.Waiting)

	// Asked again, validated with an instruction: returned by the call.
	d = cpDecision(t, svc, ctx, "cp-2")
	require.NoError(t, svc.Resolve(ctx, tool, *d, ChoiceApprove, "squash avant"))
	assert.Equal(t, []string{"reject", "once"}, tool.replies)
	res, err := svc.Reached(ctx, "ses_a", Call{ID: "cp-2"})
	require.NoError(t, err)
	assert.Equal(t, "squash avant", res.Message)
	cs, _ = svc.State(ctx, "ses_a")
	assert.True(t, cs.HasPassed("cp-2"))
	kinds := []string{}
	for _, e := range cs.Timeline {
		kinds = append(kinds, e.Kind+":"+e.ID+":"+e.By)
	}
	assert.Equal(t, []string{"passed:cp-1:mode", "waiting:cp-2:", "refused:cp-2:oh", "waiting:cp-2:", "passed:cp-2:oh"}, kinds)
	assert.Error(t, svc.Resolve(ctx, tool, *d, "maybe", ""))
}

func TestCircuitBreaker(t *testing.T) {
	svc, ctx := newTestService(t)
	writeBundle(t, svc.BundlesDir, "h_cb", &sessionspec.WorkflowRuntime{ID: "cb", MaxConsecutiveSubagents: 2})
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_cb", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: "h_cb", Mode: "manuel", State: domain.RunActive}))
	call := adapters.ToolCall{Action: sessionspec.ActionSubagent, Status: adapters.CallCalled, Input: map[string]any{"agent": "developer"}}

	changed, raised, err := svc.OnCall(ctx, "ses_cb", call)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Nil(t, raised)
	svc.OnUserInput(ctx, "ses_cb")
	_, raised, _ = svc.OnCall(ctx, "ses_cb", call)
	assert.Nil(t, raised, "the user spoke: counter reset")
	changed, raised, _ = svc.OnCall(ctx, "ses_cb", call)
	require.NotNil(t, raised)
	assert.True(t, changed)
	assert.Equal(t, domain.DecisionCircuit, raised.Kind)
	rules, _ := svc.Rules(ctx, "ses_cb")
	assert.Contains(t, rules, sessionspec.PermissionRule{Action: sessionspec.ActionSubagent, Resource: "*", Effect: sessionspec.EffectDeny})
	_, again, _ := svc.OnCall(ctx, "ses_cb", call)
	assert.Nil(t, again, "raised once")

	tool := &fakeTool{}
	require.NoError(t, svc.Resolve(ctx, tool, *raised, ChoiceDismiss, ""))
	assert.Equal(t, 1, tool.refresh)
	rules, _ = svc.Rules(ctx, "ses_cb")
	assert.NotContains(t, rules, sessionspec.PermissionRule{Action: sessionspec.ActionSubagent, Resource: "*", Effect: sessionspec.EffectDeny})

	changed, _, _ = svc.OnCall(ctx, "ses_cb", adapters.ToolCall{Action: sessionspec.ActionSubagent, Status: adapters.CallOK, Input: map[string]any{"agent": "developer"}})
	assert.False(t, changed, "developer locks nothing here")
	cs, _ := svc.State(ctx, "ses_cb")
	assert.Equal(t, []string{"developer"}, cs.Ran)
}
