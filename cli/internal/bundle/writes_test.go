package bundle

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// R4 (E8): the planner of the spec workflow writes and commits its spec
// under docs/wiki/specs/ and nothing else; the commit waits for cp-spec;
// the guards of the bundle still win; the other agents and the planner of
// the other workflows are unchanged.
func TestWritesPlannerOfSpec(t *testing.T) {
	b := scratchBundle(t, "spec", "")
	open := &domain.CheckpointState{Unlocks: map[string]domain.CheckpointUnlock{sessionspec.UnlockCommit: {Checkpoint: "cp-spec"}}}
	agent := func(id string, st *domain.CheckpointState, plugin bool) scratchAgent {
		var own []sessionspec.PermissionRule
		for _, a := range b.Spec.Agents {
			if a.ID == id {
				own = a.Permissions
			}
		}
		require.NotNil(t, own, id)
		session := SessionRules(b.Spec.Workflow, "semi-auto", st)
		if plugin {
			return scratchAgent{rules: append(append([]sessionspec.PermissionRule(nil), own...), session...), guard: b.Spec.AgentGuard}
		}
		return scratchAgent{rules: append(append(append([]sessionspec.PermissionRule(nil), own...), b.Spec.AgentGuard...), session...)}
	}
	for _, plugin := range []bool{true, false} {
		p := agent("planner", open, plugin)
		for _, f := range []string{"docs/wiki/specs/percent.md", "docs/wiki/specs/team/auth.md"} {
			assert.Equal(t, sessionspec.EffectAllow, p.effect(sessionspec.ActionEdit, f), "%s (plugin %v)", f, plugin)
		}
		for _, f := range []string{"calc.go", "docs/wiki/index.md", "docs/wiki/glossary.md", "README.md", "docs/wiki/specs.md", ".git/config", ".beads/config.yaml"} {
			assert.Equal(t, sessionspec.EffectDeny, p.effect(sessionspec.ActionEdit, f), "%s (plugin %v)", f, plugin)
		}
		for _, cmd := range []string{
			"git add docs/wiki/specs/percent.md",
			"git add -- docs/wiki/specs/percent.md",
			`git commit -m "docs(spec): percent" -- docs/wiki/specs/percent.md`,
			`git add docs/wiki/specs/percent.md && git commit -m "docs(spec): percent" -- docs/wiki/specs/percent.md`,
			`git commit -m 'docs(spec): percent' -- docs/wiki/specs/percent.md`,
			`git commit -m "docs(spec): l'API de calcul" -- docs/wiki/specs/team/auth.md`,
			`git commit -m "docs(spec): percent" -- docs/wiki/specs/`,
		} {
			assert.Equal(t, sessionspec.EffectAllow, writesShell(p, cmd), "%s (plugin %v)", cmd, plugin)
		}
		for _, cmd := range []string{
			`git commit -m "docs(spec): percent"`,
			`git commit -am "x" -- docs/wiki/specs/percent.md`,
			`git commit -a -m "x" -- docs/wiki/specs/percent.md`,
			`git commit --amend -m "x" -- docs/wiki/specs/percent.md`,
			`git commit -m "x" --include -- docs/wiki/specs/percent.md`,
			`git commit -m "x" --no-verify -- docs/wiki/specs/percent.md`,
			"git add .", "git add -A", "git add calc.go", "git add docs/wiki/specs/percent.md .",
			"git add -f docs/wiki/specs/percent.md",
			"git push", "git checkout docs/wiki/specs/percent.md", "rm docs/wiki/specs/percent.md",
			// p4-fixes: only the declared paths, before or after `--`
			"git commit -m x src/a.go -- docs/wiki/specs/x.md",
			`git commit -m "x" src/a.go -- docs/wiki/specs/x.md`,
			`git commit -m "x" src/a.go "" -- docs/wiki/specs/x.md`,
			`git commit -m 'x' src/a.go '' -- docs/wiki/specs/x.md`,
			`git commit -m "x" src/a.go ' "' -- docs/wiki/specs/x.md`,
			`git commit -m "x" -m "y" src/a.go -- docs/wiki/specs/x.md`,
			`git commit -m "x" ':(glob)**' -- docs/wiki/specs/x.md`,
			`git commit -m "x" -p -- docs/wiki/specs/x.md`,
			`git commit -m "x" --pathspec-from-file=list -- docs/wiki/specs/x.md`,
			`git commit --pathspec-from-file=list -m "x" -- docs/wiki/specs/x.md`,
			`git commit -m "x" -- docs/wiki/specs/x.md src/a.go`,
			`git commit -m "x" -- docs/wiki/specs/x.md docs/wiki/specs/y.md`,
			`git commit -m "x" -- docs/wiki/specs/../../src/a.go`,
			`git commit -m "x" -- docs/wiki/specs/\.\./\.\./src/a.go`,
			`git commit -m "x" -- docs/wiki/specs/'..'/../src/a.go`,
			`git commit -m "x" -- docs/wiki/specs/$(echo a)`,
			`git commit -m "x" -- docs/wiki/specs/x.md>src/a.go`,
			`git commit -m "x" -- ':(top)src/a.go'`,
			`git commit -m "x" -- src/a.go`,
			"git add docs/wiki/specs/x.md src/a.go", "git add -- docs/wiki/specs/x.md src/a.go",
			"git add docs/wiki/specs/../../src/a.go", "git add docs/wiki/specs/x.md	src/a.go",
			"git add ':(glob)**'", "git add --pathspec-from-file=list",
		} {
			assert.Equal(t, sessionspec.EffectDeny, writesShell(p, cmd), "%s (plugin %v)", cmd, plugin)
		}
		locked := agent("planner", nil, plugin)
		assert.Equal(t, sessionspec.EffectDeny, writesShell(locked, `git commit -m "x" -- docs/wiki/specs/percent.md`), "the commit waits for cp-spec (plugin %v)", plugin)
		assert.Equal(t, sessionspec.EffectAllow, locked.effect(sessionspec.ActionEdit, "docs/wiki/specs/percent.md"), "writing does not wait")

		for _, id := range []string{"conductor", "designer"} {
			o := agent(id, open, plugin)
			assert.Equal(t, sessionspec.EffectDeny, o.effect(sessionspec.ActionEdit, "docs/wiki/specs/percent.md"), "%s (plugin %v)", id, plugin)
		}
	}

	// The planner of cadrage keeps its refusal.
	c := scratchBundle(t, "cadrage", "")
	for _, a := range c.Spec.Agents {
		if a.ID != "planner" {
			continue
		}
		rules := append(append([]sessionspec.PermissionRule(nil), a.Permissions...), c.Spec.AgentGuard...)
		assert.Equal(t, sessionspec.EffectDeny, ToolEffect(rules, sessionspec.ActionEdit, "docs/wiki/specs/percent.md"))
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rules, "git add docs/wiki/specs/percent.md"))
		assert.NotContains(t, a.Body, "## Spec écrite")
		assert.Contains(t, a.Body, "Tu ne modifies jamais de fichiers")
	}
	for _, a := range b.Spec.Agents {
		if a.ID == "planner" {
			assert.Contains(t, a.Body, "## Spec écrite")
			assert.Contains(t, a.Body, "- **Écritures** (`writes:` du workflow) : tu peux créer et modifier `docs/wiki/specs/**`")
			assert.NotContains(t, a.Body, "Tu ne modifies jamais de fichiers")
		}
	}
}

