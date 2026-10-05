package bundle

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/deploy"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// repoHub returns the repository root, which has the hub layout (agents/, skills/, permissions/).
func repoHub(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..")
	if _, err := os.Stat(filepath.Join(root, "agents")); err != nil {
		t.Skip("hub content not found")
	}
	return root
}

func TestBuildOrchestratorDevBundle(t *testing.T) {
	out := t.TempDir()
	project := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(project, "CONVENTIONS.md"), []byte("Always write tests.\n"), 0o644))

	b, err := Build(Request{HubDir: repoHub(t), OutDir: out, ProjectPath: project, EntryAgent: "orchestrator-dev", Provider: "bedrock"})
	require.NoError(t, err)
	s := b.Spec

	assert.Equal(t, "orchestrator-dev", s.EntryAgent)
	assert.Equal(t, filepath.Join(out, s.Hash), b.Dir)
	ids := s.AgentIDs()
	assert.Equal(t, "orchestrator-dev", ids[0])
	assert.Contains(t, ids, "developer")
	assert.Contains(t, ids, "reviewer")
	assert.NotContains(t, ids, "orchestrator", "agents not reachable from the entry are excluded")
	assert.NotContains(t, ids, "planner")

	assert.Contains(t, s.SubagentGraph["orchestrator-dev"], "developer")
	assert.GreaterOrEqual(t, s.MaxDepth, 1)
	require.NotEmpty(t, s.Skills)
	for _, sk := range s.Skills {
		_, err := os.Stat(filepath.Join(sk.Dir, "SKILL.md"))
		assert.NoError(t, err, sk.ID)
		assert.True(t, strings.HasPrefix(sk.Dir, b.Dir))
	}

	dev := findAgent(s.Agents, "developer")
	require.NotNil(t, dev)
	assert.Equal(t, "subagent", dev.Mode)
	assert.NotEmpty(t, dev.Description)
	assert.NotContains(t, dev.Body, "native_skills:", "frontmatter stripped")
	assert.Contains(t, dev.Body, "# Project instructions")
	assert.Contains(t, dev.Body, "Always write tests.")
	assert.Contains(t, dev.Body, "\n---\n", "Bucket A skills inlined")

	hasShell := false
	for _, r := range dev.Permissions {
		if r.Action == sessionspec.ActionShell {
			hasShell = true
		}
		assert.NotEqual(t, "bash", r.Action)
	}
	assert.True(t, hasShell)

	data, err := os.ReadFile(filepath.Join(b.Dir, "agents", "developer.md"))
	require.NoError(t, err)
	assert.Equal(t, dev.Body, string(data))

	// Idempotent: same inputs → same hash and directory.
	again, err := Build(Request{HubDir: repoHub(t), OutDir: out, ProjectPath: project, EntryAgent: "orchestrator-dev", Provider: "bedrock"})
	require.NoError(t, err)
	assert.Equal(t, s.Hash, again.Spec.Hash)
	entries, _ := os.ReadDir(out)
	assert.Len(t, entries, 1, "no leftover temp dirs")

	loaded, err := Load(out, s.Hash)
	require.NoError(t, err)
	assert.Equal(t, s.AgentIDs(), loaded.Spec.AgentIDs())

	// Changing the project instructions changes the bundle.
	require.NoError(t, os.WriteFile(filepath.Join(project, "CONVENTIONS.md"), []byte("Other rule.\n"), 0o644))
	changed, err := Build(Request{HubDir: repoHub(t), OutDir: out, ProjectPath: project, EntryAgent: "orchestrator-dev", Provider: "bedrock"})
	require.NoError(t, err)
	assert.NotEqual(t, s.Hash, changed.Spec.Hash)
}

func TestBuildOrchestratorDepth(t *testing.T) {
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), EntryAgent: "orchestrator", Provider: "bedrock"})
	require.NoError(t, err)
	assert.Contains(t, b.Spec.AgentIDs(), "orchestrator-dev")
	assert.Contains(t, b.Spec.AgentIDs(), "developer")
	assert.GreaterOrEqual(t, b.Spec.MaxDepth, 2, "orchestrator → orchestrator-dev → developer")
}

