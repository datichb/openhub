package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// R6, CP-R3: no session of a fan-out pushes, merges, closes a ticket or
// moves the branches of the fan-out (tests of getting around the guard).

var (
	fanoutPushForms  = []string{"git push", "git push origin HEAD", "env git push -f", "/usr/bin/git push", "git -C ../x push", "sh -c 'git push'"}
	fanoutMergeForms = []string{
		"git merge oh/fanout/abc/integration", "git merge --no-ff feat/x", "git pull", "git pull --rebase origin main", "env git merge x",
		"git -C ../integ merge x", "sh -c 'git merge x'", "glab mr merge 12", "gh pr merge 3 --squash", "glab api -X PUT projects/1/merge_requests/2/merge",
		"gh api -X PUT repos/o/r/pulls/3/merge",
	}
	fanoutCloseForms  = []string{"bd close ev-1", "bd close ev-1 --reason done", "bd --json close ev-1", "bd done ev-1", "bd update ev-1 --status closed", "env bd close ev-1"}
	fanoutBranchForms = []string{
		"git update-ref refs/heads/oh/fanout/abc/integration HEAD", "git branch -f oh/fanout/abc/integration HEAD", "git branch -D oh/fanout/abc/ev-2",
		"git checkout oh/fanout/abc/integration", "git switch oh/fanout/abc/integration", "git worktree add ../x oh/fanout/abc/integration",
		"git reset --hard oh/fanout/abc/integration", "echo x > .git/refs/heads/oh/fanout/abc/integration",
	}
	fanoutAllowed = []string{"git status", "git diff", "git log --oneline -5", "git merge-base HEAD main", "bd show ev-1", "go test ./...", "git add calc.go"}
)

func TestFanoutRoleRules(t *testing.T) {
	assert.Nil(t, FanoutRoleRules(""), "not a fan-out session")
	for _, role := range []string{FanoutParent, FanoutChild, FanoutMerger} {
		rules := FanoutRoleRules(role)
		for _, group := range [][]string{fanoutPushForms, fanoutMergeForms, fanoutCloseForms, fanoutBranchForms} {
			for _, cmd := range group {
				assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), "%s: %s", role, cmd)
			}
		}
		for _, cmd := range fanoutAllowed {
			assert.NotEqual(t, sessionspec.EffectDeny, ShellEffect(rules, cmd), "%s: %s", role, cmd)
		}
		assert.True(t, denies(rules, sessionspec.MCPToolAction("gitlab", "merge_mr")), role)
		assert.True(t, denies(rules, sessionspec.MCPToolAction("gitlab", "create_mr")), role)
	}
	// The parent never commits: oh merges the integration branch.
	assert.Equal(t, sessionspec.EffectDeny, ShellEffect(FanoutRoleRules(FanoutParent), "git commit -m x"))
	for _, role := range []string{FanoutChild, FanoutMerger} {
		assert.NotEqual(t, sessionspec.EffectDeny, ShellEffect(FanoutRoleRules(role), "git commit --no-edit"), role)
	}
	assert.Equal(t, []string{"push", "merge", "close"}, FanoutLockedOps(FanoutChild))
	assert.Equal(t, []string{"commit", "push", "merge", "close"}, FanoutLockedOps(FanoutParent))
}

func TestSubstitutes(t *testing.T) {
	merge := sessionspec.CheckpointDef{ID: "cp-merge", Mandatory: true, Unlocks: []string{sessionspec.UnlockMerge},
		Behaviors: map[string]string{"manuel": "pause", "semi-auto": "pause", "auto": "pause"},
		Verify:    &sessionspec.VerifyDef{From: "stack", Expect: "pass", On: sessionspec.VerifyOnIntegration}}
	wf := func(mut func(*sessionspec.CheckpointDef, *sessionspec.FanoutDef)) *sessionspec.WorkflowRuntime {
		m := merge
		v := *merge.Verify
		m.Verify = &v
		m.Behaviors = map[string]string{}
		for k, b := range merge.Behaviors {
			m.Behaviors[k] = b
		}
		f := &sessionspec.FanoutDef{Child: "ticket", Merge: "cp-merge", Delegates: []string{"cp-2"}}
		mut(&m, f)
		return &sessionspec.WorkflowRuntime{Checkpoints: []sessionspec.CheckpointDef{m}, Fanout: f}
	}
	assert.True(t, wf(func(*sessionspec.CheckpointDef, *sessionspec.FanoutDef) {}).Substitutes("cp-2"))
	assert.False(t, wf(func(*sessionspec.CheckpointDef, *sessionspec.FanoutDef) {}).Substitutes("cp-1"), "not delegated")
	refused := map[string]func(*sessionspec.CheckpointDef, *sessionspec.FanoutDef){
		"not mandatory":      func(m *sessionspec.CheckpointDef, _ *sessionspec.FanoutDef) { m.Mandatory = false },
		"no verification":    func(m *sessionspec.CheckpointDef, _ *sessionspec.FanoutDef) { m.Verify = nil },
		"not on integration": func(m *sessionspec.CheckpointDef, _ *sessionspec.FanoutDef) { m.Verify.On = "" },
		"expects a failure":  func(m *sessionspec.CheckpointDef, _ *sessionspec.FanoutDef) { m.Verify.Expect = "fail" },
		"no merge unlocked":  func(m *sessionspec.CheckpointDef, _ *sessionspec.FanoutDef) { m.Unlocks = nil },
		"auto in a mode":     func(m *sessionspec.CheckpointDef, _ *sessionspec.FanoutDef) { m.Behaviors["auto"] = "auto" },
		"no behavior":        func(m *sessionspec.CheckpointDef, _ *sessionspec.FanoutDef) { m.Behaviors = nil },
		"unknown merge":      func(_ *sessionspec.CheckpointDef, f *sessionspec.FanoutDef) { f.Merge = "cp-nope" },
		"no delegates":       func(_ *sessionspec.CheckpointDef, f *sessionspec.FanoutDef) { f.Delegates = nil },
	}
	for name, mut := range refused {
		assert.False(t, wf(mut).Substitutes("cp-2"), name)
	}
	assert.False(t, (&sessionspec.WorkflowRuntime{}).Substitutes("cp-2"), "no fan-out")
}
