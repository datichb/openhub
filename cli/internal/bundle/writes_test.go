package bundle

import (
	"strings"
	"testing"
	"time"

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
	t.Parallel()
	b := scratchBundle(t, "spec", "")
	understood := &domain.CheckpointState{Passed: map[string]time.Time{"cp-understanding": time.Now()}}
	open := &domain.CheckpointState{Passed: understood.Passed, Unlocks: map[string]domain.CheckpointUnlock{sessionspec.UnlockCommit: {Checkpoint: "cp-spec"}}}
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
		assert.Equal(t, sessionspec.EffectAllow, p.effect(sessionspec.ActionEdit, "docs/wiki/index.md"), "p6-debts: the planner adds its spec to the index (plugin %v)", plugin)
		for _, f := range []string{"calc.go", "docs/wiki/glossary.md", "README.md", "docs/wiki/specs.md", ".git/config", ".beads/config.yaml"} {
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
		// p6-debts: several declared paths, several -m: the rules ask, oh
		// reads the line and lets it through.
		paths := b.Spec.Workflow.Writes["planner"]
		require.NotEmpty(t, paths)
		for _, cmd := range []string{
			`git commit -m "x" -- docs/wiki/specs/x.md docs/wiki/specs/y.md`,
			`git commit -m "docs(spec): percent" -m "Corps du commit, l'API." -- docs/wiki/specs/percent.md`,
			`git commit -m 'docs(spec): x' -m 'corps' -- docs/wiki/specs/x.md docs/wiki/specs/`,
			"git add docs/wiki/specs/x.md docs/wiki/specs/team/y.md",
			"git add -- docs/wiki/specs/x.md docs/wiki/specs/team/y.md",
			`git add docs/wiki/specs/x.md docs/wiki/specs/y.md && git commit -m "x" -m "y" -- docs/wiki/specs/x.md docs/wiki/specs/y.md`,
			"git add docs/wiki/specs/x.md docs/wiki/index.md",
			`git commit -m "docs(spec): x" -- docs/wiki/specs/x.md docs/wiki/index.md`,
		} {
			assert.Equal(t, sessionspec.EffectAsk, writesShell(p, cmd), "%s (plugin %v)", cmd, plugin)
			assert.Equal(t, sessionspec.EffectAllow, writesOh(p, paths, cmd), "%s (plugin %v)", cmd, plugin)
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
			// p6-debts: one undeclared path among several, injection in -m
			`git commit -m "x" -- docs/wiki/specs/x.md docs/wiki/specs/y.md src/a.go`,
			`git commit -m "x" -- docs/wiki/specs/x.md docs/wiki/specs/../../src/a.go`,
			`git commit -m "x" -- docs/wiki/specs/x.md ../src/a.go`,
			`git commit -m "x" -- docs/wiki/specs/x.md .`,
			`git commit -m "x" -- docs/wiki/specs/x.md docs/wiki/specs/.hidden`,
			`git commit -m "x" -- docs/wiki/specs/x.md -a`,
			`git commit -m "x" -- docs/wiki/specs/x.md ':(top)src'`,
			`git commit -m "x" -- docs/wiki/specs/x.md docs/wiki`,
			`git commit -m "x" -m "$(rm -rf src)" -- docs/wiki/specs/x.md`,
			"git commit -m \"x\" -m \"`id`\" -- docs/wiki/specs/x.md",
			`git commit -m "x" -m "y" -a -- docs/wiki/specs/x.md`,
			`git commit -m "x" -m "y" src/a.go -- docs/wiki/specs/x.md`,
			`git commit -m "x" -m "y" --amend -- docs/wiki/specs/x.md`,
			`git commit -m "x" -m 'y'"z" -- docs/wiki/specs/x.md`,
			`git commit -m "x" -m "y -- docs/wiki/specs/x.md`,
			"git add docs/wiki/specs/x.md docs/wiki/specs/y.md src/a.go",
			"git add docs/wiki/specs/x.md docs/wiki/specs/y.md > /tmp/x",
			"git add docs/wiki/specs/x.md docs/wiki/specs/y.md *",
		} {
			assert.Equal(t, sessionspec.EffectDeny, writesOh(p, paths, cmd), "%s (plugin %v)", cmd, plugin)
		}
		locked := agent("planner", understood, plugin)
		assert.Equal(t, sessionspec.EffectDeny, writesShell(locked, `git commit -m "x" -- docs/wiki/specs/percent.md`), "the commit waits for cp-spec (plugin %v)", plugin)
		assert.Equal(t, sessionspec.EffectAllow, locked.effect(sessionspec.ActionEdit, "docs/wiki/specs/percent.md"), "writing waits for cp-understanding only")
		// r5-fixes: before cp-understanding the edit asks, so that oh
		// refuses it with its message (services/checkpoint EvaluateEdit);
		// the other files keep their refusal.
		early := agent("planner", nil, plugin)
		assert.Equal(t, sessionspec.EffectAsk, early.effect(sessionspec.ActionEdit, "docs/wiki/specs/percent.md"), "writing waits for cp-understanding (plugin %v)", plugin)
		assert.Equal(t, sessionspec.EffectDeny, early.effect(sessionspec.ActionEdit, "calc.go"), "plugin %v", plugin)

		for _, id := range []string{"conductor", "designer"} {
			o := agent(id, open, plugin)
			assert.Equal(t, sessionspec.EffectDeny, o.effect(sessionspec.ActionEdit, "docs/wiki/specs/percent.md"), "%s (plugin %v)", id, plugin)
		}
	}

	// The planner of libre (no writes:) keeps its refusal.
	c := scratchBundle(t, "libre", "")
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
			assert.Contains(t, a.Body, "Seulement une fois `cp-understanding` passé : avant, oh bloque l'écriture")
			assert.NotContains(t, a.Body, "Tu ne modifies jamais de fichiers")
		}
	}
}

