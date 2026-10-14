package checkpoint

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// B1: an output `record: written` holds the files the session changed under
// its paths, read when a checkpoint passes; with `match:`, the first group
// of the expression in them (the verdict of a spike).
func TestRecordWritten(t *testing.T) {
	svc, ctx := newTestService(t)
	paths := []string{"docs/wiki/decisions/**"}
	wf := &sessionspec.WorkflowRuntime{ID: "spike", DefaultMode: "auto", Outputs: []sessionspec.OutputDef{
		{ID: "decision", Type: "path", Record: RecordWritten, Paths: paths},
		{ID: "verdict", Type: "text", Record: RecordWritten, Paths: paths, Match: `(?mi)^\*\*D[ée]cision\s*:\s*(go|clarify|kill)\b`},
		{ID: "branch", Type: "branch"},
	}, Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-question", Behaviors: map[string]string{"auto": "auto"}},
		{ID: "cp-decision", Behaviors: map[string]string{"auto": "auto"}},
	}}
	writeBundle(t, svc.BundlesDir, "h_spike", wf)
	dir := t.TempDir()
	require.NoError(t, svc.Sessions.Create(ctx, &domain.Session{ID: "ses_k", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1",
		BundleHash: "h_spike", Mode: "auto", State: domain.RunActive, LaunchPath: dir, StartRef: "c0"}))
	var changed []string
	svc.Changed = func(context.Context, string, string) ([]string, error) { return changed, nil }
	write := func(rel, s string) {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(s), 0o644))
	}
	outputs := func() map[string]any {
		sess, err := svc.Sessions.Get(ctx, "ses_k")
		require.NoError(t, err)
		return sess.Outputs
	}

	changed = []string{"src/proto.go"}
	_, err := svc.Reached(ctx, "ses_k", Call{ID: "cp-question"})
	require.NoError(t, err)
	assert.Empty(t, outputs(), "nothing written under the paths")

	// A deleted file is not written; the decision is read once it exists.
	changed = []string{"docs/wiki/decisions/gone.md", "docs/wiki/decisions/cache.md", "src/proto.go"}
	write("docs/wiki/decisions/cache.md", "# Spike — cache\n\n**Décision : Go**\n\n## Suites\n")
	_, err = svc.Reached(ctx, "ses_k", Call{ID: "cp-decision"})
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"decision": "docs/wiki/decisions/cache.md", "verdict": "Go"}, outputs())

	// A correction changes the verdict: recorded again at the next pass.
	write("docs/wiki/decisions/cache.md", "# Spike — cache\n\n**Decision : kill**\n")
	_, err = svc.Reached(ctx, "ses_k", Call{ID: "cp-decision"})
	require.NoError(t, err)
	assert.Equal(t, "kill", outputs()["verdict"])

	// No line that matches: the verdict stays as recorded.
	write("docs/wiki/decisions/cache.md", "# Spike — cache\n\nPas encore tranché.\n")
	_, err = svc.Reached(ctx, "ses_k", Call{ID: "cp-decision"})
	require.NoError(t, err)
	assert.Equal(t, "kill", outputs()["verdict"])
}

func TestMatchIn(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("rien"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.md"), []byte("verdict: clarify"), 0o644))
	assert.Equal(t, "clarify", matchIn(dir, []string{"a.md", "b.md"}, `verdict: (\w+)`))
	assert.Empty(t, matchIn(dir, []string{"a.md", "b.md"}, `verdict`), "no group")
	assert.Empty(t, matchIn(dir, []string{"a.md", "b.md"}, `(`), "invalid")
	assert.Empty(t, matchIn(dir, []string{"missing.md"}, `(\w+)`))
}
