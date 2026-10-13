package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/sessionspec"
)

// CP-R10 (R3): the launch shows the pauses planned in the mode.
func TestRecapPauses(t *testing.T) {
	wf := &sessionspec.WorkflowRuntime{ID: "ticket", Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-1", Behaviors: map[string]string{"manuel": "pause", "auto": "auto"}},
		{ID: "cp-2", Behaviors: map[string]string{"manuel": "pause", "auto": "pause"}, Mandatory: true},
		{ID: "cp-spec", Behaviors: map[string]string{"manuel": "conditional", "auto": "conditional"}},
	}}
	got := recapPauses(wf, "auto")
	assert.Contains(t, got, "cp-2")
	assert.Contains(t, got, "cp-spec")
	assert.NotContains(t, got, "cp-1")
	assert.Contains(t, got, "auto =", "auto keeps the mandatory decisions")
	assert.NotContains(t, recapPauses(wf, "manuel"), "auto =")
	assert.Empty(t, recapPauses(nil, "auto"))
	none := &sessionspec.WorkflowRuntime{Checkpoints: []sessionspec.CheckpointDef{{ID: "cp-1", Behaviors: map[string]string{"auto": "auto"}}}}
	assert.NotEmpty(t, recapPauses(none, "auto"))
}
