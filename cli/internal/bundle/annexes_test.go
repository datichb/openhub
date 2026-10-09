package bundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

func TestBuildShipsAnnexes(t *testing.T) {
	hub := fakeHub(t, map[string]string{
		"agents/test/solo.md":        agentMD("solo", "a/inline", "b/native"),
		"skills/a/inline.md":         skillMD("inline", "annexes: [templates/block.md]\n", "Format in `templates/block.md` — load it with read."),
		"skills/b/native.md":         skillMD("native", "annexes: [templates/report.md]\n", "See templates/report.md."),
		"skills/templates/block.md":  "BLOCK TEMPLATE\n",
		"skills/templates/report.md": "REPORT TEMPLATE\n",
	})
	b, err := buildSolo(t, hub)
	require.NoError(t, err)

	solo := findAgent(b.Spec.Agents, "solo")
	want := sessionspec.BundleRootVar + "/skills/inline/templates/block.md"
	assert.Contains(t, solo.Body, "`"+want+"`", "inlined references point into the bundle")
	assert.NotContains(t, strings.ReplaceAll(solo.Body, want, ""), "templates/block.md")

	block := filepath.Join(b.Dir, "skills", "inline", "templates", "block.md")
	data, err := os.ReadFile(block)
	require.NoError(t, err)
	assert.Equal(t, "BLOCK TEMPLATE\n", string(data))
	st, err := os.Stat(block)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o444), st.Mode().Perm(), "bundle files are read-only")
	_, err = os.Stat(filepath.Join(b.Dir, "skills", "inline", "SKILL.md"))
	assert.True(t, os.IsNotExist(err), "an inlined skill is not delivered as an on-demand skill")
	assert.Equal(t, []string{"native"}, b.Spec.SkillIDs())

	native, err := os.ReadFile(filepath.Join(b.Dir, "skills", "native", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(native), "See templates/report.md.", "on-demand skills keep relative paths (base directory)")
	assert.NotContains(t, string(native), "annexes:")
	data, err = os.ReadFile(filepath.Join(b.Dir, "skills", "native", "templates", "report.md"))
	require.NoError(t, err)
	assert.Equal(t, "REPORT TEMPLATE\n", string(data))

	expanded := b.Spec.WithBundleRoot(b.Spec.Root)
	assert.Contains(t, findAgent(expanded.Agents, "solo").Body, block)
}

func TestBuildAnnexErrors(t *testing.T) {
	for name, annex := range map[string]string{"missing": "templates/ghost.md", "outside": "../secret.md"} {
		t.Run(name, func(t *testing.T) {
			hub := fakeHub(t, map[string]string{
				"agents/test/solo.md": agentMD("solo", "", "b/native"),
				"skills/b/native.md":  skillMD("native", "annexes: ["+annex+"]\n", "x"),
				"secret.md":           "secret",
			})
			_, err := buildSolo(t, hub)
			assert.ErrorContains(t, err, "annex")
		})
	}
}

// ADR-051: a package left in ~/.oh/skills by the former community registry
// is neither shipped nor checked.
func TestBuildIgnoresCommunitySkillPackage(t *testing.T) {
	hub := fakeHub(t, map[string]string{
		"agents/test/solo.md": agentMD("solo", "", "golang-idioms"),
	})
	pkg := filepath.Join(config.HubDir(), "skills", "golang-idioms")
	for rel, content := range map[string]string{
		"manifest.json": `{"name":"golang-idioms","version":"1.0.0"}`,
		"SKILL.md":      skillMD("golang-idioms", "", "COMMUNITY"),
	} {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(pkg, rel)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(pkg, rel), []byte(content), 0o644))
	}
	b, err := buildSolo(t, hub)
	require.NoError(t, err)
	assert.Empty(t, b.Spec.SkillIDs())
	_, err = os.Stat(filepath.Join(b.Dir, "skills", "golang-idioms"))
	assert.True(t, os.IsNotExist(err))

	problems, err := CheckSkills(hub)
	require.NoError(t, err)
	assert.Contains(t, problems, problem(SkillMissingAgentSkill, "golang-idioms", "solo"), "a missing skill for the catalogue")
}

func TestCheckSkillsAnnexes(t *testing.T) {
	hub := fakeHub(t, map[string]string{
		"skills/a/ok.md":             skillMD("ok", "annexes: [templates/used.md]\n", "templates/used.md"),
		"skills/a/missing.md":        skillMD("missing", "annexes: [templates/ghost.md]\n", "x"),
		"skills/a/undeclared.md":     skillMD("undeclared", "", "see templates/used.md and templates/used.md"),
		"skills/templates/used.md":   "x",
		"skills/templates/orphan.md": "x",
	})
	problems, err := CheckSkills(hub)
	require.NoError(t, err)
	got := map[string]bool{}
	for _, p := range problems {
		got[string(p.Kind)+" "+p.Ref+" "+p.Detail] = p.Error
	}
	assert.Equal(t, map[string]bool{
		"missing_annex a/missing templates/ghost.md":      true,
		"undeclared_annex a/undeclared templates/used.md": false,
		"orphan_annex templates/orphan.md ":               false,
	}, got)
}

func TestHubAnnexesResolve(t *testing.T) {
	b, err := Build(Request{HubDir: repoHub(t), OutDir: t.TempDir(), Spec: agentSpec(t, repoHub(t), "orchestrator"), Provider: "bedrock"})
	require.NoError(t, err)
	found := 0
	for _, a := range b.Spec.WithBundleRoot(b.Spec.Root).Agents {
		for _, line := range strings.Split(a.Body, "\n") {
			i := strings.Index(line, b.Spec.Root+"/skills/")
			if i < 0 {
				continue
			}
			p := line[i:]
			p = p[:strings.Index(p, ".md")+3]
			_, err := os.Stat(p)
			assert.NoError(t, err, "agent %s references %s", a.ID, p)
			found++
		}
	}
	assert.Greater(t, found, 0, "hub agents reference templates")
}
