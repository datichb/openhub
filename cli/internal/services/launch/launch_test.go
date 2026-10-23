package launch

import (
	"testing"
	"time"

	"github.com/datichb/openhub/cli/internal/workflow"
)

func TestSessionBranch(t *testing.T) {
	doc, diags := workflow.Parse([]byte("apiVersion: oh/v1\nkind: Workflow\nid: ticket\ninputs:\n  branch: { type: branch }\n"),
		workflow.Source{Layer: workflow.LayerHub})
	if diags.HasErrors() {
		t.Fatal(diags)
	}
	if got := SessionBranch(doc.Spec, map[string]any{"branch": "feat/bd-1"}, "bd-1", "x", time.Time{}); got != "feat/bd-1" {
		t.Fatalf("branch input: %q", got)
	}
	if got := SessionBranch(doc.Spec, map[string]any{}, "bd-1", "", time.Time{}); got != "oh/ticket-bd-1" {
		t.Fatalf("fallback: %q", got)
	}
	if got := SessionBranch(doc.Spec, map[string]any{}, "bd-1", "feat/w", time.Time{}); got != "feat/w" {
		t.Fatalf("explicit fallback: %q", got)
	}
	if got := SessionBranch(doc.Spec, nil, "", "", time.Date(2026, 10, 23, 14, 5, 9, 0, time.UTC)); got != "oh/ticket-20261023-140509" {
		t.Fatalf("timestamp fallback: %q", got)
	}
}
