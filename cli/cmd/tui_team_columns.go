package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/datichb/openhub/cli/internal/i18n"
	"github.com/datichb/openhub/cli/internal/safego"
	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/shell"
	"github.com/datichb/openhub/cli/internal/tui/v2/views"
)

// ─────────────────────────────────────────────────────────────────────────────
// Board column editor (shared helper + standalone action)
// ─────────────────────────────────────────────────────────────────────────────

// columnEditorRoles is the cycle order for the 'r' key in the column editor.
var columnEditorRoles = []string{
	teamstate.ColumnRoleInitial,
	teamstate.ColumnRoleActive,
	teamstate.ColumnRoleReview,
	teamstate.ColumnRoleTerminal,
	teamstate.ColumnRoleBlocked,
}

// columnEditorUI is what the column editor asks of the shell: an input box, a
// list to choose from, a confirmation and a toast.
type columnEditorUI interface {
	ShowInputModal(title, currentValue string, onConfirm func(newValue string))
	ShowSelectModal(title string, options []views.SelectOption, currentValue string, onConfirm func(value string))
	Confirm(title, detail string, danger bool, onYes func())
	ShowToast(msg string, level shell.ToastLevel)
}

// columnUI is the shell of the TUI, nil outside of it; a variable so that the
// tests give their own.
var columnUI = func() columnEditorUI {
	if tuiShell == nil {
		return nil
	}
	return tuiShell
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
	editor := &columnEditor{
		columns:        columns,
		result:         result,
		labelMappings:  labelMappings,
		statusMappings: statusMappings,
		trackerType:    trackerType,
	}
	return views.WizardStep{
		Label:      i18n.T("cmd.discovery.step_columns"),
		Processing: i18n.T("cmd.discovery.processing_columns"),
		CustomView: editor.mount,
		Validate:   func() string { return columnsProblem(*result) },
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

// columnsProblem says why columns cannot be kept: at least two, one of them
// terminal. It is empty for a valid layout.
func columnsProblem(columns []teamstate.BoardColumnConfig) string {
	if len(columns) < 2 {
		return i18n.T("cmd.discovery.validate.min_columns")
	}
	for _, c := range columns {
		if c.Role == teamstate.ColumnRoleTerminal {
			return ""
		}
	}
	return i18n.T("cmd.discovery.validate.need_terminal")
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
	case teamstate.ColumnRoleReview:
		return theme.TextSecondaryHex
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

// actionBoardColumnConfig opens the editor of the columns of the board and of
// the labels mapped to them, for the active team. It is the « Colonnes du
// board » entry of the Team mode and the C key of the team board. afterSave
// (nil: none) runs on the event loop once the changes are saved, so that the
// board reads its tickets again.
func actionBoardColumnConfig(afterSave func()) {
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
	repo := teamstate.NewRepo(activeTeam.StateRepo, statePath)
	if !repo.IsCloned() {
		tuiShell.ShowToast(i18n.T("tui.team.board_config.not_cloned"), shell.ToastError)
		return
	}

	// Load current board config
	cfg, err := repo.LoadConfig(ctx)
	if err != nil {
		tuiShell.ShowToast(i18n.T("tui.team.board_config.config_load_error")+err.Error(), shell.ToastError)
		return
	}

	// The config as read, before the editor changes the maps it shares with it:
	// the save writes the difference on top of the latest version.
	base := teamstate.CloneConfig(cfg)
	columns := boardColumnsOf(cfg)

	// Load label/status mappings for the integrated editor
	labelMappings := cfg.Tracker.LabelStatusMapping
	if labelMappings == nil {
		labelMappings = make(map[string]string)
	}
	statusMappings := cfg.Tracker.StatusMapping
	if statusMappings == nil {
		statusMappings = make(map[string]string)
	}
	listUnderTheirColumns(columns, labelMappings, statusMappings)

	var finalColumns []teamstate.BoardColumnConfig

	editorStep := buildColumnEditorStep(&columns, &finalColumns, &labelMappings, &statusMappings, cfg.Tracker.Type)

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
				saveBoardColumns(ctx, repo, base, finalColumns, labelMappings, statusMappings, afterSave)
			})
		},
	})

	tuiShell.PushView(wizard)
}

// listUnderTheirColumns gives the mappings the ID of their column: the editor
// lists the labels under the column they are mapped to, by ID, and "planned",
// the initial status of a board without columns of its own, is its initial
// column.
func listUnderTheirColumns(columns []teamstate.BoardColumnConfig, mappings ...map[string]string) {
	initialID := teamstate.BoardConfig{Columns: columns}.InitialStatus()
	for _, m := range mappings {
		for k, v := range m {
			if v == teamstate.ClaimStatusPlanned {
				m[k] = initialID
			}
		}
	}
}

// boardColumnsOf is a copy of the columns of the board of the team, or of the
// default board when the team has none of its own.
func boardColumnsOf(cfg *teamstate.TeamConfig) []teamstate.BoardColumnConfig {
	src := cfg.Board.Columns
	if !cfg.Board.HasCustomColumns() {
		src = teamstate.DefaultBoardConfig().Columns
	}
	columns := make([]teamstate.BoardColumnConfig, len(src))
	copy(columns, src)
	return columns
}

// boardConfigStore is what saving the columns of the board asks of the
// team-state: to write its configuration on top of the latest version.
type boardConfigStore interface {
	teamstate.ConfigWriter
}

// applyBoardColumns sets the columns and the mappings of the editor in cfg. A
// mapping to "todo" on a board that has no such column is the status of its
// initial column; the others that are no status of the board are refused, with
// their names, when the config is saved.
func applyBoardColumns(cfg *teamstate.TeamConfig, columns []teamstate.BoardColumnConfig, labelMappings, statusMappings map[string]string) {
	cfg.Board.Columns = columns
	cfg.Tracker.LabelStatusMapping = labelMappings
	cfg.Tracker.StatusMapping = statusMappings
	cfg.Tracker.NormalizeMappings(cfg.Board)
}

// saveBoardColumns writes the columns and the mappings of the editor to the
// config of the team-state, off the event loop, and says how it went; afterSave
// (nil: none) is called on the event loop once they are written. base is the
// config the editor started from: only what the editor changed is written, on
// top of the latest config.toml, and a column list or a mapping that a
// colleague changed meanwhile is not overwritten (the editor is opened again).
func saveBoardColumns(ctx context.Context, repo boardConfigStore, base *teamstate.TeamConfig, columns []teamstate.BoardColumnConfig, labelMappings, statusMappings map[string]string, afterSave func()) {
	edited := teamstate.CloneConfig(base)
	applyBoardColumns(edited, columns, labelMappings, statusMappings)
	if err := repo.UpdateConfigFrom(ctx, base, edited); err != nil {
		queueOnShell(tuiShell, func() {
			tuiShell.ShowToast(i18n.T("tui.team.board_config.save_error")+configSaveErrorText(err), shell.ToastError)
		})
		return
	}
	queueOnShell(tuiShell, func() {
		tuiShell.ShowToast(
			i18n.Tf("tui.team.board_config.saved", theme.IconSuccess, len(columns)),
			shell.ToastSuccess,
		)
		if afterSave != nil {
			afterSave()
		}
	})
}

// configSaveErrorText is the reason a write of the team config failed. A
// setting changed by a colleague meanwhile is said as such: nothing was
// overwritten, the screen is opened again to read their version.
func configSaveErrorText(err error) string {
	if c, ok := teamstate.IsConfigConflict(err); ok {
		return i18n.Tf("v533b.team_config.conflict_toast", strings.Join(c.Keys, ", "))
	}
	return err.Error()
}
