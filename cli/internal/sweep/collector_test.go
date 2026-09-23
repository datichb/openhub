package sweep

import (
	"testing"
	"time"

	"github.com/datichb/openhub/cli/internal/parallel"
	"github.com/datichb/openhub/cli/internal/task"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollect(t *testing.T) {
	tasks := []task.Task{
		{ID: "sweep-auth", Kind: task.KindSweep, Label: "Auth"},
		{ID: "sweep-api", Kind: task.KindSweep, Label: "API"},
		{ID: "sweep-config", Kind: task.KindSweep, Label: "Config"},
	}

	state := parallel.NewState("/tmp/project", 5)
	now := time.Now().UTC()
	state.AddSession(parallel.SessionInfo{
		TicketID:      "sweep-auth",
		Status:        parallel.StatusCompleted,
		StartedAt:     now.Add(-10 * time.Minute),
		CompletedAt:   now,
		FilesModified: []string{"auth.go", "auth_test.go"},
		FilesCreated:  []string{"auth_mock.go"},
	})
	state.AddSession(parallel.SessionInfo{
		TicketID:    "sweep-api",
		Status:      parallel.StatusFailed,
		StartedAt:   now.Add(-5 * time.Minute),
		CompletedAt: now,
		Error:       "build failed",
	})
	state.AddSession(parallel.SessionInfo{
		TicketID:  "sweep-config",
		Status:    parallel.StatusCompleted,
		StartedAt: now.Add(-3 * time.Minute),
		CompletedAt: now,
		FilesModified: []string{"config.go"},
	})

	results := Collect(tasks, state)
	require.Len(t, results, 3)

	// Check first result
	assert.Equal(t, "sweep-auth", results[0].TaskID)
	assert.Equal(t, "Auth", results[0].Label)
	assert.Equal(t, "completed", results[0].Status)
	assert.Len(t, results[0].FilesModified, 2)
	assert.Len(t, results[0].FilesCreated, 1)
	assert.InDelta(t, 10*time.Minute, results[0].Duration, float64(time.Second))
	assert.Empty(t, results[0].Error)

	// Check failed result
	assert.Equal(t, "sweep-api", results[1].TaskID)
	assert.Equal(t, "failed", results[1].Status)
	assert.Equal(t, "build failed", results[1].Error)

	// Check third result
	assert.Equal(t, "sweep-config", results[2].TaskID)
	assert.Equal(t, "completed", results[2].Status)
}

func TestCollect_NoSessions(t *testing.T) {
	tasks := []task.Task{
		{ID: "sweep-auth", Kind: task.KindSweep},
	}
	state := parallel.NewState("/tmp/project", 5)

	results := Collect(tasks, state)
	assert.Empty(t, results)
}

func TestCollect_FallbackLabel(t *testing.T) {
	// Task not found in lookup — should fallback to TicketID
	tasks := []task.Task{} // empty task list
	state := parallel.NewState("/tmp/project", 5)
	state.AddSession(parallel.SessionInfo{
		TicketID: "sweep-orphan",
		Status:   parallel.StatusCompleted,
	})

	results := Collect(tasks, state)
	require.Len(t, results, 1)
	assert.Equal(t, "sweep-orphan", results[0].Label) // fallback
}

func TestCollect_FilesAggregation(t *testing.T) {
	tasks := []task.Task{
		{ID: "t1", Kind: task.KindSweep},
	}
	state := parallel.NewState("/tmp/project", 5)
	state.AddSession(parallel.SessionInfo{
		TicketID:      "t1",
		Status:        parallel.StatusCompleted,
		FilesModified: []string{"a.go", "b.go", "c.go"},
		FilesCreated:  []string{"d.go"},
	})

	results := Collect(tasks, state)
	require.Len(t, results, 1)
	assert.Len(t, results[0].FilesModified, 3)
	assert.Len(t, results[0].FilesCreated, 1)
}
