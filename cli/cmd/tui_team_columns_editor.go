package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/keys"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// columnEditor is the interactive editor of the columns of the board: a list
// of the columns, and, when the mappings are given, a second list of the
// labels and statuses that land in one column (mappings mode).
type columnEditor struct {
	columns        *[]teamstate.BoardColumnConfig
	result         *[]teamstate.BoardColumnConfig
	labelMappings  *map[string]string
	statusMappings *map[string]string
	trackerType    string

	list   *tview.List
	hints  *tview.TextView
	onDone func()

	// editingMappingsFor is "" in columns mode, the id of the column in
	// mappings mode.
	editingMappingsFor string
}

// mappingRef is a label or a status mapped to the column being edited.
type mappingRef struct {
	kind string // "label" or "status"
	key  string
}

// mount puts the editor in the container of the wizard step.
func (e *columnEditor) mount(tvApp *tview.Application, container *tview.Flex, onDone func()) {
	e.onDone = onDone
	e.list = tview.NewList().
		ShowSecondaryText(true).
		SetHighlightFullLine(true).
		SetMainTextColor(theme.FgPrimary).
		SetSecondaryTextColor(theme.FgSecondary).
		SetSelectedBackgroundColor(theme.BgElement).
		SetSelectedTextColor(theme.FgPrimary)
	e.list.SetBackgroundColor(theme.BgPanel)

	e.hints = tview.NewTextView().SetDynamicColors(true)
	e.hints.SetBackgroundColor(theme.BgPanel)

	e.renderColumns()
	e.list.SetInputCapture(e.handleKey)

	container.AddItem(e.list, 0, 1, true)
	container.AddItem(e.hints, 1, 0, false)
	tvApp.SetFocus(e.list)
}

// hasMappings tells whether the editor manages the label → column rules.
func (e *columnEditor) hasMappings() bool { return e.labelMappings != nil }

func accentText(text string) string { return theme.Paint(theme.ActiveMode.AccentHex, text) }

// labelsForCol are the labels and the statuses mapped to a column, sorted.
func (e *columnEditor) labelsForCol(colID string) (labels, statuses []string) {
	if e.labelMappings != nil {
		for k, v := range *e.labelMappings {
			if v == colID {
				labels = append(labels, k)
			}
		}
	}
	if e.statusMappings != nil {
		for k, v := range *e.statusMappings {
			if v == colID {
				statuses = append(statuses, k)
			}
		}
	}
	sort.Strings(labels)
	sort.Strings(statuses)
	return labels, statuses
}

// truncLabels lists at most limit items, with the count of the others.
func truncLabels(items []string, limit int) string {
	if len(items) == 0 {
		return "—"
	}
	if len(items) <= limit {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:limit], ", ") + fmt.Sprintf(" +%d", len(items)-limit)
}

// hintOf is a key of the editor and its label, the key in the accent color.
func hintOf(a keys.Action, label string) string { return accentText(a.Keys()) + " " + label }

// updateHints writes the keys of the mode the editor is in.
func (e *columnEditor) updateHints() {
	hintSep := " " + theme.Paint(theme.TextMutedHex, "│") + " "
	var parts []string
	if e.editingMappingsFor == "" {
		parts = []string{
			hintOf(keys.Navigate, keys.Navigate.Label()),
			hintOf(views.ColumnMoveUp, views.ColumnMoveUp.Label()),
			hintOf(views.ColumnMoveDown, views.ColumnMoveDown.Label()),
			hintOf(keys.Create, i18n.T("tui.team.columns.hint_add")),
			hintOf(keys.Edit, i18n.T("tui.team.columns.hint_rename")),
			hintOf(keys.Delete, i18n.T("tui.team.columns.hint_delete")),
			hintOf(views.ColumnRole, views.ColumnRole.Label()),
		}
		if e.hasMappings() {
			parts = append(parts, hintOf(views.ColumnMappings, views.ColumnMappings.Label()))
		}
		parts = append(parts, hintOf(keys.Save, i18n.T("tui.team.columns.hint_save")))
	} else {
		parts = []string{
			hintOf(keys.Navigate, keys.Navigate.Label()),
			hintOf(keys.Create, i18n.T("tui.team.columns.hint_add_label")),
			hintOf(keys.Edit, i18n.T("tui.team.columns.hint_rename")),
			hintOf(keys.Delete, i18n.T("tui.team.columns.hint_delete")),
			hintOf(views.ColumnBack, views.ColumnBack.Label()),
		}
	}
	e.hints.SetText("  " + strings.Join(parts, hintSep))
}

