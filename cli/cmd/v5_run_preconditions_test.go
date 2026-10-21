package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
	"github.com/datichb/openhub/cli/internal/runsvc"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// E7 (R8): the git and tool preconditions are read on the project at
// launch; a failed block names what to fix and refuses the launch.
func TestPreconditionsGit(t *testing.T) {
	i18ntest.Set(t, "fr")
	doc, diags := workflow.Parse([]byte(`apiVersion: oh/v1
kind: Workflow
id: release
preconditions:
  clean:
    label: { fr: L'arbre de travail doit être propre, en: The work tree must be clean }
    check: { git_clean: true }
    on_fail: block
  branch:
    label: { fr: Release depuis main ou release/*, en: Release from main or release/* }
    check: { branch: [main, "release/*"] }
    on_fail: block
  tools:
    check: { tools: [oh-tool-that-does-not-exist] }
`), workflow.Source{Layer: workflow.LayerHub})
	if diags.HasErrors() {
		t.Fatal(diags)
	}
	dir := t.TempDir()
	if _, _, err := preconditionWarnings(doc.Spec, dir); err == nil || !strings.Contains(err.Error(), "pas un dépôt git") {
		t.Fatalf("outside git: %v", err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	write := func(name, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("CHANGELOG.md", "# Changelog\n")
	git("add", ".")
	git("commit", "-q", "-m", "init")

	warns, _, err := preconditionWarnings(doc.Spec, dir)
	if err != nil {
		t.Fatalf("clean main: %v", err)
	}
	if len(warns) != 1 || !strings.Contains(warns[0].Args[0].(string), "oh-tool-that-does-not-exist") {
		t.Fatalf("missing tool warned: %+v", warns)
	}

	write(".beads/issues.jsonl", "{}\n") // Beads writes its own files
	if _, _, err := preconditionWarnings(doc.Spec, dir); err != nil {
		t.Fatalf(".beads ignored: %v", err)
	}

	write("CHANGELOG.md", "# Changelog\n\n## [Unreleased]\n")
	write("notes.txt", "x")
	_, _, err = preconditionWarnings(doc.Spec, dir)
	if err == nil || !strings.Contains(err.Error(), "L'arbre de travail doit être propre") || !strings.Contains(err.Error(), "2 modification(s) non commitée(s)") ||
		!strings.Contains(err.Error(), "CHANGELOG.md") || !strings.Contains(err.Error(), "lancement refusé") {
		t.Fatalf("dirty: %v", err)
	}
	git("stash", "-u", "-q")

	git("checkout", "-q", "-b", "feat/x")
	if _, _, err = preconditionWarnings(doc.Spec, dir); err == nil || !strings.Contains(err.Error(), "branche courante feat/x, attendu : main, release/*") {
		t.Fatalf("branch: %v", err)
	}
	git("checkout", "-q", "-b", "release/5.1")
	if _, _, err = preconditionWarnings(doc.Spec, dir); err != nil {
		t.Fatalf("release branch: %v", err)
	}
	git("checkout", "-q", "--detach")
	if _, _, err = preconditionWarnings(doc.Spec, dir); err == nil || !strings.Contains(err.Error(), "HEAD détaché") {
		t.Fatalf("detached: %v", err)
	}
}

// p6-debts: the preconditions are read where the session works: a worktree
// created at launch (--location new) is clean, on its branch, with the
// files of that branch or of the HEAD of the project; an existing worktree
// is read as it is.
func TestPreconditionsInSessionLocation(t *testing.T) {
	doc, diags := workflow.Parse([]byte(`apiVersion: oh/v1
kind: Workflow
id: release
preconditions:
  clean: { check: { git_clean: true }, on_fail: block }
  branch: { check: { branch: [main, "release/*"] }, on_fail: block }
  changelog: { check: { path_exists: [CHANGELOG.md] }, on_fail: block }
`), workflow.Source{Layer: workflow.LayerHub})
	if diags.HasErrors() {
		t.Fatal(diags)
	}
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte("# Changelog\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "init")
	git("checkout", "-q", "-b", "feat/x")
	if err := os.WriteFile(filepath.Join(dir, "wip.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	check := func(sessions []runsvc.PlannedSession, remote bool) error {
		fsys, probe := preconditionTarget(dir, sessions, remote)
		_, _, err := preconditionWarningsIn(doc.Spec, fsys, probe)
		return err
	}
	if check(nil, false) == nil {
		t.Fatal("the project directory (feat/x, dirty) refuses the release")
	}
	created := []runsvc.PlannedSession{{Location: runsvc.Location{Path: filepath.Join(dir, "..", "wt"), Kind: runsvc.LocationWorktree, Branch: "release/5.2", Create: true}}}
	if err := check(created, false); err != nil {
		t.Fatalf("new worktree on release/5.2: %v", err)
	}
	if check(created, true) == nil {
		t.Fatal("remote: the project")
	}
	created[0].Location.Branch = "feat/y"
	if err := check(created, false); err == nil || !strings.Contains(err.Error(), "feat/y") {
		t.Fatalf("new worktree on feat/y: %v", err)
	}
	git("rm", "-q", "CHANGELOG.md")
	git("commit", "-q", "-m", "rm")
	created[0].Location.Branch = "release/5.3"
	if check(created, false) == nil {
		t.Fatal("the files of the HEAD of the project: no CHANGELOG.md")
	}
	git("branch", "release/5.3", "main")
	if err := check(created, false); err != nil {
		t.Fatalf("an existing branch: its files: %v", err)
	}
	existing := []runsvc.PlannedSession{{Location: runsvc.Location{Path: dir, Kind: runsvc.LocationWorktree}}}
	if check(existing, false) == nil {
		t.Fatal("an existing worktree is read as it is")
	}
}
