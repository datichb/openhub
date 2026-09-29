package views

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// ─────────────────────────────────────────────────────────────────────────────
// ColumnsFromConfig tests
// ─────────────────────────────────────────────────────────────────────────────

func TestColumnsFromConfig_Default(t *testing.T) {
	// Nil columns (empty BoardConfig) should return DefaultColumns().
	cfg := teamstate.BoardConfig{}
	cols := ColumnsFromConfig(cfg)

	defaults := DefaultColumns()
	require.Len(t, cols, len(defaults))
	for i := range defaults {
		assert.Equal(t, defaults[i].Name, cols[i].Name, "column %d Name", i)
		assert.Equal(t, defaults[i].Status, cols[i].Status, "column %d Status", i)
		assert.Equal(t, defaults[i].Color, cols[i].Color, "column %d Color", i)
	}
}

func TestColumnsFromConfig_Custom(t *testing.T) {
	cfg := teamstate.BoardConfig{
		Columns: []teamstate.BoardColumnConfig{
			{ID: "backlog", Name: "BACKLOG", Role: teamstate.ColumnRoleInitial},
			{ID: "dev", Name: "DEV", Role: teamstate.ColumnRoleActive},
			{ID: "qa", Name: "QA", Role: teamstate.ColumnRoleActive},
			{ID: "done", Name: "DONE", Role: teamstate.ColumnRoleTerminal},
			{ID: "blocked", Name: "BLOCKED", Role: teamstate.ColumnRoleBlocked},
		},
	}

	cols := ColumnsFromConfig(cfg)
	require.Len(t, cols, 5)

	// Names and statuses should match the config.
	assert.Equal(t, "BACKLOG", cols[0].Name)
	assert.Equal(t, "backlog", cols[0].Status)
	assert.Equal(t, "DEV", cols[1].Name)
	assert.Equal(t, "dev", cols[1].Status)
	assert.Equal(t, "QA", cols[2].Name)
	assert.Equal(t, "qa", cols[2].Status)
	assert.Equal(t, "DONE", cols[3].Name)
	assert.Equal(t, "done", cols[3].Status)
	assert.Equal(t, "BLOCKED", cols[4].Name)
	assert.Equal(t, "blocked", cols[4].Status)

	// Role-based colors:
	// initial → theme.Warning
	assert.Equal(t, theme.Warning, cols[0].Color)
	// active columns cycle through activeColorCycle: Blue, FgSecondary, Info
	assert.Equal(t, tcell.GetColor(theme.BlueHex), cols[1].Color) // first active
	assert.Equal(t, theme.FgSecondary, cols[2].Color) // second active
	// terminal → theme.Success
	assert.Equal(t, theme.Success, cols[3].Color)
	// blocked → theme.Error
	assert.Equal(t, theme.Error, cols[4].Color)
}

func TestColumnsFromConfig_ColorOverride(t *testing.T) {
	cfg := teamstate.BoardConfig{
		Columns: []teamstate.BoardColumnConfig{
			{ID: "todo", Name: "TODO", Role: teamstate.ColumnRoleInitial, Color: "purple"},
			{ID: "dev", Name: "DEV", Role: teamstate.ColumnRoleActive, Color: "yellow"},
			{ID: "done", Name: "DONE", Role: teamstate.ColumnRoleTerminal},
		},
	}

	cols := ColumnsFromConfig(cfg)
	require.Len(t, cols, 3)

	// Explicit color overrides should take precedence over role-based colors.
	assert.Equal(t, colorOverrides["purple"], cols[0].Color, "initial column should use purple override, not role-based Warning")
	assert.Equal(t, colorOverrides["yellow"], cols[1].Color, "active column should use yellow override, not role-based Accent")
	// No explicit color → role-based (terminal = Success)
	assert.Equal(t, theme.Success, cols[2].Color, "terminal column without override should use role-based Success")
}
