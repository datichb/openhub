package evalrun

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

func ev(kind string, f func(e *sessionsvc.TraceEvent)) sessionsvc.TraceEvent {
	e := sessionsvc.TraceEvent{Kind: kind}
	if f != nil {
		f(&e)
	}
	return e
}

func shell(agent, cmd string, sub bool) sessionsvc.TraceEvent {
	return ev(sessionsvc.TraceTool, func(e *sessionsvc.TraceEvent) {
		e.Tool, e.Agent, e.InSubagent, e.Input = "shell", agent, sub, map[string]any{"command": cmd}
	})
}

// sampleTrace is a ticket session: skill, delegation, tests, question in a
// subagent, refused commit (twice), checkpoint, commit.
func sampleTrace() []sessionsvc.TraceEvent {
	evs := []sessionsvc.TraceEvent{
		ev(sessionsvc.TraceSession, nil),
		ev(sessionsvc.TraceSkill, func(e *sessionsvc.TraceEvent) {
			e.Agent, e.Skill, e.Tool = "lead", "developer/dev-standards-backend", "skill"
		}),
		ev(sessionsvc.TraceSubagent, func(e *sessionsvc.TraceEvent) { e.Agent, e.Subagent, e.Tool = "lead", "developer", "subagent" }),
		ev(sessionsvc.TraceTool, func(e *sessionsvc.TraceEvent) {
			e.Agent, e.Tool, e.InSubagent, e.Input = "developer", "read", true, map[string]any{"filePath": "/p/calc.go"}
		}),
		shell("developer", "cd /p && go test ./...", true),
		ev(sessionsvc.TraceTool, func(e *sessionsvc.TraceEvent) {
			e.Agent, e.Tool, e.InSubagent, e.ToolSession = "developer", "question", true, "ses_c"
		}),
		ev(sessionsvc.TraceTool, func(e *sessionsvc.TraceEvent) {
			e.Agent, e.Tool, e.InSubagent, e.ToolSession, e.Denied, e.Input = "developer", "shell", true, "ses_c", true, map[string]any{"command": "git commit -m x"}
		}),
		ev(sessionsvc.TraceTool, func(e *sessionsvc.TraceEvent) {
			e.Agent, e.Tool, e.InSubagent, e.ToolSession, e.Denied, e.Input = "developer", "shell", true, "ses_c", true, map[string]any{"command": "git commit -m x"}
		}),
		ev(sessionsvc.TraceText, func(e *sessionsvc.TraceEvent) {
			e.Agent, e.Text = "developer", "## Retour vers conductor\n### Questions bloquantes\nAucune."
		}),
		ev(sessionsvc.TraceDecision, func(e *sessionsvc.TraceEvent) { e.DecisionKind, e.Checkpoint = "checkpoint", "cp-1" }),
		ev(sessionsvc.TraceCheckpoint, func(e *sessionsvc.TraceEvent) { e.Event, e.Checkpoint = "passed", "cp-1" }),
		ev(sessionsvc.TraceDecision, func(e *sessionsvc.TraceEvent) {
			e.DecisionKind, e.Title, e.Data = "question", "Division", map[string]any{"fields": []any{map[string]any{"title": "Que faire si b vaut zéro ?"}}}
		}),
		ev(sessionsvc.TraceCheckpoint, func(e *sessionsvc.TraceEvent) { e.Event, e.Checkpoint = "passed", "cp-2" }),
		shell("developer", "git add -A && git commit -m 'fix: sub'", true),
		ev(sessionsvc.TraceSummary, nil),
	}
	for i := range evs {
		evs[i].Seq = i + 1
	}
	return evs
}

func one(t *testing.T, a Assertion, e Evidence) AssertionResult {
	t.Helper()
	require.NoError(t, a.Validate())
	return Check([]Assertion{a}, e)[0]
}

