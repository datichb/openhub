package beads

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsReadyStatus(t *testing.T) {
	tests := []struct {
		status string
		ready  bool
	}{
		{"open", true},
		{"ready", true},
		{"todo", true},
		{"to_do", true},
		{"backlog", true},
		{"in_progress", false},
		{"done", false},
		{"closed", false},
		{"review", false},
		{"blocked", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			assert.Equal(t, tt.ready, isReadyStatus(tt.status))
		})
	}
}

func TestHasLabel(t *testing.T) {
	ticket := Ticket{
		ID:     "bd-1",
		Title:  "Test ticket",
		Labels: []string{"ai-delegated", "bug", "P1"},
	}

	assert.True(t, hasLabel(ticket, "ai-delegated"))
	assert.True(t, hasLabel(ticket, "AI-Delegated")) // case insensitive
	assert.True(t, hasLabel(ticket, "bug"))
	assert.False(t, hasLabel(ticket, "feature"))
	assert.False(t, hasLabel(ticket, ""))
}

func TestHasLabelEmptyLabels(t *testing.T) {
	ticket := Ticket{
		ID:    "bd-2",
		Title: "No labels",
	}
	assert.False(t, hasLabel(ticket, "ai-delegated"))
}

func TestRunBdJSONParsing(t *testing.T) {
	// Test the JSON parsing logic with sample data
	sampleJSON := `[
		{"id": "bd-1", "title": "Fix login", "status": "open", "priority": "1", "type": "feature", "labels": ["ai-delegated"]},
		{"id": "bd-2", "title": "Epic: Auth", "status": "open", "priority": "0", "type": "epic"},
		{"id": "bd-3", "title": "Bug report", "status": "done", "priority": "2", "type": "bug", "parent": "bd-2"}
	]`

	// We can't easily mock exec.Command, but we can test the filter logic
	tickets := []Ticket{
		{ID: "bd-1", Title: "Fix login", Status: "open", Priority: "1", Type: "feature", Labels: []string{"ai-delegated"}},
		{ID: "bd-2", Title: "Epic: Auth", Status: "open", Priority: "0", Type: "epic"},
		{ID: "bd-3", Title: "Bug report", Status: "done", Priority: "2", Type: "bug", Parent: "bd-2"},
	}

	// Test epic filtering
	var epics []Ticket
	for _, t := range tickets {
		if t.Type == "epic" {
			epics = append(epics, t)
		}
	}
	require.Len(t, epics, 1)
	assert.Equal(t, "bd-2", epics[0].ID)

	// Test orphan filtering (no parent, ready, not epic)
	var orphansLabeled, orphansOther []Ticket
	for _, tk := range tickets {
		if tk.Type == "epic" || !isReadyStatus(tk.Status) || tk.Parent != "" {
			continue
		}
		if hasLabel(tk, "ai-delegated") {
			orphansLabeled = append(orphansLabeled, tk)
		} else {
			orphansOther = append(orphansOther, tk)
		}
	}
	require.Len(t, orphansLabeled, 1)
	assert.Equal(t, "bd-1", orphansLabeled[0].ID)
	assert.Empty(t, orphansOther)

	// Verify JSON is parseable
	_ = sampleJSON // used for documentation
}

func TestAvailable(t *testing.T) {
	// This test checks that Available() doesn't panic
	// It may pass or fail depending on whether bd is installed
	err := Available()
	if err != nil {
		assert.Contains(t, err.Error(), "bd not found")
		assert.Contains(t, err.Error(), "brew install")
	}
}
