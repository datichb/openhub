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

func TestCheckFanout(t *testing.T) {
	evs := []sessionsvc.TraceEvent{
		{Kind: sessionsvc.TraceFanout, Event: "merged", Title: "ev-1"},
		{Kind: sessionsvc.TraceFanout, Event: "merged", Title: "ev-2"},
		{Kind: sessionsvc.TraceFanout, Event: "child_checkpoint", Title: "ev-1", Checkpoint: "cp-2", By: "delegated"},
		{Kind: sessionsvc.TraceFanout, Event: "child_checkpoint", Title: "ev-2", Checkpoint: "cp-1", By: "mode"},
	}
	ok, _ := checkFanout(Assertion{Type: AssertFanout, Event: "merged", Count: 2}, evs)
	assert.True(t, ok)
	ok, detail := checkFanout(Assertion{Type: AssertFanout, Event: "merged", Count: 3}, evs)
	assert.False(t, ok)
	assert.Equal(t, "2 of 3", detail)
	ok, _ = checkFanout(Assertion{Type: AssertFanout, Event: "child_checkpoint", Checkpoint: "cp-2", By: "delegated", Count: 2}, evs)
	assert.False(t, ok)
	ok, _ = checkFanout(Assertion{Type: AssertFanout, Event: "conflict", Absent: true}, evs)
	assert.True(t, ok)
	assert.Error(t, Assertion{Type: AssertFanout}.Validate())
	assert.Equal(t, "fan-out merged ×2", Assertion{Type: AssertFanout, Event: "merged", Count: 2}.Label())
}
