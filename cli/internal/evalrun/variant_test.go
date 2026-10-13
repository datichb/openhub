package evalrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

func TestParseVariants(t *testing.T) {
	vs, err := ParseVariants([]string{"base", "sans-23=-skill:debugger-phase-2-3", "old=../hub, -skill:quality/x ,-skill:y"})
	require.NoError(t, err)
	require.Len(t, vs, 3)
	assert.Equal(t, Variant{Name: "base"}, vs[0])
	assert.True(t, vs[0].Plain())
	assert.Equal(t, Variant{Name: "sans-23", RemoveSkills: []string{"debugger-phase-2-3"}}, vs[1])
	assert.False(t, vs[1].Plain())
	assert.Equal(t, Variant{Name: "old", Source: "../hub", RemoveSkills: []string{"quality/x", "y"}}, vs[2])
	assert.Equal(t, "hub ../hub sans quality/x, y", vs[2].Describe())

	for _, bad := range []string{"", "=x", "a b", "x=", "x=-skill:", "x=a,b", "x=+skill:y", "x=-agent:y", "x=a,,b", "x=-skill:../y"} {
		_, err := ParseVariant(bad)
		assert.Error(t, err, bad)
	}
	_, err = ParseVariants([]string{"a", "a=-skill:x"})
	assert.ErrorContains(t, err, "twice")
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o644))
	}
}

func TestRemoveSkill(t *testing.T) {
	hub := t.TempDir()
	writeFiles(t, hub, map[string]string{
		"skills/quality/dbg-2.md":   "---\nname: dbg-2\n---\n",
		"skills/quality/dbg-4.md":   "---\nname: dbg-4\n---\n",
		"skills/other/dup.md":       "---\nname: dup\n---\n",
		"skills/quality/dup.md":     "---\nname: dup\n---\n",
		"skills/templates/dbg-2.md": "annexe, pas un skill",
		"agents/q/flow.md":          "---\nid: flow\nskills: [shared/a, quality/dbg-2]\nnative_skills: [\"quality/dbg-4\", dbg-2, shared/b]\n---\n\nCharge `dbg-2` en phase 2.\nskills: [quality/dbg-2]\n",
		"agents/q/block.md":         "---\nid: block\nnative_skills:\n  - shared/a\n  - quality/dbg-2\n  - shared/b\nmode: primary\n---\nbody\n",
		"agents/q/none.md":          "---\nid: none\nskills: [shared/a]\n---\n",
	})
	changed, err := RemoveSkill(hub, "dbg-2")
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"skills/quality/dbg-2.md", "agents/q/flow.md (skills, native_skills)", "agents/q/block.md (native_skills)"}, changed)
	assert.NoFileExists(t, filepath.Join(hub, "skills/quality/dbg-2.md"))
	assert.FileExists(t, filepath.Join(hub, "skills/templates/dbg-2.md"), "annexes are not skills")

	flow, err := os.ReadFile(filepath.Join(hub, "agents/q/flow.md"))
	require.NoError(t, err)
	assert.Equal(t, "---\nid: flow\nskills: [shared/a]\nnative_skills: [\"quality/dbg-4\", shared/b]\n---\n\nCharge `dbg-2` en phase 2.\nskills: [quality/dbg-2]\n", string(flow),
		"the frontmatter only: the body is left as written")
	block, err := os.ReadFile(filepath.Join(hub, "agents/q/block.md"))
	require.NoError(t, err)
	assert.Equal(t, "---\nid: block\nnative_skills:\n  - shared/a\n  - shared/b\nmode: primary\n---\nbody\n", string(block))
	none, err := os.ReadFile(filepath.Join(hub, "agents/q/none.md"))
	require.NoError(t, err)
	assert.Equal(t, "---\nid: none\nskills: [shared/a]\n---\n", string(none))

	_, err = RemoveSkill(hub, "dup")
	assert.ErrorContains(t, err, "ambiguous")
	changed, err = RemoveSkill(hub, "quality/dup")
	require.NoError(t, err)
	assert.Equal(t, []string{"skills/quality/dup.md"}, changed)
	_, err = RemoveSkill(hub, "nope")
	assert.ErrorContains(t, err, "not in the hub")
}

// repoRoot is the repository: its agents/, skills/, permissions/ and
// workflows/ are the hub.
func repoRoot() string { return filepath.Join("..", "..", "..") }