func TestAssertions(t *testing.T) {
	e := Evidence{Events: sampleTrace(), Changed: []string{"calc.go", "calc_test.go"}}
	cases := []struct {
		name string
		a    Assertion
		pass bool
	}{
		{"ran", Assertion{Type: AssertRan, Command: "go test*"}, true},
		{"ran by agent", Assertion{Type: AssertRan, Command: "go test*", Agent: "lead"}, false},
		{"ran before checkpoint", Assertion{Type: AssertRan, Command: "go test*", Before: &Anchor{Checkpoint: "cp-1"}}, true},
		{"ran never", Assertion{Type: AssertRan, Command: "make lint*"}, false},
		{"ran after read", Assertion{Type: AssertRan, Command: "go test*", Before: &Anchor{Tool: "read", Arg: "*calc.go"}}, false},
		{"absent commit before cp-2", Assertion{Type: AssertRan, Command: "git commit*", Absent: true, Before: &Anchor{CheckpointPassed: "cp-2"}}, true},
		{"absent commit before cp-1", Assertion{Type: AssertRan, Command: "git commit*", Absent: true, Before: &Anchor{CheckpointPassed: "cp-1"}}, true},
		{"absent go test before cp-2", Assertion{Type: AssertRan, Command: "go test*", Absent: true, Before: &Anchor{CheckpointPassed: "cp-2"}}, false},
		{"order", Assertion{Type: AssertCheckpointOrder, Order: []string{"cp-1", "cp-2"}}, true},
		{"order reversed", Assertion{Type: AssertCheckpointOrder, Order: []string{"cp-2", "cp-1"}}, false},
		{"order missing", Assertion{Type: AssertCheckpointOrder, Order: []string{"cp-1", "cp-3"}}, false},
		{"question in subagent", Assertion{Type: AssertNoQuestionInSubagent}, false},
		{"question in other subagent", Assertion{Type: AssertNoQuestionInSubagent, Agents: []string{"reviewer"}}, true},
		{"skill before delegation", Assertion{Type: AssertSkillLoadedBefore, Skill: "dev-standards-backend", Before: &Anchor{Subagent: "developer"}}, true},
		{"skill by another agent", Assertion{Type: AssertSkillLoadedBefore, Skill: "dev-standards-backend", Agent: "developer"}, false},
		{"skill never", Assertion{Type: AssertSkillLoadedBefore, Skill: "audit-security"}, false},
		{"skill anchor missing", Assertion{Type: AssertSkillLoadedBefore, Skill: "dev-standards-backend", Before: &Anchor{Subagent: "reviewer"}}, true},
		{"changed within", Assertion{Type: AssertChangedWithin, Paths: []string{"*.go"}}, true},
		{"changed outside", Assertion{Type: AssertChangedWithin, Paths: []string{"calc.go"}}, false},
		{"changed max", Assertion{Type: AssertChangedWithin, MaxFiles: 1}, false},
		{"no change", Assertion{Type: AssertChangedWithin}, false},
		{"denied loop", Assertion{Type: AssertNoDeniedLoop, MaxRepeats: 1}, false},
		{"denied twice allowed", Assertion{Type: AssertNoDeniedLoop}, true},
		{"text", Assertion{Type: AssertTextMatches, Pattern: "retour vers conductor", Agent: "developer"}, true},
		{"text multi-line", Assertion{Type: AssertTextMatches, Pattern: `Retour vers conductor[\s\S]*Questions bloquantes`}, true},
		{"text absent", Assertion{Type: AssertTextMatches, Pattern: "orchestrator-dev", Absent: true}, true},
		{"text other agent", Assertion{Type: AssertTextMatches, Pattern: "conductor", Agent: "reviewer"}, false},
		{"text in questions", Assertion{Type: AssertTextMatches, Pattern: "z[ée]ro", Questions: true}, true},
		{"text not in questions", Assertion{Type: AssertTextMatches, Pattern: "z[ée]ro"}, false},
		{"called", Assertion{Type: AssertCalled, Tool: "read", Agent: "developer"}, true},
		{"called before cp-1", Assertion{Type: AssertCalled, Tool: "read", Before: &Anchor{Checkpoint: "cp-1"}}, true},
		{"called refused", Assertion{Type: AssertCalled, Tool: "shell", Agent: "developer", Before: &Anchor{Text: "Retour vers"}}, true},
		{"called never", Assertion{Type: AssertCalled, Tool: "*wiki/propose"}, false},
		{"called absent", Assertion{Type: AssertCalled, Tool: "*wiki/apply", Absent: true}, true},
		{"changed include", Assertion{Type: AssertChangedWithin, Paths: []string{"*.go"}, Include: []string{"calc_test.go"}}, true},
		{"changed include missing", Assertion{Type: AssertChangedWithin, Include: []string{"docs/wiki/**"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := one(t, c.a, e)
			assert.Equal(t, c.pass, r.Pass, r.Detail)
			assert.NotEmpty(t, r.Name)
		})
	}
	assert.True(t, one(t, Assertion{Type: AssertChangedWithin}, Evidence{}).Pass, "nothing changed")
	// A decision without agent belongs to the last agent of its tool session.
	planner := Evidence{Events: []sessionsvc.TraceEvent{
		ev(sessionsvc.TraceTool, func(e *sessionsvc.TraceEvent) {
			e.Agent, e.Tool, e.InSubagent, e.ToolSession = "planner", "read", true, "ses_p"
		}),
		ev(sessionsvc.TraceDecision, func(e *sessionsvc.TraceEvent) {
			e.DecisionKind, e.InSubagent, e.ToolSession, e.Data = "question", true, "ses_p", map[string]any{"fields": []any{"zéro"}}
		}),
	}}
	assert.True(t, one(t, Assertion{Type: AssertNoQuestionInSubagent, Agents: []string{"pathfinder"}}, planner).Pass)
	assert.False(t, one(t, Assertion{Type: AssertNoQuestionInSubagent, Agents: []string{"planner"}}, planner).Pass)
	assert.False(t, one(t, Assertion{Type: AssertTextMatches, Pattern: "z[ée]ro", Questions: true, Agent: "orchestrator"}, planner).Pass, "asked by the planner, not relayed")
	late := Evidence{Events: []sessionsvc.TraceEvent{
		ev(sessionsvc.TraceText, func(e *sessionsvc.TraceEvent) { e.Agent, e.Text = "debugger", "## Phase 3 — Analyse" }),
		ev(sessionsvc.TraceSkill, func(e *sessionsvc.TraceEvent) { e.Agent, e.Skill = "debugger", "debugger-phase-2-3" }),
	}}
	assert.False(t, one(t, Assertion{Type: AssertSkillLoadedBefore, Skill: "debugger-phase-2-3", Before: &Anchor{Text: `#+[^\n]*Phase [23]`}}, late).Pass, "loaded after its phase")
	// A load that failed (skill absent from the bundle) is not a load.
	missing := Evidence{Events: []sessionsvc.TraceEvent{
		ev(sessionsvc.TraceSkill, func(e *sessionsvc.TraceEvent) {
			e.Agent, e.Skill, e.Status, e.Error = "auditor", "auditor-phase-3", "failed", "Unable to load skill auditor-phase-3"
		}),
		ev(sessionsvc.TraceSubagent, func(e *sessionsvc.TraceEvent) { e.Agent, e.Subagent = "auditor", "auditor-subagent" }),
	}}
	res := one(t, Assertion{Type: AssertSkillLoadedBefore, Skill: "auditor-phase-3", Before: &Anchor{Subagent: "auditor-subagent"}}, missing)
	assert.False(t, res.Pass)
	assert.Equal(t, "skill never loaded (1 failed attempt(s))", res.Detail)
}

func TestHiddenTestsAssertion(t *testing.T) {
	a := Assertion{Type: AssertHiddenTests}
	assert.False(t, one(t, a, Evidence{}).Pass, "not available")
	r := one(t, a, Evidence{HiddenTests: func() (bool, string, error) { return false, "FAIL TestSub", nil }})
	assert.False(t, r.Pass)
	assert.Equal(t, "FAIL TestSub", r.Detail)
	assert.True(t, one(t, a, Evidence{HiddenTests: func() (bool, string, error) { return true, "", nil }}).Pass)
}

func TestAssertionValidation(t *testing.T) {
	for _, a := range []Assertion{
		{Type: "nope"},
		{Type: AssertRan},
		{Type: AssertCheckpointOrder},
		{Type: AssertSkillLoadedBefore},
		{Type: AssertCalled},
		{Type: AssertTextMatches},
		{Type: AssertTextMatches, Pattern: "("},
		{Type: AssertRan, Command: "x", Before: &Anchor{}},
		{Type: AssertRan, Command: "x", Before: &Anchor{Tool: "read", Subagent: "x"}},
	} {
		assert.Error(t, a.Validate(), "%+v", a)
	}
}

func TestGlobAndShell(t *testing.T) {
	assert.True(t, globMatch("strx/**", "strx/a/b.go"))
	assert.True(t, globMatch("**/*_test.go", "a/b/c_test.go"))
	assert.True(t, globMatch("**/*_test.go", "c_test.go"))
	assert.False(t, globMatch("strx/**", "calc.go"))
	assert.True(t, globMatch("*_test.go", "calc_test.go"))
	assert.True(t, shellMatch("go test*", "cd /x && go test -v ./..."))
	assert.True(t, shellMatch("git commit*", "git add -A; git commit -m x"))
	assert.False(t, shellMatch("git commit*", "echo git commit"))
}

func TestDecide(t *testing.T) {
	a := Answers{Checkpoints: map[string]string{"cp-2": "fix: ajoute un test", "*": "approve"}, Question: "Rouge"}
	cp := func(id string) domain.Decision {
		return domain.Decision{Kind: domain.DecisionCheckpoint, Payload: domain.DecisionPayload{Data: map[string]any{"checkpoint": id}}}
	}
	assert.Equal(t, Answer{Decision: "fix", Message: "ajoute un test"}, a.Decide(cp("cp-2")))
	assert.Equal(t, "approve", a.Decide(cp("cp-1")).Decision)
	assert.Equal(t, "approve", Answers{}.Decide(cp("cp-1")).Decision, "default")
	assert.Equal(t, "once", a.Decide(domain.Decision{Kind: domain.DecisionPermission}).Decision)
	assert.Equal(t, "reject", Answers{Permission: "reject"}.Decide(domain.Decision{Kind: domain.DecisionPermission}).Decision)
	b := a.Decide(domain.Decision{Kind: domain.DecisionBudget})
	assert.True(t, b.Stop)
	assert.Equal(t, "stop", b.Decision)

	q := domain.Decision{Kind: domain.DecisionQuestion, Payload: domain.DecisionPayload{Fields: []domain.DecisionField{
		{Key: "color", Options: []domain.DecisionOption{{Value: "blue", Label: "Bleu"}, {Value: "red", Label: "Rouge"}}},
		{Key: "free", Custom: true},
		{Key: "many", Type: "multiselect", Options: []domain.DecisionOption{{Value: "x"}}},
		{Key: "ok", Type: "boolean"},
		{Key: "ext", Type: "external"},
	}}}
	got := a.Decide(q).Answer
	assert.Equal(t, map[string]any{"color": "red", "free": "Rouge", "many": []string{"x"}, "ok": true}, got)
	assert.NoError(t, sessionsvc.ValidateAnswers(q.Payload.Fields, got))
	first := Answers{}.Decide(q).Answer
	assert.Equal(t, "blue", first["color"])
	assert.NoError(t, sessionsvc.ValidateAnswers(q.Payload.Fields, first))

	_, _, err := ParseChoice("maybe")
	assert.Error(t, err)
}

func writeCase(t *testing.T, root, target, name, yaml string) string {
	t.Helper()
	dir := filepath.Join(root, target, name)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, FixtureDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, CaseFile), []byte(yaml), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, FixtureDir, "go.mod"), []byte("module example.com/x\n\ngo 1.22\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, FixtureDir, "x.go"), []byte("package x\n\nfunc One() int { return 1 }\n"), 0o644))
	return dir
}

