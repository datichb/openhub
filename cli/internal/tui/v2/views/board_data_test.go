package views

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// MapClaimStatusWithConfig tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMapClaimStatusWithConfig_DirectMatch(t *testing.T) {
	columns := []BoardColumnDef{
		{Name: "TODO", Status: "todo", Color: theme.Warning},
		{Name: "TESTING", Status: "testing", Color: theme.Info},
		{Name: "DONE", Status: "done", Color: theme.Success},
	}
	// A status that matches a column directly should be returned as-is.
	assert.Equal(t, "testing", MapClaimStatusWithConfig("testing", columns))
}

func TestMapClaimStatusWithConfig_LegacyPlanned(t *testing.T) {
	columns := []BoardColumnDef{
		{Name: "BACKLOG", Status: "backlog", Color: theme.Warning},
		{Name: "DEV", Status: "dev", Color: tcell.GetColor(theme.BlueHex)},
		{Name: "DONE", Status: "done", Color: theme.Success},
	}
	// "planned" is a legacy status — should map to the first column.
	assert.Equal(t, "backlog", MapClaimStatusWithConfig(teamstate.ClaimStatusPlanned, columns))
}

func TestMapClaimStatusWithConfig_Fallback(t *testing.T) {
	columns := []BoardColumnDef{
		{Name: "TODO", Status: "todo", Color: theme.Warning},
		{Name: "DEV", Status: "dev", Color: tcell.GetColor(theme.BlueHex)},
		{Name: "DONE", Status: "done", Color: theme.Success},
	}
	// An unknown status should fall back to the first column.
	assert.Equal(t, "todo", MapClaimStatusWithConfig("nonexistent_status", columns))
}

func TestMapClaimStatusWithConfig_DefaultColumns(t *testing.T) {
	columns := DefaultColumns()

	// Verify backward compatibility: all built-in statuses should match directly.
	tests := []struct {
		input    string
		expected string
	}{
		{"todo", "todo"},
		{"in_progress", "in_progress"},
		{"review", "review"},
		{"validation", "validation"},
		{"done", "done"},
		{"blocked", "blocked"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, MapClaimStatusWithConfig(tt.input, columns))
		})
	}

	// "planned" should map to the first column ("todo").
	assert.Equal(t, "todo", MapClaimStatusWithConfig(teamstate.ClaimStatusPlanned, columns))
}
