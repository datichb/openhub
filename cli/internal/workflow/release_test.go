package workflow

import (
	"errors"
	"reflect"
	"testing"
	"testing/fstest"
)

const releaseModes = "mode: { manuel: pause, semi-auto: pause, auto: pause }"

// E7: git_clean, branch and tools are tests of their own, checked by the
// schema.
func TestValidate_PreconditionsRepo(t *testing.T) {
	t.Parallel()
	ok := vHeader + `risk: write
preconditions:
  clean: { check: { git_clean: true }, on_fail: block }
  branch: { check: { branch: [main, "release/*"] }, on_fail: block }
  tools: { check: { tools: [goreleaser, git] } }
  both: { check: { git_clean: true, path_exists: [CHANGELOG.md] } }
`
	for _, d := range checkYAML(t, ok, testEnv()) {
		if d.Severity == SeverityError {
			t.Fatalf("valid: %v", d)
		}
	}
	diags := checkYAML(t, vHeader+`risk: write
preconditions:
  empty: { check: { git_clean: false } }
  bad-branch: { check: { branch: ["", "release/["] } }
  bad-tool: { check: { tools: ["/usr/bin/goreleaser", "go test"] } }
`, testEnv())
	assertDiag(t, diags, "precondition_check_invalid", "preconditions.empty.check", SeverityError)
	assertDiag(t, diags, "precondition_branch_invalid", "preconditions.bad-branch.check.branch[0]", SeverityError)
	assertDiag(t, diags, "precondition_branch_invalid", "preconditions.bad-branch.check.branch[1]", SeverityError)
	assertDiag(t, diags, "precondition_tool_invalid", "preconditions.bad-tool.check.tools[0]", SeverityError)
	assertDiag(t, diags, "precondition_tool_invalid", "preconditions.bad-tool.check.tools[1]", SeverityError)
}

func TestEvaluatePreconditionsRepo(t *testing.T) {
	t.Parallel()
	cat := catalogOf(t, map[string]string{"hub:x": vHeader + `risk: publish
preconditions:
  clean: { check: { git_clean: true }, on_fail: block }
  branch: { check: { branch: [main, "release/*"] }, on_fail: block }
  tools: { check: { tools: [goreleaser, git] } }
  changelog: { check: { path_exists: [CHANGELOG.md] } }
`})
	doc, _ := cat.Lookup(Ref{LayerHub, "x"})
	fsys := fstest.MapFS{"CHANGELOG.md": {}}
	reads := 0
	probe := func(st GitState, err error) PreconditionProbe {
		return PreconditionProbe{
			Git:      func() (GitState, error) { reads++; return st, err },
			LookPath: func(cmd string) bool { return cmd == "git" },
		}
	}
	failures := func(res []PreconditionResult) map[string]*PreconditionFailure {
		out := map[string]*PreconditionFailure{}
		for _, r := range FailedPreconditions(res) {
			out[r.ID] = r.Failure
		}
		return out
	}

	got := failures(EvaluatePreconditionsWith(doc.Spec, fsys, probe(GitState{Branch: "release/5.1"}, nil)))
	if len(got) != 1 || !reflect.DeepEqual(got["tools"], &PreconditionFailure{Kind: FailTools, Missing: []string{"goreleaser"}}) {
		t.Fatalf("clean release branch: %+v", got)
	}
	if reads != 1 {
		t.Fatalf("git read %d times, want once", reads)
	}

	got = failures(EvaluatePreconditionsWith(doc.Spec, fsys, probe(GitState{Branch: "feat/x", Dirty: []string{"M CHANGELOG.md", "?? notes.txt"}}, nil)))
	if f := got["clean"]; f == nil || f.Kind != FailDirty || len(f.Dirty) != 2 {
		t.Fatalf("dirty: %+v", f)
	}
	if f := got["branch"]; f == nil || f.Kind != FailBranch || f.Branch != "feat/x" || !reflect.DeepEqual(f.Want, []string{"main", "release/*"}) {
		t.Fatalf("branch: %+v", f)
	}

	got = failures(EvaluatePreconditionsWith(doc.Spec, fsys, probe(GitState{}, nil)))
	if f := got["branch"]; f == nil || f.Branch != "" {
		t.Fatalf("detached HEAD matches no branch: %+v", f)
	}

	got = failures(EvaluatePreconditionsWith(doc.Spec, fstest.MapFS{}, probe(GitState{}, errors.New("not a git repository"))))
	for _, id := range []string{"clean", "branch"} {
		if f := got[id]; f == nil || f.Kind != FailNotGit || f.Err == "" {
			t.Fatalf("%s outside git: %+v", id, f)
		}
	}
	if f := got["changelog"]; f == nil || f.Kind != FailPathAny {
		t.Fatalf("missing path: %+v", f)
	}

	// Without a probe (file system only), the git and tool checks fail
	// closed.
	got = failures(EvaluatePreconditions(doc.Spec, fsys))
	if got["clean"] == nil || got["branch"] == nil || got["tools"] == nil || got["changelog"] != nil {
		t.Fatalf("no probe: %+v", got)
	}
}

