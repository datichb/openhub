//go:build integration

package remote

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The real bd: claim, snapshot (dependencies, children, revisions), unclaim.
func TestRealBdSnapshot(t *testing.T) {
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not installed")
	}
	ctx := context.Background()
	dir := t.TempDir()
	run := func(args ...string) string {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%v: %s", args, out)
		return strings.TrimSpace(string(out))
	}
	run("git", "init", "-q")
	run("bd", "init", "--quiet", "--prefix", "t")
	parent := run("bd", "q", "Parent")
	blocker := run("bd", "q", "Blocker")
	child := run("bd", "q", "Child", "--parent", parent)
	run("bd", "dep", "add", parent, blocker)

	b := BdCLI{}
	require.NoError(t, b.Claim(ctx, dir, parent))
	snap, err := TakeSnapshot(ctx, b, dir, []string{parent}, time.Now())
	require.NoError(t, err)
	assert.Contains(t, snap.Issues, parent)
	assert.Contains(t, snap.Issues, blocker, "dependency")
	assert.Contains(t, snap.Issues, child, "child")
	assert.Equal(t, []string{child}, snap.Children[parent])
	assert.NotEmpty(t, snap.Revisions[parent])
	h, err := headOf(snap.Issues[parent])
	require.NoError(t, err)
	assert.Equal(t, "in_progress", h.Status)

	require.NoError(t, b.Unclaim(ctx, dir, parent))
	recs, err := b.Show(ctx, dir, parent)
	require.NoError(t, err)
	h, _ = headOf(recs[0])
	assert.Equal(t, "open", h.Status)
	assert.NotEqual(t, snap.Revisions[parent], h.version(), "the revision changes with the issue")
}
