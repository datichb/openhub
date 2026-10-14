package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/workflow"
)

// E7 (R8): the git and tool preconditions are read on the project at
// launch; a failed block names what to fix and refuses the launch.
func TestPreconditionsGit(t *testing.T) {
	i18n.SetLocale("fr")
	t.Cleanup(func() { i18n.SetLocale("en") })
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
