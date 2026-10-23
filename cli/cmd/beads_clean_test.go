package cmd

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/i18n/i18ntest"
)

func beadsImpactProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range map[string]string{
		".git/info/exclude":  "",
		".beads/config.yaml": "",
		"CLAUDE.md":          "# Mon projet\n\n<!-- BEGIN BEADS INTEGRATION v:1 -->\nbd ready\n<!-- END BEADS INTEGRATION -->\n",
		".cursor/hooks.json": `{"hooks":{"sessionStart":[{"command":"bd cursor-hook sessionStart"}]},"version":1}`,
	} {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func doctorFixCmd(args ...string) (*cobra.Command, *bytes.Buffer) {
	c := &cobra.Command{}
	c.Flags().Bool("yes", false, "")
	_ = c.ParseFlags(args)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&out)
	c.SetContext(context.Background())
	return c, &out
}

// A5 follow-up: oh doctor --fix lists the clean-up, needs --yes without
// terminal, then takes back out what bd init wrote (the user's text stays).
func TestDoctorFixBeads(t *testing.T) {
	i18ntest.Set(t, "en")
	dir := beadsImpactProject(t)
	a := newMockApp(nil, nil)
	a.Projects = &mockProjectStore{projects: []domain.Project{{ID: "p1", Name: "web", Path: dir, Status: domain.ProjectStatusActive}}}
	prev := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = prev })
	stdinIsTerminal = func() bool { return false }

	cmd, out := doctorFixCmd()
	if err := runDoctorFix(cmd, a); err == nil || !strings.Contains(err.Error(), "--yes") || ExitCode(err) != ExitUsage {
		t.Fatalf("without terminal nor --yes: %v (code %d)", err, ExitCode(err))
	}
	if !strings.Contains(out.String(), "CLAUDE.md") || !strings.Contains(out.String(), ".cursor/hooks.json") {
		t.Fatalf("plan:\n%s", out.String())
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md")); !strings.Contains(string(data), "BEADS") {
		t.Fatal("changed without confirmation")
	}

	cmd, _ = doctorFixCmd("--yes")
	if err := runDoctorFix(cmd, a); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md")); string(data) != "# Mon projet\n" {
		t.Fatalf("CLAUDE.md = %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cursor")); err == nil {
		t.Fatal(".cursor/ left")
	}
	cmd, out = doctorFixCmd("--yes")
	if err := runDoctorFix(cmd, a); err != nil || !strings.Contains(out.String(), "nothing to repair") {
		t.Fatalf("second run: %v\n%s", err, out.String())
	}
}

// N17: registering a project lists what bd init wrote; without a terminal
// nothing is changed (oh doctor --fix is named).
func TestPrintBeadsCleanWithoutTerminal(t *testing.T) {
	i18ntest.Set(t, "en")
	dir := beadsImpactProject(t)
	withTerminal(t, false)
	asked := withConfirm(t, true)
	var out bytes.Buffer
	printBeadsClean(&out, newMockApp(nil, nil), dir, false)
	if !strings.Contains(out.String(), "CLAUDE.md") || !strings.Contains(out.String(), "oh doctor --fix") {
		t.Fatalf("output:\n%s", out.String())
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md")); !strings.Contains(string(data), "BEADS") {
		t.Fatal("changed without confirmation")
	}
	if *asked != 0 {
		t.Fatal("asked without terminal")
	}
	out.Reset()
	printBeadsClean(&out, newMockApp(nil, nil), t.TempDir(), false)
	if out.Len() != 0 {
		t.Fatalf("project without .beads: %q", out.String())
	}
}

