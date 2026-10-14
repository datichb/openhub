package evalrun

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

// tierCases are four active cases and an inactive one.
func tierCases(t *testing.T) []*Case {
	t.Helper()
	root := t.TempDir()
	for _, id := range []string{"S1", "S2", "S3", "S4"} {
		writeCase(t, root, "quick", strings.ToLower(id), "id: "+id+"\nworkflow: quick\nassertions:\n  - type: ran\n    command: go test*\n")
	}
	writeCase(t, root, "ticket", "s8", "id: S8\nworkflow: ticket\nactive: false\ninactive_reason: plus tard\nassertions:\n  - type: no_denied_loop\n")
	cases, err := LoadCases(root)
	require.NoError(t, err)
	return cases
}

func day(d int) time.Time { return time.Date(2026, 10, d, 12, 0, 0, 0, time.UTC) }

// statsReport is a report of the given results (case → passed/runs).
func statsReport(at time.Time, model string, cases []*Case, res map[string][2]int) *Report {
	r := &Report{StartedAt: at, Models: []string{model}, Reps: 3}
	for _, c := range cases {
		r.Cases = append(r.Cases, c.Info())
		v, ok := res[c.ID]
		if !ok || !c.IsActive() {
			continue
		}
		s := CaseStats{Case: c.ID, Model: model, Runs: v[1], Passed: v[0], MeanCost: 0.1, Outcomes: map[string]int{}}
		for _, a := range c.Assertions {
			s.Assertions = append(s.Assertions, AssertionStat{Name: a.Label(), Type: a.Type, Runs: v[1]})
		}
		r.Stats = append(r.Stats, s)
	}
	return r
}

func planOf(plan []CasePlan) map[string]int {
	out := map[string]int{}
	for _, p := range plan {
		out[p.Case] = p.Reps
	}
	return out
}

func reasonOf(plan []CasePlan, id string) string {
	for _, p := range plan {
		if p.Case == id {
			return p.Reason
		}
	}
	return ""
}

// A case at 100 % on the last two measures of the baseline, taken on the
// case as it is, runs once; the others run the full repetitions.
func TestTieredPlan(t *testing.T) {
	cases := tierCases(t)
	const m = "claude-haiku-4-5"
	ref1 := statsReport(day(1), m, cases, map[string][2]int{"S1": {3, 3}, "S2": {2, 3}, "S4": {3, 3}})
	ref2 := statsReport(day(2), m, cases, map[string][2]int{"S1": {3, 3}, "S2": {3, 3}, "S3": {3, 3}, "S4": {1, 1}})
	base := ref2.Baseline("ref 2").WithHistory(ref1.Baseline("ref 1"))

	plan := TieredPlan(base, cases, []string{m}, 3, 0)
	assert.Equal(t, map[string]int{"S1": 1, "S2": 3, "S3": 3, "S4": 1}, planOf(plan), "the inactive case is left out")
	assert.Equal(t, "100 % sur les deux dernières mesures (3/3, 3/3)", reasonOf(plan, "S1"))
	assert.Equal(t, "dernières mesures 3/3, 2/3", reasonOf(plan, "S2"))
	assert.Equal(t, "une seule mesure (3/3)", reasonOf(plan, "S3"))
	assert.Equal(t, "100 % sur les deux dernières mesures (1/1, 3/3)", reasonOf(plan, "S4"), "a case run once at 100 % stays at ×1")

	// One stable measure is enough when asked.
	assert.Equal(t, 1, planOf(TieredPlan(base, cases, []string{m}, 3, 1))["S3"])
	assert.Equal(t, 3, planOf(TieredPlan(base, cases, []string{m}, 3, 3))["S1"])

	// A case changed since a measure runs the full repetitions.
	require.NoError(t, os.WriteFile(filepath.Join(cases[0].Dir, FixtureDir, "x.go"), []byte("package x\n"), 0o644))
	changed, err := LoadCase(cases[0].Dir)
	require.NoError(t, err)
	plan = TieredPlan(base, []*Case{changed}, []string{m}, 3, 0)
	assert.Equal(t, 3, plan[0].Reps)
	assert.Equal(t, "cas modifié depuis la dernière mesure", plan[0].Reason)

	// Every model must be stable; no baseline: every case runs in full.
	plan = TieredPlan(base, cases[:1], []string{m, "claude-sonnet-4-6"}, 3, 0)
	assert.Equal(t, 3, plan[0].Reps)
	assert.Equal(t, "claude-sonnet-4-6 : absent de la référence", plan[0].Reason)
	assert.Equal(t, 3, planOf(TieredPlan(nil, cases, []string{m}, 3, 0))["S1"])
}

