package views

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

func TestColumnsFromConfig(t *testing.T) {
	t.Run("no column: the default columns", func(t *testing.T) {
		cols := ColumnsFromConfig(teamstate.BoardConfig{})
		defaults := DefaultColumns()
		require.Len(t, cols, len(defaults), "number of columns")
		for i := range defaults {
			assert.Equal(t, defaults[i].Name, cols[i].Name, "column %d Name", i)
			assert.Equal(t, defaults[i].Status, cols[i].Status, "column %d Status", i)
			assert.Equal(t, defaults[i].Color, cols[i].Color, "column %d Color", i)
		}
	})

	t.Run("custom columns take their colour from their role", func(t *testing.T) {
		cols := ColumnsFromConfig(teamstate.BoardConfig{
			Columns: []teamstate.BoardColumnConfig{
				{ID: "backlog", Name: "BACKLOG", Role: teamstate.ColumnRoleInitial},
				{ID: "dev", Name: "DEV", Role: teamstate.ColumnRoleActive},
				{ID: "qa", Name: "QA", Role: teamstate.ColumnRoleActive},
				{ID: "done", Name: "DONE", Role: teamstate.ColumnRoleTerminal},
				{ID: "blocked", Name: "BLOCKED", Role: teamstate.ColumnRoleBlocked},
			},
		})
		want := []struct {
			name, status string
			color        tcell.Color
		}{
			{"BACKLOG", "backlog", theme.Warning},
			{"DEV", "dev", tcell.GetColor(theme.BlueHex)}, // first active
			{"QA", "qa", theme.FgSecondary},               // second active (activeColorCycle)
			{"DONE", "done", theme.Success},
			{"BLOCKED", "blocked", theme.Error},
		}
		require.Len(t, cols, len(want), "number of columns")
		for i, w := range want {
			assert.Equal(t, w.name, cols[i].Name, "column %d Name", i)
			assert.Equal(t, w.status, cols[i].Status, "column %d Status", i)
			assert.Equal(t, w.color, cols[i].Color, "column %d Color", i)
		}
	})

	t.Run("an explicit colour wins over the role", func(t *testing.T) {
		cols := ColumnsFromConfig(teamstate.BoardConfig{
			Columns: []teamstate.BoardColumnConfig{
				{ID: "todo", Name: "TODO", Role: teamstate.ColumnRoleInitial, Color: "purple"},
				{ID: "dev", Name: "DEV", Role: teamstate.ColumnRoleActive, Color: "yellow"},
				{ID: "done", Name: "DONE", Role: teamstate.ColumnRoleTerminal},
			},
		})
		require.Len(t, cols, 3, "number of columns")
		assert.Equal(t, colorOverrides["purple"], cols[0].Color, "initial column uses the purple override, not the role colour")
		assert.Equal(t, colorOverrides["yellow"], cols[1].Color, "active column uses the yellow override, not the role colour")
		assert.Equal(t, theme.Success, cols[2].Color, "terminal column without override uses the role colour")
	})
}
