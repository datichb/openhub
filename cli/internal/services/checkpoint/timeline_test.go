package checkpoint

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/domain"
	"github.com/datichb/openhub/cli/internal/sessionspec"
)

func TestTimeline(t *testing.T) {
	wf := &sessionspec.WorkflowRuntime{Checkpoints: []sessionspec.CheckpointDef{
		{ID: "cp-0", Behaviors: map[string]string{"manuel": "auto"}},
		{ID: "cp-1", Label: map[string]string{"": "Démarrer"}, Behaviors: map[string]string{"manuel": "pause"}},
		{ID: "cp-2", Behaviors: map[string]string{"manuel": "pause"}},
		{ID: "cp-3", Behaviors: map[string]string{"manuel": "skip"}},
		{ID: "cp-4", Behaviors: map[string]string{"manuel": "pause"}},
	}}
	at := time.Date(2026, 10, 6, 10, 3, 0, 0, time.Local)
	st := domain.CheckpointState{
		Passed:  map[string]time.Time{"cp-0": at, "cp-1": at},
		Waiting: "cp-2",
		Timeline: []domain.CheckpointEvent{
			{Kind: domain.CheckpointPassed, ID: "cp-0", At: at},
			{Kind: domain.CheckpointWaiting, ID: "cp-1"},
			{Kind: domain.CheckpointRefused, ID: "cp-1"},
			{Kind: domain.CheckpointWaiting, ID: "cp-1"},
			{Kind: domain.CheckpointPassed, ID: "cp-1", At: at},
			{Kind: domain.CheckpointDelegate, ID: "developer"},
			{Kind: domain.CheckpointDelegate, ID: "developer"},
			{Kind: domain.CheckpointDelegate, ID: "developer"},
			{Kind: domain.CheckpointDelegate, ID: "reviewer"},
			{Kind: domain.CheckpointWaiting, ID: "cp-2"},
		},
	}
	steps := Timeline(wf, "manuel", "fr", st)
	assert.Equal(t, "✔ cp-0 10:03 → ↺ cp-1 → ✔ cp-1 10:03 → developer (3) → reviewer → ⏸ cp-2 → ○ cp-4", TimelineText(steps, 0),
		"skipped checkpoint left out, steps to come at the end")
	assert.Equal(t, "Démarrer", steps[2].Label)
	assert.Equal(t, "… → ⏸ cp-2 → ○ cp-4", TimelineText(steps, 2))

	// The session no longer waits for cp-2 (answered elsewhere, refused):
	// its waiting step is a past round.
	st.Waiting = ""
	assert.Contains(t, TimelineText(Timeline(wf, "manuel", "fr", st), 0), "reviewer → ↺ cp-2 → ○ cp-2")
	assert.Nil(t, Timeline(nil, "", "", st))
}
