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

	"github.com/datichb/openhub/cli/internal/beads/bdtest"
	"github.com/datichb/openhub/cli/internal/gateway"
)

// The real bd: claim, snapshot (dependencies, children, revisions), unclaim.
func TestRealBdSnapshot(t *testing.T) {
	bdtest.RequireRealBd(t)
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

// The replay runs the canonical form of the checked entries (bdargs): the
// real bd reads it as the journal meant it, and statuses are read for the
// closing check.
func TestRealBdReplaysCanonicalForm(t *testing.T) {
	bdtest.RequireRealBd(t)
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
	id := run("bd", "q", "Ticket")
	b := BdCLI{}
	canon := func(argv ...string) []string {
		c, why := parseEntry(argv)
		require.Empty(t, why, "%v", argv)
		return c.Argv()
	}
	_, err := b.Exec(ctx, dir, canon("update", id, "-sin_progress", "--notes", "from the runner"), nil)
	require.NoError(t, err)
	out, err := b.Exec(ctx, dir, canon("create", "Follow-up", "--json"), nil)
	require.NoError(t, err)
	created := createdID(out)
	require.NotEmpty(t, created)
	_, err = b.Exec(ctx, dir, canon("dep", "add", created, id), nil)
	require.NoError(t, err)
	_, err = b.Exec(ctx, dir, canon("note", id, "a", "note"), nil)
	require.NoError(t, err)
	recs, err := b.Show(ctx, dir, id)
	require.NoError(t, err)
	h, _ := headOf(recs[0])
	assert.Equal(t, "in_progress", h.Status)
	assert.Contains(t, string(recs[0]), "from the runner")

	st, err := b.Exec(ctx, dir, []string{"statuses", "--json"}, nil)
	require.NoError(t, err)
	assert.True(t, gateway.ClosingStatus(st, "no-such-status"), "unknown status counts as closing")
}
