package checkpoint

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// L3: passing a checkpoint the workflow gives to another agent is warned,
// never refused.
func TestByWarning(t *testing.T) {
	c := sessionspec.CheckpointDef{ID: "cp-2", By: []string{"orchestrator-dev"}}
	assert.Empty(t, byWarning("s", c, "orchestrator-dev"))
	assert.Empty(t, byWarning("s", c, ""), "unknown caller")
	assert.Empty(t, byWarning("s", sessionspec.CheckpointDef{ID: "cp-1"}, "developer"), "no by")
	w := byWarning("s", c, "developer")
	assert.Contains(t, w, "cp-2")
	assert.Contains(t, w, "developer")
	assert.Contains(t, w, "orchestrator-dev")
}
