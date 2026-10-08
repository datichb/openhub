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
	allow := []string{"close", "done", "update", "show"}
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
		{[]string{"update", "pt-1", "-s", "review"}, nil},
		{[]string{"update", "pt-1", "--title", "closed"}, nil},
		{[]string{"show", "pt-1"}, nil},
	} {
		c, err := checkBeads(tc.argv, allow)
		require.NoError(t, err, tc.argv)
		op, ok := ticketOp(tc.argv, c)
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
