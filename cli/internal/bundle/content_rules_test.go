package bundle

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

func shippedBundle(t *testing.T, id string) *Bundle {
	t.Helper()
	_, b := shippedBuild(t, "", workflow.Ref{Layer: workflow.LayerHub, ID: id})
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
// which pauses in every mode. In debug, the fix ticket waits for cp-ticket.
func TestQuickAndDebugCommitGate(t *testing.T) {
	locked := map[string][]string{
		"quick": {sessionspec.UnlockCommit, sessionspec.UnlockClose},
		"debug": {sessionspec.UnlockCreate, sessionspec.UnlockCommit, sessionspec.UnlockClose},
	}
	for _, id := range []string{"quick", "debug"} {
		wf := shippedBundle(t, id).Spec.Workflow
		require.NotNil(t, wf, id)
		cp, ok := wf.Checkpoint("cp-commit")
		require.True(t, ok, id)
		for _, mode := range []string{"manuel", "semi-auto", "auto"} {
			assert.Equal(t, sessionspec.CheckpointPause, cp.Behavior(mode), "%s %s", id, mode)
			assert.Equal(t, locked[id], LockedOps(wf, mode, nil), "%s %s", id, mode)
		}
	}
	cp, ok := shippedBundle(t, "debug").Spec.Workflow.Checkpoint("cp-ticket")
	require.True(t, ok)
	for _, mode := range []string{"manuel", "semi-auto", "auto"} {
		assert.Equal(t, sessionspec.CheckpointPause, cp.Behavior(mode), "debug cp-ticket %s", mode)
	}
}

// C0.2: closing a ticket is locked wherever an agent may close one, so a
// ticket is never marked done without a commit (and its provenance never
// records a commit that did not happen). libre has no checkpoint by design.
func TestShippedWorkflowsLockClose(t *testing.T) {
	_, cat, _ := shippedWorkflows(t)
	for _, ref := range cat.Refs() {
		r, diags := workflow.ResolveSpec(cat, ref, nil)
		require.Empty(t, diags.Errors(), ref.ID)
		if ref.ID == "libre" || r.Spec.Beads == nil {
			continue
		}
		allowsClose := false
		for _, cmd := range r.Spec.Beads.Allow {
			if cmd == "close" {
				allowsClose = true
			}
		}
		if !allowsClose {
			continue
		}
		locked := false
		for _, key := range r.Spec.Checkpoints.Keys() {
			cp, _ := r.Spec.Checkpoints.Get(key)
			for _, op := range cp.Unlocks {
				if string(op) == sessionspec.UnlockClose {
					locked = true
				}
			}
		}
		assert.True(t, locked, "%s allows bd close but no checkpoint unlocks close", ref.ID)
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

// C-5 (P1.1): no agent writes .git/info/exclude. The edit guard refuses
// .git/* to the agents that write the wiki, and the shipped content never
// tells them to add a file there: a line that names it forbids it first.
func TestNoAgentWritesGitInfoExclude(t *testing.T) {
	for wf, agents := range map[string][]string{
		"onboarding":     {"onboarder"},
		"implement-spec": {"documentarian", "developer"},
		"ticket":         {"documentarian", "developer"},
		"debug":          {"developer"},
	} {
		b := shippedBundle(t, wf)
		for _, agent := range agents {
			edit := agentRules(b, agent, sessionspec.ActionEdit)
			require.NotEmpty(t, edit, "%s %s", wf, agent)
			e, _ := evaluate(edit, ".git/info/exclude")
			assert.Equal(t, sessionspec.EffectDeny, e, "%s %s", wf, agent)
		}
	}
	hub := repoHub(t)
	forbids := regexp.MustCompile(`(?i)\bni\b|jamais|refus|interdit|\bpas\b|aucun`)
	for _, dir := range []string{"agents", "skills", "workflows/prompts"} {
		require.NoError(t, filepath.WalkDir(filepath.Join(hub, dir), func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			for i, line := range strings.Split(string(data), "\n") {
				before, _, found := strings.Cut(line, ".git/info/exclude")
				if found && !forbids.MatchString(before) {
					rel, _ := filepath.Rel(hub, p)
					t.Errorf("%s:%d tells an agent to write .git/info/exclude: %s", rel, i+1, line)
				}
			}
			return nil
		}))
	}
}

// N11: the instructions of the agents match their permissions (matrix).
func TestAgentInstructionsMatchPermissions(t *testing.T) {
	feature := shippedBundle(t, "libre")
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
	// C1: the developer runs the DB migration commands of the former
	// developer-migrator (domain migration).
	dev := agentRules(feature, "developer", sessionspec.ActionShell)
	for _, c := range []string{"alembic upgrade head", "python manage.py migrate --plan", "python manage.py makemigrations", "rails db:migrate", "rails db:rollback", "flask db upgrade", "npx prisma migrate diff"} {
		assert.Equal(t, sessionspec.EffectAllow, eff(dev, c), "developer: %s", c)
	}

	sweep := shippedBundle(t, "sweep")
	for _, action := range []string{sessionspec.ActionRead, "glob", "grep"} {
		r := agentRules(sweep, "conductor", action)
		require.NotEmpty(t, r, action)
		assert.Equal(t, sessionspec.EffectDeny, eff(r, "src/main.go"), "conductor %s", action)
	}
}