func TestLoadCases(t *testing.T) {
	root := t.TempDir()
	writeCase(t, root, "quick", "s10", "id: S10\nworkflow: quick\nassertions:\n  - type: no_denied_loop\n")
	writeCase(t, root, "quick", "s2", "id: S2\nworkflow: quick\ntimeout: 3m\nassertions:\n  - type: ran\n    command: go test*\n")
	writeCase(t, root, "ticket", "s8", "id: S8\nworkflow: ticket\nactive: false\ninactive_reason: après L4\nassertions:\n  - type: no_denied_loop\n")
	cases, err := LoadCases(root)
	require.NoError(t, err)
	require.Len(t, cases, 3)
	assert.Equal(t, []string{"S2", "S8", "S10"}, []string{cases[0].ID, cases[1].ID, cases[2].ID}, "S2 before S10")
	assert.Equal(t, 3*time.Minute, cases[0].TimeoutOrDefault())
	assert.Equal(t, DefaultTimeout, cases[2].TimeoutOrDefault())
	assert.Equal(t, "quick/S2", cases[0].Key())
	assert.False(t, cases[1].IsActive())
	sel, err := Select(cases, []string{"s10"})
	require.NoError(t, err)
	assert.Equal(t, "S10", sel[0].ID)
	_, err = Select(cases, []string{"S99"})
	assert.Error(t, err)

	bad := t.TempDir()
	writeCase(t, bad, "x", "a", "id: A\nworkflow: quick\nactive: false\nassertions:\n  - type: hidden_tests\n  - type: nope\nunknown_field: 1\n")
	_, err = LoadCases(bad)
	assert.Error(t, err)
	bad2 := t.TempDir()
	writeCase(t, bad2, "x", "a", "id: A\nworkflow: quick\nactive: false\nreports: maybe\nanswers: {checkpoints: {cp-1: maybe}}\nassertions:\n  - type: hidden_tests\n")
	_, err = LoadCases(bad2)
	require.Error(t, err)
	for _, want := range []string{"inactive_reason", "hidden_tests", "cp-1", "reports"} {
		assert.Contains(t, err.Error(), want)
	}
}

