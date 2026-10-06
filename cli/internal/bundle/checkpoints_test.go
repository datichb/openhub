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
