package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// N9: ticket creation is reported to the workflow guard.
func TestCreateOp(t *testing.T) {
	allow := []string{"create", "create-form", "q", "show", "list"}
	for _, tc := range []struct {
		argv []string
		want bool
	}{
		{[]string{"create", "Titre", "-t", "task"}, true},
		{[]string{"--json", "create", "--title", "x"}, true},
		{[]string{"q", "Titre"}, true},
		{[]string{"create-form"}, true},
		{[]string{"list", "--created-after", "2026-01-01"}, false},
		{[]string{"show", "pt-1"}, false},
	} {
		c, err := checkBeads(tc.argv, allow)
		require.NoError(t, err, tc.argv)
		op, ok := createOp(c)
		assert.Equal(t, tc.want, ok, tc.argv)
		if ok {
			assert.Equal(t, OpCreate, op.Kind)
		}
	}
}
