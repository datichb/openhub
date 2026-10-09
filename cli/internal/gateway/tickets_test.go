package gateway

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/gateway/beadswire"
)

func TestTicketOp(t *testing.T) {
	allow := []string{"close", "done", "update", "show", "duplicate", "supersede", "import", "ready", "todo"}
	for _, tc := range []struct {
		argv []string
		want *BeadsOp
	}{
		{[]string{"close", "pt-1", "--reason", "Implemented in commit abc", "--suggest-next"}, &BeadsOp{Kind: OpClose, IDs: []string{"pt-1"}}},
		{[]string{"--json", "close", "pt-1", "pt-2", "-r=ok"}, &BeadsOp{Kind: OpClose, IDs: []string{"pt-1", "pt-2"}}},
		{[]string{"done", "pt-1", "Implemented in commit abc"}, &BeadsOp{Kind: OpClose, IDs: []string{"pt-1"}}},
		{[]string{"update", "pt-1", "-s", "closed"}, &BeadsOp{Kind: OpClose, IDs: []string{"pt-1"}}},
		{[]string{"update", "pt-1", "--status=done"}, &BeadsOp{Kind: OpClose, IDs: []string{"pt-1"}}},
		{[]string{"update", "pt-1", "--claim"}, &BeadsOp{Kind: OpClaim, IDs: []string{"pt-1"}}},
		{[]string{"update", "pt-1", "-s", "in_progress", "--notes", "x y"}, &BeadsOp{Kind: OpClaim, IDs: []string{"pt-1"}}},
		{[]string{"update", "pt-1", "-s", "blocked"}, nil},
		// S-07: attached and combined forms, closing commands.
		{[]string{"update", "pt-1", "-sclosed"}, &BeadsOp{Kind: OpClose, IDs: []string{"pt-1"}}},
		{[]string{"update", "pt-1", "-s=closed"}, &BeadsOp{Kind: OpClose, IDs: []string{"pt-1"}}},
		{[]string{"-q", "update", "-qsclosed", "pt-1"}, &BeadsOp{Kind: OpClose, IDs: []string{"pt-1"}}},
		{[]string{"update", "pt-1", "-s", "open", "--status", "closed"}, &BeadsOp{Kind: OpClose, IDs: []string{"pt-1"}}},
		{[]string{"-s", "closed", "update", "pt-1"}, &BeadsOp{Kind: OpClose, IDs: []string{"pt-1"}}},
		{[]string{"update", "pt-1", "-s", "shipped"}, &BeadsOp{Kind: OpClose, IDs: []string{"pt-1"}, Status: "shipped"}},
		{[]string{"duplicate", "pt-1", "--of", "pt-2"}, &BeadsOp{Kind: OpClose, IDs: []string{"pt-1"}}},
		{[]string{"supersede", "pt-1", "--with=pt-2"}, &BeadsOp{Kind: OpClose, IDs: []string{"pt-1"}}},
		{[]string{"import", "-i", "x.jsonl"}, &BeadsOp{Kind: OpClose}},
		{[]string{"todo", "done", "pt-3"}, &BeadsOp{Kind: OpClose, IDs: []string{"pt-3"}}},
		{[]string{"ready", "--claim"}, &BeadsOp{Kind: OpClaim}},
		{[]string{"close", "--help"}, nil},
		{[]string{"update", "pt-1", "--title", "closed"}, nil},
		{[]string{"show", "pt-1"}, nil},
	} {
		c, err := checkBeads(tc.argv, allow)
		require.NoError(t, err, tc.argv)
		op, ok := ticketOp(c)
		if tc.want == nil {
			assert.False(t, ok, "%v: %+v", tc.argv, op)
			continue
		}
		assert.True(t, ok, tc.argv)
		assert.Equal(t, *tc.want, op, tc.argv)
	}
}

// A17: the workflow guard refuses closing with its message; what ran is reported.
func TestBeadsWorkflowGuard(t *testing.T) {
	f := newFixture(t, []string{"close", "update", "show"})
	var done []BeadsOp
	f.b.Guard = func(_ context.Context, g Grant, op BeadsOp) error {
		assert.Equal(t, "ses_1", g.SessionID)
		if op.Kind == OpClose {
			return errors.New("closing refused by oh: cp-2")
		}
		return nil
	}
	f.b.Done = func(_ context.Context, _ Grant, op BeadsOp) { done = append(done, op) }

	code, resp := f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"close", "pt-1"}, Cwd: "/work/proj"})
	assert.Equal(t, http.StatusForbidden, code)
	assert.Equal(t, "closing refused by oh: cp-2", resp.Error)
	assert.Empty(t, resp.Stdout, "bd did not run")

	code, _ = f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"update", "pt-1", "--claim"}, Cwd: "/work/proj"})
	assert.Equal(t, http.StatusOK, code)
	code, _ = f.call(t, f.token, beadswire.ExecRequest{Argv: []string{"show", "boom"}, Cwd: "/work/proj"})
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, []BeadsOp{{Kind: OpClaim, IDs: []string{"pt-1"}}}, done, "only ticket operations that succeeded")
}

// A19: the Beads git hooks run whatever beads.allow says, only when git
// runs them; in another runtime they are not run on the machine.
func TestBeadsGitHooks(t *testing.T) {
	f := newFixture(t, []string{"show"})
	hook := func(gitHook bool, argv ...string) (int, beadswire.ExecResponse) {
		tok := ""
		if name := GitHookName(argv); name != "" {
			tok = f.b.Store.IssueHookToken(f.grant, name) // given by the daemon to a hook run by git
		}
		return f.call(t, f.token, beadswire.ExecRequest{Argv: argv, Cwd: "/work/proj", GitHook: gitHook, HookToken: tok})
	}
	code, resp := hook(true, "hooks", "run", "pre-commit")
	assert.Equal(t, http.StatusOK, code, resp.Error)
	assert.Empty(t, resp.Stdout, "container view: skipped, not run on the machine")

	f.b.View = nil // local session: same paths
	code, resp = hook(true, "hooks", "run", "prepare-commit-msg", ".git/COMMIT_EDITMSG", "message")
	require.Equal(t, http.StatusOK, code, resp.Error)
	assert.Contains(t, string(resp.Stdout), "argv:hooks run -- prepare-commit-msg .git/COMMIT_EDITMSG message")
	code, _ = hook(false, "hooks", "run", "pre-commit")
	assert.Equal(t, http.StatusForbidden, code, "not run by git: beads.allow applies")
	code, _ = hook(true, "hooks", "install")
	assert.Equal(t, http.StatusForbidden, code, "only hooks run")
	code, _ = hook(true, "hooks", "run", "post-rewrite")
	assert.Equal(t, http.StatusForbidden, code, "unknown hook")
	code, _ = hook(true, "close", "pt-1")
	assert.Equal(t, http.StatusForbidden, code, "the hook flag does not open other commands")
	code, _ = hook(true, "--db", "/x", "hooks", "run", "pre-commit")
	assert.Equal(t, http.StatusForbidden, code, "global flags still checked")
}