// A baseline recorded before the history (no hash, no history): its stats
// are its only measure, and a change of the assertions is seen.
func TestTieredPlanLegacyBaseline(t *testing.T) {
	cases := tierCases(t)
	const m = "claude-haiku-4-5"
	legacy := statsReport(day(2), m, cases, map[string][2]int{"S1": {3, 3}, "S2": {3, 3}})
	for i := range legacy.Cases {
		legacy.Cases[i].Hash = ""
	}
	legacy.Stats[1].Assertions[0].Name = "ran make test*"
	data, err := json.Marshal(legacy)
	require.NoError(t, err)
	var read Report
	require.NoError(t, json.Unmarshal(data, &read))
	plan := TieredPlan(&read, cases, []string{m}, 3, 1)
	assert.Equal(t, 1, planOf(plan)["S1"])
	assert.Equal(t, 3, planOf(plan)["S2"])
	assert.Equal(t, "assertions modifiées depuis la dernière mesure", reasonOf(plan, "S2"))

	// Recorded again, the new baseline holds both measures.
	next := statsReport(day(3), m, cases, map[string][2]int{"S1": {1, 1}, "S2": {3, 3}})
	b := next.Baseline("ref 3").WithHistory(&read)
	assert.Len(t, b.Measures("S1", m), 2)
	assert.Equal(t, "1/1", b.Measures("S1", m)[0].String())
	assert.NotEmpty(t, b.Measures("S1", m)[0].Hash)
	assert.Empty(t, b.Measures("S1", m)[1].Hash)
	assert.Equal(t, 1, planOf(TieredPlan(b, cases, []string{m}, 3, 0))["S1"])
}

// The history of a baseline: the new measure first, a rescore of the same
// run replaces its measure, at most maxHistory measures, the cases measured
// before only kept.
func TestBaselineHistory(t *testing.T) {
	cases := tierCases(t)
	const m = "m"
	var base *Report
	for d := 1; d <= 6; d++ {
		res := map[string][2]int{"S1": {d % 4, 3}}
		if d == 1 {
			res["S2"] = [2]int{3, 3}
		}
		base = statsReport(day(d), m, cases, res).Baseline("ref").WithHistory(base)
	}
	ms := base.Measures("S1", m)
	require.Len(t, ms, maxHistory)
	assert.True(t, ms[0].At.Equal(day(6)))
	assert.True(t, ms[3].At.Equal(day(3)))
	assert.Len(t, base.Measures("S2", m), 1, "a case left out of a measure keeps its history")

	// A rescore of the last run is the same measure.
	rescored := *base
	rescored.Stats = []CaseStats{{Case: "S1", Model: m, Runs: 3, Passed: 3}}
	again := rescored.Baseline("rescore").WithHistory(base)
	ms = again.Measures("S1", m)
	require.Len(t, ms, maxHistory)
	assert.Equal(t, "3/3", ms[0].String())
	assert.True(t, ms[1].At.Equal(day(5)))

	// JSON round trip.
	path := filepath.Join(t.TempDir(), "b.json")
	require.NoError(t, again.WriteJSON(path))
	read, err := ReadReport(path)
	require.NoError(t, err)
	assert.Equal(t, again.History, read.History)

	// Reports of the lots count after the baseline, by date.
	lot := statsReport(day(7), m, cases, map[string][2]int{"S3": {2, 2}, "S1": {1, 1}})
	lot.Variants = nil
	with := read.WithReports([]*Report{lot})
	assert.Equal(t, "1/1", with.Measures("S1", m)[0].String())
	assert.Len(t, with.Measures("S1", m), maxHistory+1, "not capped: only the plan reads it")
	assert.Equal(t, "2/2", with.Measures("S3", m)[0].String())
	assert.Len(t, read.Measures("S3", m), 0, "the baseline itself is unchanged")
}

