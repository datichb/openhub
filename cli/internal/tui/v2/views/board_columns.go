package views

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/teamstate"
	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// BoardColumnDef defines a kanban column.
type BoardColumnDef struct {
	Name   string
	Status string
	Color  tcell.Color
}

// DefaultColumns returns the standard kanban columns.
func DefaultColumns() []BoardColumnDef {
	return []BoardColumnDef{
		{Name: "TODO", Status: "todo", Color: theme.Warning},
		{Name: "IN PROGRESS", Status: "in_progress", Color: tcell.GetColor(theme.BlueHex)},
		{Name: "REVIEW", Status: "review", Color: theme.FgSecondary},
		{Name: "VALIDATION", Status: "validation", Color: theme.Info},
		{Name: "DONE", Status: "done", Color: theme.Success},
		{Name: "BLOCKED", Status: "blocked", Color: theme.Error},
	}
}

// ColumnsFromConfig builds board column definitions from a BoardConfig.
// Falls back to DefaultColumns() if no custom columns are configured.
// Colors are assigned based on column roles using the DS palette, with
// optional per-column color overrides.
func ColumnsFromConfig(cfg teamstate.BoardConfig) []BoardColumnDef {
	if !cfg.HasCustomColumns() {
		return DefaultColumns()
	}
	cols := make([]BoardColumnDef, len(cfg.Columns))
	activeIdx := 0
	for i, c := range cfg.Columns {
		cols[i] = BoardColumnDef{
			Name:   c.Name,
			Status: c.ID,
			Color:  ResolveColumnColor(c, activeIdx),
		}
		if c.Role == teamstate.ColumnRoleActive || c.Role == "" {
			activeIdx++
		}
	}
	return cols
}

// ResolveColumnColor determines the tcell.Color for a column.
// Priority: explicit color override > role-based DS palette > active cycling.
// activeIdx should be the count of active-role columns seen so far (for color cycling).
func ResolveColumnColor(c teamstate.BoardColumnConfig, activeIdx int) tcell.Color {
	// Explicit color override.
	if c.Color != "" {
		if color, ok := colorOverrides[c.Color]; ok {
			return color
		}
	}
	// Role-based DS palette.
	switch c.Role {
	case teamstate.ColumnRoleInitial:
		return theme.Warning
	case teamstate.ColumnRoleTerminal:
		return theme.Success
	case teamstate.ColumnRoleBlocked:
		return theme.Error
	default: // active or unset
		return activeColorCycle[activeIdx%len(activeColorCycle)]
	}
}

// activeColorCycle is the DS palette for "active" columns, cycling through
// six Catppuccin-derived hues so boards with many active columns stay visually
// distinct: Azure (blue), Overlay (gray), Sapphire (cyan), Mauve (purple),
// Yellow, Green.
var activeColorCycle = []tcell.Color{
	tcell.GetColor(theme.BlueHex),    // Azure (blue)
	theme.FgSecondary,                // Overlay (gray)
	theme.Info,                       // Sapphire (cyan)
	tcell.GetColor(theme.MauveHex),   // Mauve (purple)
	tcell.GetColor(theme.WarningHex), // Yellow
	tcell.GetColor(theme.SuccessHex), // Green
}

// colorOverrides maps user-facing color names to tcell.Color values.
var colorOverrides = map[string]tcell.Color{
	"orange": theme.Warning,
	"blue":   tcell.GetColor(theme.BlueHex),
	"gray":   theme.FgSecondary,
	"cyan":   theme.Info,
	"green":  theme.Success,
	"red":    theme.Error,
	"purple": tcell.GetColor(theme.MauveHex),
	"yellow": tcell.GetColor(theme.WarningHex),
}

// newColumnSeparator returns a 1-char-wide vertical line drawn between kanban
// columns. The line uses BorderCard (#45475a) — subtle enough to delimit
// columns without competing with card borders.
func newColumnSeparator() *tview.Box {
	sep := tview.NewBox()
	sep.SetBackgroundColor(theme.BgPanel)
	sep.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		style := tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.BorderCard)
		for dy := 0; dy < height; dy++ {
			screen.SetContent(x, y+dy, '│', nil, style)
		}
		return x, y, width, height
	})
	return sep
}

func priorityColor(priority string) tcell.Color {
	switch priority {
	case "P0", "critical":
		return theme.Error
	case "P1", "high":
		return theme.Warning
	case "P2", "medium":
		return theme.Accent
	case "P3", "low":
		return theme.FgSecondary
	default:
		return theme.FgMuted
	}
}

// priorityPrefix returns a short coloured prefix for the ticket priority,
// e.g. "[P0] " in red, "[P1] " in yellow, etc.
func priorityPrefix(priority string) string {
	if priority == "" {
		return ""
	}
	return priority + " · "
}