// N17: with a terminal, declined → nothing; accepted → cleaned.
func TestPrintBeadsCleanConfirmed(t *testing.T) {
	i18ntest.Set(t, "en")
	dir := beadsImpactProject(t)
	withTerminal(t, true)
	withConfirm(t, false)
	var out bytes.Buffer
	printBeadsClean(&out, newMockApp(nil, nil), dir, false)
	if data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md")); !strings.Contains(string(data), "BEADS") {
		t.Fatal("changed although declined")
	}
	withConfirm(t, true)
	out.Reset()
	printBeadsClean(&out, newMockApp(nil, nil), dir, false)
	if data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md")); string(data) != "# Mon projet\n" {
		t.Fatalf("CLAUDE.md = %q\n%s", data, out.String())
	}
}

// N17: [beads] zero_impact = off → nothing; auto → applied without asking.
func TestPrintBeadsCleanModes(t *testing.T) {
	i18ntest.Set(t, "en")
	withTerminal(t, true)
	asked := withConfirm(t, false)
	a := newMockApp(nil, nil)
	a.Config.Beads.ZeroImpact = zeroImpactOff
	dir := beadsImpactProject(t)
	var out bytes.Buffer
	printBeadsClean(&out, a, dir, false)
	if out.Len() != 0 {
		t.Fatalf("off: %q", out.String())
	}
	a.Config.Beads.ZeroImpact = zeroImpactAuto
	printBeadsClean(&out, a, dir, false)
	if data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md")); string(data) != "# Mon projet\n" {
		t.Fatalf("auto: CLAUDE.md = %q", data)
	}
	if *asked != 0 {
		t.Fatal("auto asked")
	}
}

var bdInitTemplates templates

func bdInitTemplate(t *testing.T, git func(dir string, args ...string) string) string {
	t.Helper()
	return bdInitTemplates.get(t, "bdinit", func(dir string) {
		git(dir, "init", "-q", "-b", "main")
		git(dir, "commit", "-q", "--allow-empty", "-m", "init")
		if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
			t.Fatal(err)
		}
		_ = os.WriteFile(filepath.Join(dir, ".beads", "config.yaml"), nil, 0o644)
		_ = os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("<!-- BEGIN BEADS INTEGRATION v:1 -->\nbd\n<!-- END BEADS INTEGRATION -->\n"), 0o644)
		git(dir, "add", ".")
		git(dir, "commit", "-qm", "bd init: initialize beads issue tracking")
	})
}