func TestParseRepsCase(t *testing.T) {
	cases := tierCases(t)
	got, err := ParseRepsCase([]string{"s1=1, S2=3", "S4=2"}, cases)
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"S1": 1, "S2": 3, "S4": 2}, got)
	for _, bad := range []string{"S1", "S1=0", "S1=x", "S99=1"} {
		_, err := ParseRepsCase([]string{bad}, cases)
		assert.Error(t, err, bad)
	}
	plan := ApplyReps(nil, map[string]int{"S2": 1}, cases, 3)
	assert.Equal(t, map[string]int{"S1": 3, "S2": 1, "S3": 3, "S4": 3}, planOf(plan))
	assert.Equal(t, "imposé (--reps-case)", reasonOf(plan, "S2"))
	assert.Nil(t, ApplyReps(nil, nil, cases, 3))
}

// failing fails every pass of the given cases at the given repetitions.
func failing(fail map[string]map[int]bool, cost float64) ExecutorFunc {
	return func(ctx context.Context, p Pass) (*PassOutput, error) {
		tr := &sessionsvc.Trace{Events: []sessionsvc.TraceEvent{ev(sessionsvc.TraceSession, nil)}}
		if !fail[p.Case.ID][p.Rep] {
			tr.Events = append(tr.Events, shell("developer", "go test ./...", false))
		}
		path := filepath.Join(p.OutDir, "trace.jsonl")
		f, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		if err := tr.WriteJSONL(f); err != nil {
			return nil, err
		}
		return &PassOutput{Outcome: OutcomeCompleted, CostUSD: cost, TracePath: path, Duration: time.Second}, nil
	}
}

