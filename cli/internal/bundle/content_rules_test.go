package bundle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

func shippedBundle(t *testing.T, id string) *Bundle {
	t.Helper()
	hub, cat, env := shippedWorkflows(t)
	r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: id}, nil, env)
	require.False(t, diags.HasErrors(), "%v", diags)
	b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), Spec: r.Spec})
	require.NoError(t, err)
	return b
}

func agentRules(b *Bundle, agent, action string) []sessionspec.PermissionRule {
	var out []sessionspec.PermissionRule
	for _, a := range b.Spec.Agents {
		if a.ID == agent {
			for _, r := range a.Permissions {
				if r.Action == action {
					out = append(out, r)
				}
			}
		}
	}
	return out
}

// DC26 (N11): quick and debug write files: the commit waits for cp-commit,
// which pauses in every mode.
func TestQuickAndDebugCommitGate(t *testing.T) {
	for _, id := range []string{"quick", "debug"} {
		wf := shippedBundle(t, id).Spec.Workflow
		require.NotNil(t, wf, id)
		cp, ok := wf.Checkpoint("cp-commit")
		require.True(t, ok, id)
		for _, mode := range []string{"manuel", "semi-auto", "auto"} {
			assert.Equal(t, sessionspec.CheckpointPause, cp.Behavior(mode), "%s %s", id, mode)
			assert.Equal(t, []string{sessionspec.UnlockCommit}, LockedOps(wf, mode, nil), "%s %s", id, mode)
		}
	}
}

// DC26 (N11): the onboarder writes only the wiki (and ONBOARDING.md,
// CONVENTIONS.md), reads the history, and never commits.
func TestOnboarderWritesOnlyTheWiki(t *testing.T) {
	b := shippedBundle(t, "onboarding")
	edit := agentRules(b, "onboarder", sessionspec.ActionEdit)
	require.NotEmpty(t, edit)
	eff := func(rules []sessionspec.PermissionRule, s string) sessionspec.Effect {
		e, _ := evaluate(rules, s)
		return e
	}
	for _, p := range []string{"docs/wiki/index.md", "docs/wiki/technical/stack.md", "ONBOARDING.md", "CONVENTIONS.md"} {
		assert.Equal(t, sessionspec.EffectAllow, eff(edit, p), p)
	}
	for _, p := range []string{".gitignore", ".git/info/exclude", "src/main.go", ".cache/context.json", "projects.md"} {
		assert.Equal(t, sessionspec.EffectDeny, eff(edit, p), p)
	}
	shell := agentRules(b, "onboarder", sessionspec.ActionShell)
	for _, c := range []string{"git log --oneline -20", "bd search auth", "bd list -s open --json", "ls -la"} {
		assert.Equal(t, sessionspec.EffectAllow, eff(shell, c), c)
	}
	for _, c := range []string{"git commit -m wiki", "shasum -a 256 go.mod", "rm -rf docs"} {
		assert.Equal(t, sessionspec.EffectDeny, eff(shell, c), c)
	}
}

// N11: the instructions of the agents match their permissions (matrix).
func TestAgentInstructionsMatchPermissions(t *testing.T) {
	feature := shippedBundle(t, "feature")
	eff := func(rules []sessionspec.PermissionRule, s string) sessionspec.Effect {
		e, _ := evaluate(rules, s)
		return e
	}
	doc := agentRules(feature, "documentarian", sessionspec.ActionShell)
	for _, c := range []string{"bd ready --label ai-delegated --json", "bd show pt-12", "bd list --json"} {
		assert.Equal(t, sessionspec.EffectAllow, eff(doc, c), "documentarian: %s", c)
	}
	designer := agentRules(feature, "designer", sessionspec.ActionShell)
	assert.Equal(t, sessionspec.EffectAllow, eff(designer, "bd list"), "designer: bd list without argument")
	assert.NotEmpty(t, agentRules(feature, "developer-migrator", sessionspec.ActionWebFetch), "developer-migrator reads changelogs on the web")

	sweep := shippedBundle(t, "sweep")
	for _, action := range []string{sessionspec.ActionRead, "glob", "grep"} {
		r := agentRules(sweep, "conductor", action)
		require.NotEmpty(t, r, action)
		assert.Equal(t, sessionspec.EffectDeny, eff(r, "src/main.go"), "conductor %s", action)
	}
}
