package bundle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// R8: the release-manager of the shipped `release` workflow reads the tags
// freely; oh refuses it the commit, the tag and the merge request until
// cp-release, the tag until the release is committed, and the push always.
func TestShippedReleaseLocks(t *testing.T) {
	hub, cat, _ := shippedWorkflows(t)
	r, diags := workflow.ResolveSpec(cat, workflow.Ref{Layer: workflow.LayerHub, ID: "release"}, nil)
	require.Empty(t, diags.Errors())
	b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec, Provider: "bedrock"})
	require.NoError(t, err)
	var own []sessionspec.PermissionRule
	for _, a := range b.Spec.Agents {
		if a.ID == "release-manager" {
			own = append(append(own, a.Permissions...), b.Spec.AgentGuard...)
		}
	}
	require.NotEmpty(t, own)
	effect := func(st *domain.CheckpointState, cmd string) sessionspec.Effect {
		return ShellEffect(append(append([]sessionspec.PermissionRule(nil), own...), SessionRules(b.Spec.Workflow, "semi-auto", st)...), cmd)
	}
	allow, deny := sessionspec.EffectAllow, sessionspec.EffectDeny

	reads := []string{"git describe --tags --abbrev=0", "git for-each-ref --sort=-v:refname refs/tags", "git log v4.2.0..HEAD --format=%s",
		"git show v4.2.0", "git rev-parse v4.2.0^{commit}", "git cat-file -t v4.2.0", "git status", "date +%Y-%m-%d"}
	never := []string{"git push origin main", "git push origin v5.0.0", "git push --tags", "git push --follow-tags", "git tag -f v4.2.0", "git tag -a -f v4.2.0 -m x", "git tag --force v4.2.0", "git tag -d v4.2.0", "git tag --delete v4.2.0",
		"git reset --soft HEAD~1", "git tag -d v4.2.0 && git push origin :refs/tags/v4.2.0"}
	for _, cmd := range reads {
		assert.Equal(t, allow, effect(nil, cmd), "read: %s", cmd)
	}
	for _, cmd := range []string{"git commit -m 'chore(release): v5.0.0'", "git tag -a v5.0.0 -m x", "git tag -l", "glab mr create --fill", "gh pr create --fill"} {
		assert.Equal(t, deny, effect(nil, cmd), "before cp-release: %s", cmd)
	}

	now := time.Now()
	open := func(ops ...string) *domain.CheckpointState {
		st := &domain.CheckpointState{Passed: map[string]time.Time{"cp-release": now}, Unlocks: map[string]domain.CheckpointUnlock{}}
		for _, op := range ops {
			st.Unlocks[op] = domain.CheckpointUnlock{Checkpoint: "cp-release", At: now}
		}
		return st
	}
	passed := open(sessionspec.UnlockCommit, sessionspec.UnlockTag, sessionspec.UnlockMR)
	assert.Equal(t, allow, effect(passed, "git commit -m 'chore(release): v5.0.0'"))
	assert.Equal(t, deny, effect(passed, "git tag -a v5.0.0 -m x"), "not committed yet")
	assert.Equal(t, allow, effect(passed, "glab mr create --title x --description y --target-branch main"))

	committed := open(sessionspec.UnlockTag, sessionspec.UnlockMR)
	assert.Equal(t, allow, effect(committed, "git tag -a v5.0.0 -m 'Release v5.0.0'"))
	assert.Equal(t, deny, effect(committed, "git commit -m again"), "the commit window closed")
	for _, st := range []*domain.CheckpointState{nil, passed, committed} {
		for _, cmd := range never {
			assert.Equal(t, deny, effect(st, cmd), "never: %s", cmd)
		}
	}
	assert.Contains(t, b.Spec.Workflow.Checkpoints[0].Unlocks, sessionspec.UnlockTag)

	// Outside a workflow that unlocks it (free session on the
	// release-manager), the tag and the merge request are always refused.
	free := agentBuild(t, "release-manager", "bedrock")
	freeRules := append(append([]sessionspec.PermissionRule(nil), own...), SessionRules(free.Spec.Workflow, "", nil)...)
	for _, cmd := range []string{"git tag -a v5.0.0 -m x", "glab mr create --fill", "gh pr create --fill"} {
		assert.Equal(t, deny, ShellEffect(freeRules, cmd), "free session: %s", cmd)
	}
}
