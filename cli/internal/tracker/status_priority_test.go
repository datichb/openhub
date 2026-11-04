package tracker_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// An issue with several mapped labels is placed by the order of the columns of
// the board, not by the order of its labels: "Blocked" and "Ready to dev" in
// either order give the same column (the one that comes first: todo).
func TestMapTrackerStatusPriorityIsTheOrderOfTheColumns(t *testing.T) {
	mapping := map[string]string{"Blocked": "blocked", "Ready to dev": "planned", "Dev doing": "in_progress", "Testing": "validation"}
	board := teamstate.BoardConfig{}
	for _, labels := range [][]string{
		{"Blocked", "Ready to dev"},
		{"Ready to dev", "Blocked"},
		{"blocked", "READY TO DEV", "bug"},
	} {
		got := tracker.MapTrackerStatusWithBoard(&tracker.IssueState{Labels: labels}, nil, mapping, board)
		assert.Equal(t, "planned", got, "%v", labels)
	}
	// Further in the board: validation before blocked, in progress before validation.
	got := tracker.MapTrackerStatusWithBoard(&tracker.IssueState{Labels: []string{"Blocked", "Testing"}}, nil, mapping, board)
	assert.Equal(t, "validation", got)
	got = tracker.MapTrackerStatusWithBoard(&tracker.IssueState{Labels: []string{"Testing", "Dev doing"}}, nil, mapping, board)
	assert.Equal(t, "in_progress", got)
}

// The order of the columns is the one the team gave its board.
func TestMapTrackerStatusPriorityFollowsTheCustomColumns(t *testing.T) {
	board := teamstate.BoardConfig{Columns: []teamstate.BoardColumnConfig{
		{ID: "stuck", Name: "STUCK", Role: teamstate.ColumnRoleBlocked},
		{ID: "backlog", Name: "BACKLOG", Role: teamstate.ColumnRoleInitial},
		{ID: "doing", Name: "DOING", Role: teamstate.ColumnRoleActive},
		{ID: "done", Name: "DONE", Role: teamstate.ColumnRoleTerminal},
	}}
	mapping := map[string]string{"Blocked": "stuck", "Ready": "backlog", "Doing": "doing"}
	got := tracker.MapTrackerStatusWithBoard(&tracker.IssueState{Labels: []string{"Doing", "Ready", "Blocked"}}, nil, mapping, board)
	assert.Equal(t, "stuck", got, "the team put its blocked column first")

	// planned is the initial column: two labels for the same column tie, and
	// the alphabetical order of the labels settles it.
	mapping = map[string]string{"B label": "backlog", "A label": "planned"}
	got = tracker.MapTrackerStatusWithBoard(&tracker.IssueState{Labels: []string{"B label", "A label"}}, nil, mapping, board)
	assert.Equal(t, "planned", got)
}

// The answer does not depend on the iteration order of the Go map: run the
// same issue many times.
func TestMapTrackerStatusPriorityIsStable(t *testing.T) {
	mapping := map[string]string{}
	for _, l := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		mapping[l] = "in_progress"
		mapping[l+"-x"] = "review"
	}
	mapping["Dup"] = "review"
	mapping["dup"] = "validation"
	issue := &tracker.IssueState{Labels: []string{"h-x", "dup", "g-x", "a"}}
	first := tracker.MapTrackerStatusWithBoard(issue, nil, mapping, teamstate.BoardConfig{})
	assert.Equal(t, "in_progress", first)
	for range 50 {
		assert.Equal(t, first, tracker.MapTrackerStatusWithBoard(issue, nil, mapping, teamstate.BoardConfig{}))
	}
	// Two keys that differ by the case only: the first in alphabetical order.
	only := &tracker.IssueState{Labels: []string{"DUP"}}
	for range 50 {
		assert.Equal(t, "review", tracker.MapTrackerStatusWithBoard(only, nil, mapping, teamstate.BoardConfig{}))
	}
}

// A value that stands for nothing never hides another mapped label, and "todo"
// is read as the initial status even when nothing migrated the config.
func TestMapTrackerStatusIgnoresWhatStandsForNothing(t *testing.T) {
	mapping := map[string]string{"Odd": "wip", "Dev doing": "in_progress", "Ready": "todo"}
	got := tracker.MapTrackerStatusWithBoard(&tracker.IssueState{Labels: []string{"Odd", "Dev doing"}}, nil, mapping, teamstate.BoardConfig{})
	assert.Equal(t, "in_progress", got)

	got = tracker.MapTrackerStatusWithBoard(&tracker.IssueState{Labels: []string{"Ready"}, StatusCategory: "opened"}, nil, mapping, teamstate.BoardConfig{})
	assert.Equal(t, "planned", got, "todo is the initial status, not the GitLab state opened → in_progress")

	got = tracker.MapTrackerStatusWithBoard(&tracker.IssueState{Labels: []string{"Odd"}, StatusCategory: "opened"}, nil, mapping, teamstate.BoardConfig{})
	assert.Equal(t, "in_progress", got, "nothing valid: the category fallback")

	status := map[string]string{"Open": "todo"}
	got = tracker.MapTrackerStatusWithBoard(&tracker.IssueState{StatusName: "open", StatusCategory: "opened"}, status, nil, teamstate.BoardConfig{})
	assert.Equal(t, "planned", got, "status_mapping gets the same reading")
}
