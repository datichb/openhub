package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// r9b: a write that neither creates nor closes nor claims is a change of
// tickets, with the tickets named where bd expects them.
func TestUpdateOp(t *testing.T) {
	allow := []string{"create", "update", "label", "dep", "comments", "reopen", "duplicate", "show", "list", "find-duplicates", "remember"}
	for _, tc := range []struct {
		argv []string
		want bool
		ids  []string
	}{
		{[]string{"update", "ev-1", "ev-2", "-p", "1"}, true, []string{"ev-1", "ev-2"}},
		{[]string{"update", "ev-1", "--add-label", "needs-split"}, true, []string{"ev-1"}},
		{[]string{"label", "add", "ev-1", "ev-2", "needs-split"}, true, []string{"ev-1", "ev-2"}},
		{[]string{"dep", "add", "ev-3", "ev-1"}, true, []string{"ev-3", "ev-1"}},
		{[]string{"comments", "add", "ev-1", "doublon de ev-2"}, true, []string{"ev-1"}},
		{[]string{"reopen", "ev-4"}, true, []string{"ev-4"}},
		{[]string{"remember", "une note"}, true, nil},
		{[]string{"update", "ev-1", "-s", "closed"}, false, nil}, // closing
		{[]string{"update", "ev-1", "--claim"}, false, nil},      // claiming
		{[]string{"duplicate", "ev-2", "--of", "ev-1"}, false, nil},
		{[]string{"create", "Titre"}, false, nil},
		{[]string{"show", "ev-1"}, false, nil},
		{[]string{"find-duplicates"}, false, nil},
		{[]string{"comments", "ev-1"}, false, nil},
	} {
		c, err := checkBeads(tc.argv, allow)
		require.NoError(t, err, tc.argv)
		op, ok := updateOp(c)
		assert.Equal(t, tc.want, ok, tc.argv)
		if ok {
			assert.Equal(t, OpUpdate, op.Kind)
			assert.Equal(t, tc.ids, op.IDs, tc.argv)
		}
	}
}
