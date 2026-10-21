package beads

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

// N17 / F-01: the commit of bd init and the files of the user are only
// changed when it is safe and asked for.

type gitRepo struct {
	t   *testing.T
	dir string
}

func (r gitRepo) git(args ...string) string { return gittest.Run(r.t, r.dir, args...) }

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

func countLines(content, line string) int {
	n := 0
	for _, l := range strings.Split(content, "\n") {
		if strings.TrimSpace(l) == line {
			n++
		}
	}
	return n
}

// Q31: the lock bd 1.3 leaves at the root of the project is excluded with
// .beads/, once, also in a project initialized before bd 1.3.
func TestExcludeAddsGateLock(t *testing.T) {
	r := gitRepo{t, t.TempDir()}
	r.git("init", "-q", "-b", "main")
	exclude := filepath.Join(r.dir, ".git", "info", "exclude")
	writeProject(t, r.dir, map[string]string{".beads/config.yaml": "", ".git/info/exclude": "# mine\n.beads/\n"})

	plan, err := CleanImpact(r.dir, CleanOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	k := kinds(plan)
	if _, ok := k[CleanExcludeLock]; !ok {
		t.Fatalf("missing lock not planned: %+v", plan)
	}
	if _, ok := k[CleanExclude]; ok {
		t.Fatalf(".beads/ planned again: %+v", plan)
	}
	if got, _ := os.ReadFile(exclude); strings.Contains(string(got), gateLockPattern) {
		t.Fatal("dry run wrote the exclude file")
	}

	for range 2 {
		if _, err := CleanImpact(r.dir, CleanOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := os.ReadFile(exclude)
	if countLines(string(got), gateLockPattern) != 1 || countLines(string(got), ".beads/") != 1 || !strings.HasPrefix(string(got), "# mine\n") {
		t.Fatalf("exclude:\n%s", got)
	}
	if out := r.git("status", "--porcelain", "--ignored=no"); out != "" {
		t.Fatalf("status not clean: %q", out)
	}
	writeProject(t, r.dir, map[string]string{".beads.gate.lock": ""})
	if out := r.git("status", "--porcelain"); out != "" {
		t.Fatalf("lock not ignored: %q", out)
	}
	report, err := CleanImpact(r.dir, CleanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Changed() {
		t.Fatalf("not idempotent: %+v", report)
	}
}

// A fresh project gets both entries; an equivalent pattern (bd doctor --fix
// writes *.gate.lock*) is not doubled.
func TestExcludeGateLockFreshAndEquivalent(t *testing.T) {
	r := gitRepo{t, t.TempDir()}
	r.git("init", "-q", "-b", "main")
	writeProject(t, r.dir, map[string]string{".beads/config.yaml": ""})
	report, err := CleanImpact(r.dir, CleanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	k := kinds(report)
	if _, ok := k[CleanExclude]; !ok {
		t.Fatalf(".beads/ not added: %+v", report)
	}
	if _, ok := k[CleanExcludeLock]; !ok {
		t.Fatalf("lock not added: %+v", report)
	}
	got, _ := os.ReadFile(filepath.Join(r.dir, ".git", "info", "exclude"))
	if countLines(string(got), gateLockPattern) != 1 || countLines(string(got), ".beads/") != 1 {
		t.Fatalf("exclude:\n%s", got)
	}

	other := gitRepo{t, t.TempDir()}
	other.git("init", "-q", "-b", "main")
	writeProject(t, other.dir, map[string]string{".beads/config.yaml": "", ".git/info/exclude": "/.beads\n*.gate.lock*\n"})
	report, err = CleanImpact(other.dir, CleanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.Changed() {
		t.Fatalf("equivalent patterns not recognized: %+v", report)
	}
}

// A .beads/ shared through git: oh writes neither .git/info/exclude nor the
// .gitignore of the team, it reports the missing entry until the team's
// .gitignore has it (a local exclude does not count).
func TestSharedGateLockUnignored(t *testing.T) {
	r := gitRepo{t, t.TempDir()}
	r.git("init", "-q", "-b", "main")
	writeProject(t, r.dir, map[string]string{".beads/issues.jsonl": "{}\n", ".gitignore": "node_modules/\n"})
	r.git("add", ".")
	r.git("commit", "-qm", "share beads")
	exclude := filepath.Join(r.dir, ".git", "info", "exclude")
	before, _ := os.ReadFile(exclude)

	report, err := CleanImpact(r.dir, CleanOptions{Tracked: true})
	if err != nil {
		t.Fatal(err)
	}
	if report.Changed() {
		t.Fatalf("changes while shared: %+v", report)
	}
	if got, _ := os.ReadFile(exclude); string(got) != string(before) {
		t.Fatalf("exclude written while shared:\n%s", got)
	}
	if got, _ := os.ReadFile(filepath.Join(r.dir, ".gitignore")); string(got) != "node_modules/\n" {
		t.Fatalf(".gitignore written: %q", got)
	}
	if !SharedGateLockUnignored(r.dir) {
		t.Fatal("missing team entry not reported")
	}
	writeProject(t, r.dir, map[string]string{".git/info/exclude": gateLockPattern + "\n"})
	if !SharedGateLockUnignored(r.dir) {
		t.Fatal("a local exclude counted as the team's .gitignore")
	}
	writeProject(t, r.dir, map[string]string{".gitignore": "node_modules/\n/.beads.gate.lock\n", ".git/info/exclude": ""})
	if SharedGateLockUnignored(r.dir) {
		t.Fatal("reported with the entry in .gitignore")
	}
	writeProject(t, r.dir, map[string]string{".gitignore": "*.gate.lock\n!.beads.gate.lock\n"})
	if !SharedGateLockUnignored(r.dir) {
		t.Fatal("a negated pattern counted")
	}
}

func TestSharedGateLockUnignoredLocalBeads(t *testing.T) {
	r := gitRepo{t, t.TempDir()}
	r.git("init", "-q", "-b", "main")
	writeProject(t, r.dir, map[string]string{".beads/config.yaml": ""})
	if SharedGateLockUnignored(r.dir) {
		t.Fatal("local .beads/ reported as shared")
	}
	if SharedGateLockUnignored(t.TempDir()) {
		t.Fatal("not a repository reported")
	}
}
