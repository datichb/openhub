package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/datichb/openhub/cli/internal/domain"
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
	useLocale(t, "en")
	dir := beadsImpactProject(t)
	a := newMockApp(nil, nil)
	a.Projects = &mockProjectStore{projects: []domain.Project{{ID: "p1", Name: "web", Path: dir, Status: domain.ProjectStatusActive}}}
	prev := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = prev })
	stdinIsTerminal = func() bool { return false }

	cmd, out := doctorFixCmd()
	if err := runDoctorFix(cmd, a); err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("without terminal nor --yes: %v", err)
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

// Registering a project cleans it after a manual bd init.
func TestPrintBeadsCleanOnRegister(t *testing.T) {
	useLocale(t, "en")
	dir := beadsImpactProject(t)
	var out bytes.Buffer
	printBeadsClean(&out, dir)
	if !strings.Contains(out.String(), "was removed") || !strings.Contains(out.String(), "CLAUDE.md") {
		t.Fatalf("output:\n%s", out.String())
	}
	out.Reset()
	printBeadsClean(&out, t.TempDir())
	if out.Len() != 0 {
		t.Fatalf("project without .beads: %q", out.String())
	}
}