func TestBuildErrors(t *testing.T) {
	_, err := Build(Request{})
	assert.Error(t, err)
	_, err = Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), EntryAgent: "nope"})
	assert.ErrorContains(t, err, "unknown entry agent")
}

func TestGraphHelpers(t *testing.T) {
	g := map[string][]string{"a": {"b", "c"}, "b": {"d"}, "d": {"a"}, "x": {"y"}}
	assert.Equal(t, []string{"a", "b", "c", "d"}, reachable("a", g))
	assert.Equal(t, 2, maxDepth("a", g))
	assert.Equal(t, 1, maxDepth("c", g))
}

func TestConvertPermissions(t *testing.T) {
	rules := ConvertPermissions(map[string]interface{}{
		"bash": map[string]interface{}{"git push*": "deny", "*": "deny", "git *": "allow", "bd *": true},
		"edit": "allow", "write": false, "skill": "allow", "question": "ask", "weird": 42,
	})
	got := []string{}
	for _, r := range rules {
		got = append(got, r.Action+" "+r.Resource+" "+string(r.Effect))
	}
	assert.Equal(t, []string{
		"shell * deny", "shell bd * allow", "shell git * allow", "shell git push* deny",
		"edit * deny", "question * ask", "skill * allow",
	}, got, "write: false is not widened by edit: allow")
}

func TestBuildResolvesModels(t *testing.T) {
	b, err := Build(Request{
		HubDir: repoHub(t), OutDir: t.TempDir(), EntryAgent: "orchestrator-dev", Provider: "bedrock",
		HubOverrides: &deploy.ModelOverrides{Default: "claude-sonnet-4-5", Agents: map[string]string{"reviewer": "claude-opus-4-6"}},
	})
	require.NoError(t, err)
	dev := findAgent(b.Spec.Agents, "developer")
	require.NotNil(t, dev.Model)
	assert.Equal(t, "amazon-bedrock", dev.Model.Provider)
	assert.Contains(t, dev.Model.Model, "claude-sonnet-4-5")
	rev := findAgent(b.Spec.Agents, "reviewer")
	require.NotNil(t, rev.Model)
	assert.Contains(t, rev.Model.Model, "claude-opus-4-6")
	require.NotNil(t, b.Spec.DefaultModel, "entry agent model becomes the default model")
}

// E14-M3: write/patch never widen edit (benchmarker: edit deny + write allow).
func TestConvertPermissionsMergedKeysAreRestrictive(t *testing.T) {
	rules := ConvertPermissions(map[string]interface{}{"edit": "deny", "write": "allow"})
	require.Len(t, rules, 1)
	assert.Equal(t, sessionspec.PermissionRule{Action: sessionspec.ActionEdit, Resource: "*", Effect: sessionspec.EffectDeny}, rules[0])

	rules = ConvertPermissions(map[string]interface{}{
		"edit":  map[string]interface{}{"*": "allow", "secrets/**": "deny"},
		"patch": "ask",
	})
	assert.Equal(t, []sessionspec.PermissionRule{
		{Action: sessionspec.ActionEdit, Resource: "*", Effect: sessionspec.EffectAsk},
		{Action: sessionspec.ActionEdit, Resource: "secrets/**", Effect: sessionspec.EffectDeny},
	}, rules)
}

// An entry agent without a model still gets a session model (never the tool's own default).
func TestBuildEntryWithoutModelGetsFallback(t *testing.T) {
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), EntryAgent: "developer", Provider: "bedrock"})
	require.NoError(t, err)
	require.NotNil(t, b.Spec.DefaultModel)
	assert.Equal(t, "amazon-bedrock", b.Spec.DefaultModel.Provider)
	assert.Contains(t, b.Spec.DefaultModel.Model, "claude-sonnet-4-6")
}
