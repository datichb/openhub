package beads

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/datichb/openhub/cli/internal/beads/bdtest"
	"github.com/datichb/openhub/cli/internal/gitutil/gittest"
)

func writeProject(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// A5 follow-up: the files of a `bd init` without --skip-agents (bd 1.3) are
// reported, CLAUDE.md and .cursor/ included; a user's own tool folder is not.
func TestDiagnoseBeadsImpactBd13AgentFiles(t *testing.T) {
	dir := t.TempDir()
	writeProject(t, dir, map[string]string{
		".beads/config.yaml":    "",
		".git/info/exclude":     ".beads/\n",
		"CLAUDE.md":             "# Project Instructions for AI Agents\n\n<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal -->\n## Tracker\n<!-- END BEADS INTEGRATION -->\n",
		".cursor/hooks.json":    `{"hooks":{"postToolUse":[{"command":"bd cursor-hook postToolUse"}]},"version":1}`,
		".claude/settings.json": `{"permissions":{"allow":["Read"]}}`,
		".gitignore":            "node_modules/\n\n# Beads / Dolt files (added by bd init)\n.dolt/\n",
	})
	var got []string
	for _, i := range DiagnoseBeadsImpact(dir) {
		got = append(got, i.Kind+": "+i.Detail)
	}
	all := strings.Join(got, "\n")
	for _, want := range []string{"CLAUDE.md", ".cursor/hooks.json", ".gitignore"} {
		if !strings.Contains(all, want) {
			t.Errorf("%s not reported:\n%s", want, all)
		}
	}
	if strings.Contains(all, ".claude/") {
		t.Errorf("a .claude/ folder without bd content is reported:\n%s", all)
	}
}

// Zero impact with a real `bd init` (bd 1.3): CleanImpact takes back out
// everything bd wrote outside .beads/ and keeps the user's own content
// (CLAUDE.md, .claude/settings.json, .gitignore). bd commits what it wrote:
// the commit is undone (UndoCommit) when the repository had one before; as
// the first commit of a repository it is kept (F-01) and nothing changes.
func TestCleanImpactAfterRealBdInit(t *testing.T) {
	bdtest.RequireRealBd(t)
	for _, firstCommit := range []bool{false, true} {
		dir := t.TempDir()
		run := func(name string, args ...string) {
			t.Helper()
			c := exec.Command(name, args...)
			c.Dir = dir
			c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
			if out, err := c.CombinedOutput(); err != nil {
				t.Fatalf("%s %v: %v\n%s", name, args, err, out)
			}
		}
		run("git", "init", "-q")
		user := map[string]string{
			"CLAUDE.md":             "# Mon projet\n\nRègles maison.\n",
			".gitignore":            "dist/\n",
			".claude/settings.json": "{\n  \"permissions\": {\n    \"allow\": [\n      \"Read\"\n    ]\n  }\n}\n",
		}
		if !firstCommit {
			writeProject(t, dir, map[string]string{"a.txt": "x\n"})
			run("git", "add", "a.txt")
			run("git", "commit", "-qm", "init")
		}
		writeProject(t, dir, user)
		run("bd", "init", "--prefix", "t", "--quiet")

		if len(DiagnoseBeadsImpact(dir)) == 0 {
			t.Fatal("bd init left no side effect: the test no longer covers bd")
		}
		report, err := CleanImpact(dir, CleanOptions{UndoCommit: true})
		if err != nil {
			t.Fatal(err)
		}
		committed := len(report.Actions) > 0 && strings.HasPrefix(report.Actions[0].Path, "commit ")
		if firstCommit {
			if committed && (report.Actions[0].Kind != CleanCommitFirst || report.Changed()) {
				t.Fatalf("first commit of bd init: %+v", report)
			}
			continue
		}
		if !report.Changed() {
			t.Fatalf("nothing cleaned: %+v", report)
		}
		for rel, want := range user {
			got, err := os.ReadFile(filepath.Join(dir, rel))
			if err != nil {
				t.Fatalf("%s: %v", rel, err)
			}
			if strings.TrimSpace(string(got)) != strings.TrimSpace(want) {
				t.Errorf("%s:\n%s\nwant:\n%s", rel, got, want)
			}
		}
		for _, rel := range []string{"AGENTS.md", ".cursor", ".codex", ".agents"} {
			if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
				t.Errorf("%s left by bd init", rel)
			}
		}
		if issues := DiagnoseBeadsImpact(dir); len(issues) != 0 {
			t.Errorf("still reported: %+v", issues)
		}
		exclude, _ := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
		if !strings.Contains(string(exclude), ".beads") || !strings.Contains(string(exclude), ".dolt/") || !excludesGateLock(string(exclude)) {
			t.Errorf("exclude:\n%s", exclude)
		}
	}
}