// A case planned once whose pass fails runs up to EscalateTo in the same
// run; the report keeps the order of a sequential run, in parallel too.
func TestEscalation(t *testing.T) {
	requireGit(t)
	cases := tierCases(t)
	plan := []CasePlan{{Case: "S1", Reps: 1, Reason: "stable"}, {Case: "S2", Reps: 1, Reason: "stable"}, {Case: "S3", Reps: 3, Reason: "nouveau"}, {Case: "S4", Reps: 2}}
	fail := map[string]map[int]bool{"S2": {1: true}, "S4": {2: true}}
	run := func(parallel int) (*Report, []string) {
		var mu sync.Mutex
		var logs []string
		r := &Runner{Exec: failing(fail, 0.1), Models: []string{"m"}, Reps: 3, WorkRoot: t.TempDir(), OutDir: t.TempDir(), Parallel: parallel,
			Plan: plan, EscalateTo: 3, Now: fixedNow, Log: func(format string, args ...any) {
				mu.Lock()
				defer mu.Unlock()
				logs = append(logs, strings.TrimSpace(fmt.Sprintf(format, args...)))
			}}
		rep, err := r.Run(context.Background(), cases)
		require.NoError(t, err)
		return rep, logs
	}
	rep, logs := run(1)
	var got []string
	for _, p := range rep.Passes {
		got = append(got, fmt.Sprintf("%s#%d:%v", p.Case, p.Rep, p.Pass))
	}
	assert.Equal(t, []string{"S1#1:true", "S2#1:false", "S2#2:true", "S2#3:true", "S3#1:true", "S3#2:true", "S3#3:true",
		"S4#1:true", "S4#2:false", "S4#3:true"}, got)
	assert.Contains(t, logs, "S2 m #1: failed, 2 more pass(es) up to ×3")
	// The escalated passes go first: S2 #2 and #3 right after S2 #1.
	assert.Equal(t, []string{"[1/7] S1 m #1: completed PASS ($0.100, 1s)", "S2 m #1: failed, 2 more pass(es) up to ×3",
		"[2/9] S2 m #1: completed FAIL ($0.100, 1s)", "[3/9] S2 m #2: completed PASS ($0.100, 1s)"}, logs[:4])
	assert.Contains(t, logs, "S4 m #2: failed, 1 more pass(es) up to ×3")
	s, _ := rep.Stat("S2", "m")
	assert.Equal(t, 3, s.Runs)
	assert.Equal(t, 2, s.Passed)
	assert.Equal(t, []CasePlan{{Case: "S1", Reps: 1, Reason: "stable"}, {Case: "S2", Reps: 1, Reason: "stable", Escalated: true},
		{Case: "S3", Reps: 3, Reason: "nouveau"}, {Case: "S4", Reps: 2, Escalated: true}}, rep.Plan)
	assert.Equal(t, 3, rep.EscalateTo)
	// TestShippedBaseline reads the passes a case must have from the plan.
	for id, want := range map[string]int{"S1": 1, "S2": 3, "S3": 3, "S4": 3} {
		s, _ := rep.Stat(id, "m")
		assert.Equal(t, want, rep.PlannedRuns(s), id)
		assert.Equal(t, want, s.Runs, id)
	}
	assert.Equal(t, 1, rep.PlannedRuns(CaseStats{Case: "S1", Runs: 1, Passed: 1}))
	assert.Equal(t, 3, rep.PlannedRuns(CaseStats{Case: "S1", Runs: 1}), "a case run once that failed must have escalated")
	assert.Equal(t, 3, (&Report{Reps: 3}).PlannedRuns(CaseStats{Case: "S1", Runs: 3}), "a baseline without plan")
	md := rep.Markdown(nil)
	assert.Contains(t, md, "répétitions : par cas (×1 : 2 cas, ×2 : 1 cas, ×3 : 1 cas) · escalade jusqu'à ×3 : S2, S4")
	assert.Contains(t, md, "| S2 | ×1 | 3 | stable ; échec : escaladé jusqu'à ×3 |")
	assert.Contains(t, md, "| S1 | ×1 | 1 | stable |")

	// Same report in parallel.
	par, _ := run(3)
	a, err := json.Marshal(rep)
	require.NoError(t, err)
	b, err := json.Marshal(par)
	require.NoError(t, err)
	assert.JSONEq(t, string(a), string(b))

	// The cost cap of the run applies to the escalated passes too.
	r := &Runner{Exec: failing(map[string]map[int]bool{"S1": {1: true}}, 0.4), Models: []string{"m"}, Reps: 3, WorkRoot: t.TempDir(), OutDir: t.TempDir(),
		Plan: []CasePlan{{Case: "S1", Reps: 1}, {Case: "S2", Reps: 1}, {Case: "S3", Reps: 1}, {Case: "S4", Reps: 1}}, EscalateTo: 3, BudgetTotalUSD: 1}
	capped, err := r.Run(context.Background(), cases)
	require.NoError(t, err)
	require.Len(t, capped.Passes, 3, "S1 #1, then its escalation S1 #2 and #3: $1.2")
	assert.Equal(t, []SkippedPass{{Case: "S2", Model: "m", Rep: 1}, {Case: "S3", Model: "m", Rep: 1}, {Case: "S4", Model: "m", Rep: 1}}, capped.Skipped)

	// Without a plan nor an escalation, the report has neither.
	r = &Runner{Exec: failing(nil, 0.1), Models: []string{"m"}, Reps: 1, WorkRoot: t.TempDir(), OutDir: t.TempDir()}
	plain, err := r.Run(context.Background(), cases)
	require.NoError(t, err)
	assert.Nil(t, plain.Plan)
	assert.Zero(t, plain.EscalateTo)
	assert.Contains(t, plain.Markdown(nil), "répétitions : 1 ·")
}

