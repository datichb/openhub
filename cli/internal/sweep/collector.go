package sweep

import (
	"time"

	"github.com/datichb/openhub/cli/internal/parallel"
	"github.com/datichb/openhub/cli/internal/task"
)

// CollectResult holds the outcome of a single sweep sub-task.
type CollectResult struct {
	TaskID        string
	Label         string
	Status        string // "completed", "failed", "aborted"
	FilesModified []string
	FilesCreated  []string
	Duration      time.Duration
	Error         string
}

// MergeSummary aggregates the outcome of merging all sweep branches.
type MergeSummary struct {
	MergedCount   int
	FailedCount   int
	ConflictCount int
	SkippedCount  int
	TotalFiles    int
	Results       []parallel.MergeResult
}

// Collect maps completed parallel sessions to CollectResults.
func Collect(tasks []task.Task, state *parallel.ParallelState) []CollectResult {
	snap := state.Snapshot()

	// Build a task lookup for labels
	taskMap := make(map[string]task.Task, len(tasks))
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	results := make([]CollectResult, 0, len(snap.Sessions))
	for _, sess := range snap.Sessions {
		var duration time.Duration
		if !sess.StartedAt.IsZero() && !sess.CompletedAt.IsZero() {
			duration = sess.CompletedAt.Sub(sess.StartedAt)
		}

		label := sess.TicketID
		if t, ok := taskMap[sess.TicketID]; ok {
			label = t.DisplayName()
		}

		results = append(results, CollectResult{
			TaskID:        sess.TicketID,
			Label:         label,
			Status:        string(sess.Status),
			FilesModified: sess.FilesModified,
			FilesCreated:  sess.FilesCreated,
			Duration:      duration,
			Error:         sess.Error,
		})
	}

	return results
}

// MergeAll performs sequential merge of all completed sweep branches.
// It creates a Merger and invokes ProposeMerge with a lookup function
// derived from the original task list.
func MergeAll(
	tasks []task.Task,
	state *parallel.ParallelState,
	projectPath string,
	cfg parallel.Config,
) (*MergeSummary, error) {
	// Build a task lookup for IsMergeable
	taskMap := make(map[string]task.Task, len(tasks))
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	isMergeable := func(ticketID string) bool {
		if t, ok := taskMap[ticketID]; ok {
			return t.IsMergeable()
		}
		return false
	}

	merger := parallel.NewMerger(state, projectPath, cfg)
	results, err := merger.ProposeMerge(isMergeable)
	if err != nil {
		return nil, err
	}

	summary := &MergeSummary{
		Results: results,
	}

	for _, r := range results {
		if r.Success {
			if r.Conflict {
				summary.ConflictCount++
			} else if r.Message == "Merge skipped" {
				summary.SkippedCount++
			} else {
				summary.MergedCount++
			}
		} else {
			if r.Conflict {
				summary.ConflictCount++
			} else {
				summary.FailedCount++
			}
		}
	}

	// Count total files from collected sessions
	snap := state.Snapshot()
	fileSet := make(map[string]bool)
	for _, sess := range snap.Sessions {
		if sess.Status == parallel.StatusCompleted {
			for _, f := range sess.FilesModified {
				fileSet[f] = true
			}
			for _, f := range sess.FilesCreated {
				fileSet[f] = true
			}
		}
	}
	summary.TotalFiles = len(fileSet)

	return summary, nil
}