func TestBranchMatches(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		branch string
		want   bool
	}{{"main", true}, {"release/5.1", true}, {"release/5/x", false}, {"mainline", false}, {"", false}} {
		if got := BranchMatches(c.branch, []string{"main", "release/*"}); got != c.want {
			t.Errorf("%q: %v", c.branch, got)
		}
	}
}

// E9 and risk: publish: tag and mr are unlocked by a publishing workflow
// only, at a checkpoint that pauses in every mode and never runs remotely.
func TestValidate_Publish(t *testing.T) {
	t.Parallel()
	cp := func(extra string) string {
		return "checkpoints:\n  cp-release: { unlocks: [commit, tag, mr], " + releaseModes + extra + " }\n"
	}
	ok := vHeader + "risk: publish\nruntime: { default: local, allowed: [local] }\n" + cp(", remote: forbid")
	for _, d := range checkYAML(t, ok, testEnv()) {
		if d.Severity == SeverityError {
			t.Fatalf("valid: %v", d)
		}
	}

	write := checkYAML(t, vHeader+"risk: write\n"+cp(""), testEnv())
	assertDiag(t, write, "unlock_requires_publish", "checkpoints.cp-release.unlocks[1]", SeverityError)
	assertDiag(t, write, "unlock_requires_publish", "checkpoints.cp-release.unlocks[2]", SeverityError)

	noForbid := checkYAML(t, vHeader+"risk: publish\nruntime: { default: local, allowed: [local] }\n"+cp(""), testEnv())
	assertDiag(t, noForbid, "publish_checkpoint_remote", "checkpoints.cp-release.remote", SeverityError)

	remote := checkYAML(t, vHeader+"risk: publish\nruntime: { default: local, allowed: [local, remote] }\n"+cp(", remote: forbid"), testEnv())
	assertDiag(t, remote, "publish_remote_runtime", "runtime.allowed", SeverityError)

	auto := vHeader + "risk: publish\nruntime: { default: local, allowed: [local] }\ncheckpoints:\n  cp-release: { unlocks: [tag], remote: forbid, mode: { manuel: pause, semi-auto: pause, auto: auto } }\n"
	assertDiag(t, checkYAML(t, auto, testEnv()), "publish_checkpoint_pause", "checkpoints.cp-release.mode.auto", SeverityError)

	// A checkpoint that publishes nothing keeps its own rules.
	plain := vHeader + "risk: publish\nruntime: { default: local, allowed: [local] }\ncheckpoints:\n  cp-plan: { mode: { manuel: pause, semi-auto: auto, auto: auto } }\n"
	for _, d := range checkYAML(t, plain, testEnv()) {
		if d.Severity == SeverityError {
			t.Fatalf("no publishing checkpoint: %v", d)
		}
	}
}

// A team layer cannot weaken the publishing checkpoint: remote is hardened
// only.
func TestPublishPatchHardened(t *testing.T) {
	t.Parallel()
	hub := vHeader + "risk: publish\nruntime: { default: local, allowed: [local] }\n" +
		"checkpoints:\n  cp-release: { unlocks: [commit, tag], remote: forbid, " + releaseModes + " }\n"
	cat := catalogOf(t, map[string]string{
		"hub:x":  hub,
		"team:x": "apiVersion: oh/v1\nkind: Workflow\nid: x\nextends: hub:x\ncheckpoints:\n  cp-release: { remote: auto }\n",
	})
	r, diags := ResolveSpec(cat, Ref{LayerTeam, "x"}, nil)
	cpr, _ := r.Spec.Checkpoints.Get("cp-release")
	if cpr.Remote != RemoteForbid {
		t.Fatalf("remote loosened: %s (%v)", cpr.Remote, diags)
	}
}