// A variant of the shipped hub without a phase skill still composes; one
// without a skill a workflow or another skill needs does not, and says why.
func TestPrepareVariantOnTheShippedHub(t *testing.T) {
	work := t.TempDir()
	v := Variant{Name: "sans-23", RemoveSkills: []string{"debugger-phase-2-3"}}
	require.NoError(t, PrepareVariant(&v, repoRoot(), work))
	assert.Equal(t, filepath.Join(work, "variants", "sans-23"), v.Hub)
	assert.Equal(t, []string{"skills/quality/debugger-phase-2-3.md", "agents/quality/debugger.md (native_skills)"}, v.Removed)
	agent, err := os.ReadFile(filepath.Join(v.Hub, "agents", "quality", "debugger.md"))
	require.NoError(t, err)
	front, _, _ := strings.Cut(strings.TrimPrefix(string(agent), "---\n"), "\n---")
	assert.NotContains(t, front, "debugger-phase-2-3")
	assert.Contains(t, front, "quality/debugger-phase-4-5")
	require.NoError(t, CheckHub(v.Hub, []string{"debug", "debug"}))
	_, err = os.Stat(filepath.Join(repoRoot(), "skills", "quality", "debugger-phase-2-3.md"))
	require.NoError(t, err, "the source is not changed")

	base := Variant{Name: "base"}
	require.NoError(t, PrepareVariant(&base, repoRoot(), work))
	require.NoError(t, CheckHub(base.Hub, []string{"audit", "review-feedback"}))

	broken := Variant{Name: "sans-feedback", RemoveSkills: []string{"orchestrator-dev-feedback-mode"}}
	require.NoError(t, PrepareVariant(&broken, repoRoot(), work))
	err = CheckHub(broken.Hub, []string{"review-feedback"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "review-feedback")
	assert.Contains(t, err.Error(), "orchestrator-dev-feedback-mode")

	required := Variant{Name: "sans-elicitation", RemoveSkills: []string{"elicitation-techniques"}}
	require.NoError(t, PrepareVariant(&required, repoRoot(), work))
	err = CheckHub(required.Hub, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing_require")

	other := Variant{Name: "x", Source: t.TempDir()}
	assert.ErrorContains(t, PrepareVariant(&other, "", work), "not a hub")
	unknown := Variant{Name: "y", RemoveSkills: []string{"nope"}}
	assert.ErrorContains(t, PrepareVariant(&unknown, repoRoot(), work), "not in the hub")
}

func stat(v string, passed, runs int, cost float64) CaseStats {
	return CaseStats{Case: "S6", Model: "m", Variant: v, Passed: passed, Runs: runs, PassRate: rate(passed, runs), MeanCost: cost, MeanSecs: 10}
}

func TestCompareStats(t *testing.T) {
	for _, tc := range []struct {
		name       string
		ref, v     CaseStats
		want, word string
	}{
		{"+2/3", stat("base", 3, 3, 0.1), stat("sans", 1, 3, 0.1), VerdictGain, "+67 %"},
		{"+3/3", stat("base", 3, 3, 0.1), stat("sans", 0, 3, 0.1), VerdictGain, "+100 %"},
		{"+1/3 only", stat("base", 2, 3, 0.1), stat("sans", 1, 3, 0.1), VerdictAmbiguous, "relancer"},
		{"+2/6 after ×3 more", stat("base", 5, 6, 0.1), stat("sans", 3, 6, 0.1), VerdictGain, "+33 %"},
		{"+1/6", stat("base", 4, 6, 0.1), stat("sans", 3, 6, 0.1), VerdictNoGain, "moins que"},
		{"one run each", stat("base", 1, 1, 0.1), stat("sans", 0, 1, 0.1), VerdictAmbiguous, "un seul passage"},
		{"drop", stat("base", 1, 3, 0.1), stat("sans", 3, 3, 0.1), VerdictNoGain, "recul"},
		{"same rate, 10 % cheaper", stat("base", 3, 3, 0.09), stat("sans", 3, 3, 0.1), VerdictGain, "-10 %"},
		{"same rate, 5 % cheaper", stat("base", 3, 3, 0.095), stat("sans", 3, 3, 0.1), VerdictNoGain, "-5 %"},
		{"same rate, dearer", stat("base", 2, 3, 0.2), stat("sans", 2, 3, 0.1), VerdictNoGain, "+100 %"},
		{"no pass", stat("base", 0, 0, 0), stat("sans", 1, 3, 0.1), VerdictIncomplete, "pas de passage"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := CompareStats(tc.ref, tc.v)
			assert.Equal(t, tc.want, c.Verdict, c.Reason)
			assert.Contains(t, c.Reason, tc.word)
		})
	}
}

func TestVerdictOf(t *testing.T) {
	cmp := func(id string, ref, v CaseStats) Comparison {
		ref.Case, v.Case = id, id
		return CompareStats(ref, v)
	}
	gain := cmp("S6", stat("base", 3, 3, 0.1), stat("sans", 0, 3, 0.1))
	same := cmp("S1", stat("base", 3, 3, 0.1), stat("sans", 3, 3, 0.1))
	small := cmp("S2", stat("base", 2, 3, 0.1), stat("sans", 3, 3, 0.1))
	big := cmp("S3", stat("base", 1, 3, 0.1), stat("sans", 3, 3, 0.1))
	amb := cmp("S4", stat("base", 2, 3, 0.1), stat("sans", 1, 3, 0.1))
	for _, tc := range []struct {
		name string
		cmps []Comparison
		want string
		word string
	}{
		{"gain, no drop", []Comparison{gain, same}, VerdictGain, "gain sur S6"},
		{"gain and a drop of one pass", []Comparison{gain, small}, VerdictAmbiguous, "recul d'un passage sur S2"},
		{"gain and a real drop", []Comparison{gain, big}, VerdictNoGain, "recul sur S3"},
		{"one pass only", []Comparison{amb, same}, VerdictAmbiguous, "S4"},
		{"nothing", []Comparison{same}, VerdictNoGain, "aucun gain"},
		{"none", nil, VerdictIncomplete, "aucun"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := VerdictOf(tc.cmps)
			assert.Equal(t, tc.want, v.Verdict, v.Reason)
			assert.Contains(t, v.Reason, tc.word)
		})
	}
}

