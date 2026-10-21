package views

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func TestMapClaimStatusWithConfig(t *testing.T) {
	custom := func(first string) []BoardColumnDef {
		return []BoardColumnDef{
			{Name: "FIRST", Status: first, Color: theme.Warning},
			{Name: "DEV", Status: "dev", Color: tcell.GetColor(theme.BlueHex)},
			{Name: "DONE", Status: "done", Color: theme.Success},
		}
	}
	tests := []struct {
		name    string
		columns []BoardColumnDef
		status  string
		want    string
	}{
		{"a status that matches a column is kept", append(custom("todo"), BoardColumnDef{Name: "TESTING", Status: "testing"}), "testing", "testing"},
		{"legacy planned maps to the first column", custom("backlog"), teamstate.ClaimStatusPlanned, "backlog"},
		{"unknown status falls back to the first column", custom("todo"), "nonexistent_status", "todo"},
		{"default columns: planned maps to todo", DefaultColumns(), teamstate.ClaimStatusPlanned, "todo"},
	}
	for _, status := range []string{"todo", "in_progress", "review", "validation", "done", "blocked"} {
		tests = append(tests, struct {
			name    string
			columns []BoardColumnDef
			status  string
			want    string
		}{"default columns: " + status + " matches directly", DefaultColumns(), status, status})
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, MapClaimStatusWithConfig(tc.status, tc.columns), "mapped status")
		})
	}
}