// renderColumns draws the columns mode, keeping the cursor on a column.
func (e *columnEditor) renderColumns() {
	columns := *e.columns
	sel := e.list.GetCurrentItem()
	e.list.Clear()

	headerText := "  " + accentText(fmt.Sprintf("── %s (%d) ──", i18n.T("tui.team.columns.header_label"), len(columns)))
	e.list.AddItem(headerText, "", 0, nil)

	activeIdx := 0
	for i, col := range columns {
		role := col.Role
		if role == "" {
			role = teamstate.ColumnRoleActive
		}
		color := views.ResolveColumnColor(col, activeIdx)
		if role == teamstate.ColumnRoleActive || col.Role == "" {
			activeIdx++
		}
		main, secondary := e.columnItem(i, col, role, color)
		e.list.AddItem(main, secondary, 0, nil)
	}

	e.list.AddItem("", "", 0, nil)
	e.addRolesLegend()

	if sel < 1 {
		sel = 1
	}
	if sel > len(columns) {
		sel = len(columns)
	}
	e.list.SetCurrentItem(sel)
	e.updateHints()
}

// columnItem is the line of a column: color, name, role, position, then its id,
// color and mapped labels.
func (e *columnEditor) columnItem(i int, col teamstate.BoardColumnConfig, role string, color tcell.Color) (main, secondary string) {
	colorDot := theme.Paint(color, theme.IconSwatch)
	roleBadge := theme.BgTag(columnRoleHex(role)) + theme.Paint(theme.BgAppHex, " "+role+" ") + theme.TagBg
	posIndicator := theme.Paint(theme.TextMutedHex, fmt.Sprintf("%d/%d", i+1, len(*e.columns)))

	main = fmt.Sprintf("  %s  %s  %s  %s",
		colorDot, theme.StyleTag(theme.AttrBold)+col.Name+theme.TagAttrs, roleBadge, posIndicator)

	secParts := []string{
		fmt.Sprintf("id: %s", col.ID),
		fmt.Sprintf(i18n.T("tui.team.columns.color_label"), columnColorName(col, color)),
	}
	if e.hasMappings() {
		labels, statuses := e.labelsForCol(col.ID)
		all := make([]string, 0, len(labels)+len(statuses))
		all = append(all, labels...)
		all = append(all, statuses...)
		secParts = append(secParts, fmt.Sprintf(i18n.T("tui.team.columns.labels_label"), truncLabels(all, 3)))
	}
	secondary = "      " + theme.Paint(theme.TextMutedHex, strings.Join(secParts, " · "))
	return main, secondary
}

// addRolesLegend ends the list with what each role means.
func (e *columnEditor) addRolesLegend() {
	legendText := "  " + accentText(fmt.Sprintf("── %s ──", i18n.T("tui.team.columns.roles_header")))
	e.list.AddItem(legendText, "", 0, nil)
	legendLine := fmt.Sprintf("    %s %s  %s %s  %s %s  %s %s",
		theme.Paint(theme.WarningHex, theme.IconSwatch), i18n.T("tui.team.columns.role_initial_desc"),
		theme.Paint(theme.ActiveMode.AccentHex, theme.IconSwatch), i18n.T("tui.team.columns.role_active_desc"),
		theme.Paint(theme.SuccessHex, theme.IconSwatch), i18n.T("tui.team.columns.role_terminal_desc"),
		theme.Paint(theme.ErrorHex, theme.IconSwatch), i18n.T("tui.team.columns.role_blocked_desc"))
	e.list.AddItem(legendLine,
		"      "+theme.Paint(theme.TextMutedHex, i18n.T("tui.team.columns.roles_explanation")),
		0, nil)
}