// The shipped cases load, S1 to S10 are there and active; S8 runs with the
// reports turned off (fallback on the whole report, L4).
func TestShippedCases(t *testing.T) {
	cases, err := LoadCases(filepath.Join("..", "..", "..", "evals"))
	require.NoError(t, err)
	inactive := map[string]bool{}
	var ids []string
	for _, c := range cases {
		ids = append(ids, c.ID)
		assert.Equal(t, !inactive[c.ID], c.IsActive(), c.ID)
		if c.ID == "S8" {
			assert.Equal(t, ReportsOff, c.Reports)
		}
		if c.ID == "S11" || c.ID == "S12" {
			assert.Equal(t, "accept", c.Answers.Wiki, "the proposals on the card are accepted")
		}
	}
	assert.Equal(t, []string{"S1", "S2", "S3", "S4", "S5", "S6", "S7", "S8", "S9", "S10", "S11", "S12", "S13", "S14", "S15", "S16", "S17", "S18", "S22", "S23", "S24", "S25"}, ids)
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}
}

func TestPrepareCloneAndChanges(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	dir := writeCase(t, root, "quick", "s1", "id: S1\nworkflow: quick\nassertions:\n  - type: hidden_tests\n")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, HiddenTestsDir), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, HiddenTestsDir, "x_test.go"), []byte("package x\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {\n\tif One() != 2 {\n\t\tt.Fatal(\"One() != 2\")\n\t}\n}\n"), 0o644))
	c, err := LoadCase(dir)
	require.NoError(t, err)
	c.Setup = []string{"git checkout -q -b feature/x", "echo '// seeded' >> x.go", "true"}
	ctx := context.Background()
	clone := filepath.Join(t.TempDir(), "clone")
	base, err := PrepareClone(ctx, c, clone)
	require.NoError(t, err)
	log, err := git(ctx, clone, "log", "--format=%s")
	require.NoError(t, err)
	assert.Equal(t, "setup: echo '// seeded' >> x.go\nfixture\n", log, "a setup step that changes files is committed")
	branch, _ := git(ctx, clone, "branch", "--show-current")
	assert.Equal(t, "feature/x", strings.TrimSpace(branch))

	changed, err := ChangedFiles(ctx, clone, base)
	require.NoError(t, err)
	assert.Empty(t, changed)
	require.NoError(t, os.WriteFile(filepath.Join(clone, "x.go"), []byte("package x\n\nfunc One() int { return 2 }\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(clone, "new.txt"), []byte("n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(clone, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(clone, "sub", "c.txt"), []byte("c"), 0o644))
	_, err = git(ctx, clone, "add", "sub/c.txt")
	require.NoError(t, err)
	_, err = git(ctx, clone, "commit", "-qm", "c")
	require.NoError(t, err)
	changed, err = ChangedFiles(ctx, clone, base)
	require.NoError(t, err)
	assert.Equal(t, []string{"new.txt", "sub/c.txt", "x.go"}, changed)

	if _, err := exec.LookPath("go"); err == nil {
		ok, detail, err := RunHiddenTests(ctx, c, clone)
		require.NoError(t, err)
		assert.True(t, ok, detail)
		_, err = os.Stat(filepath.Join(clone, "x_test.go"))
		assert.NoError(t, err, "hidden tests copied over the clone")
	}
}

// The runner prepares each pass, calls the executor, then checks the
// assertions on the trace it wrote; reports aggregate the passes.
func TestRunnerAndReport(t *testing.T) {
	requireGit(t)
	root := t.TempDir()
	writeCase(t, root, "quick", "s1", "id: S1\nworkflow: quick\nassertions:\n  - type: ran\n    command: go test*\n  - type: changed_within\n    paths: [\"*.go\"]\n")
	writeCase(t, root, "ticket", "s8", "id: S8\nworkflow: ticket\nactive: false\ninactive_reason: après L4\nassertions:\n  - type: no_denied_loop\n")
	cases, err := LoadCases(root)
	require.NoError(t, err)

	calls := 0
	exec := ExecutorFunc(func(ctx context.Context, p Pass) (*PassOutput, error) {
		calls++
		assert.FileExists(t, filepath.Join(p.Project, "x.go"), "clone ready")
		assert.Equal(t, 0.5, p.BudgetUSD)
		tr := &sessionsvc.Trace{Events: []sessionsvc.TraceEvent{ev(sessionsvc.TraceSession, nil)}}
		if p.Rep == 1 {
			tr.Events = append(tr.Events, shell("developer", "go test ./...", false))
			require.NoError(t, os.WriteFile(filepath.Join(p.Project, "x.go"), []byte("package x\n"), 0o644))
		} else {
			require.NoError(t, os.WriteFile(filepath.Join(p.Project, "README"), []byte("x"), 0o644))
		}
		path := filepath.Join(p.OutDir, "trace.jsonl")
		f, err := os.Create(path)
		require.NoError(t, err)
		require.NoError(t, tr.WriteJSONL(f))
		f.Close()
		return &PassOutput{SessionID: "ses_x", Outcome: OutcomeCompleted, CostUSD: 0.1, TracePath: path}, nil
	})
	out := t.TempDir()
	r := &Runner{Exec: exec, Models: []string{"claude-haiku-4-5"}, Reps: 2, BudgetUSD: 0.5, WorkRoot: t.TempDir(), OutDir: out}
	rep, err := r.Run(context.Background(), cases)
	require.NoError(t, err)
	assert.Equal(t, 2, calls, "the inactive case does not run")
	require.Len(t, rep.Passes, 2)
	assert.True(t, rep.Passes[0].Pass, "%+v", rep.Passes[0])
	assert.False(t, rep.Passes[1].Pass)
	assert.Equal(t, filepath.Join("passes", "S1", "claude-haiku-4-5", "1", "trace.jsonl"), rep.Passes[0].Trace)
	s, ok := rep.Stat("S1", "claude-haiku-4-5")
	require.True(t, ok)
	assert.Equal(t, 2, s.Runs)
	assert.InDelta(t, 0.5, s.PassRate, 1e-9)
	assert.InDelta(t, 0.2, rep.TotalCost, 1e-9)
	require.Len(t, s.Assertions, 2)
	assert.Equal(t, 1, s.Assertions[0].Passed)

	path := filepath.Join(out, "report.json")
	require.NoError(t, rep.Baseline("note").WriteJSON(path))
	base, err := ReadReport(path)
	require.NoError(t, err)
	assert.Equal(t, "note", base.Note)
	assert.Empty(t, base.Passes[0].SessionID, "no session ids in a baseline")
	md := rep.Markdown(base)
	for _, want := range []string{"| S1 | quick | claude-haiku-4-5 | 1/2 (50 %) | 1/2 |", "S8 (ticket) : après L4", "## Échecs", "changed within *.go"} {
		assert.Contains(t, md, want)
	}

	// Rescoring with another assertion: the trace one is checked again, the
	// clone one keeps its result.
	cases[0].Assertions[0].Command = "make lint*"
	again, err := Rescore(rep, out, cases)
	require.NoError(t, err)
	assert.False(t, again.Passes[0].Pass)
	assert.Equal(t, "ran make lint*", again.Passes[0].Assertions[0].Name)
	assert.False(t, again.Passes[0].Assertions[0].Pass)
	assert.Equal(t, rep.Passes[0].Assertions[1], again.Passes[0].Assertions[1], "changed_within kept")
	assert.True(t, rep.Passes[0].Pass, "the report rescored is not changed")
	s2, _ := again.Stat("S1", "claude-haiku-4-5")
	assert.Zero(t, s2.Passed)

	// An executor error is a failed pass, the run goes on.
	r.Exec = ExecutorFunc(func(context.Context, Pass) (*PassOutput, error) { return nil, os.ErrPermission })
	rep, err = r.Run(context.Background(), cases)
	require.NoError(t, err)
	assert.Equal(t, OutcomeError, rep.Passes[0].Outcome)
	assert.False(t, rep.Passes[0].Pass)
}

// notInBaseline are the active cases added after the reference measure
// (measured at the next make test-eval OH_EVAL_RECORD=1).
var notInBaseline = map[string]string{
	"S16": "added by R13 (skill-refresh: stale skill refreshed)",
	"S17": "added by R13 (skill-refresh: proposal refused, nothing written)",
	"S18": "added by R13 (skill-refresh: nothing to review)",
	"S22": "added by R1-b (ticket, tdd method: cp-red passed before the commit)",
	"S23": "added by R1-b (quick: go vet of the CI outside the changed package)",
	"S24": "added by R1-b (debug: misleading stack, root cause upstream)",
	"S25": "added by R1-b (quick: merge committed with conflict markers, both sides kept)",
}

// The reference measure kept in the repository (make test-eval
// OH_EVAL_RECORD=1) covers every active case, with three repetitions, and
// holds no session id nor trace path.
func TestShippedBaseline(t *testing.T) {
	base, err := ReadReport(filepath.Join("testdata", "evals", "baseline.json"))
	require.NoError(t, err)
	cases, err := LoadCases(filepath.Join("..", "..", "..", "evals"))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, base.Reps, 3)
	assert.NotEmpty(t, base.Note)
	for _, m := range base.Models {
		for _, c := range cases {
			s, ok := base.Stat(c.ID, m)
			if !c.IsActive() {
				assert.False(t, ok, "%s is inactive", c.ID)
				continue
			}
			if why, pending := notInBaseline[c.ID]; pending && !ok {
				t.Logf("%s not in the baseline yet: %s", c.ID, why)
				continue
			}
			require.True(t, ok, "%s measured with %s", c.ID, m)
			assert.Equal(t, base.Reps, s.Runs, c.ID)
			assert.Len(t, s.Assertions, len(c.Assertions), "%s: the baseline matches its assertions (oh eval --rescore)", c.ID)
		}
	}
	for _, p := range base.Passes {
		assert.Empty(t, p.SessionID)
		assert.Empty(t, p.Trace)
	}
}
