package bundle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

func testRuntime() *sessionspec.WorkflowRuntime {
	return &sessionspec.WorkflowRuntime{
		ID: "ticket", DefaultMode: "semi-auto",
		Checkpoints: []sessionspec.CheckpointDef{
			{ID: "cp-1", Behaviors: map[string]string{"manuel": "pause", "semi-auto": "auto", "auto": "skip"}},
			{ID: "cp-2", Behaviors: map[string]string{"manuel": "pause", "semi-auto": "pause", "auto": "auto"}},
		},
		Gates: []sessionspec.AgentGate{{Agent: "developer", After: "cp-1"}, {Agent: "reviewer", After: "developer"}},
	}
}

func rule(action, res string, e sessionspec.Effect) sessionspec.PermissionRule {
	return sessionspec.PermissionRule{Action: action, Resource: res, Effect: e}
}

func TestSessionRules(t *testing.T) {
	wf := testRuntime()
	sub := sessionspec.ActionSubagent
	assert.Equal(t, []sessionspec.PermissionRule{
		rule(CheckpointAction(), "*", sessionspec.EffectAsk),
		rule(sub, "developer", sessionspec.EffectDeny),
		rule(sub, "reviewer", sessionspec.EffectDeny),
	}, SessionRules(wf, "", nil), "default mode: cp-2 pauses, both agents locked")

	st := &domain.CheckpointState{Passed: map[string]time.Time{"cp-1": time.Now()}}
	assert.Equal(t, []sessionspec.PermissionRule{
		rule(CheckpointAction(), "*", sessionspec.EffectAsk),
		rule(sub, "reviewer", sessionspec.EffectDeny),
	}, SessionRules(wf, "manuel", st), "cp-1 passed releases developer")

	st.Ran = []string{"developer"}
	assert.Equal(t, []sessionspec.PermissionRule{rule(CheckpointAction(), "*", sessionspec.EffectAsk)}, SessionRules(wf, "manuel", st))

	assert.Equal(t, []sessionspec.PermissionRule{
		rule(CheckpointAction(), "*", sessionspec.EffectAllow),
		rule(sub, "reviewer", sessionspec.EffectDeny),
	}, SessionRules(wf, "auto", nil), "nothing pauses in auto; cp-1 skipped releases developer")

	st.Breaker = true
	assert.Equal(t, []sessionspec.PermissionRule{
		rule(CheckpointAction(), "*", sessionspec.EffectAsk),
		rule(sub, "*", sessionspec.EffectDeny),
	}, SessionRules(wf, "manuel", st), "breaker holds every delegation")

	assert.Nil(t, SessionRules(nil, "manuel", nil))
}

func TestBaseCheckpointRulesInBundle(t *testing.T) {
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: parseSpec(t, checkpointSpec), Provider: "bedrock"})
	if err != nil {
		t.Fatal(err)
	}
	assert.Subset(t, b.Spec.Permissions, BaseCheckpointRules())
	assert.Equal(t, "semi-auto", b.Spec.Workflow.DefaultMode)
}

// A17: the operations a checkpoint unlocks are refused on the shell until it
// is passed, also while the circuit breaker holds; a skipped checkpoint
// locks nothing.
func TestSessionRulesLockedOperations(t *testing.T) {
	wf := testRuntime()
	wf.Checkpoints[1].Unlocks = []string{sessionspec.UnlockCommit, sessionspec.UnlockClose}
	shell := sessionspec.ActionShell
	locks := []sessionspec.PermissionRule{
		rule(shell, "*git*commit*", sessionspec.EffectDeny),
		rule(shell, "*bd*close*", sessionspec.EffectDeny),
		rule(shell, "*bd*update*closed*", sessionspec.EffectDeny),
	}
	assert.Subset(t, SessionRules(wf, "manuel", nil), locks)
	assert.Equal(t, []string{"commit", "close"}, LockedOps(wf, "manuel", nil))
	assert.Subset(t, SessionRules(wf, "manuel", &domain.CheckpointState{Breaker: true}), locks)

	open := &domain.CheckpointState{Passed: map[string]time.Time{"cp-2": time.Now()},
		Unlocks: map[string]domain.CheckpointUnlock{"commit": {Checkpoint: "cp-2"}, "close": {Checkpoint: "cp-2"}}}
	for _, r := range SessionRules(wf, "manuel", open) {
		assert.NotEqual(t, shell, r.Action, "window open: %v", r)
	}
	passedOnly := &domain.CheckpointState{Passed: map[string]time.Time{"cp-2": time.Now()}}
	assert.Equal(t, []string{"commit", "close"}, LockedOps(wf, "manuel", passedOnly), "a closed window locks again")

	wf.Checkpoints[1].Behaviors["auto"] = "skip"
	assert.Empty(t, LockedOps(wf, "auto", nil))
	c, ok := UnlockingCheckpoint(wf, sessionspec.UnlockClose)
	assert.True(t, ok)
	assert.Equal(t, "cp-2", c.ID)
}
