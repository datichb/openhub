package bundle

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

func held(guard []sessionspec.PermissionRule, action, resource string) bool {
	eff, ok := SessionRulesEffect(guard, action, []string{resource})
	return ok && eff == sessionspec.EffectDeny
}

// p6-debts: a subagent session not adopted yet runs with the rules of its
// agent only. The operations its checkpoints lock are held by the plugin
// until oh confirms the session rules; the rest is unchanged.
func TestPendingGuardTicket(t *testing.T) {
	b := shippedBundle(t, "ticket")
	guard := PendingGuard(b.Spec.Workflow)
	require.NotEmpty(t, guard)

	// The bypass: the developer's own rules commit (developer-rw).
	dev := agentRules(b, "developer", sessionspec.ActionShell)
	require.NotEmpty(t, dev)
	assert.Equal(t, sessionspec.EffectAllow, ShellEffect(dev, "git commit -m wip"), "the agent rules alone let the commit through")

	for _, cmd := range []string{
		"git commit -m wip", "git -C . commit -m x", "env git commit -m x", "git push origin HEAD", "git merge main",
		"git cherry-pick abc", "git revert HEAD", "bd close oh-1", "bd update oh-1 --status closed",
		"git tag v1", "glab mr create", "terraform apply",
	} {
		assert.True(t, held(guard, sessionspec.ActionShell, cmd), "held until oh confirms: %s", cmd)
	}
	for _, cmd := range []string{"git status", "git log --grep commit", "git diff src/pushX.ts", "go test ./...", "bd show oh-1", "git merge-base a b", "git add src/a.go"} {
		assert.False(t, held(guard, sessionspec.ActionShell, cmd), "unchanged: %s", cmd)
	}
	for _, r := range guard {
		assert.Equal(t, sessionspec.EffectDeny, r.Effect)
		assert.NotEqual(t, CheckpointAction(), r.Action, "workflow_checkpoint is relayed as gated")
	}
}

// writes_after: the spec of the planner (subagent of spec) is held too.
func TestPendingGuardWritesAfter(t *testing.T) {
	b := shippedBundle(t, "spec")
	guard := PendingGuard(b.Spec.Workflow)
	require.NotEmpty(t, b.Spec.Workflow.WritesGates)
	for _, g := range b.Spec.Workflow.WritesGates {
		for _, p := range g.Paths {
			assert.True(t, held(guard, sessionspec.ActionEdit, WritesPattern(p)), p)
		}
	}
	assert.False(t, held(guard, sessionspec.ActionEdit, "src/a.go"))
}

// Every shipped workflow: what its session rules refuse at the start, in
// any mode, is held; the guard stays small enough for the inline config.
func TestPendingGuardShipped(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"ticket", "spec", "implement-spec", "incident", "security-fix", "release", "sweep", "sweep-task", "infra-change", "onboarding", "quick", "debug", "review"} {
		b := shippedBundle(t, id)
		wf := b.Spec.Workflow
		guard := PendingGuard(wf)
		modes := []string{wf.DefaultMode, "auto", "semi-auto", "manual"}
		for _, m := range modes {
			for _, r := range SessionRules(wf, m, &domain.CheckpointState{}) {
				if r.Effect == sessionspec.EffectDeny && r.Action == sessionspec.ActionShell {
					assert.True(t, held(guard, r.Action, r.Resource), "%s/%s: %s", id, m, r.Resource)
				}
			}
		}
		data, err := json.Marshal(guard)
		require.NoError(t, err)
		assert.Less(t, len(data), 48<<10, "%s: pending guard of %d bytes", id, len(data))
	}
	assert.Nil(t, PendingGuard(nil))
}

func TestSessionRulesEffect(t *testing.T) {
	rules := []sessionspec.PermissionRule{
		{Action: "shell", Resource: "git commit*", Effect: sessionspec.EffectDeny},
		{Action: "edit", Resource: "docs/*", Effect: sessionspec.EffectAsk},
	}
	eff, ok := SessionRulesEffect(rules, "shell", []string{"git status", "git commit -m x"})
	assert.True(t, ok)
	assert.Equal(t, sessionspec.EffectDeny, eff)
	_, ok = SessionRulesEffect(rules, "shell", []string{"git status"})
	assert.False(t, ok)
	eff, ok = SessionRulesEffect(rules, "edit", []string{"docs/a.md"})
	assert.True(t, ok)
	assert.Equal(t, sessionspec.EffectAsk, eff)
}

// p6-debts: the mitigation of incident is committed at cp-mitigated without
// the postmortem, which waits for cp-postmortem.
func TestEarlyCommitWindowKeepsWritesPaths(t *testing.T) {
	b := shippedBundle(t, "incident")
	wf := b.Spec.Workflow
	require.Equal(t, []string{"docs/wiki/postmortems/**"}, wf.Writes["debugger"])
	at := func(cp string) []sessionspec.PermissionRule {
		return SessionRules(wf, "semi-auto", &domain.CheckpointState{Unlocks: map[string]domain.CheckpointUnlock{sessionspec.UnlockCommit: {Checkpoint: cp}}})
	}
	early, late := at("cp-mitigated"), at("cp-postmortem")
	for _, cmd := range []string{
		"git add docs/wiki/postmortems/2026-10-14.md", "git add docs/", "git add docs/wiki", "git add -A", "git add .", "git commit -am fix",
		`git commit -m "fix" -- docs/wiki/postmortems/x.md`, "git add 'docs/*'", "git add :/",
	} {
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(early, cmd), "cp-mitigated: %s", cmd)
		assert.NotEqual(t, sessionspec.EffectDeny, ShellEffect(late, cmd), "cp-postmortem: %s", cmd)
	}
	for _, cmd := range []string{"git add src/cache.go", `git commit -m "fix(cache): ttl" -- src/cache.go`, "git add docs-site/x.md"} {
		assert.NotEqual(t, sessionspec.EffectDeny, ShellEffect(early, cmd), cmd)
	}
}
