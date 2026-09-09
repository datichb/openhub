package views

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/tui/v2/widgets"
)

// ─────────────────────────────────────────────────────────────────────────────
// Layout modes
// ─────────────────────────────────────────────────────────────────────────────

type homeLayoutMode int

const (
	layoutNarrow homeLayoutMode = iota // < 100 cols → 72 chars
	layoutWide                         // 100-139 → 90 chars
	layoutDual                         // >= 140 → 2 columns
)

func layoutModeForWidth(width int) homeLayoutMode {
	switch {
	case width >= 140:
		return layoutDual
	case width >= 100:
		return layoutWide
	default:
		return layoutNarrow
	}
}

func contentWidthForMode(mode homeLayoutMode) int {
	switch mode {
	case layoutDual:
		return 134 // 2 × 65 + 4 gap
	case layoutWide:
		return 90
	default:
		return 72
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Dual-column state
// ─────────────────────────────────────────────────────────────────────────────

type homeDualLayout struct {
	left      *widgets.SectionedList
	right     *widgets.SectionedList
	activeCol int // 0 = left, 1 = right
	app       *tview.Application
}

func (d *homeDualLayout) activeList() *widgets.SectionedList {
	if d.activeCol == 1 {
		return d.right
	}
	return d.left
}

func (d *homeDualLayout) focusLeft() {
	d.activeCol = 0
	d.left.SetSelectedBackgroundColor(theme.BgElement)
	d.right.SetSelectedBackgroundColor(theme.BgPanel) // muted
	if d.app != nil {
		d.app.SetFocus(d.left)
	}
}

func (d *homeDualLayout) focusRight() {
	d.activeCol = 1
	d.right.SetSelectedBackgroundColor(theme.BgElement)
	d.left.SetSelectedBackgroundColor(theme.BgPanel) // muted
	if d.app != nil {
		d.app.SetFocus(d.right)
	}
}

// HandleKey processes inter-column navigation.
// Returns nil if consumed, or the original event to propagate.
func (d *homeDualLayout) HandleKey(event *tcell.EventKey) *tcell.EventKey {
	switch event.Key() {
	case tcell.KeyTab:
		if d.activeCol == 0 {
			d.focusRight()
		} else {
			d.focusLeft()
		}
		return nil
	case tcell.KeyBacktab:
		if d.activeCol == 1 {
			d.focusLeft()
		} else {
			d.focusRight()
		}
		return nil
	}
	switch event.Rune() {
	case 'l':
		if d.activeCol == 0 {
			d.focusRight()
		}
		return nil // always consume in dual mode to prevent omnibar activation
	case 'h':
		if d.activeCol == 1 {
			d.focusLeft()
		}
		return nil // always consume in dual mode to prevent omnibar activation
	}
	return event
}

// ─────────────────────────────────────────────────────────────────────────────
// Home flex builder
// ─────────────────────────────────────────────────────────────────────────────

// homeFlexConfig holds the parameters for building an adaptive home layout.
type homeFlexConfig struct {
	App          *tview.Application
	Header       tview.Primitive
	HeaderHeight int
	Footer       tview.Primitive
	FooterHeight int
	// LeftItems and RightItems are used in dual-column mode.
	LeftItems  []widgets.SectionItem
	RightItems []widgets.SectionItem
	// OnSelect is called when Enter is pressed on a non-header item.
	OnSelect func(index int, item widgets.SectionItem)
}

// homeFlexResult holds the constructed layout and optional dual state.
type homeFlexResult struct {
	// Root is the top-level flex to add to the content area.
	Root *tview.Flex
	// Dual is non-nil when the layout is 2-column. Used for HandleKey delegation.
	Dual *homeDualLayout
	// SingleList is the single SectionedList in narrow/wide mode. Nil in dual mode.
	SingleList *widgets.SectionedList
	// Mode is the layout mode that was chosen.
	Mode homeLayoutMode
}

// buildHomeLayout constructs a responsive home layout for the given width.
func buildHomeLayout(width int, cfg homeFlexConfig) homeFlexResult {
	mode := layoutModeForWidth(width)
	contentWidth := contentWidthForMode(mode)

	var centerContent tview.Primitive
	var result homeFlexResult
	result.Mode = mode

	if mode == layoutDual {
		result.Dual = buildDualColumn(cfg)
		centerContent = buildDualFlex(result.Dual)
	} else {
		list := buildSingleList(cfg)
		result.SingleList = list
		centerContent = list
	}

	// ── Inner flex: header + content + footer ──
	innerFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	innerFlex.SetBackgroundColor(theme.BgPanel)
	innerFlex.AddItem(cfg.Header, cfg.HeaderHeight, 0, false)
	innerFlex.AddItem(centerContent, 0, 1, true)
	if cfg.Footer != nil {
		innerFlex.AddItem(cfg.Footer, cfg.FooterHeight, 0, false)
	}

	// ── Horizontal centering ──
	hCenter := tview.NewFlex()
	hCenter.SetBackgroundColor(theme.BgPanel)
	hCenter.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)
	hCenter.AddItem(innerFlex, contentWidth, 0, true)
	hCenter.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)

	// ── Vertical centering (1:5:1 ratio) ──
	root := tview.NewFlex().SetDirection(tview.FlexRow)
	root.SetBackgroundColor(theme.BgPanel)
	root.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)
	root.AddItem(hCenter, 0, 5, true)
	root.AddItem(tview.NewBox().SetBackgroundColor(theme.BgPanel), 0, 1, false)

	result.Root = root
	return result
}

