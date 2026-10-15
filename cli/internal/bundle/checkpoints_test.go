package bundle

import (
	"regexp"
	"strings"
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
	// Without a checkpoint unlocking `apply`, infrastructure changes stay
	// refused (N4): those rules come last.
	locked := func(rules ...sessionspec.PermissionRule) []sessionspec.PermissionRule {
		return append(append(rules, ApplyLockedByDefault(wf)...), PublishLockedByDefault(wf)...)
	}
	assert.Equal(t, locked(
		rule(CheckpointAction(), "*", sessionspec.EffectAsk),
		rule(sub, "developer", sessionspec.EffectDeny),
		rule(sub, "reviewer", sessionspec.EffectDeny),
	), SessionRules(wf, "", nil), "default mode: cp-2 pauses, both agents locked")

	st := &domain.CheckpointState{Passed: map[string]time.Time{"cp-1": time.Now()}}
	assert.Equal(t, locked(
		rule(CheckpointAction(), "*", sessionspec.EffectAsk),
		rule(sub, "reviewer", sessionspec.EffectDeny),
	), SessionRules(wf, "manuel", st), "cp-1 passed releases developer")

	st.Ran = []string{"developer"}
	assert.Equal(t, locked(rule(CheckpointAction(), "*", sessionspec.EffectAsk)), SessionRules(wf, "manuel", st))

	assert.Equal(t, locked(
		rule(CheckpointAction(), "*", sessionspec.EffectAllow),
		rule(sub, "reviewer", sessionspec.EffectDeny),
	), SessionRules(wf, "auto", nil), "nothing pauses in auto; cp-1 skipped releases developer")

	st.Breaker = true
	assert.Equal(t, locked(
		rule(CheckpointAction(), "*", sessionspec.EffectAsk),
		rule(sub, "*", sessionspec.EffectDeny),
	), SessionRules(wf, "manuel", st), "breaker holds every delegation")

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
// is passed, also while the circuit breaker holds.
func TestSessionRulesLockedOperations(t *testing.T) {
	wf := testRuntime()
	wf.Checkpoints[1].Unlocks = []string{sessionspec.UnlockCommit, sessionspec.UnlockClose}
	shell := sessionspec.ActionShell
	locks := []sessionspec.PermissionRule{
		rule(shell, "git commit*", sessionspec.EffectDeny),
		rule(shell, "bd close*", sessionspec.EffectDeny),
		rule(shell, "bd update *closed*", sessionspec.EffectDeny),
	}
	assert.Subset(t, SessionRules(wf, "manuel", nil), locks)
	assert.Equal(t, []string{"commit", "close"}, LockedOps(wf, "manuel", nil))
	assert.Subset(t, SessionRules(wf, "manuel", &domain.CheckpointState{Breaker: true}), locks)

	open := &domain.CheckpointState{Passed: map[string]time.Time{"cp-2": time.Now()},
		Unlocks: map[string]domain.CheckpointUnlock{"commit": {Checkpoint: "cp-2"}, "close": {Checkpoint: "cp-2"}}}
	for _, r := range SessionRules(wf, "manuel", open) {
		if !lockedByDefault(r.Resource) {
			assert.NotEqual(t, shell, r.Action, "window open: %v", r)
		}
	}
	passedOnly := &domain.CheckpointState{Passed: map[string]time.Time{"cp-2": time.Now()}}
	assert.Equal(t, []string{"commit", "close"}, LockedOps(wf, "manuel", passedOnly), "a closed window locks again")

	// A checkpoint that unlocks an operation is never skipped (N24, S-17):
	// skip counts as pause and the lock stays.
	wf.Checkpoints[1].Behaviors["auto"] = "skip"
	assert.Equal(t, []string{"commit", "close"}, LockedOps(wf, "auto", nil))
	c, ok := UnlockingCheckpoint(wf, sessionspec.UnlockClose)
	assert.True(t, ok)
	assert.Equal(t, "cp-2", c.ID)
}

func TestSessionRulesLockCreateAndApply(t *testing.T) {
	wf := testRuntime()
	wf.Checkpoints[1].Unlocks = []string{sessionspec.UnlockCreate, sessionspec.UnlockApply}
	rules := SessionRules(wf, "manuel", nil)
	shell := sessionspec.ActionShell
	assert.Contains(t, rules, rule(shell, "bd create*", sessionspec.EffectDeny))
	assert.Contains(t, rules, rule(shell, "*terraform* apply*", sessionspec.EffectDeny))
	assert.Contains(t, rules, rule(shell, "*kubectl* delete*", sessionspec.EffectDeny))
	assert.Equal(t, []string{"create", "apply"}, LockedOps(wf, "manuel", nil))

	// An operation unknown to this version locks nothing (DC9).
	wf.Checkpoints[1].Unlocks = []string{"deploy"}
	for _, r := range SessionRules(wf, "manuel", nil) {
		if !lockedByDefault(r.Resource) { // apply, tag and mr stay refused: nothing unlocks them (N4, E9)
			assert.NotEqual(t, shell, r.Action, "unknown operation: %v", r)
		}
	}
}

// N9: creating tickets (create, quick capture, duplicate, supersede) is
// refused on the shell until the checkpoint that unlocks `create`.
func TestSessionRulesLockCreate(t *testing.T) {
	wf := testRuntime()
	wf.Checkpoints[0].Unlocks = []string{sessionspec.UnlockCreate}
	rules := SessionRules(wf, "manuel", nil)
	for _, p := range []string{"bd create*", "bd q *", "bd duplicate*", "bd supersede*"} {
		assert.Contains(t, rules, rule(sessionspec.ActionShell, p, sessionspec.EffectDeny), p)
	}
	open := &domain.CheckpointState{Unlocks: map[string]domain.CheckpointUnlock{"create": {Checkpoint: wf.Checkpoints[0].ID}}}
	assert.Empty(t, LockedOps(wf, "manuel", open))
}

// r9b: changing tickets (update, labels, dependencies, comments,
// duplicates…) is refused on the shell until the checkpoint that unlocks
// `update`; reading them is not.
func TestSessionRulesLockUpdate(t *testing.T) {
	wf := testRuntime()
	wf.Checkpoints[0].Unlocks = []string{sessionspec.UnlockUpdate}
	var rules []sessionspec.PermissionRule
	for _, r := range SessionRules(wf, "manuel", nil) {
		if r.Action == sessionspec.ActionShell {
			rules = append(rules, r)
		}
	}
	for _, line := range []string{"bd update ev-1 -p 1", "bd label add ev-1 bug", "bd dep add ev-2 ev-1", "bd comments add ev-1 x",
		"bd comment ev-1 x", "bd duplicate ev-2 --of ev-1", "bd --json update ev-1 -p 0", "env bd reopen ev-3"} {
		assert.True(t, lockedLine(rules, line), line)
	}
	for _, line := range []string{"bd show ev-1", "bd list --label bug", "bd label list-all", "bd comments ev-1", "bd dep tree ev-1", "bd find-duplicates"} {
		assert.False(t, lockedLine(rules, line), line)
	}
	open := &domain.CheckpointState{Unlocks: map[string]domain.CheckpointUnlock{"update": {Checkpoint: wf.Checkpoints[0].ID}}}
	assert.Empty(t, LockedOps(wf, "manuel", open))
}

// lockedLine tells whether a shell line is refused by the deny rules, the
// way the tool evaluates it: each command of a compound line on its own
// (&&, ||, ;, |, subshells), a VAR=x prefix kept (both forms are covered).
func lockedLine(rules []sessionspec.PermissionRule, line string) bool {
	for _, cmd := range regexp.MustCompile(`&&|\|\||;|\||\(|\)|\$\(`).Split(line, -1) {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		if eff, ok := evaluate(rules, cmd); ok && eff == sessionspec.EffectDeny {
			return true
		}
	}
	return false
}

// N10: the lock patterns refuse the operation and its detours, not the
// commands that merely mention it.
func TestLockPatternsWithoutFalsePositives(t *testing.T) {
	wf := testRuntime()
	wf.Checkpoints[1].Unlocks = []string{sessionspec.UnlockCommit, sessionspec.UnlockPush, sessionspec.UnlockClose, sessionspec.UnlockCreate}
	var rules []sessionspec.PermissionRule
	for _, r := range SessionRules(wf, "manuel", nil) {
		if r.Action == sessionspec.ActionShell {
			rules = append(rules, r)
		}
	}
	for _, line := range []string{
		"git commit -m 'feat: x'", "git commit", "env git commit -m x", "GIT_X=1 git commit -m x", "git -c core.hooksPath=/dev/null commit -m x",
		"git -C /src commit -m x", "git --no-pager commit -m x", "/usr/bin/git commit -m x", "sh -c 'git commit -m x'", "bash -c \"git -c a=b commit\"",
		"git add . && git commit -m x", "git -c alias.ci='!git commit' ci", "git revert HEAD", "git cherry-pick abc", "git merge feat", "git pull",
		"git push", "git push origin main", "env git push", "git -C . push",
		"bd close pt-1", "bd --json close pt-1", "bd done pt-1", "bd update pt-1 -s closed", "env bd close pt-1",
		"bd create 'x'", "bd --json create --title x", "bd q 'x'", "bd duplicate pt-1 --of pt-2", "bd supersede pt-1",
	} {
		assert.True(t, lockedLine(rules, line), "must be refused: %s", line)
	}
	for _, line := range []string{
		"git log --grep commit", "git log --grep=commit", "git log -- commit.go", "git show HEAD:docs/commit.md", "git status",
		"git diff src/pushNotifications.ts", "git log --oneline -- push.go", "git merge-base HEAD main", "git diff main...HEAD",
		"bd list --status closed", "bd --json list --status closed", "bd list --created-after 2026-01-01", "bd show pt-1", "bd search create",
		"go test ./...", "rg 'git commit' docs",
	} {
		assert.False(t, lockedLine(rules, line), "must stay allowed: %s", line)
	}
}

// lockedByDefault: the patterns refused when no checkpoint unlocks their
// operation (apply, tag, mr).
func lockedByDefault(p string) bool {
	return contains(ApplyLockPatterns, p) || contains(tagLockPatterns, p) || contains(mrLockPatterns, p)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
