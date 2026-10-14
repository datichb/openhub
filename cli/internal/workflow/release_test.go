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
