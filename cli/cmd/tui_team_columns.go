package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/config"
	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tracker"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Board column editor (shared helper + standalone action)
// ─────────────────────────────────────────────────────────────────────────────

// columnEditorRoles is the cycle order for the 'r' key in the column editor.
var columnEditorRoles = []string{
	teamstate.ColumnRoleInitial,
	teamstate.ColumnRoleActive,
	teamstate.ColumnRoleTerminal,
	teamstate.ColumnRoleBlocked,
}

// buildColumnEditorStep creates a WizardStep with an interactive column editor.
// The editor allows adding, removing, renaming, reordering, and cycling roles
// on board columns. When labelMappings/statusMappings are non-nil, the editor
// also supports a "mappings mode" (toggled via 'm') for managing the label→column
// translation rules.
//
// Used by both the discovery wizard (step 3, mappings=nil) and the standalone
// board config action (mappings loaded from team-state).
func buildColumnEditorStep(
	columns *[]teamstate.BoardColumnConfig,
	result *[]teamstate.BoardColumnConfig,
	labelMappings *map[string]string, //nolint:gocritic // ptrToRefParam: pointer needed to lazily initialize nil maps in the caller
	statusMappings *map[string]string, //nolint:gocritic // ptrToRefParam: pointer needed to lazily initialize nil maps in the caller
	trackerType string,
) views.WizardStep {
	return views.WizardStep{
		Label:      i18n.T("cmd.discovery.step_columns"),
		Processing: i18n.T("cmd.discovery.processing_columns"),
		CustomView: func(app *tview.Application, container *tview.Flex, onDone func()) {
			list := tview.NewList().
				ShowSecondaryText(true).
				SetHighlightFullLine(true).
				SetMainTextColor(theme.FgPrimary).
				SetSecondaryTextColor(theme.FgSecondary).
				SetSelectedBackgroundColor(theme.BgElement).
				SetSelectedTextColor(theme.FgPrimary)
			list.SetBackgroundColor(theme.BgPanel)

			hints := tview.NewTextView().SetDynamicColors(true)
			hints.SetBackgroundColor(theme.BgPanel)
			hintSep := fmt.Sprintf(" [%s]│[-] ", theme.TextMutedHex)

			// ── State ──
			hasMappings := labelMappings != nil
			var editingMappingsFor string // "" = columns mode, "col_id" = mappings mode

			// ── Helpers ──
			labelsForCol := func(colID string) (labels []string, statuses []string) {
				if labelMappings != nil {
					for k, v := range *labelMappings {
						if v == colID {
							labels = append(labels, k)
						}
					}
				}
				if statusMappings != nil {
					for k, v := range *statusMappings {
						if v == colID {
							statuses = append(statuses, k)
						}
					}
				}
				sort.Strings(labels)
				sort.Strings(statuses)
				return
			}

			truncLabels := func(items []string, max int) string {
				if len(items) == 0 {
					return "—"
				}
				if len(items) <= max {
					return strings.Join(items, ", ")
				}
				return strings.Join(items[:max], ", ") + fmt.Sprintf(" +%d", len(items)-max)
			}

			type mappingRef struct {
				kind string // "label" or "status"
				key  string
			}

			// ── Render functions ──
			var renderColumns func()
			var renderMappings func()

			updateHints := func() {
				if editingMappingsFor == "" {
					h := fmt.Sprintf("  %s↑↓[-] nav%s%sJ/K[-] %s%s%sa[-] %s%s%sEnter[-] %s%s%sd[-] %s%s%sr[-] %s",
						theme.ColorTag(theme.ActiveMode.AccentHex), hintSep,
						theme.ColorTag(theme.ActiveMode.AccentHex), i18n.T("tui.team.columns.hint_move"), hintSep,
						theme.ColorTag(theme.ActiveMode.AccentHex), i18n.T("tui.team.columns.hint_add"), hintSep,
						theme.ColorTag(theme.ActiveMode.AccentHex), i18n.T("tui.team.columns.hint_rename"), hintSep,
						theme.ColorTag(theme.ActiveMode.AccentHex), i18n.T("tui.team.columns.hint_delete"), hintSep,
						theme.ColorTag(theme.ActiveMode.AccentHex), i18n.T("tui.team.columns.hint_role"))
					if hasMappings {
						h += fmt.Sprintf("%s%sm[-] mappings", hintSep, theme.ColorTag(theme.ActiveMode.AccentHex))
					}
					h += fmt.Sprintf("%s%sCtrl+S[-] %s", hintSep, theme.ColorTag(theme.ActiveMode.AccentHex), i18n.T("tui.team.columns.hint_save"))
					hints.SetText(h)
				} else {
					hints.SetText(fmt.Sprintf("  %s↑↓[-] nav%s%sa[-] %s%s%sEnter[-] %s%s%sd[-] %s%s%sq[-] %s",
						theme.ColorTag(theme.ActiveMode.AccentHex), hintSep,
						theme.ColorTag(theme.ActiveMode.AccentHex), i18n.T("tui.team.columns.hint_add_label"), hintSep,
						theme.ColorTag(theme.ActiveMode.AccentHex), i18n.T("tui.team.columns.hint_rename"), hintSep,
						theme.ColorTag(theme.ActiveMode.AccentHex), i18n.T("tui.team.columns.hint_delete"), hintSep,
						theme.ColorTag(theme.ActiveMode.AccentHex), i18n.T("tui.team.columns.hint_back_columns")))
				}
			}

			renderColumns = func() {
				sel := list.GetCurrentItem()
				list.Clear()

				headerText := fmt.Sprintf("  %s── %s (%d) ──%s",
					theme.ColorTag(theme.ActiveMode.AccentHex), i18n.T("tui.team.columns.header_label"), len(*columns), theme.TagColor)
				list.AddItem(headerText, "", 0, nil)

				activeIdx := 0
				for i, col := range *columns {
					role := col.Role
					if role == "" {
						role = teamstate.ColumnRoleActive
					}
					color := views.ResolveColumnColor(col, activeIdx)
					if role == teamstate.ColumnRoleActive || col.Role == "" {
						activeIdx++
					}
					colorDot := widgets.ColorTag(color) + "●" + "[-]"
					roleHex := columnRoleHex(role)
					roleBadge := fmt.Sprintf("[black:%s] %s [-:-]", roleHex, role)
					posIndicator := fmt.Sprintf("[%s]%d/%d[-]", theme.TextMutedHex, i+1, len(*columns))

					mainText := fmt.Sprintf("  %s  [::b]%s[-:-:-]  %s  %s",
						colorDot, col.Name, roleBadge, posIndicator)

					colorName := columnColorName(col, color)
					secParts := []string{
						fmt.Sprintf("id: %s", col.ID),
						fmt.Sprintf(i18n.T("tui.team.columns.color_label"), colorName),
					}
					if hasMappings {
						labels, statuses := labelsForCol(col.ID)
						all := make([]string, 0, len(labels)+len(statuses))
						all = append(all, labels...)
						all = append(all, statuses...)
						secParts = append(secParts, fmt.Sprintf(i18n.T("tui.team.columns.labels_label"), truncLabels(all, 3)))
					}
					secondaryText := fmt.Sprintf("      [%s]%s[-]",
						theme.TextMutedHex, strings.Join(secParts, " · "))

					list.AddItem(mainText, secondaryText, 0, nil)
				}

				list.AddItem("", "", 0, nil)
				legendText := fmt.Sprintf("  %s── %s ──%s",
					theme.ColorTag(theme.ActiveMode.AccentHex), i18n.T("tui.team.columns.roles_header"), theme.TagColor)
				list.AddItem(legendText, "", 0, nil)
				legendLine := fmt.Sprintf("    [%s]●[-] %s  [%s]●[-] %s  [%s]●[-] %s  [%s]●[-] %s",
					theme.WarningHex, i18n.T("tui.team.columns.role_initial_desc"),
					theme.ActiveMode.AccentHex, i18n.T("tui.team.columns.role_active_desc"),
					theme.SuccessHex, i18n.T("tui.team.columns.role_terminal_desc"),
					theme.ErrorHex, i18n.T("tui.team.columns.role_blocked_desc"))
				list.AddItem(legendLine,
					fmt.Sprintf("      [%s]%s[-]", theme.TextMutedHex, i18n.T("tui.team.columns.roles_explanation")),
					0, nil)

				maxSel := len(*columns)
				if sel < 1 {
					sel = 1
				}
				if sel > maxSel {
					sel = maxSel
				}
				list.SetCurrentItem(sel)
				updateHints()
			}

			renderMappings = func() {
				list.Clear()

				colName := editingMappingsFor
				for _, c := range *columns {
					if c.ID == editingMappingsFor {
						colName = c.Name
						break
					}
				}

				headerText := fmt.Sprintf("  %s── %s → %s ──%s",
					theme.ColorTag(theme.ActiveMode.AccentHex), i18n.T("tui.team.columns.mappings_title"), colName, theme.TagColor)
				list.AddItem(headerText, "", 0, nil)

				labels, statuses := labelsForCol(editingMappingsFor)

				// Labels section
				labelHeader := fmt.Sprintf("    %s── %s ──%s",
					theme.ColorTag(theme.TextMutedHex), i18n.T("tui.team.columns.labels_section"), theme.TagColor)
				list.AddItem(labelHeader, "", 0, nil)

				if len(labels) == 0 {
					list.AddItem(
						fmt.Sprintf("    [%s]%s[-]", theme.TextMutedHex, i18n.T("tui.team.columns.no_label_mapped")),
						fmt.Sprintf("      [%s]%s[-]", theme.TextMutedHex, i18n.T("tui.team.columns.press_a_to_add")),
						0, nil)
				}
				for _, l := range labels {
					list.AddItem(
						fmt.Sprintf("    %s▸[-] %s", theme.ColorTag(theme.ActiveMode.AccentHex), l),
						fmt.Sprintf("      [%s]→ %s[-]", theme.TextMutedHex, editingMappingsFor),
						0, nil)
				}

				// Statuses section (Jira only)
				if trackerType == "jira" && statusMappings != nil {
					statusHeader := fmt.Sprintf("    %s── Statuses (Jira) ──%s",
						theme.ColorTag(theme.TextMutedHex), theme.TagColor)
					list.AddItem(statusHeader, "", 0, nil)

					if len(statuses) == 0 {
						list.AddItem(
							fmt.Sprintf("    [%s]%s[-]", theme.TextMutedHex, i18n.T("tui.team.columns.no_status_mapped")),
							fmt.Sprintf("      [%s]%s[-]", theme.TextMutedHex, i18n.T("tui.team.columns.press_a_to_add")),
							0, nil)
					}
					for _, s := range statuses {
						list.AddItem(
							fmt.Sprintf("    %s▸[-] %s", theme.ColorTag(theme.InfoHex), s),
							fmt.Sprintf("      [%s]→ %s[-]", theme.TextMutedHex, editingMappingsFor),
							0, nil)
					}
				}

				// Select first mapping item (skip headers)
				total := list.GetItemCount()
				sel := 1
				for sel < total {
					main, _ := list.GetItemText(sel)
					if strings.Contains(main, "──") || main == "" {
						sel++
					} else {
						break
					}
				}
				if sel >= total {
					sel = 0
				}
				list.SetCurrentItem(sel)
				updateHints()
			}

			// ── Mapping item resolution ──
			resolveMappingItem := func() *mappingRef {
				if editingMappingsFor == "" {
					return nil
				}
				idx := list.GetCurrentItem()
				main, _ := list.GetItemText(idx)
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
				name = strings.TrimPrefix(name, "▸ ")
				name = strings.TrimSpace(name)
				if name == "" {
					return nil
				}
				if labelMappings != nil {
					for k, v := range *labelMappings {
						if k == name && v == editingMappingsFor {
							return &mappingRef{kind: "label", key: k}
						}
					}
				}
				if statusMappings != nil {
					for k, v := range *statusMappings {
						if k == name && v == editingMappingsFor {
							return &mappingRef{kind: "status", key: k}
						}
					}
				}
				return nil
			}

			isHeaderLine := func(listIdx int) bool {
				main, _ := list.GetItemText(listIdx)
				return strings.Contains(main, "──") || main == "" || strings.Contains(main, i18n.T("tui.team.columns.no_label_mapped")) || strings.Contains(main, i18n.T("tui.team.columns.no_status_mapped"))
			}

			renderColumns()

			// ── Key handler ─────────────────────────────────────────
			list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				// ═══════════════════════════════════════════════════
				// MODE: Mappings
				// ═══════════════════════════════════════════════════
				if editingMappingsFor != "" {
					switch {
					case event.Rune() == 'q':
						editingMappingsFor = ""
						renderColumns()
						return nil

					case event.Key() == tcell.KeyDown || event.Rune() == 'j':
						cur := list.GetCurrentItem()
						total := list.GetItemCount()
						next := cur + 1
						for next < total && isHeaderLine(next) {
							next++
						}
						if next < total {
							list.SetCurrentItem(next)
						}
						return nil

					case event.Key() == tcell.KeyUp || event.Rune() == 'k':
						cur := list.GetCurrentItem()
						prev := cur - 1
						for prev >= 0 && isHeaderLine(prev) {
							prev--
						}
						if prev >= 0 {
							list.SetCurrentItem(prev)
						}
						return nil

					case event.Rune() == 'a':
						if tuiShell != nil {
							tuiShell.ShowInputModal(i18n.T("tui.team.columns.label_name_prompt"), "", func(name string) {
								if name == "" {
									return
								}
								name = strings.TrimSpace(name)
								if labelMappings != nil {
									if *labelMappings == nil {
										*labelMappings = make(map[string]string)
									}
									(*labelMappings)[name] = editingMappingsFor
								}
								renderMappings()
								if tuiShell != nil {
									tuiShell.ShowToast(i18n.Tf("tui.team.columns.label_added", name, editingMappingsFor), shell.ToastSuccess)
								}
							})
						}
						return nil

					case event.Key() == tcell.KeyEnter:
						ref := resolveMappingItem()
						if ref != nil && tuiShell != nil {
							tuiShell.ShowInputModal(i18n.T("tui.team.columns.rename_prompt"), ref.key, func(newName string) {
								if newName == "" || newName == ref.key {
									return
								}
								newName = strings.TrimSpace(newName)
								if ref.kind == "label" && labelMappings != nil {
									delete(*labelMappings, ref.key)
									(*labelMappings)[newName] = editingMappingsFor
								} else if ref.kind == "status" && statusMappings != nil {
									delete(*statusMappings, ref.key)
									(*statusMappings)[newName] = editingMappingsFor
								}
								renderMappings()
							})
						}
						return nil

					case event.Rune() == 'd':
						ref := resolveMappingItem()
						if ref != nil {
							if ref.kind == "label" && labelMappings != nil {
								delete(*labelMappings, ref.key)
							} else if ref.kind == "status" && statusMappings != nil {
								delete(*statusMappings, ref.key)
							}
							renderMappings()
							if tuiShell != nil {
								tuiShell.ShowToast(i18n.Tf("tui.team.columns.mapping_deleted", ref.key), shell.ToastSuccess)
							}
						}
						return nil
					}
					return event
				}

				// ═══════════════════════════════════════════════════
				// MODE: Columns
				// ═══════════════════════════════════════════════════
				idx := list.GetCurrentItem() - 1
				cols := *columns
				n := len(cols)
				isOnColumn := idx >= 0 && idx < n

				switch {
				case event.Key() == tcell.KeyDown || event.Rune() == 'j':
					cur := list.GetCurrentItem()
					if cur < n {
						list.SetCurrentItem(cur + 1)
					}
					return nil

				case event.Key() == tcell.KeyUp || event.Rune() == 'k':
					cur := list.GetCurrentItem()
					if cur > 1 {
						list.SetCurrentItem(cur - 1)
					}
					return nil

				case (event.Key() == tcell.KeyUp && event.Modifiers()&tcell.ModShift != 0) || event.Rune() == 'K':
					if isOnColumn && idx > 0 {
						cols[idx], cols[idx-1] = cols[idx-1], cols[idx]
						renderColumns()
						list.SetCurrentItem(idx)
						if tuiShell != nil {
							tuiShell.ShowToast(i18n.T("tui.team.columns.moved_up"), shell.ToastSuccess)
						}
					}
					return nil

				case (event.Key() == tcell.KeyDown && event.Modifiers()&tcell.ModShift != 0) || event.Rune() == 'J':
					if isOnColumn && idx < n-1 {
						cols[idx], cols[idx+1] = cols[idx+1], cols[idx]
						renderColumns()
						list.SetCurrentItem(idx + 2)
						if tuiShell != nil {
							tuiShell.ShowToast(i18n.T("tui.team.columns.moved_down"), shell.ToastSuccess)
						}
					}
					return nil

				case event.Rune() == 'a':
					if tuiShell != nil {
						tuiShell.ShowInputModal(i18n.T("cmd.discovery.field.column_name"), "", func(name string) {
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
							*columns = updated
							renderColumns()
							list.SetCurrentItem(pos + 1)
							tuiShell.ShowToast(i18n.Tf("tui.team.columns.column_added", newCol.Name), shell.ToastSuccess)
						})
					}
					return nil

				case event.Key() == tcell.KeyEnter:
					if isOnColumn && tuiShell != nil {
						tuiShell.ShowInputModal(i18n.T("cmd.discovery.field.column_rename"), cols[idx].Name, func(name string) {
							if name != "" {
								(*columns)[idx].Name = strings.TrimSpace(name)
								renderColumns()
							}
						})
					}
					return nil

				case event.Rune() == 'd':
					if !isOnColumn {
						return nil
					}
					if n <= 2 {
						if tuiShell != nil {
							tuiShell.ShowToast(i18n.T("cmd.discovery.validate.min_columns"), shell.ToastError)
						}
						return nil
					}
					deletedID := cols[idx].ID
					deletedName := cols[idx].Name
					copy(cols[idx:], cols[idx+1:])
					*columns = cols[:len(cols)-1]
					cleaned := 0
					if labelMappings != nil {
						for k, v := range *labelMappings {
							if v == deletedID {
								delete(*labelMappings, k)
								cleaned++
							}
						}
					}
					if statusMappings != nil {
						for k, v := range *statusMappings {
							if v == deletedID {
								delete(*statusMappings, k)
								cleaned++
							}
						}
					}
					renderColumns()
					msg := i18n.Tf("tui.team.columns.column_deleted", deletedName)
					if cleaned > 0 {
						msg += i18n.Tf("tui.team.columns.mappings_cleaned", cleaned)
					}
					if tuiShell != nil {
						tuiShell.ShowToast(msg, shell.ToastSuccess)
					}
					return nil

				case event.Rune() == 'r':
					if isOnColumn {
						current := cols[idx].Role
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
						(*columns)[idx].Role = columnEditorRoles[nextIdx]
						renderColumns()
					}
					return nil

				case event.Rune() == 'm':
					if hasMappings && isOnColumn {
						editingMappingsFor = cols[idx].ID
						renderMappings()
					}
					return nil

				case event.Key() == tcell.KeyCtrlS:
					if len(*columns) < 2 {
						if tuiShell != nil {
							tuiShell.ShowToast(i18n.T("cmd.discovery.validate.min_columns"), shell.ToastError)
						}
						return nil
					}
					hasTerminal := false
					for _, c := range *columns {
						if c.Role == teamstate.ColumnRoleTerminal {
							hasTerminal = true
							break
						}
					}
					if !hasTerminal {
						if tuiShell != nil {
							tuiShell.ShowToast(i18n.T("cmd.discovery.validate.need_terminal"), shell.ToastError)
						}
						return nil
					}
					*result = make([]teamstate.BoardColumnConfig, len(*columns))
					copy(*result, *columns)
					onDone()
					return nil
				}

				return event
			})

			container.AddItem(list, 0, 1, true)
			container.AddItem(hints, 1, 0, false)
			app.SetFocus(list)
		},
		Validate: func() string {
			if len(*result) < 2 {
				return i18n.T("cmd.discovery.validate.min_columns")
			}
			hasTerminal := false
			for _, c := range *result {
				if c.Role == teamstate.ColumnRoleTerminal {
					hasTerminal = true
					break
				}
			}
			if !hasTerminal {
				return i18n.T("cmd.discovery.validate.need_terminal")
			}
			return ""
		},
		OnDone: func() error {
			return nil
		},
		InfoFields: func() []views.InfoField {
			names := make([]string, len(*result))
			for i, c := range *result {
				names[i] = c.Name
			}
			return []views.InfoField{
				{Label: i18n.T("tui.team.columns.header_label"), Value: i18n.Tf("cmd.discovery.info.columns_selected", len(*result))},
				{Label: "Layout", Value: strings.Join(names, " → ")},
			}
		},
	}
}

