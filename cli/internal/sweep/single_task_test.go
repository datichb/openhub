package sweep

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/parallel"
	"github.com/datichb/openhub/cli/internal/task"
)

func TestGitDiffFiles_EmptyDir(t *testing.T) {
	// Non-git directory returns empty slices.
	modified, created := gitDiffFiles(t.TempDir())
	assert.Empty(t, modified)
	assert.Empty(t, created)
}

func TestSyntheticState_CompatibleWithCollect(t *testing.T) {
	// Verify that a synthetic ParallelState produced by the single-task path
	// is correctly consumed by the Collect function.
	state := parallel.NewState("", 1)
	state.AddSession(parallel.SessionInfo{
		TicketID:      "task-1",
		Branch:        "sweep/task-1",
		WorktreePath:  "/tmp/wt",
		Status:        parallel.StatusCompleted,
		FilesModified: []string{"a.go", "b.go"},
		FilesCreated:  []string{"c.go"},
	})

	tasks := []task.Task{
		{ID: "task-1", Description: "Fix the bug"},
	}

	results := Collect(tasks, state)
	assert.Len(t, results, 1)
	assert.Equal(t, "task-1", results[0].TaskID)
	assert.Equal(t, string(parallel.StatusCompleted), results[0].Status)
	assert.Equal(t, []string{"a.go", "b.go"}, results[0].FilesModified)
	assert.Equal(t, []string{"c.go"}, results[0].FilesCreated)
}

func TestSyntheticState_FailedTask(t *testing.T) {
	state := parallel.NewState("", 1)
	state.AddSession(parallel.SessionInfo{
		TicketID: "task-1",
		Status:   parallel.StatusFailed,
		Error:    "headless run failed: context canceled",
	})

	tasks := []task.Task{
		{ID: "task-1", Description: "Task that failed"},
	}

	results := Collect(tasks, state)
	assert.Len(t, results, 1)
	assert.Equal(t, string(parallel.StatusFailed), results[0].Status)
	assert.Equal(t, "headless run failed: context canceled", results[0].Error)
}