func buildSingleList(cfg homeFlexConfig) *widgets.SectionedList {
	list := widgets.NewSectionedList()
	list.SetApp(cfg.App)
	list.SetBackgroundColor(theme.BgPanel)
	list.SetBorderPadding(0, 0, 3, 3)

	// Merge left + right items for single-column display
	allItems := make([]widgets.SectionItem, 0, len(cfg.LeftItems)+len(cfg.RightItems))
	allItems = append(allItems, cfg.LeftItems...)
	allItems = append(allItems, cfg.RightItems...)
	list.SetItems(allItems)

	list.SetItemSelectedFunc(cfg.OnSelect)
	return list
}

func buildDualColumn(cfg homeFlexConfig) *homeDualLayout {
	left := widgets.NewSectionedList()
	left.SetApp(cfg.App)
	left.SetBackgroundColor(theme.BgPanel)
	left.SetBorderPadding(0, 0, 3, 1)
	left.SetItems(cfg.LeftItems)
	left.SetItemSelectedFunc(cfg.OnSelect)
	left.SetTabCaptureDisabled(true) // Tab switches columns, not sections

	right := widgets.NewSectionedList()
	right.SetApp(cfg.App)
	right.SetBackgroundColor(theme.BgPanel)
	right.SetBorderPadding(0, 0, 1, 3)
	right.SetItems(cfg.RightItems)
	right.SetItemSelectedFunc(cfg.OnSelect)
	right.SetTabCaptureDisabled(true) // Tab switches columns, not sections

	// Right column starts muted (left has focus by default)
	right.SetSelectedBackgroundColor(theme.BgPanel)

	d := &homeDualLayout{
		left:  left,
		right: right,
		app:   cfg.App,
	}
	return d
}

func buildDualFlex(d *homeDualLayout) *tview.Flex {
	sep := tview.NewBox().SetBackgroundColor(theme.BgPanel)
	sep.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		// Draw a thin vertical separator
		for row := y; row < y+height; row++ {
			screen.SetContent(x, row, '│', nil,
				tcell.StyleDefault.Foreground(theme.FgMuted).Background(theme.BgPanel))
		}
		return x, y, width, height
	})

	flex := tview.NewFlex()
	flex.SetBackgroundColor(theme.BgPanel)
	flex.AddItem(d.left, 0, 1, true)
	flex.AddItem(sep, 1, 0, false)
	flex.AddItem(d.right, 0, 1, false)
	return flex
}

// ─────────────────────────────────────────────────────────────────────────────
// Resize-reactive wrapper
// ─────────────────────────────────────────────────────────────────────────────

// adaptiveHomeMount installs a responsive layout into content that reacts to
// terminal resize. It calls buildFn(width) to construct the layout, and
// reinstalls it whenever the terminal width crosses a breakpoint.
//
// buildFn receives the current terminal width and must return a homeFlexResult.
// The returned result's Root is installed into content.
//
// Returns the initial homeFlexResult.
func adaptiveHomeMount(
	app *tview.Application,
	content *tview.Flex,
	buildFn func(width int) homeFlexResult,
) homeFlexResult {
	// Initial build with a fallback width
	_, _, initWidth, _ := content.GetInnerRect()
	if initWidth <= 0 {
		initWidth = 80 // fallback before first draw
	}

	result := buildFn(initWidth)
	content.AddItem(result.Root, 0, 1, true)

	// Track the current layout mode to detect breakpoint crossings
	currentMode := result.Mode

	content.SetDrawFunc(func(screen tcell.Screen, x, y, width, height int) (int, int, int, int) {
		newMode := layoutModeForWidth(width)
		if newMode != currentMode {
			currentMode = newMode
			// Rebuild on the next event loop tick to avoid mutating during draw
			go func() {
				if app != nil {
					app.QueueUpdateDraw(func() {
						// Clear and rebuild
						content.RemoveItem(result.Root)
						result = buildFn(width)
						content.Clear()
						content.AddItem(result.Root, 0, 1, true)

						// Focus the appropriate widget
						if result.Dual != nil {
							result.Dual.focusLeft()
						} else if result.SingleList != nil {
							app.SetFocus(result.SingleList)
						}
					})
				}
			}()
		}
		return x, y, width, height
	})

	return result
}