// ── Column editor helpers ───────────────────────────────────────────────────

// columnRoleHex returns the hex background color for a role badge.
func columnRoleHex(role string) string {
	switch role {
	case teamstate.ColumnRoleInitial:
		return theme.WarningHex
	case teamstate.ColumnRoleTerminal:
		return theme.SuccessHex
	case teamstate.ColumnRoleBlocked:
		return theme.ErrorHex
	default:
		return theme.AccentHex
	}
}

// columnColorName returns a human-readable color description.
func columnColorName(col teamstate.BoardColumnConfig, resolved tcell.Color) string {
	if col.Color != "" {
		return col.Color
	}
	r, g, b := resolved.RGB()
	return fmt.Sprintf("auto (#%02x%02x%02x)", r, g, b)
}

// ─────────────────────────────────────────────────────────────────────────────
// Board column config action (standalone, no tracker required)
// ─────────────────────────────────────────────────────────────────────────────

func actionBoardColumnConfig() {
	if tuiShell == nil {
		return
	}

	a := MustApp()
	ctx := tuiShell.Context()

	// Resolve the active team
	activeTeam := a.Config.ActiveTeam()
	if !activeTeam.Enabled {
		tuiShell.ShowToast(i18n.T("tui.team.board_config.no_team"), shell.ToastError)
		return
	}

	statePath := activeTeam.StatePath
	if statePath == "" {
		statePath = config.TeamStatePath(activeTeam.StateRepo)
	}
	repo := teamstate.NewRepo(activeTeam.StateRepo, statePath)
	if !repo.IsCloned() {
		tuiShell.ShowToast(i18n.T("tui.team.board_config.not_cloned"), shell.ToastError)
		return
	}

	// Load current board config
	cfg, err := repo.LoadConfig()
	if err != nil {
		tuiShell.ShowToast(i18n.T("tui.team.board_config.config_load_error")+err.Error(), shell.ToastError)
		return
	}

	// Use existing columns or defaults
	var columns []teamstate.BoardColumnConfig
	if cfg.Board.HasCustomColumns() {
		columns = make([]teamstate.BoardColumnConfig, len(cfg.Board.Columns))
		copy(columns, cfg.Board.Columns)
	} else {
		def := teamstate.DefaultBoardConfig()
		columns = make([]teamstate.BoardColumnConfig, len(def.Columns))
		copy(columns, def.Columns)
	}

	// Load label/status mappings for the integrated editor
	labelMappings := cfg.Tracker.LabelStatusMapping
	if labelMappings == nil {
		labelMappings = make(map[string]string)
	}
	statusMappings := cfg.Tracker.StatusMapping
	if statusMappings == nil {
		statusMappings = make(map[string]string)
	}
	tType := cfg.Tracker.Type

	var finalColumns []teamstate.BoardColumnConfig

	editorStep := buildColumnEditorStep(&columns, &finalColumns, &labelMappings, &statusMappings, tType)

	wizard := views.NewInlineWizardView(views.InlineWizardConfig{
		ID:    "wizard.board.columns",
		Title: i18n.T("tui.tm.item.board_config"),
		Steps: []views.WizardStep{editorStep},
		OnComplete: func(completed bool, wizErr error) {
			if !completed || wizErr != nil {
				return
			}
			// Save columns + mappings to team-state config
			safego.Go(func() {
				freshCfg, err := repo.LoadConfig()
				if err != nil {
					tuiShell.App().QueueUpdateDraw(func() {
						tuiShell.ShowToast(i18n.T("tui.team.board_config.error_prefix")+err.Error(), shell.ToastError)
					})
					return
				}
				freshCfg.Board.Columns = finalColumns
				freshCfg.Tracker.LabelStatusMapping = labelMappings
				freshCfg.Tracker.StatusMapping = statusMappings
				if err := repo.SaveConfig(ctx, freshCfg); err != nil {
					tuiShell.App().QueueUpdateDraw(func() {
						tuiShell.ShowToast(i18n.T("tui.team.board_config.save_error")+err.Error(), shell.ToastError)
					})
					return
				}
				tuiShell.App().QueueUpdateDraw(func() {
					tuiShell.ShowToast(
						i18n.Tf("tui.team.board_config.saved", theme.IconSuccess, len(finalColumns)),
						shell.ToastSuccess,
					)
				})
			})
		},
	})

	tuiShell.PushView(wizard)
}