// writesShell is what the tool decides for a command line: the guard of the
// plugin refuses first, then the rules of the agent.
func writesShell(a scratchAgent, line string) sessionspec.Effect {
	if len(a.guard) > 0 && ShellEffect(a.guard, line) == sessionspec.EffectDeny {
		return sessionspec.EffectDeny
	}
	return ShellEffect(a.rules, line)
}

func TestWritesRules(t *testing.T) {
	assert.Nil(t, WritesRules(nil))
	assert.Equal(t, "docs/wiki/specs/*", WritesPattern("docs/wiki/specs/**"))
	assert.Equal(t, "docs/adr/001.md", WritesPattern("docs/adr/001.md"))
	rules := append([]sessionspec.PermissionRule{
		{Action: sessionspec.ActionEdit, Resource: "*", Effect: sessionspec.EffectDeny},
		{Action: sessionspec.ActionShell, Resource: "*", Effect: sessionspec.EffectDeny},
	}, WritesRules([]string{"docs/adr/001.md"})...)
	assert.Equal(t, sessionspec.EffectAllow, ToolEffect(rules, sessionspec.ActionEdit, "docs/adr/001.md"))
	assert.Equal(t, sessionspec.EffectDeny, ToolEffect(rules, sessionspec.ActionEdit, "docs/adr/002.md"))
	assert.Equal(t, sessionspec.EffectAllow, ShellEffect(rules, `git commit -m "adr" -- docs/adr/001.md`))
	for _, r := range rules[2:] {
		assert.False(t, strings.Contains(r.Resource, "push"), r.Resource)
	}
}
