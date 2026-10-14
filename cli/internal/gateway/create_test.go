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

// R5: the tickets a creation printed (bd 1.3 forms), recorded by oh as an
// output of the session.
func TestCreatedIDs(t *testing.T) {
	for out, want := range map[string][]string{
		"✓ Created issue: ev-3na — Hello\n  Priority: P2\n  Status: open\n\n💡 Tip: run 'bd setup claude'\n": {"ev-3na"},
		"✓ Created issue: ev-3na.1 — child\n":                                                           {"ev-3na.1"},
		"✓ Created 2 issues from plan.md:\n  ev-svh: Premier [P2, task]\n  ev-34m: Second [P2, task]\n": {"ev-svh", "ev-34m"},
		"ev-206\n":                       {"ev-206"},
		"ev-1\nev-2\n":                   {"ev-1", "ev-2"},
		`{"id": "ev-ecm", "title": "x"}`: {"ev-ecm"},
		`[{"id": "ev-kal"}, {"id": "ev-pel"}, {"id": "ev-kal"}]`:                                     {"ev-kal", "ev-pel"},
		"Created issue: pending-3 (recorded; the real id is assigned when the session is fetched)\n": nil,
		"  ev-1: not after a header\n":                                                               nil,
		"done\n":                                                                                     nil,
		"":                                                                                           nil,
		"{not json":                                                                                  nil,
	} {
		assert.Equal(t, want, CreatedIDs([]byte(out)), out)
	}
}
