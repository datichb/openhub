package checkpoint

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// p6-debts: sweep-task launched by hand: the scope given as input is the
// scope of the session; a change outside it refuses its checkpoint.
func TestScopeInputOfASession(t *testing.T) {
	svc, ctx := newTestService(t)
	wf := &sessionspec.WorkflowRuntime{ID: "sweep-task", ScopeInput: "files",
		Checkpoints: []sessionspec.CheckpointDef{{ID: "cp-commit", Behaviors: map[string]string{"semi-auto": "pause"}}}}
	writeBundle(t, svc.BundlesDir, "h_scope", wf)
	sess := &domain.Session{ID: "ses_sc", ProjectID: "p1", Status: domain.SessionStatusRunning, GroupKey: "g1", BundleHash: "h_scope",
		Mode: "semi-auto", State: domain.RunActive, LaunchPath: t.TempDir(), Outputs: map[string]any{"files": "api/\ngo.sum, ./docs/x.md"}}
	require.NoError(t, svc.Sessions.Create(ctx, sess))
	svc.Changed = func(context.Context, string, string) ([]string, error) { return []string{"api/a.go", "docs/x.md"}, nil }
	r, ok := svc.scopeCheck(ctx, sess)
	assert.True(t, ok, "%v", r)
	svc.Changed = func(context.Context, string, string) ([]string, error) { return []string{"api/a.go", "web/x.ts"}, nil }
	r, ok = svc.scopeCheck(ctx, sess)
	assert.False(t, ok)
	assert.Contains(t, r.Reason, "web/x.ts")

	sess.Outputs = nil // no scope recorded: nothing to check
	_, ok = svc.scopeCheck(ctx, sess)
	assert.True(t, ok)
}

func TestCheckScope(t *testing.T) {
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	run("init", "-q")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "api", "a.go"), []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.sum"), []byte("x"), 0o600))
	ctx := context.Background()
	scope, problems := CheckScope(ctx, dir, "files", "api/\ngo.sum, api/new.go, api/*.go")
	assert.Empty(t, problems)
	assert.Equal(t, []string{"api/", "go.sum", "api/new.go", "api/*.go"}, scope)
	for _, bad := range []string{"../x", "/etc/passwd", ".git/config", "-rf", "web/x.ts", "nope/*.go", ""} {
		_, problems = CheckScope(ctx, dir, "files", bad)
		assert.NotEmpty(t, problems, bad)
	}
}
