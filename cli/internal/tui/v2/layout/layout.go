// Package layout provides the unified shell layout for all TUI views.
//
// Every full-screen view uses layout.Build() to get a consistent structure:
//   - Header bar (project name, accent icon)
//   - Sidebar (current command, future: navigable menu)
//   - Content panel (the view fills this)
//   - Info panel (optional, right side — summary, context)
//   - Status bar (contextual hints)
//
// The Layout handles global keybindings (Ctrl+N for sidebar toggle, Ctrl+C for quit)
// and the tview.Application lifecycle (theme, mouse, root).
package layout

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/v2/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// Config configures the app shell layout.
type Config struct {
	// ProjectName is the hub/project name displayed in the header bar.
	ProjectName string

	// Command is the current command name (e.g., "team init", "board").
	// Displayed in the sidebar and as the content panel border title.
	Command string

	// StatusHints is the initial footer hint text (view-specific shortcuts).
	// "ctrl+n menu · ctrl+c quit" is appended automatically.
	StatusHints string

	// InfoPanel enables the right-side info panel.
	// When true, the layout is: sidebar (1) | content (4) | info (2).
	// When false: sidebar (1) | content (5).
	InfoPanel bool

	// OnQuit is called when Ctrl+C is pressed, before app.Stop().
	// Use this to set result flags (e.g., WizardResult.Aborted = true).
	// If nil, Ctrl+C simply stops the application.
	OnQuit func()
}

// Result is returned by Build() for the view to use.
type Result struct {
	// Root is the top-level primitive. Pass to App.SetRoot().
	Root tview.Primitive

	// Content is the main panel where the view inserts its widgets.
	// It's a vertical Flex with BgPanel background and border padding.
	Content *tview.Flex

	// Sidebar is the left panel (tview.List). Currently shows the command name.
	// Will become a navigable menu in the future.
	Sidebar *tview.List

	// InfoPanel is the right-side panel for contextual information.
	// nil if Config.InfoPanel is false.
	// Use SetText() with dynamic colors to update content.
	InfoPanel *tview.TextView

	// StatusBar allows the view to update footer hints dynamically.
	StatusBar *widgets.StatusBar

	// App is the pre-configured tview.Application (theme applied, mouse enabled).
	// The view calls App.Run() after inserting content.
	App *tview.Application
}

// Build creates the full-screen shell layout with all elements pre-configured.
// The view inserts its content into Result.Content, then calls Result.App.SetRoot(Result.Root, true).Run().
func Build(cfg Config) *Result {
	app := tview.NewApplication()

	// ── Apply global theme (once for all views) ──
	tview.Styles = tview.Theme{
		PrimitiveBackgroundColor:    theme.BgApp,
		ContrastBackgroundColor:     theme.BgPanel,
		MoreContrastBackgroundColor: theme.BgElement,
		BorderColor:                 theme.BorderNormal,
		TitleColor:                  theme.FgPrimary,
		GraphicsColor:               theme.BorderNormal,
		PrimaryTextColor:            theme.FgPrimary,
		SecondaryTextColor:          theme.FgSecondary,
		TertiaryTextColor:           theme.FgMuted,
		InverseTextColor:            theme.BgApp,
		ContrastSecondaryTextColor:  theme.Accent,
	}

	// ── Header bar ──
	// 2 rows: 1 row of breathing space + 1 row centered bold text.
	// Fond BgElement (slightly lighter) creates a distinct title bar.
	headerBar := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText(fmt.Sprintf("\n%s◆[-] [::b]%s[-:-:-]",
			widgets.ColorTag(theme.Accent), cfg.ProjectName))
	headerBar.SetBackgroundColor(theme.BgElement)

	// ── Sidebar ──
	// tview.List préparé pour devenir un menu navigable (Ctrl+N toggle).
	// Pour l'instant : affiche le nom de la commande en cours.
	sidebar := tview.NewList().
		SetMainTextColor(theme.FgSecondary).
		SetSelectedBackgroundColor(theme.BgElement).
		SetSelectedTextColor(theme.Accent).
		SetHighlightFullLine(true)
	sidebar.SetBackgroundColor(theme.BgPanel).
		SetBorder(true).
		SetBorderColor(theme.BorderNormal).
		SetTitle(" Menu ").
		SetTitleColor(theme.FgSecondary)
	sidebar.AddItem(cfg.Command, "", 0, nil)

	// ── Content panel ──
	// The view inserts its widgets here. Border padding provides internal spacing
	// without needing nil spacer rows (which cause black lines).
	content := tview.NewFlex().SetDirection(tview.FlexRow)
	content.SetBackgroundColor(theme.BgPanel).
		SetBorder(true).
		SetBorderColor(theme.BorderFocus).
		SetTitle(" " + cfg.Command + " ").
		SetTitleColor(theme.Accent).
		SetBorderPadding(1, 0, 2, 2)

	// ── Info panel (optional) ──
	var infoPanel *tview.TextView
	if cfg.InfoPanel {
		infoPanel = tview.NewTextView().
			SetDynamicColors(true).
			SetScrollable(true)
		infoPanel.SetBackgroundColor(theme.BgPanel).
			SetBorder(true).
			SetBorderColor(theme.BorderNormal).
			SetTitle(" Info ").
			SetTitleColor(theme.FgSecondary).
			SetBorderPadding(1, 0, 1, 1)
	}

	// ── Status bar ──
	hints := cfg.StatusHints
	if hints != "" {
		hints += " · "
	}
	hints += "ctrl+n menu · ctrl+c quit"
	statusBar := widgets.NewStatusBar(hints)

	// ── Assembly ──
	// middle = sidebar + content (+ info panel if enabled)
	// Proportions:
	//   Without info: sidebar(1) + content(5) = ~16% / ~84%
	//   With info:    sidebar(1) + content(4) + info(2) = ~14% / ~57% / ~29%
	middle := tview.NewFlex()
	middle.AddItem(sidebar, 0, 1, false)
	if cfg.InfoPanel {
		middle.AddItem(content, 0, 4, true)
		middle.AddItem(infoPanel, 0, 2, false)
	} else {
		middle.AddItem(content, 0, 5, true)
	}
	middle.SetBackgroundColor(theme.BgApp)

	// root = header (2 rows) + middle (flexible) + status (1 row)
	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(headerBar, 2, 0, false).
		AddItem(middle, 0, 1, true).
		AddItem(statusBar.TextView, 1, 0, false)
	root.SetBackgroundColor(theme.BgApp)

	// ── Global keybindings ──
	sidebarFocused := false

	root.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyCtrlN:
			// Toggle focus between sidebar and content
			if sidebarFocused {
				app.SetFocus(content)
				sidebar.SetBorderColor(theme.BorderNormal)
				content.SetBorderColor(theme.BorderFocus)
			} else {
				app.SetFocus(sidebar)
				sidebar.SetBorderColor(theme.BorderFocus)
				content.SetBorderColor(theme.BorderNormal)
			}
			sidebarFocused = !sidebarFocused
			return nil

		case event.Key() == tcell.KeyCtrlC:
			if cfg.OnQuit != nil {
				cfg.OnQuit()
			}
			app.Stop()
			return nil
		}
		return event
	})

	return &Result{
		Root:      root,
		Content:   content,
		Sidebar:   sidebar,
		InfoPanel: infoPanel,
		StatusBar: statusBar,
		App:       app,
	}
}