// onboarding-refresh: the onboarder of onboarding commits the wiki once
// cp-wiki is passed; p6-debts: with ONBOARDING.md and CONVENTIONS.md, its
// writes: paths too, in one commit.
func TestWritesOnboarderOfOnboarding(t *testing.T) {
	t.Parallel()
	b := scratchBundle(t, "onboarding", "")
	var own []sessionspec.PermissionRule
	var body string
	for _, a := range b.Spec.Agents {
		if a.ID == "onboarder" {
			own, body = a.Permissions, a.Body
		}
	}
	require.NotNil(t, own)
	open := &domain.CheckpointState{Unlocks: map[string]domain.CheckpointUnlock{sessionspec.UnlockCommit: {Checkpoint: "cp-wiki"}}}
	agent := func(st *domain.CheckpointState) scratchAgent {
		return scratchAgent{rules: append(append(append([]sessionspec.PermissionRule(nil), own...), b.Spec.AgentGuard...), SessionRules(b.Spec.Workflow, "semi-auto", st)...)}
	}
	o := agent(open)
	for _, f := range []string{"docs/wiki/index.md", "docs/wiki/technical/architecture.md", "ONBOARDING.md", "CONVENTIONS.md"} {
		assert.Equal(t, sessionspec.EffectAllow, o.effect(sessionspec.ActionEdit, f), f)
	}
	paths := b.Spec.Workflow.Writes["onboarder"]
	// p6-debts: ONBOARDING.md and CONVENTIONS.md are committed with the
	// wiki at cp-wiki, nothing else.
	for _, cmd := range []string{"git add ONBOARDING.md", `git commit -m "x" -- ONBOARDING.md`,
		"git add docs/wiki/ ONBOARDING.md CONVENTIONS.md", `git commit -m "docs(wiki): onboarding" -- docs/wiki/ ONBOARDING.md CONVENTIONS.md`} {
		assert.Equal(t, sessionspec.EffectAllow, writesOh(o, paths, cmd), cmd)
	}
	for _, cmd := range []string{"git add docs/wiki/ ONBOARDING.md README.md", `git commit -m "x" -- ONBOARDING.md main.go`, "git add ONBOARDING.md .gitignore"} {
		assert.Equal(t, sessionspec.EffectDeny, writesOh(o, paths, cmd), cmd)
	}
	for _, f := range []string{"main.go", "README.md", ".gitignore"} {
		assert.Equal(t, sessionspec.EffectDeny, o.effect(sessionspec.ActionEdit, f), f)
	}
	for _, cmd := range []string{"git add docs/wiki/", `git commit -m "docs(wiki): onboarding" -- docs/wiki/`} {
		assert.Equal(t, sessionspec.EffectAllow, writesShell(o, cmd), cmd)
	}
	assert.Equal(t, sessionspec.EffectDeny, writesShell(agent(nil), `git commit -m "docs(wiki): onboarding" -- docs/wiki/`), "the commit waits for cp-wiki")
	for _, cmd := range []string{"git add -A", `git commit -am "x"`, "git push"} {
		assert.Equal(t, sessionspec.EffectDeny, writesShell(o, cmd), cmd)
	}
	assert.Contains(t, body, "- **Écritures** (`writes:` du workflow) : tu peux créer et modifier `docs/wiki/**`, `ONBOARDING.md`, `CONVENTIONS.md`")
	assert.Contains(t, body, "un ou plusieurs de ces chemins")
}

