package beads

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// N17 / F-01: the commit of bd init and the files of the user are only
// changed when it is safe and asked for.

type gitRepo struct {
	t   *testing.T
	dir string
}

func (r gitRepo) git(args ...string) string {
	r.t.Helper()
	c := exec.Command("git", args...)
	c.Dir = r.dir
	c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=maintenance.auto", "GIT_CONFIG_VALUE_0=false")
	out, err := c.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// bdCommitRepo is a repository whose HEAD is a commit of bd init (after a
// first commit of the user unless first).
func bdCommitRepo(t *testing.T, first bool) gitRepo {
	t.Helper()
	r := gitRepo{t, t.TempDir()}
	r.git("init", "-q", "-b", "main")
	if !first {
		writeProject(t, r.dir, map[string]string{"a.txt": "x\n"})
		r.git("add", "a.txt")
		r.git("commit", "-qm", "init")
	}
	writeProject(t, r.dir, map[string]string{
		".beads/config.yaml": "",
		"CLAUDE.md":          "# Mon projet\n\n<!-- BEGIN BEADS INTEGRATION v:1 -->\nbd ready\n<!-- END BEADS INTEGRATION -->\n",
	})
	r.git("add", ".")
	r.git("commit", "-qm", bdInitCommitSubject)
	return r
}

func kinds(r CleanReport) map[string]string {
	out := map[string]string{}
	for _, a := range r.Actions {
		out[a.Kind] = a.Path
	}
	return out
}

func TestUndoCommitOnlyWhenAsked(t *testing.T) {
	r := bdCommitRepo(t, false)
	report, err := CleanImpact(r.dir, CleanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.git("log", "-1", "--format=%s") != bdInitCommitSubject {
		t.Fatal("commit undone without UndoCommit")
	}
	if _, ok := kinds(report)[CleanCommitKept]; !ok {
		t.Fatalf("commit not reported as kept: %+v", report)
	}
	plan, err := CleanImpact(r.dir, CleanOptions{UndoCommit: true, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := kinds(plan)[CleanUndoCommit]; !ok || !plan.Changed() {
		t.Fatalf("plan does not list the undo: %+v", plan)
	}
	if r.git("log", "-1", "--format=%s") != bdInitCommitSubject {
		t.Fatal("commit undone by a plan")
	}
	report, err = CleanImpact(r.dir, CleanOptions{UndoCommit: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := kinds(report)[CleanUncommit]; !ok {
		t.Fatalf("undo not reported: %+v", report)
	}
	if got := r.git("log", "-1", "--format=%s"); got != "init" {
		t.Fatalf("HEAD = %q", got)
	}
}

// Remote branches are fetched before undoing: a ref left behind by an older
// fetch does not hide a push.
func TestUndoCommitFetchesTheRemotes(t *testing.T) {
	r := bdCommitRepo(t, false)
	bare := gitRepo{t, t.TempDir()}
	bare.git("init", "-q", "--bare")
	r.git("remote", "add", "origin", bare.dir)
	r.git("push", "-q", "origin", "main")
	r.git("update-ref", "refs/remotes/origin/main", "HEAD~1") // stale ref
	report, err := CleanImpact(r.dir, CleanOptions{UndoCommit: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.git("log", "-1", "--format=%s") != bdInitCommitSubject {
		t.Fatalf("a pushed commit was undone: %+v", report)
	}
	if _, ok := kinds(report)[CleanSkipped]; !ok {
		t.Fatalf("pushed commit not reported: %+v", report)
	}
}

// A remote that cannot be fetched: the commit may be pushed, never undone.
func TestUndoCommitRemoteUnreachable(t *testing.T) {
	r := bdCommitRepo(t, false)
	r.git("remote", "add", "origin", filepath.Join(t.TempDir(), "missing.git"))
	report, err := CleanImpact(r.dir, CleanOptions{UndoCommit: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.git("log", "-1", "--format=%s") != bdInitCommitSubject {
		t.Fatal("commit undone with an unreachable remote")
	}
	if _, ok := kinds(report)[CleanCommitUnverified]; !ok {
		t.Fatalf("not reported: %+v", report)
	}
}

// The first commit of a repository is never undone (no unborn branch).
func TestFirstCommitNeverUndone(t *testing.T) {
	r := bdCommitRepo(t, true)
	report, err := CleanImpact(r.dir, CleanOptions{UndoCommit: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.git("log", "-1", "--format=%s") != bdInitCommitSubject {
		t.Fatal("first commit undone")
	}
	if _, ok := kinds(report)[CleanCommitFirst]; !ok {
		t.Fatalf("not reported: %+v", report)
	}
}

// An untracked settings file (maybe the user's own) is left alone by an
// automatic clean-up, cleaned when the user agreed.
func TestKeepUntrackedSettings(t *testing.T) {
	r := gitRepo{t, t.TempDir()}
	r.git("init", "-q", "-b", "main")
	settings := `{"hooks":{"SessionStart":[{"hooks":[{"command":"bd prime --hook-json","type":"command"}]},{"hooks":[{"command":"./mine.sh","type":"command"}]}]}}`
	writeProject(t, r.dir, map[string]string{".beads/config.yaml": "", ".claude/settings.json": settings})
	report, err := CleanImpact(r.dir, CleanOptions{KeepUntracked: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(r.dir, ".claude", "settings.json")); string(got) != settings {
		t.Fatalf("untracked settings changed: %s", got)
	}
	if kinds(report)[CleanAsk] != filepath.Join(".claude", "settings.json") {
		t.Fatalf("not reported: %+v", report)
	}
	if _, err := CleanImpact(r.dir, CleanOptions{}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(r.dir, ".claude", "settings.json"))
	if strings.Contains(string(got), "bd prime") || !strings.Contains(string(got), "mine.sh") {
		t.Fatalf("settings = %s", got)
	}
}

// A .beads/ shared through git is not excluded, nor are bd's .gitignore
// entries moved to .git/info/exclude.
func TestSharedBeadsNotExcluded(t *testing.T) {
	r := gitRepo{t, t.TempDir()}
	r.git("init", "-q", "-b", "main")
	gitignore := "node_modules/\n\n# Beads / Dolt files (added by bd init)\n.dolt/\n*.db\n"
	writeProject(t, r.dir, map[string]string{".beads/issues.jsonl": "{}\n", ".gitignore": gitignore})
	r.git("add", ".")
	r.git("commit", "-qm", "share beads")
	report, err := CleanImpact(r.dir, CleanOptions{Tracked: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(r.dir, ".gitignore")); string(got) != gitignore {
		t.Fatalf(".gitignore changed: %q", got)
	}
	if data, _ := os.ReadFile(filepath.Join(r.dir, ".git", "info", "exclude")); excludesBeads(string(data)) {
		t.Fatal(".beads/ excluded while shared")
	}
	if report.Changed() {
		t.Fatalf("changes: %+v", report)
	}
}
