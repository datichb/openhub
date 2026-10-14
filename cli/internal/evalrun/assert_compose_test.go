package evalrun

import (
	"testing"

	"github.com/stretchr/testify/assert"

	sessionsvc "github.com/datichb/openhub/cli/internal/services/session"
)

// R5: output and next assertions read the output events of the session and
// the next events added by the pass.
func TestCompositionAssertions(t *testing.T) {
	evs := []sessionsvc.TraceEvent{
		{Kind: sessionsvc.TraceOutput, Title: "tickets", Data: map[string]any{"value": []any{"ev-1", "ev-2"}}},
		{Kind: sessionsvc.TraceOutput, Title: "spec", Data: map[string]any{"value": "docs/wiki/specs/export.md"}},
		{Kind: sessionsvc.TraceNext, Title: "implement-spec", Text: "tickets=ev-1,ev-2 spec=docs/wiki/specs/export.md"},
	}
	for _, c := range []struct {
		a    Assertion
		pass bool
	}{
		{Assertion{Type: AssertOutput, Output: "tickets"}, true},
		{Assertion{Type: AssertOutput, Output: "tickets", Pattern: `^ev-1,ev-2$`}, true},
		{Assertion{Type: AssertOutput, Output: "tickets", Pattern: `ev-3`}, false},
		{Assertion{Type: AssertOutput, Output: "branch"}, false},
		{Assertion{Type: AssertOutput, Output: "branch", Absent: true}, true},
		{Assertion{Type: AssertOutput, Output: "spec", Pattern: `specs/`, Absent: true}, false},
		{Assertion{Type: AssertNext, Workflow: "implement-spec", Pattern: `tickets=ev-1,ev-2`}, true},
		{Assertion{Type: AssertNext, Workflow: "release"}, false},
		{Assertion{Type: AssertNext, Workflow: "release", Absent: true}, true},
	} {
		assert.NoError(t, c.a.Validate(), c.a.Label())
		pass, detail := check(c.a, Evidence{Events: evs})
		assert.Equal(t, c.pass, pass, "%s: %s", c.a.Label(), detail)
	}
	assert.Error(t, Assertion{Type: AssertOutput}.Validate())
	assert.Error(t, Assertion{Type: AssertNext}.Validate())
	assert.Error(t, Assertion{Type: AssertOutput, Output: "x", Pattern: "("}.Validate())
}
