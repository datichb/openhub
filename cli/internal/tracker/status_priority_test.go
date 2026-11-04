package tracker_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
)

// An issue with several mapped labels is placed by what they mean, not by the
// order of its labels: a blocked column always wins, then the most advanced
// column (the furthest in the order of the board).
func TestMapTrackerStatusPriorityBlockedThenMostAdvanced(t *testing.T) {
	mapping := map[string]string{"Blocked": "blocked", "Ready to dev": "planned", "Dev doing": "in_progress", "To review": "review", "Testing": "validation", "Released": "done"}
	board := teamstate.BoardConfig{}
	for _, tt := range []struct {
		labels []string
		want   string
	}{
		// #491: blocked wins over "ready to dev", in either order of the labels.
		{[]string{"Blocked", "Ready to dev"}, "blocked"},
		{[]string{"Ready to dev", "Blocked"}, "blocked"},
		{[]string{"blocked", "READY TO DEV", "bug"}, "blocked"},
		// Blocked beats even the most advanced columns.
		{[]string{"Released", "Blocked", "Testing"}, "blocked"},
		// An old label left on the issue does not make it fall back.
		{[]string{"Ready to dev", "Dev doing"}, "in_progress"},
		{[]string{"Dev doing", "Ready to dev"}, "in_progress"},
		{[]string{"Dev doing", "To review"}, "review"},
		{[]string{"To review", "Testing", "Dev doing"}, "validation"},
		{[]string{"Testing", "Released"}, "done"},
		{[]string{"Ready to dev"}, "planned"},
	} {
		got := tracker.MapTrackerStatusWithBoard(&tracker.IssueState{Labels: tt.labels}, nil, mapping, board)
		assert.Equal(t, tt.want, got, "%v", tt.labels)
	}
}

// The order and the blocked columns are those the team gave its board.
func TestMapTrackerStatusPriorityFollowsTheCustomColumns(t *testing.T) {
	board := teamstate.BoardConfig{Columns: []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "BACKLOG", Role: teamstate.ColumnRoleInitial},
		{ID: "stuck", Name: "STUCK", Role: teamstate.ColumnRoleBlocked},
		{ID: "doing", Name: "DOING", Role: teamstate.ColumnRoleActive},
		{ID: "qa", Name: "QA", Role: teamstate.ColumnRoleActive},
		{ID: "done", Name: "DONE", Role: teamstate.ColumnRoleTerminal},
	}}
	mapping := map[string]string{"Blocked": "stuck", "Ready": "backlog", "Doing": "doing", "QA": "qa", "Planned": "planned"}
	got := func(labels ...string) string {
		return tracker.MapTrackerStatusWithBoard(&tracker.IssueState{Labels: labels}, nil, mapping, board)
	}
	assert.Equal(t, "stuck", got("Doing", "Ready", "Blocked", "QA"), "the blocked role wins wherever its column is")
	assert.Equal(t, "qa", got("Doing", "Ready", "QA"), "the furthest column")
	assert.Equal(t, "doing", got("Ready", "Doing"))

	// A column called "blocked" without the role is an ordinary column.
	named := teamstate.BoardConfig{Columns: []teamstate.BoardColumnConfig{
		{ID: "backlog", Name: "B", Role: teamstate.ColumnRoleInitial},
		{ID: "blocked", Name: "X", Role: teamstate.ColumnRoleActive},
		{ID: "doing", Name: "D", Role: teamstate.ColumnRoleActive},
		{ID: "done", Name: "E", Role: teamstate.ColumnRoleTerminal},
	}}
	m2 := map[string]string{"B": "blocked", "D": "doing"}
	assert.Equal(t, "doing", tracker.MapTrackerStatusWithBoard(&tracker.IssueState{Labels: []string{"B", "D"}}, nil, m2, named))

	// planned is the initial column: two labels for it tie, the alphabetical
	// order of the labels settles it.
	mapping = map[string]string{"B label": "backlog", "A label": "planned"}
	assert.Equal(t, "planned", got("B label", "A label"))
	assert.Equal(t, "planned", got("A label", "B label"))
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
	assert.Equal(t, "review", first, "the most advanced of the mapped labels")
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
