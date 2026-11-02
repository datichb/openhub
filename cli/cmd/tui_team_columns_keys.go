package cmd

import (
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// handleKey is the key handler of the list: the mappings mode has its own
// keys, the columns mode the others. A key that is not the editor's goes on.
func (e *columnEditor) handleKey(event *tcell.EventKey) *tcell.EventKey {
	if e.editingMappingsFor != "" {
		return e.handleMappingsKey(event)
	}
	return e.handleColumnsKey(event)
}

// isRenameKey is e (keys.Edit) or Enter, which opens the item under the cursor.
func isRenameKey(event *tcell.EventKey) bool {
	return keys.Edit.Matches(event) || keys.Open.Matches(event)
}

// toast shows a message in the shell, when there is one.
func toast(msg string, level shell.ToastLevel) {
	if ui := columnUI(); ui != nil {
		ui.ShowToast(msg, level)
	}
}

// askName opens the input box of the shell, when there is one.
func askName(title, current string, onConfirm func(string)) {
	if ui := columnUI(); ui != nil {
		ui.ShowInputModal(title, current, onConfirm)
	}
}

func (e *columnEditor) handleMappingsKey(event *tcell.EventKey) *tcell.EventKey {
	switch {
	case views.ColumnBack.Matches(event):
		e.editingMappingsFor = ""
		e.renderColumns()

	case keys.Down.Matches(event):
		e.moveToMapping(+1)

	case keys.Up.Matches(event):
		e.moveToMapping(-1)

	case keys.Create.Matches(event):
		e.addLabel()

	case isRenameKey(event):
		e.renameMapping()

	case keys.Delete.Matches(event):
		e.deleteMapping()

	default:
		return event
	}
	return nil
}

// moveToMapping moves the cursor to the next (or previous) item that is not a
// header.
func (e *columnEditor) moveToMapping(delta int) {
	total := e.list.GetItemCount()
	next := e.list.GetCurrentItem() + delta
	for next >= 0 && next < total && e.isHeaderLine(next) {
		next += delta
	}
	if next >= 0 && next < total {
		e.list.SetCurrentItem(next)
	}
}

// addLabel asks the name of a label and maps it to the column.
func (e *columnEditor) addLabel() {
	askName(i18n.T("tui.team.columns.label_name_prompt"), "", func(name string) {
		if name == "" {
			return
		}
		name = strings.TrimSpace(name)
		if e.labelMappings != nil {
			if *e.labelMappings == nil {
				*e.labelMappings = make(map[string]string)
			}
			(*e.labelMappings)[name] = e.editingMappingsFor
		}
		e.renderMappings()
		toast(i18n.Tf("tui.team.columns.label_added", name, e.editingMappingsFor), shell.ToastSuccess)
	})
}

// renameMapping renames the label or status under the cursor.
func (e *columnEditor) renameMapping() {
	ref := e.resolveMappingItem()
	if ref == nil {
		return
	}
	askName(i18n.T("tui.team.columns.rename_prompt"), ref.key, func(newName string) {
		if newName == "" || newName == ref.key {
			return
		}
		newName = strings.TrimSpace(newName)
		if ref.kind == "label" && e.labelMappings != nil {
			delete(*e.labelMappings, ref.key)
			(*e.labelMappings)[newName] = e.editingMappingsFor
		} else if ref.kind == "status" && e.statusMappings != nil {
			delete(*e.statusMappings, ref.key)
			(*e.statusMappings)[newName] = e.editingMappingsFor
		}
		e.renderMappings()
	})
}

// deleteMapping removes the label or status under the cursor.
func (e *columnEditor) deleteMapping() {
	ref := e.resolveMappingItem()
	if ref == nil {
		return
	}
	if ref.kind == "label" && e.labelMappings != nil {
		delete(*e.labelMappings, ref.key)
	} else if ref.kind == "status" && e.statusMappings != nil {
		delete(*e.statusMappings, ref.key)
	}
	e.renderMappings()
	toast(i18n.Tf("tui.team.columns.mapping_deleted", ref.key), shell.ToastSuccess)
}

func (e *columnEditor) handleColumnsKey(event *tcell.EventKey) *tcell.EventKey {
	idx := e.list.GetCurrentItem() - 1
	n := len(*e.columns)
	isOnColumn := idx >= 0 && idx < n

	switch {
	case keys.Down.Matches(event):
		if cur := e.list.GetCurrentItem(); cur < n {
			e.list.SetCurrentItem(cur + 1)
		}

	case keys.Up.Matches(event):
		if cur := e.list.GetCurrentItem(); cur > 1 {
			e.list.SetCurrentItem(cur - 1)
		}

	case views.ColumnMoveUp.Matches(event):
		if isOnColumn && idx > 0 {
			e.moveColumn(idx, -1)
		}

	case views.ColumnMoveDown.Matches(event):
		if isOnColumn && idx < n-1 {
			e.moveColumn(idx, +1)
		}

	case keys.Create.Matches(event):
		e.addColumn(idx)

	case isRenameKey(event):
		if isOnColumn {
			e.renameColumn(idx)
		}

	case keys.Delete.Matches(event):
		if isOnColumn {
			e.deleteColumn(idx)
		}

	case views.ColumnRole.Matches(event):
		if isOnColumn {
			e.cycleRole(idx)
		}

	case views.ColumnMappings.Matches(event):
		if e.hasMappings() && isOnColumn {
			e.editingMappingsFor = (*e.columns)[idx].ID
			e.renderMappings()
		}

	case keys.Save.Matches(event):
		e.save()

	default:
		return event
	}
	return nil
}

// moveColumn swaps column idx with its neighbour (delta -1 or +1); the cursor
// follows it.
func (e *columnEditor) moveColumn(idx, delta int) {
	cols := *e.columns
	cols[idx], cols[idx+delta] = cols[idx+delta], cols[idx]
	e.renderColumns()
	if delta < 0 {
		e.list.SetCurrentItem(idx)
		toast(i18n.T("tui.team.columns.moved_up"), shell.ToastSuccess)
		return
	}
	e.list.SetCurrentItem(idx + 2)
	toast(i18n.T("tui.team.columns.moved_down"), shell.ToastSuccess)
}

// addColumn asks the name of a new column and inserts it after column idx.
func (e *columnEditor) addColumn(idx int) {
	cols := *e.columns
	n := len(cols)
	askName(i18n.T("cmd.discovery.field.column_name"), "", func(name string) {
		if name == "" {
			return
		}
		id := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", "_"))
		role := tracker.SuggestColumnRole(name)
		newCol := teamstate.BoardColumnConfig{ID: id, Name: strings.TrimSpace(name), Role: role}
		pos := idx + 1
		if pos < 0 {
			pos = 0
		}
		if pos > n {
			pos = n
		}
		updated := make([]teamstate.BoardColumnConfig, 0, n+1)
		updated = append(updated, cols[:pos]...)
		updated = append(updated, newCol)
		updated = append(updated, cols[pos:]...)
		*e.columns = updated
		e.renderColumns()
		e.list.SetCurrentItem(pos + 1)
		toast(i18n.Tf("tui.team.columns.column_added", newCol.Name), shell.ToastSuccess)
	})
}

// renameColumn asks the new name of column idx.
func (e *columnEditor) renameColumn(idx int) {
	askName(i18n.T("cmd.discovery.field.column_rename"), (*e.columns)[idx].Name, func(name string) {
		if name != "" {
			(*e.columns)[idx].Name = strings.TrimSpace(name)
			e.renderColumns()
		}
	})
}

// deleteColumn removes column idx (two are kept) with the mappings that point
// to it.
func (e *columnEditor) deleteColumn(idx int) {
	cols := *e.columns
	if len(cols) <= 2 {
		toast(i18n.T("cmd.discovery.validate.min_columns"), shell.ToastError)
		return
	}
	deletedID := cols[idx].ID
	deletedName := cols[idx].Name
	copy(cols[idx:], cols[idx+1:])
	*e.columns = cols[:len(cols)-1]
	cleaned := e.dropMappingsOf(deletedID)
	e.renderColumns()
	msg := i18n.Tf("tui.team.columns.column_deleted", deletedName)
	if cleaned > 0 {
		msg += i18n.Tf("tui.team.columns.mappings_cleaned", cleaned)
	}
	toast(msg, shell.ToastSuccess)
}

// dropMappingsOf removes the labels and statuses mapped to a column and
// counts them.
func (e *columnEditor) dropMappingsOf(colID string) (cleaned int) {
	for _, m := range []*map[string]string{e.labelMappings, e.statusMappings} {
		if m == nil {
			continue
		}
		for k, v := range *m {
			if v == colID {
				delete(*m, k)
				cleaned++
			}
		}
	}
	return cleaned
}

// cycleRole gives column idx the next role (a column with none counts as
// active).
func (e *columnEditor) cycleRole(idx int) {
	current := (*e.columns)[idx].Role
	if current == "" {
		current = teamstate.ColumnRoleActive
	}
	nextIdx := 0
	for i, r := range columnEditorRoles {
		if r == current {
			nextIdx = (i + 1) % len(columnEditorRoles)
			break
		}
	}
	(*e.columns)[idx].Role = columnEditorRoles[nextIdx]
	e.renderColumns()
}

// save hands the columns to the wizard when they are valid, else says why not.
func (e *columnEditor) save() {
	if msg := columnsProblem(*e.columns); msg != "" {
		toast(msg, shell.ToastError)
		return
	}
	*e.result = make([]teamstate.BoardColumnConfig, len(*e.columns))
	copy(*e.result, *e.columns)
	e.onDone()
}
