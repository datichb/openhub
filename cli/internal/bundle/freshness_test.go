package bundle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	reactRef = "developer/stacks/dev-standards-react"
	goRef    = "developer/stacks/dev-standards-golang"
	jestRef  = "developer/stacks/dev-standards-jest"
	rustRef  = "developer/stacks/dev-standards-rust"
)

func freshnessMD(name, extra string) string {
	return skillMD(name, extra, "# "+name)
}

func TestCheckSkillsFreshnessFields(t *testing.T) {
	hub := fakeHub(t, map[string]string{
		"skills/" + reactRef + ".md": freshnessMD("dev-standards-react", "versions:\n  react: \"19.3\"\nreviewed: 2026-10-13\n"),
		"skills/" + goRef + ".md":    freshnessMD("dev-standards-golang", "reviewed: 2026-10-13\n"),
		"skills/" + jestRef + ".md":  freshnessMD("dev-standards-jest", ""),
		"skills/" + rustRef + ".md":  freshnessMD("dev-standards-rust", "versions: {Rust: \"1.99\", cargo: \"\"}\nreviewed: 13/10/2026\n"),
		"skills/a/future.md":         freshnessMD("future", "versions: {x: \"1\"}\nreviewed: 2999-01-01\n"),
		"skills/a/plain.md":          freshnessMD("plain", ""),
	})
	problems, err := CheckSkills(hub)
	require.NoError(t, err)
	got := map[string]bool{}
	for _, p := range problems {
		got[string(p.Kind)+" "+p.Ref+" "+p.Detail] = p.Error
	}
	assert.Equal(t, map[string]bool{
		"missing_freshness " + goRef + " versions":                                          true,
		"missing_freshness " + jestRef + " versions, reviewed":                              true,
		"invalid_freshness " + rustRef + ` versions: key "Rust" (lowercase technology)`:     true,
		"invalid_freshness " + rustRef + ` reviewed: "13/10/2026" is not a YYYY-MM-DD date`: true,
		"invalid_freshness a/future reviewed: 2999-01-01 is in the future":                  true,
	}, got)
}

func TestFreshnessFieldsNotDelivered(t *testing.T) {
	d, err := parseSkill(reactRef, "", []byte(freshnessMD("dev-standards-react", "versions:\n  react: \"19.3\"\n  react-compiler: \"1.0\"\nreviewed: 2026-10-13\n")))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"react": "19.3", "react-compiler": "1.0"}, d.Front.Versions)
	assert.Equal(t, "2026-10-13", d.Front.Reviewed)
	out := string(d.delivered())
	assert.NotContains(t, out, "versions")
	assert.NotContains(t, out, "reviewed")
	assert.NotContains(t, out, "19.3")
	assert.NotContains(t, string(d.body()), "reviewed")
}

func TestCheckFreshness(t *testing.T) {
	hub := fakeHub(t, map[string]string{
		"skills/" + reactRef + ".md": freshnessMD("dev-standards-react", "versions: {react: \"19.3\"}\nreviewed: 2026-10-13\n"),
		"skills/" + goRef + ".md":    freshnessMD("dev-standards-golang", "versions: {go: \"1.26\"}\nreviewed: 2026-05-01\n"),
		"skills/" + rustRef + ".md":  freshnessMD("dev-standards-rust", "versions: {rust: \"1.95\"}\nreviewed: 2026-01-02\n"),
		"skills/" + jestRef + ".md":  freshnessMD("dev-standards-jest", "versions: {jest: \"30\"}\n"),
		"skills/a/tool.md":           freshnessMD("tool", "versions: {tool: \"2\"}\nreviewed: 2026-02-01\n"),
		"skills/a/plain.md":          freshnessMD("plain", ""),
		"skills/templates/x.md":      "---\nreviewed: 2000-01-01\n---\n",
	})
	today := time.Date(2026, 10, 13, 22, 30, 0, 0, time.UTC)

	rep, err := CheckFreshness(hub, today, DefaultMaxAgeDays)
	require.NoError(t, err)
	assert.Equal(t, "2026-10-13", rep.Today)
	assert.Equal(t, 90, rep.MaxAgeDays)
	assert.Equal(t, 5, rep.Checked, "stack skills and skills with metadata, templates aside")
	var refs []string
	for _, s := range rep.Skills {
		refs = append(refs, string(s.Status)+" "+s.Ref)
	}
	assert.Equal(t, []string{
		"missing " + jestRef,
		"stale " + rustRef,
		"stale a/tool",
		"stale " + goRef,
	}, refs, "missing first, then the oldest review first")
	assert.Equal(t, "reviewed", rep.Skills[0].Detail)
	assert.Equal(t, 284, rep.Skills[1].AgeDays)
	assert.Equal(t, map[string]string{"rust": "1.95"}, rep.Skills[1].Versions)
	assert.Equal(t, 165, rep.Skills[3].AgeDays)

	rep, err = CheckFreshness(hub, today, 180)
	require.NoError(t, err)
	refs = nil
	for _, s := range rep.Skills {
		refs = append(refs, s.Ref)
	}
	assert.Equal(t, []string{jestRef, rustRef, "a/tool"}, refs)
}

