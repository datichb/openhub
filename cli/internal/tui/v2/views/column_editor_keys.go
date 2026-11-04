package views

import "github.com/datichb/openhub/cli/internal/tui/v2/keys"

// The keys of the board column editor (cmd/tui_team_columns*.go) that the
// convention does not give: the editor reads them through these actions, and
// through keys.Create (n add), keys.Edit (e rename), keys.Delete (d), keys.Up
// and keys.Down for the rest. The role is on o: r refreshes everywhere.
var (
	// ColumnMoveUp moves the column under the cursor up.
	ColumnMoveUp = keys.Own("columns.move_up", 'K', "tui.team.columns.hint_move_up")
	// ColumnMoveDown moves the column under the cursor down.
	ColumnMoveDown = keys.Own("columns.move_down", 'J', "tui.team.columns.hint_move_down")
	// ColumnRole gives the column the next role (initial, active, review, terminal, blocked).
	ColumnRole = keys.Own("columns.role", 'o', "tui.team.columns.hint_role")
	// ColumnMappings opens the labels and statuses mapped to the column.
	ColumnMappings = keys.Own("columns.mappings", 'm', "tui.team.columns.hint_mappings")
	// ColumnMoveTo moves the label or the status under the cursor, in the
	// mappings of a column, to another column.
	ColumnMoveTo = keys.Own("columns.move_to", 'v', "tui.team.columns.hint_move_to")
	// ColumnBack leaves the mappings for the columns (Esc asks to quit the wizard).
	ColumnBack = keys.Own("columns.back", 'q', "tui.team.columns.hint_back_columns")
)
