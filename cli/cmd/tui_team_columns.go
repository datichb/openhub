package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"

	"github.com/datichb/openhub/cli/internal/config"
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
	teamstate.ColumnRoleTerminal,
	teamstate.ColumnRoleBlocked,
}

// columnEditorUI is what the column editor asks of the shell: an input box and
// a toast.
type columnEditorUI interface {
	ShowInputModal(title, currentValue string, onConfirm func(newValue string))
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
				saveBoardColumns(ctx, repo, finalColumns, labelMappings, statusMappings)
			})
		},
	})

	tuiShell.PushView(wizard)
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

// saveBoardColumns writes the columns and the mappings to the config of the
// team-state, off the event loop, and says how it went.
func saveBoardColumns(ctx context.Context, repo *teamstate.Repo, columns []teamstate.BoardColumnConfig, labelMappings, statusMappings map[string]string) {
	freshCfg, err := repo.LoadConfig()
	if err != nil {
		tuiShell.App().QueueUpdateDraw(func() {
			tuiShell.ShowToast(i18n.T("tui.team.board_config.error_prefix")+err.Error(), shell.ToastError)
		})
		return
	}
	freshCfg.Board.Columns = columns
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
			i18n.Tf("tui.team.board_config.saved", theme.IconSuccess, len(columns)),
			shell.ToastSuccess,
		)
	})
}