// p4-debts: in a `risk: publish` workflow, the inherited preconditions are
// security fields: a layer adds or hardens them, never removes nor loosens
// them (the parent's value is kept); elsewhere a layer may still do so.
func TestPublishPreconditionsHardened(t *testing.T) {
	t.Parallel()
	hubPre := "preconditions:\n" +
		"  clean: { check: { git_clean: true }, on_fail: block }\n" +
		"  branch: { check: { branch: [main, \"release/*\"] }, on_fail: block }\n" +
		"  tools: { check: { tools: [git, bd] }, on_fail: block }\n" +
		"  file: { check: { path_exists: [CHANGELOG.md] }, on_fail: block }\n"
	publishHub := vHeader + "risk: publish\nruntime: { default: local, allowed: [local] }\n" + hubPre
	resolve := func(t *testing.T, hub, team string) (*Spec, Diagnostics) {
		t.Helper()
		cat := catalogOf(t, map[string]string{
			"hub:x":  hub,
			"team:x": "apiVersion: oh/v1\nkind: Workflow\nid: x\nextends: hub:x\n" + team,
		})
		r, diags := ResolveSpec(cat, Ref{LayerTeam, "x"}, nil)
		if r == nil {
			t.Fatalf("not resolved: %v", diags)
		}
		return r.Spec, diags
	}
	loosened := []struct{ name, team, path string }{
		{"disabled", "preconditions:\n  clean: { disabled: true }\n", "preconditions.clean.disabled"},
		{"git_clean off", "preconditions:\n  clean: { check: { git_clean: false } }\n", "preconditions.clean.check.git_clean"},
		{"git_clean replaced", "preconditions:\n  clean: { check: { tools: [git] } }\n", "preconditions.clean.check.git_clean"},
		{"on_fail", "preconditions:\n  clean: { on_fail: suggest }\n", "preconditions.clean.on_fail"},
		{"branch widened", "preconditions:\n  branch: { check: { branch: [main, \"*\"] } }\n", "preconditions.branch.check.branch"},
		{"branch dropped", "preconditions:\n  branch: { check: { git_clean: true } }\n", "preconditions.branch.check.branch"},
		{"tool removed", "preconditions:\n  tools: { check: { tools: [git] } }\n", "preconditions.tools.check.tools"},
		{"path added", "preconditions:\n  file: { check: { path_exists: [CHANGELOG.md, README.md] } }\n", "preconditions.file.check.path_exists"},
	}
	for _, c := range loosened {
		t.Run(c.name, func(t *testing.T) {
			s, diags := resolve(t, publishHub, c.team)
			assertDiag(t, diags, "loosening", c.path, SeverityError)
			parent, _ := resolve(t, publishHub, "")
			for _, k := range parent.Preconditions.Keys() {
				want, _ := parent.Preconditions.Get(k)
				got, ok := s.Preconditions.Get(k)
				if !ok || !reflect.DeepEqual(got.Check, want.Check) || got.OnFail != want.OnFail {
					t.Fatalf("%s: parent value not kept: %+v, want %+v", k, got, want)
				}
			}
		})
	}

	hardened := "preconditions:\n" +
		"  clean: { check: { git_clean: true, tools: [make] }, label: { fr: Propre, en: Clean } }\n" +
		"  branch: { check: { branch: [\"release/v*\"] } }\n" +
		"  tools: { check: { tools: [git, bd, gh] } }\n" +
		"  file: { check: { path_exists: [CHANGELOG.md] } }\n" +
		"  extra: { check: { tools: [glab] }, on_fail: block }\n"
	s, diags := resolve(t, publishHub, hardened)
	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Fatalf("hardening refused: %v", d)
		}
	}
	if b, _ := s.Preconditions.Get("branch"); !reflect.DeepEqual(b.Check.Branch, []string{"release/v*"}) {
		t.Fatalf("narrowed branch not applied: %v", b.Check.Branch)
	}
	if c, _ := s.Preconditions.Get("clean"); !c.Check.GitClean || !reflect.DeepEqual(c.Check.Tools, []string{"make"}) || c.Label.Text("fr") != "Propre" {
		t.Fatalf("hardened check or label not applied: %+v", c)
	}
	if _, ok := s.Preconditions.Get("extra"); !ok {
		t.Fatal("added precondition dropped")
	}

	// Outside publish, a layer still removes or replaces a precondition.
	writeHub := vHeader + "risk: write\n" + hubPre
	s, diags = resolve(t, writeHub, "preconditions:\n  clean: { disabled: true }\n  branch: { check: { branch: [\"*\"] }, on_fail: suggest }\n")
	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Fatalf("write workflow: %v", d)
		}
	}
	if _, ok := s.Preconditions.Get("clean"); ok {
		t.Fatal("write workflow: precondition not removed")
	}
	if b, _ := s.Preconditions.Get("branch"); b.OnFail != OnFailSuggest {
		t.Fatalf("write workflow: on_fail not replaced: %s", b.OnFail)
	}
}
