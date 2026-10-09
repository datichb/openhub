package views

import (
	"strings"
	"testing"
)

// S-05: text from the job shown in the TUI: control characters made
// visible, color tags of tview escaped.
func TestSafeText(t *testing.T) {
	got := safeLines([]string{"bd note bd-42 [red]x\x1b[2K", "ok"})
	if strings.Contains(got, "\x1b") || strings.Contains(got, "[red]") {
		t.Fatalf("not neutralized: %q", got)
	}
	if !strings.Contains(got, `\x1b[2K`) || !strings.Contains(got, "\nok") {
		t.Fatalf("content lost: %q", got)
	}
}
