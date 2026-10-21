package beads

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/beads/bdtest"
)

func TestQueryExpr(t *testing.T) {
	expr, err := QueryExpr(QueryTerm{"label", "=", "ready-for-review"}, QueryTerm{"status", "!=", "closed"}, QueryTerm{"updated", ">", "7d"})
	require.NoError(t, err)
	assert.Equal(t, `label="ready-for-review" AND status!="closed" AND updated>"7d"`, expr)

	expr, err = QueryExpr(QueryTerm{"title", "=", `x" OR status="closed`}, QueryTerm{"label", "=", `a\b`})
	require.NoError(t, err)
	assert.Equal(t, `title="x\" OR status=\"closed" AND label="a\\b"`, expr, "a value never changes the expression")

	_, err = QueryExpr(QueryTerm{"secret", "=", "x"})
	assert.Error(t, err)
	_, err = QueryExpr(QueryTerm{"title", "~", "x"})
	assert.Error(t, err)
	_, err = QueryExpr()
	assert.Error(t, err)
}

// patientBd gives bd the time it takes on a loaded machine: the contract
// tests check what bd answers, not how fast (the embedded Dolt of bd may
// take more than the 10 s of oh when several suites run at once).
func patientBd(t *testing.T) {
	t.Helper()
	prev := bdTimeout
	bdTimeout = 2 * time.Minute
	t.Cleanup(func() { bdTimeout = prev })
}

// Contract with the real bd: a quoted value is read as one value.
func TestQueryRealBd(t *testing.T) {
	bdtest.RequireRealBd(t)
	patientBd(t)
	dir := bdtest.InitedRepo(t)
	run := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "BEADS_DIR=")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "%v: %s", args, out)
	}
	run("bd", "create", "CSV export", "-l", "ready-for-review")
	run("bd", "create", "x OR status=open")
	t.Setenv("BEADS_DIR", "")
	expr, err := QueryExpr(QueryTerm{"label", "=", "ready-for-review"})
	require.NoError(t, err)
	got, err := Query(dir, expr, false)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "CSV export", got[0].Title)

	expr, _ = QueryExpr(QueryTerm{"title", "=", "x OR status=open"})
	got, err = Query(dir, expr, false)
	require.NoError(t, err)
	require.Len(t, got, 1, "the value is not an expression")
}

// bd lists 50 tickets (ready: 100) by default: oh asks for all of them.
func TestListsAreNotTruncated(t *testing.T) {
	patientBd(t)
	dir := t.TempDir()
	log := filepath.Join(dir, "args")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bd"), []byte("#!/bin/sh\necho \"$*\" >> "+log+"\necho '[]'\n"), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	_, err := ListAll("/p")
	require.NoError(t, err)
	_, err = ListReady("/p", ReadyOpts{Label: "ai-delegated"})
	require.NoError(t, err)
	_, err = Query("/p", `label="x"`, true)
	require.NoError(t, err)
	data, err := os.ReadFile(log)
	require.NoError(t, err)
	assert.Equal(t, "-C /p list --json --flat -n 0\n-C /p ready --label ai-delegated --json -n 0\n-C /p --readonly query label=\"x\" --json -n 0 --all\n", string(data))
}

// Contract with the real bd: lint names the missing sections, and exits 1
// with a JSON answer.
func TestLintRealBd(t *testing.T) {
	bdtest.RequireRealBd(t)
	patientBd(t)
	dir := bdtest.InitedRepo(t)
	out := func(args ...string) string {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "BEADS_DIR=")
		b, err := cmd.Output()
		require.NoError(t, err, "%v", args)
		return strings.TrimSpace(string(b))
	}
	bare := out("bd", "q", "bare", "-t", "task")
	done := out("bd", "q", "done", "-t", "task")
	out("bd", "update", done, "--acceptance", "- works", "-q")
	bug := out("bd", "q", "bug", "-t", "bug")
	out("bd", "update", bug, "--acceptance", "- fixed", "-d", "## Étapes de reproduction (Steps to Reproduce)\n1. x", "-q")
	t.Setenv("BEADS_DIR", "")

	res, err := Lint(dir, bare, done, bug)
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, bare, res[0].ID)
	assert.Equal(t, []string{"## Acceptance Criteria"}, res[0].Missing)

	res, err = Lint(dir, done)
	require.NoError(t, err)
	assert.Empty(t, res)
}

// Contract with the real bd: follow-up of done work and closable epics.
func TestTrackingRealBd(t *testing.T) {
	bdtest.RequireRealBd(t)
	patientBd(t)
	dir := bdtest.InitedRepo(t)
	out := func(args ...string) string {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "BEADS_DIR=")
		b, err := cmd.Output()
		require.NoError(t, err, "%v", args)
		return strings.TrimSpace(string(b))
	}
	done := out("bd", "q", "done")
	out("git", "-c", "user.name=A", "-c", "user.email=a@b", "-c", "commit.gpgsign=false", "commit", "-q", "--allow-empty", "-m", "feat(x): done ("+done+")")
	epic := out("bd", "q", "epic", "-t", "epic")
	child := out("bd", "q", "child", "--parent", epic)
	out("bd", "close", child, "-q")
	t.Setenv("BEADS_DIR", "")

	orphans, err := Orphans(dir)
	require.NoError(t, err)
	require.Len(t, orphans, 1)
	assert.Equal(t, done, orphans[0].ID)

	epics, err := EpicStatus(dir, true)
	require.NoError(t, err)
	require.Len(t, epics, 1)
	assert.Equal(t, epic, epics[0].Epic.ID)
	assert.True(t, epics[0].Eligible)

	stale, err := Stale(dir, "in_progress", 1)
	require.NoError(t, err)
	assert.Empty(t, stale)

	m, err := Memories(dir)
	require.NoError(t, err)
	assert.Empty(t, m)
	out("bd", "remember", "Les tests e2e tournent avec make e2e")
	out("bd", "remember", "Ne pas toucher\nau module billing")
	m, err = Memories(dir)
	require.NoError(t, err)
	assert.Equal(t, "- Les tests e2e tournent avec make e2e\n- Ne pas toucher au module billing", m)
}

// A bd that does not answer in time: the error says so.
func TestQueryTimeout(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bd"), []byte("#!/bin/sh\nexec sleep 5\n"), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	prev := bdTimeout
	bdTimeout = 100 * time.Millisecond
	t.Cleanup(func() { bdTimeout = prev })
	_, err := Query("/p", `label="x"`, false)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.ErrorContains(t, err, `bd --readonly query label="x"`)
}