// A run with variants: every case runs with every variant (its hub given
// to the executor), the report compares them, rescoring keeps them.
func TestRunnerWithVariants(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	writeCase(t, root, "debug", "s6", "id: S6\nworkflow: debug\nassertions:\n  - type: skill_loaded_before\n    skill: dbg-2\n  - type: text_matches\n    name: diagnostic\n    pattern: Sub\n")
	cases, err := LoadCases(root)
	require.NoError(t, err)
	variants := []Variant{{Name: "base", Hub: "/hubs/base"}, {Name: "sans", RemoveSkills: []string{"dbg-2"}, Hub: "/hubs/sans"}}
	exec := ExecutorFunc(func(ctx context.Context, p Pass) (*PassOutput, error) {
		assert.Equal(t, "/hubs/"+p.Variant, p.HubDir)
		evs := []sessionsvc.TraceEvent{ev(sessionsvc.TraceSession, nil),
			ev(sessionsvc.TraceText, func(e *sessionsvc.TraceEvent) { e.Agent, e.Text = "debugger", "Sub additionne" })}
		if p.Variant == "base" {
			evs = append(evs, ev(sessionsvc.TraceSkill, func(e *sessionsvc.TraceEvent) { e.Agent, e.Skill, e.Tool = "debugger", "quality/dbg-2", "skill" }))
		}
		path := filepath.Join(p.OutDir, "trace.jsonl")
		f, err := os.Create(path)
		require.NoError(t, err)
		require.NoError(t, (&sessionsvc.Trace{Events: evs}).WriteJSONL(f))
		f.Close()
		cost := 0.1
		if p.Variant == "sans" {
			cost = 0.12
		}
		return &PassOutput{Outcome: OutcomeCompleted, CostUSD: cost, TracePath: path}, nil
	})
	out := t.TempDir()
	r := &Runner{Exec: exec, Models: []string{"m"}, Reps: 3, BudgetUSD: 1, WorkRoot: t.TempDir(), OutDir: out, Variants: variants, Now: fixedNow}
	rep, err := r.Run(context.Background(), cases)
	require.NoError(t, err)
	require.Len(t, rep.Passes, 6)
	assert.Equal(t, []string{"base", "sans", "base", "sans", "base", "sans"}, func() []string {
		var vs []string
		for _, p := range rep.Passes {
			vs = append(vs, p.Variant)
		}
		return vs
	}(), "the variants of a repetition run side by side")
	assert.Equal(t, filepath.Join("passes", "S6", "sans", "m", "1", "trace.jsonl"), rep.Passes[1].Trace)
	s, ok := rep.VariantStat("S6", "sans", "m")
	require.True(t, ok)
	assert.Zero(t, s.Passed)
	_, ok = rep.Stat("S6", "m")
	assert.False(t, ok, "no stats without variant")

	require.Len(t, rep.Comparisons, 1)
	c := rep.Comparisons[0]
	assert.Equal(t, VerdictGain, c.Verdict)
	assert.Equal(t, 3, c.RefPassed)
	assert.InDelta(t, 0.1/0.12-1, c.CostGap, 1e-9)
	require.Len(t, c.Assertions, 2)
	assert.Equal(t, AssertionGap{Name: "diagnostic", RefPassed: 3, RefRuns: 3, Passed: 3, Runs: 3}, c.Assertions[1], "same business result")
	require.NotNil(t, c.Outcome, "the case has a loading assertion")
	assert.Equal(t, OutcomeGap{RefPassed: 3, Passed: 3, Verdict: VerdictGain, Reason: "réussite égale, coût moyen -17 %"}, *c.Outcome,
		"same business result, the reference is cheaper")
	require.Len(t, rep.Verdicts, 1)
	assert.Equal(t, VariantVerdict{Model: "m", Reference: "base", Variant: "sans", Verdict: VerdictGain, Reason: "gain sur S6",
		OutcomeVerdict: VerdictGain, OutcomeReason: "gain sur S6"}, rep.Verdicts[0])

	md := rep.Markdown(&Report{Stats: []CaseStats{{Case: "S6", Model: "m", Passed: 2, Runs: 3, PassRate: 2.0 / 3}}})
	for _, want := range []string{
		"| S6 | debug | base | m | 3/3 (100 %) | 2/3 ▲ |",
		"| S6 | debug | sans | m | 0/3 (0 %) | — |",
		"## Comparaison avec la référence",
		"- **sans** : hub embarqué sans dbg-2",
		"| S6 | m | sans | 0/3 | 3/3 | +100 % | $0.120 | $0.100 | -17 % | 0 s | 0 s | **gain** : réussite +100 % |",
		"- base contre sans · m : **gain** (gain sur S6) ; hors chargement des skills : **gain** (gain sur S6)",
		"| S6 | m | sans | 3/3 | 3/3 | +0 % | **gain** : réussite égale, coût moyen -17 % |",
		"| skill dbg-2 ◆ | 3/3 | 0/3 |",
		"| diagnostic | 3/3 | 3/3 |",
		"### S6 · sans · m",
		"- S6 · sans · m · #1 :",
	} {
		assert.Contains(t, md, want)
	}

	path := filepath.Join(out, "report.json")
	require.NoError(t, rep.WriteJSON(path))
	back, err := ReadReport(path)
	require.NoError(t, err)
	assert.Equal(t, rep.Comparisons, back.Comparisons)
	assert.Empty(t, back.Variants[0].Hub, "the work directory is not recorded")

	// Rescoring without the loading assertion: same pass rate, the reference
	// is cheaper by 17 %: still a gain, on the cost.
	cases[0].Assertions = cases[0].Assertions[1:]
	again, err := Rescore(back, out, cases)
	require.NoError(t, err)
	require.Len(t, again.Comparisons, 1)
	assert.Equal(t, VerdictGain, again.Comparisons[0].Verdict)
	assert.Contains(t, again.Comparisons[0].Reason, "coût moyen -17 %")
	assert.Equal(t, "sans", again.Passes[1].Variant)
	assert.Nil(t, again.Comparisons[0].Outcome, "no loading assertion left")
	assert.Empty(t, again.Verdicts[0].OutcomeVerdict)
}

// Without variants the report is the same as before: no variant column, no
// comparison (the format of baseline.json).
func TestReportWithoutVariants(t *testing.T) {
	rep := &Report{Models: []string{"m"}, Passes: []PassResult{{Case: "S1", Model: "m", Rep: 1, Outcome: OutcomeCompleted, Pass: true}}}
	rep.compute()
	assert.Empty(t, rep.Comparisons)
	md := rep.Markdown(nil)
	assert.Contains(t, md, "| Cas | Workflow | Modèle | Réussite | Issues |")
	assert.NotContains(t, md, "Variante")
	assert.NotContains(t, md, "Comparaison")
}