// The estimate counts the passes of the plan and their mean costs: the
// baseline, the other reports, the mean of the workflow, of the baseline,
// else the cap of a pass.
func TestEstimateRun(t *testing.T) {
	cases := tierCases(t)
	cases[3].Workflow = "ticket"
	const m = "m"
	base := statsReport(day(2), m, cases, map[string][2]int{"S1": {3, 3}})
	base.Stats[0].MeanCost = 0.2
	lot := statsReport(day(3), m, cases, map[string][2]int{"S2": {1, 1}})
	lot.Stats[0].MeanCost = 0.5
	book := &CostBook{Base: base, Extra: []CostSource{{Name: "lot/report.json", Report: lot}}, Budget: 1}

	c, src := book.Lookup(cases[0], m)
	assert.InDelta(t, 0.2, c, 1e-9)
	assert.Equal(t, "référence du 2026-10-02", src)
	c, src = book.Lookup(cases[1], m)
	assert.InDelta(t, 0.5, c, 1e-9)
	assert.Equal(t, "lot/report.json", src)
	c, src = book.Lookup(cases[2], m)
	assert.InDelta(t, 0.35, c, 1e-9)
	assert.Equal(t, "moyenne du workflow quick (2 cas)", src)
	c, src = book.Lookup(cases[3], m)
	assert.InDelta(t, 0.2, c, 1e-9)
	assert.Equal(t, "moyenne de la référence", src)
	c, src = (&CostBook{Budget: 1}).Lookup(cases[0], m)
	assert.InDelta(t, 1, c, 1e-9)
	assert.Equal(t, "plafond par passage", src)

	plan := []CasePlan{{Case: "S1", Reps: 1, Reason: "stable"}, {Case: "S2", Reps: 3}}
	e := EstimateRun(cases, plan, []string{m}, 3, 0, 3, book)
	e.Tiered, e.BudgetTotal = true, 3
	assert.Equal(t, 1+3+3+3, e.Passes)
	assert.InDelta(t, 0.2+1.5+1.05+0.6, e.Cost, 1e-9)
	assert.Equal(t, 12, e.WorstPasses)
	assert.InDelta(t, 0.6+1.5+1.05+0.6, e.WorstCost, 1e-9)
	md := e.Markdown()
	for _, want := range []string{
		"référence par niveaux : ×1 si 100 % sur les deux dernières mesures, ×3 sinon · escalade jusqu'à ×3",
		"| S1 | quick | 1 | stable | $0.200 | référence du 2026-10-02 | $0.20 |",
		"- Cas : 4 (×1 : 1 cas, ×3 : 3 cas)",
		"- Total : 10 passages · ≈ $3.35",
		"- Si tous les cas à moins de ×3 échouent (escalade) : 12 passages · ≈ $3.75",
		"- Plafond global : $3.00 (dépassé : des passages ne seront pas lancés)",
	} {
		assert.Contains(t, md, want)
	}
	e = EstimateRun(cases, nil, []string{m, "m2"}, 2, 2, 0, book)
	assert.Equal(t, 4*2*2*2, e.Passes, "cases × models × repetitions × variants")
}

func TestHashDir(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a"), []byte("x"), 0o644))
	h1, err := HashDir(dir)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".DS_Store"), []byte("junk"), 0o644))
	h2, err := HashDir(dir)
	require.NoError(t, err)
	assert.Equal(t, h1, h2)
	require.NoError(t, os.Rename(filepath.Join(dir, "a"), filepath.Join(dir, "b")))
	h3, err := HashDir(dir)
	require.NoError(t, err)
	assert.NotEqual(t, h1, h3, "a renamed file changes the hash")
}