// bdInitCommitRepo is a repository whose last commit is the one of bd init
// (not the first one, not pushed).
func bdInitCommitRepo(t *testing.T) (dir string, git func(dir string, args ...string) string) {
	t.Helper()
	git = func(dir string, args ...string) string {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, err := c.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	dir = t.TempDir()
	copyTree(t, bdInitTemplate(t, git), dir)
	return dir, git
}

const bdInitSubject = "bd init: initialize beads issue tracking"

// N17 (finitions): the commit of bd init is undone once listed and
// confirmed in a terminal, or with --yes; never without a terminal nor
// --yes, never by the automatic mode, never when pushed.
func TestPrintBeadsCleanCommit(t *testing.T) {
	i18ntest.Set(t, "en")
	a := newMockApp(nil, nil)
	var out bytes.Buffer

	// Terminal, confirmed: listed, then undone.
	withTerminal(t, true)
	asked := withConfirm(t, true)
	dir, git := bdInitCommitRepo(t)
	printBeadsClean(&out, a, dir, false)
	if got := git(dir, "log", "-1", "--format=%s"); got != "init" {
		t.Fatalf("confirmed: HEAD = %q\n%s", got, out.String())
	}
	if *asked != 1 || !strings.Contains(out.String(), "undone if not pushed") {
		t.Fatalf("the undo was not listed before the confirmation:\n%s", out.String())
	}

	// Terminal, declined: kept.
	withConfirm(t, false)
	dir, git = bdInitCommitRepo(t)
	printBeadsClean(&out, a, dir, false)
	if git(dir, "log", "-1", "--format=%s") != bdInitSubject {
		t.Fatal("commit undone although declined")
	}

	// Without a terminal nor --yes: kept, the hint names the ways.
	withTerminal(t, false)
	withConfirm(t, true)
	dir, git = bdInitCommitRepo(t)
	out.Reset()
	printBeadsClean(&out, a, dir, false)
	if git(dir, "log", "-1", "--format=%s") != bdInitSubject {
		t.Fatal("commit undone without terminal nor --yes")
	}
	if !strings.Contains(out.String(), "--yes") {
		t.Fatalf("no hint:\n%s", out.String())
	}

	// --yes without a terminal: undone.
	dir, git = bdInitCommitRepo(t)
	printBeadsClean(&out, a, dir, true)
	if got := git(dir, "log", "-1", "--format=%s"); got != "init" {
		t.Fatalf("--yes: HEAD = %q", got)
	}

	// Automatic mode, terminal: files cleaned, commit kept (nobody asked).
	withTerminal(t, true)
	auto := newMockApp(nil, nil)
	auto.Config.Beads.ZeroImpact = zeroImpactAuto
	dir, git = bdInitCommitRepo(t)
	printBeadsClean(&out, auto, dir, false)
	if git(dir, "log", "-1", "--format=%s") != bdInitSubject {
		t.Fatal("commit undone by the automatic mode")
	}

	// Pushed: never.
	dir, git = bdInitCommitRepo(t)
	bare := t.TempDir()
	git(bare, "init", "-q", "--bare")
	git(dir, "remote", "add", "origin", bare)
	git(dir, "push", "-q", "origin", "main")
	printBeadsClean(&out, a, dir, true)
	if git(dir, "log", "-1", "--format=%s") != bdInitSubject {
		t.Fatal("pushed commit undone")
	}
	dir, git = bdInitCommitRepo(t)
	git(bare, "init", "-q", "--bare")
	git(dir, "remote", "add", "origin", bare)
	git(dir, "push", "-q", "-f", "origin", "main")
	git(dir, "update-ref", "refs/remotes/origin/main", "HEAD~1") // stale ref: the plan cannot tell
	printBeadsClean(&out, a, dir, false)
	if git(dir, "log", "-1", "--format=%s") != bdInitSubject {
		t.Fatal("pushed commit undone after a confirmation (remotes not fetched?)")
	}
}

// N17 (finitions): oh doctor --fix undoes the commit of bd init once
// confirmed in a terminal; without a terminal nothing happens without --yes.
func TestDoctorFixUndoesCommitOnceConfirmed(t *testing.T) {
	i18ntest.Set(t, "en")
	dir, git := bdInitCommitRepo(t)
	a := newMockApp(nil, nil)
	a.Projects = &mockProjectStore{projects: []domain.Project{{ID: "p1", Name: "web", Path: dir, Status: domain.ProjectStatusActive}}}

	withTerminal(t, false)
	cmd, _ := doctorFixCmd()
	if err := runDoctorFix(cmd, a); err == nil || ExitCode(err) != ExitUsage {
		t.Fatalf("without terminal nor --yes: applied (%v)", err)
	}
	if git(dir, "log", "-1", "--format=%s") != bdInitSubject {
		t.Fatal("commit undone without terminal nor --yes")
	}

	withTerminal(t, true)
	stubConfirmForm(t, false)
	cmd, _ = doctorFixCmd()
	if err := runDoctorFix(cmd, a); err != nil {
		t.Fatal(err)
	}
	if git(dir, "log", "-1", "--format=%s") != bdInitSubject {
		t.Fatal("commit undone although declined")
	}

	asked := stubConfirmForm(t, true)
	cmd, out := doctorFixCmd()
	if err := runDoctorFix(cmd, a); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "undone if not pushed") || len(asked.questions) != 1 {
		t.Fatalf("the undo was not listed before the confirmation:\n%s", out.String())
	}
	if got := git(dir, "log", "-1", "--format=%s"); got != "init" {
		t.Fatalf("confirmed: HEAD = %q\n%s", got, out.String())
	}
}
