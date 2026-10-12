package bundle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/sessionspec"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// verifyProject is a Go module with a TypeScript front tested by Vitest.
func verifyProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":       "module example.com/x\n",
		"package.json": `{"scripts":{"test":"vitest run"},"devDependencies":{"typescript":"6","vitest":"4"}}`,
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	}
	return dir
}

// shellOf returns the rules of an agent as the tool applies them: its own,
// then the global guard and the session locks.
func shellOf(t *testing.T, b *Bundle, id string, locks []sessionspec.PermissionRule) []sessionspec.PermissionRule {
	t.Helper()
	a := findAgent(b.Spec.Agents, id)
	require.NotNil(t, a, id)
	rules := append(append([]sessionspec.PermissionRule(nil), a.Permissions...), b.Spec.AgentGuard...)
	return append(rules, locks...)
}

// C1.6: the reviewer (verify: true) runs the build and tests of the project,
// asked in the review of a merge request (risk: read); the other agents get
// nothing, the guards and the locks still win.
func TestReviewerVerifyCommands(t *testing.T) {
	hub, cat, env := shippedWorkflows(t)
	project := verifyProject(t)
	build := func(id string) (*Bundle, *workflow.Spec) {
		r, diags := workflow.Check(cat, workflow.Ref{Layer: workflow.LayerHub, ID: id}, nil, env)
		require.False(t, diags.HasErrors(), "%v", diags)
		b, err := Build(Request{HubDir: hub, OutDir: t.TempDir(), ProjectPath: project, Spec: r.Spec})
		require.NoError(t, err)
		return b, r.Spec
	}

	ticket, spec := build("ticket")
	locks := SessionRules(ticket.Spec.Workflow, spec.DefaultMode(), nil)
	rev := shellOf(t, ticket, "reviewer", locks)
	for _, cmd := range []string{"go test ./...", "go build ./...", "go vet ./...", "npm test", "npm run build", "npm run typecheck", "npx --no tsc --noEmit"} {
		assert.Equal(t, sessionspec.EffectAllow, ShellEffect(rev, cmd), cmd)
	}
	for _, cmd := range []string{
		"npm install", "npm ci", "npm i -D x", "npx tsc --noEmit", "go install ./...", "go get x", "npm publish",
		"pip install pytest", "cargo test", "pytest", "npm run lint -- --fix", "npm test -- -u", "go test ./... -update",
		"npm test --global", "git commit -m x", "rm -rf dist",
	} {
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rev, cmd), cmd)
	}
	for _, a := range ticket.Spec.Agents {
		if a.ID == "reviewer" {
			continue
		}
		for _, r := range a.Permissions {
			assert.NotContains(t, []string{"go vet*", "npx --no tsc --noEmit*", "*--fix*"}, r.Resource, "%s: verification rules are the reviewer's", a.ID)
		}
	}

	review, spec := build("review")
	rev = shellOf(t, review, "reviewer", SessionRules(review.Spec.Workflow, spec.DefaultMode(), nil))
	for _, cmd := range []string{"go test ./...", "npm test", "npm run build"} {
		assert.Equal(t, sessionspec.EffectAsk, ShellEffect(rev, cmd), cmd)
	}
	for _, cmd := range []string{"npm install", "npm run lint -- --fix", "git commit -m x"} {
		assert.Equal(t, sessionspec.EffectDeny, ShellEffect(rev, cmd), cmd)
	}
}

// Without project (or a project without known stack) the reviewer keeps its
// read-only shell.
func TestReviewerVerifyWithoutProject(t *testing.T) {
	assert.Nil(t, VerifyRules(nil, workflow.RiskWrite))
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: agentSpec(t, repoHub(t), "reviewer")})
	require.NoError(t, err)
	assert.Equal(t, sessionspec.EffectDeny, ShellEffect(shellOf(t, b, "reviewer", nil), "go test ./..."))
}