// A committed CLAUDE.md with bd's block is a change of the repository: the
// automatic clean-up leaves it (reported as skipped), `oh doctor --fix`
// (Tracked) removes the block and keeps the rest.
func TestCleanImpactCommittedFile(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) { gittest.Run(t, dir, args...) }
	run("init", "-q")
	writeProject(t, dir, map[string]string{
		".beads/config.yaml": "",
		"CLAUDE.md":          "# Mon projet\n\nRègles.\n\n<!-- BEGIN BEADS INTEGRATION v:1 -->\nbd ready\n<!-- END BEADS INTEGRATION -->\n",
	})
	run("add", "CLAUDE.md")
	run("commit", "-qm", "claude")

	report, err := CleanImpact(dir, CleanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Skipped(); len(got) != 1 || got[0] != "CLAUDE.md" {
		t.Fatalf("skipped = %v (%+v)", got, report)
	}
	if _, err := CleanImpact(dir, CleanOptions{Tracked: true}); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if string(got) != "# Mon projet\n\nRègles.\n" {
		t.Fatalf("CLAUDE.md = %q", got)
	}
}

// The commit of `bd init` is undone when it is HEAD and not pushed; a pushed
// one is never touched.
func TestCleanImpactUndoesBdInitCommit(t *testing.T) {
	bdtest.RequireRealBd(t)
	setup := func() string {
		dir := t.TempDir()
		gittest.Run(t, dir, "init", "-q", "-b", "main")
		writeProject(t, dir, map[string]string{"a.txt": "x\n"})
		gittest.Run(t, dir, "add", "a.txt")
		gittest.Run(t, dir, "commit", "-qm", "init")
		c := exec.Command("bd", "init", "--prefix", "t", "--quiet")
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("bd init: %v\n%s", err, out)
		}
		if gittest.Run(t, dir, "log", "-1", "--format=%s") != bdInitCommitSubject {
			t.Skip("this bd does not commit at init")
		}
		return dir
	}

	dir := setup()
	report, err := CleanImpact(dir, CleanOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if gittest.Run(t, dir, "log", "-1", "--format=%s") != bdInitCommitSubject {
		t.Fatalf("commit undone without UndoCommit (N17): %+v", report)
	}
	report, err = CleanImpact(dir, CleanOptions{UndoCommit: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := gittest.Run(t, dir, "log", "-1", "--format=%s"); got != "init" {
		t.Fatalf("HEAD = %q, report %+v", got, report)
	}
	if status := gittest.Run(t, dir, "status", "--porcelain"); status != "" {
		t.Fatalf("not zero impact:\n%q\n%+v", status, report)
	}

	pushed := setup()
	bare := t.TempDir()
	gittest.Run(t, bare, "init", "-q", "--bare")
	gittest.Run(t, pushed, "remote", "add", "origin", bare)
	gittest.Run(t, pushed, "push", "-q", "origin", "main")
	report, err = CleanImpact(pushed, CleanOptions{UndoCommit: true})
	if err != nil {
		t.Fatal(err)
	}
	if gittest.Run(t, pushed, "log", "-1", "--format=%s") != bdInitCommitSubject {
		t.Fatal("a pushed commit was undone")
	}
	if !strings.Contains(strings.Join(report.Skipped(), " "), "commit ") {
		t.Fatalf("pushed commit not reported: %+v", report)
	}
}