func TestCheckFreshnessBoundary(t *testing.T) {
	hub := fakeHub(t, map[string]string{
		"skills/" + reactRef + ".md": freshnessMD("dev-standards-react", "versions: {react: \"19\"}\nreviewed: 2026-07-15\n"),
	})
	rep, err := CheckFreshness(hub, time.Date(2026, 10, 13, 0, 0, 0, 0, time.UTC), 90)
	require.NoError(t, err)
	assert.Empty(t, rep.Skills, "90 days old is not over 90 days")
	rep, err = CheckFreshness(hub, time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC), 90)
	require.NoError(t, err)
	require.Len(t, rep.Skills, 1)
	assert.Equal(t, 91, rep.Skills[0].AgeDays)
}

func TestCheckFreshnessBrokenStackSkill(t *testing.T) {
	hub := fakeHub(t, map[string]string{
		"skills/" + reactRef + ".md": "---\nname: [\n---\nx\n",
		"skills/a/broken.md":         "---\nname: [\n---\nx\n",
	})
	rep, err := CheckFreshness(hub, time.Now(), 90)
	require.NoError(t, err)
	require.Len(t, rep.Skills, 1)
	assert.Equal(t, FreshnessMissing, rep.Skills[0].Status)
	assert.Equal(t, reactRef, rep.Skills[0].Ref)
}

func TestParseMaxAge(t *testing.T) {
	for in, want := range map[string]int{"90": 90, "90d": 90, "180D": 180, "12w": 84, " 30d ": 30} {
		got, err := ParseMaxAge(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"", "0", "3m", "-5d", "d", "1.5d"} {
		_, err := ParseMaxAge(in)
		assert.Error(t, err, in)
	}
}

// Every stack skill of the hub carries its freshness metadata, and the
// versions it declares are those of its « Versions de référence » line when
// it has one.
func TestShippedStackSkillsFreshness(t *testing.T) {
	hub := repoHub(t)
	t.Setenv("OH_HOME", t.TempDir())
	rep, err := CheckFreshness(hub, time.Now(), 1<<30)
	require.NoError(t, err)
	assert.Empty(t, rep.Skills, "every stack skill has usable versions: and reviewed:")
	entries, err := os.ReadDir(filepath.Join(hub, "skills", "developer", "stacks"))
	require.NoError(t, err)
	assert.Equal(t, len(entries), rep.Checked, "only the stack skills carry freshness metadata")
	for _, e := range entries {
		ref := "developer/stacks/" + strings.TrimSuffix(e.Name(), ".md")
		content, err := os.ReadFile(filepath.Join(hub, "skills", "developer", "stacks", e.Name()))
		require.NoError(t, err)
		d, err := parseSkill(ref, "", content)
		require.NoError(t, err)
		line := ""
		for _, l := range strings.Split(string(d.body()), "\n") {
			if strings.HasPrefix(l, "> Versions de référence") {
				line = l
				break
			}
		}
		if line == "" {
			continue
		}
		for tech, v := range d.Front.Versions {
			assert.Contains(t, line, strings.TrimSuffix(v, "+"), "%s: versions.%s is on its « Versions de référence » line", ref, tech)
		}
	}
}