// renderMappings draws the mappings mode: the labels (and for Jira the
// statuses) that land in the column being edited.
func (e *columnEditor) renderMappings() {
	e.list.Clear()

	colName := e.editingMappingsFor
	for _, c := range *e.columns {
		if c.ID == e.editingMappingsFor {
			colName = c.Name
			break
		}
	}

	headerText := "  " + accentText(fmt.Sprintf("── %s → %s ──", i18n.T("tui.team.columns.mappings_title"), colName))
	e.list.AddItem(headerText, "", 0, nil)

	labels, statuses := e.labelsForCol(e.editingMappingsFor)
	e.addMappingSection(
		theme.Paint(theme.TextMutedHex, fmt.Sprintf("── %s ──", i18n.T("tui.team.columns.labels_section"))),
		i18n.T("tui.team.columns.no_label_mapped"), accentText(theme.SymCursor), labels)
	if e.trackerType == "jira" && e.statusMappings != nil {
		e.addMappingSection(
			theme.Paint(theme.TextMutedHex, "── Statuses (Jira) ──"),
			i18n.T("tui.team.columns.no_status_mapped"), theme.Paint(theme.InfoHex, theme.SymCursor), statuses)
	}

	// Select first mapping item (skip headers)
	total := e.list.GetItemCount()
	sel := 1
	for sel < total {
		main, _ := e.list.GetItemText(sel)
		if strings.Contains(main, "──") || main == "" {
			sel++
		} else {
			break
		}
	}
	if sel >= total {
		sel = 0
	}
	e.list.SetCurrentItem(sel)
	e.updateHints()
}

// addMappingSection adds a section header and its items (or a line saying
// there is none).
func (e *columnEditor) addMappingSection(header, none, bullet string, items []string) {
	e.list.AddItem("    "+header, "", 0, nil)
	if len(items) == 0 {
		e.list.AddItem(
			"    "+theme.Paint(theme.TextMutedHex, none),
			"      "+theme.Paint(theme.TextMutedHex, i18n.Tf("tui.team.columns.press_to_add", keys.Create.Keys())),
			0, nil)
	}
	for _, item := range items {
		e.list.AddItem(
			"    "+bullet+" "+item,
			"      "+theme.Paint(theme.TextMutedHex, "→ "+e.editingMappingsFor),
			0, nil)
	}
}

// isHeaderLine tells whether line i of the mappings list is a header or the
// « none mapped » line.
func (e *columnEditor) isHeaderLine(listIdx int) bool {
	main, _ := e.list.GetItemText(listIdx)
	return strings.Contains(main, "──") || main == "" || strings.Contains(main, i18n.T("tui.team.columns.no_label_mapped")) || strings.Contains(main, i18n.T("tui.team.columns.no_status_mapped"))
}

// resolveMappingItem is the label or status under the cursor of the mappings
// mode, nil on a header.
func (e *columnEditor) resolveMappingItem() *mappingRef {
	if e.editingMappingsFor == "" {
		return nil
	}
	main, _ := e.list.GetItemText(e.list.GetCurrentItem())
	if strings.Contains(main, "──") || strings.Contains(main, i18n.T("tui.team.columns.no_label_mapped")) || strings.Contains(main, i18n.T("tui.team.columns.no_status_mapped")) || main == "" {
		return nil
	}
	// Strip tview color tags to get the raw name
	name := main
	for strings.Contains(name, "[") && strings.Contains(name, "]") {
		start := strings.Index(name, "[")
		end := strings.Index(name, "]")
		if end > start {
			name = name[:start] + name[end+1:]
		} else {
			break
		}
	}
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, theme.SymCursor+" ")
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if e.labelMappings != nil {
		for k, v := range *e.labelMappings {
			if k == name && v == e.editingMappingsFor {
				return &mappingRef{kind: "label", key: k}
			}
		}
	}
	if e.statusMappings != nil {
		for k, v := range *e.statusMappings {
			if k == name && v == e.editingMappingsFor {
				return &mappingRef{kind: "status", key: k}
			}
		}
	}
	return nil
}
