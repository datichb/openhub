package beads

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A5 follow-up: the files of a `bd init` without --skip-agents (bd 1.3) are
// reported, CLAUDE.md and .cursor/ included; a user's own tool folder is not.
func TestDiagnoseBeadsImpactBd13AgentFiles(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".beads/config.yaml", "")
	write(".git/info/exclude", ".beads/\n")
	write("CLAUDE.md", "# Project Instructions\n\n<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal -->\n## Tracker\n<!-- END BEADS INTEGRATION -->\n")
	write(".cursor/hooks.json", `{"hooks":{"postToolUse":[{"command":"bd cursor-hook postToolUse"}]}}`)
	write(".claude/settings.json", `{"permissions":{"allow":["Read"]}}`)
	write(".gitignore", "node_modules/\n\n# Beads / Dolt files (added by bd init)\n.dolt/\n")

	var got []string
	for _, i := range DiagnoseBeadsImpact(dir) {
		got = append(got, i.Kind+": "+i.Detail)
	}
	all := strings.Join(got, "\n")
	for _, want := range []string{"CLAUDE.md", ".cursor/", "added by bd init"} {
		if !strings.Contains(all, want) {
			t.Errorf("%s not reported:\n%s", want, all)
		}
	}
	if strings.Contains(all, ".claude/") {
		t.Errorf("a .claude/ folder without bd content is reported:\n%s", all)
	}

	// SanitizeBeadsInit (run after oh's own bd init) never deletes them.
	_ = SanitizeBeadsInit(dir)
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); err != nil {
		t.Fatal("CLAUDE.md deleted by the sanitizer")
	}
}
