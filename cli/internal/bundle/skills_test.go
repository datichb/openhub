package bundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeHub writes a minimal hub: files maps a path relative to the hub root to
// its content.
func fakeHub(t *testing.T, files map[string]string) string {
	t.Helper()
	t.Setenv("OH_HOME", t.TempDir()) // isolate the user data (~/.oh)
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	return dir
}

func agentMD(id, skills, native string) string {
	return "---\nid: " + id + "\ndescription: test agent\nmode: primary\nskills: [" + skills + "]\nnative_skills: [" + native + "]\n---\n\n# " + id + " body\n"
}

func skillMD(name, extra, body string) string {
	return "---\nname: " + name + "\ndescription: " + name + " skill\n" + extra + "---\n\n" + body + "\n"
}

func buildSolo(t *testing.T, hub string, mutate ...func(*Request)) (*Bundle, error) {
	t.Helper()
	req := Request{HubDir: hub, OutDir: t.TempDir(), Spec: agentSpec(t, hub, "solo")}
	for _, m := range mutate {
		m(&req)
	}
	return Build(req)
}

func TestBuildSkillRequiresClosure(t *testing.T) {
	hub := fakeHub(t, map[string]string{
		"agents/test/solo.md": agentMD("solo", "a/inline", "b/native"),
		"skills/a/inline.md":  skillMD("inline", "requires: [a/base]\n", "INLINE BODY"),
		"skills/a/base.md":    skillMD("base", "", "BASE BODY"),
		"skills/b/native.md":  skillMD("native", "requires:\n  - b/dep\n", "NATIVE BODY"),
		"skills/b/dep.md":     skillMD("dep", "", "DEP BODY"),
		"skills/b/extra.md":   skillMD("extra", "", "EXTRA BODY"),
	})
	b, err := buildSolo(t, hub, func(r *Request) { r.ExtraSkills = []string{"b/extra"} })
	require.NoError(t, err)

	solo := findAgent(b.Spec.Agents, "solo")
	require.NotNil(t, solo)
	base, inline := strings.Index(solo.Body, "BASE BODY"), strings.Index(solo.Body, "INLINE BODY")
	require.True(t, base > 0 && inline > 0, "inline requirement is inlined too")
	assert.Less(t, base, inline, "requirement before the skill that requires it")
	assert.NotContains(t, solo.Body, "requires:")

	assert.Equal(t, []string{"dep", "extra", "native"}, b.Spec.SkillIDs())
	data, err := os.ReadFile(filepath.Join(b.Dir, "skills", "native", "SKILL.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "requires", "hub-only keys are stripped")
	assert.Contains(t, string(data), "name: native")
	assert.Contains(t, string(data), "NATIVE BODY")
}

func TestBuildSkillErrors(t *testing.T) {
	cases := map[string]struct {
		files  map[string]string
		mutate func(*Request)
		want   string
	}{
		"missing requirement": {
			files: map[string]string{"skills/b/native.md": skillMD("native", "requires: [b/ghost]\n", "x")},
			want:  "requires b/ghost, which was not found",
		},
		"cycle": {
			files: map[string]string{
				"skills/b/native.md": skillMD("native", "requires: [b/other]\n", "x"),
				"skills/b/other.md":  skillMD("other", "requires: [b/native]\n", "x"),
			},
			want: "cycle: b/native → b/other → b/native",
		},
		"duplicate id": {
			files: map[string]string{
				"agents/test/solo.md": agentMD("solo", "", "b/native, c/native"),
				"skills/b/native.md":  skillMD("native", "", "x"),
				"skills/c/native.md":  skillMD("native", "", "y"),
			},
			want: `duplicate skill id "native"`,
		},
		"denied requirement": {
			files: map[string]string{
				"skills/b/native.md": skillMD("native", "requires: [b/dep]\n", "x"),
				"skills/b/dep.md":    skillMD("dep", "", "x"),
			},
			mutate: func(r *Request) { r.DenySkills = []string{"dep"} },
			want:   "denied by the workflow",
		},
		"name mismatch": {
			files: map[string]string{"skills/b/native.md": skillMD("other-name", "", "x")},
			want:  `frontmatter name "other-name" must equal its identifier "native"`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			files := map[string]string{"agents/test/solo.md": agentMD("solo", "", "b/native")}
			for k, v := range tc.files {
				files[k] = v
			}
			mutate := func(*Request) {}
			if tc.mutate != nil {
				mutate = tc.mutate
			}
			_, err := buildSolo(t, fakeHub(t, files), mutate)
			assert.ErrorContains(t, err, tc.want)
		})
	}
}

func TestBuildSkillTolerances(t *testing.T) {
	hub := fakeHub(t, map[string]string{
		"agents/test/solo.md": agentMD("solo", "a/ghost", "b/ghost, b/native, b/denied"),
		"skills/b/native.md":  skillMD("native", "", "x"),
		"skills/b/denied.md":  skillMD("denied", "", "x"),
	})
	b, err := buildSolo(t, hub, func(r *Request) { r.DenySkills = []string{"b/denied"} })
	require.NoError(t, err, "missing direct references are warnings")
	assert.Equal(t, []string{"native"}, b.Spec.SkillIDs(), "denied roots are removed")
}

func TestStripFrontmatterKeys(t *testing.T) {
	in := "---\nname: x\nrequires:\n  - a\n- b\nrequires2: keep\nrequires: [c]\ndescription: d\n---\nbody requires: stays\n"
	assert.Equal(t, "---\nname: x\nrequires2: keep\ndescription: d\n---\nbody requires: stays\n", string(stripFrontmatterKeys([]byte(in), "requires")))
	assert.Equal(t, "no frontmatter\n", string(stripFrontmatterKeys([]byte("no frontmatter\n"), "requires")))
}

func TestCheckSkills(t *testing.T) {
	hub := fakeHub(t, map[string]string{
		"agents/test/solo.md":       agentMD("solo", "a/ok", "b/ghost"),
		"skills/a/ok.md":            skillMD("ok", "", "x"),
		"skills/a/dup.md":           skillMD("dup", "", "x"),
		"skills/b/dup.md":           skillMD("dup", "", "x"),
		"skills/b/req.md":           skillMD("req", "requires: [b/missing]\n", "x"),
		"skills/c/one.md":           skillMD("one", "requires: [c/two]\n", "x"),
		"skills/c/two.md":           skillMD("two", "requires: [c/one]\n", "x"),
		"skills/c/wrong.md":         skillMD("right", "", "x"),
		"skills/c/legacy.md":        skillMD("legacy", "bucket: A\n", "x"),
		"skills/c/nodesc.md":        "---\nname: nodesc\n---\nx\n",
		"skills/c/broken.md":        "---\nname: [\n---\nx\n",
		"skills/templates/annex.md": "# not a skill\n",
	})
	problems, err := CheckSkills(hub)
	require.NoError(t, err)

	got := map[string]bool{}
	for _, p := range problems {
		got[string(p.Kind)+" "+p.Ref+" "+p.Detail] = p.Error
	}
	assert.Equal(t, map[string]bool{
		"duplicate_id a/dup b/dup":                  true,
		"missing_require b/req b/missing":           true,
		"require_cycle c/one c/one → c/two → c/one": true,
		"name_mismatch c/wrong right":               true,
		"legacy_bucket c/legacy A":                  false,
		"no_description c/nodesc ":                  false,
		"missing_agent_skill b/ghost solo":          false,
		"orphan_annex templates/annex.md ":          false,
	}, withoutKind(got, SkillInvalidFrontmatter))
	assert.True(t, hasKind(problems, SkillInvalidFrontmatter, "c/broken"))
}

func TestCheckSkillsHubIsClean(t *testing.T) {
	t.Setenv("OH_HOME", t.TempDir())
	problems, err := CheckSkills(repoHub(t))
	require.NoError(t, err)
	assert.Empty(t, problems)
}

func withoutKind(m map[string]bool, kind SkillProblemKind) map[string]bool {
	out := map[string]bool{}
	for k, v := range m {
		if !strings.HasPrefix(k, string(kind)+" ") {
			out[k] = v
		}
	}
	return out
}

func hasKind(ps []SkillProblem, kind SkillProblemKind, ref string) bool {
	for _, p := range ps {
		if p.Kind == kind && p.Ref == ref {
			return true
		}
	}
	return false
}