// writesOh is what the session gets for a line once oh answered the
// requests the rules ask (EvaluateWrites).
func writesOh(a scratchAgent, paths []string, line string) sessionspec.Effect {
	eff := writesShell(a, line)
	if eff != sessionspec.EffectAsk {
		return eff
	}
	for _, c := range ShellCommands(line) {
		if !IsWritesLine(c) {
			return eff
		}
	}
	for _, c := range ShellCommands(line) {
		if WritesLineProblem(paths, c) != "" {
			return sessionspec.EffectDeny
		}
	}
	return sessionspec.EffectAllow
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

// p6-debts: the lines oh reads for a writes: agent.
func TestWritesLineProblem(t *testing.T) {
	paths := []string{"docs/wiki/**", "ONBOARDING.md", "CONVENTIONS.md"}
	for _, cmd := range []string{
		"git add docs/wiki/ ONBOARDING.md CONVENTIONS.md",
		"git add -- ONBOARDING.md docs/wiki/technical/architecture.md",
		`git commit -m "docs(wiki): onboarding" -- docs/wiki/ ONBOARDING.md CONVENTIONS.md`,
		`git commit -m "docs: guide" -m "Corps : l'équipe, les conventions." -m 'Ref: oh-12' -- ONBOARDING.md`,
		"git commit -m wip -- docs/wiki",
	} {
		assert.Empty(t, WritesLineProblem(paths, cmd), cmd)
	}
	for _, cmd := range []string{
		"git add docs/wiki/ ONBOARDING.md src/main.go",
		"git add README.md",
		"git add ONBOARDING.md/",
		"git add ONBOARDING.mdx",
		"git add docs/wiki/../../src",
		"git add docs/wiki/./x",
		"git add docs/wiki/.git",
		"git add /etc/passwd",
		"git add ':(top)src'",
		"git add -f ONBOARDING.md",
		"git add -A",
		"git add",
		"git add --",
		"git add docs/wiki/*",
		"git add docs/wiki/$X",
		"git add docs/wiki/x;rm",
		"git add ONBOARDING.md && rm -rf src",
		`git commit -m "x" ONBOARDING.md`,
		`git commit -- ONBOARDING.md`,
		`git commit -m "x" -a -- ONBOARDING.md`,
		`git commit -m "x" --amend -- ONBOARDING.md`,
		`git commit -m "x" src/a.go -- ONBOARDING.md`,
		`git commit -m "$(id)" -- ONBOARDING.md`,
		"git commit -m \"`id`\" -- ONBOARDING.md",
		`git commit -m "a\" b" -- ONBOARDING.md`,
		`git commit -m "x -- ONBOARDING.md`,
		`git commit -m "x"y -- ONBOARDING.md`,
		"git commit -m 'a\nb' -- ONBOARDING.md",
		`git commit -m "x" -- "ONBOARDING.md"`,
		`git commit -m "x" -- ONBOARDING.md -- src`,
		"git -C src add ONBOARDING.md",
		"git push",
	} {
		assert.NotEmpty(t, WritesLineProblem(paths, cmd), cmd)
	}
	assert.NotEmpty(t, WritesLineProblem(nil, "git add docs/wiki/x"))
}
