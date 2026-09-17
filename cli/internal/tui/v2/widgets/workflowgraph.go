package widgets

import (
	"fmt"

	"github.com/datichb/openhub/cli/internal/tui/theme"
	"github.com/datichb/openhub/cli/internal/workflow"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// WorkflowGraph is a custom tview primitive that renders an interactive
// workflow DAG with checkpoints, agents, and edges.
type WorkflowGraph struct {
	*tview.Box

	workflow *workflow.WorkflowDefinition
	layout   *GraphLayout
	selected int  // index into layout.Elements
	readonly bool // if true, no edits allowed

	// Callbacks
	onSelect func(elem GraphElement)
	onChange func() // called when selection changes (for detail panel refresh)

	// Scroll offset for large graphs.
	scrollX, scrollY int
}

// NewWorkflowGraph creates a new workflow graph widget.
func NewWorkflowGraph(wf *workflow.WorkflowDefinition, readonly bool) *WorkflowGraph {
	g := &WorkflowGraph{
		Box:      tview.NewBox(),
		workflow: wf,
		readonly: readonly,
	}
	g.recomputeLayout()
	return g
}

// SetWorkflow updates the workflow and recomputes the layout.
func (g *WorkflowGraph) SetWorkflow(wf *workflow.WorkflowDefinition) {
	g.workflow = wf
	g.recomputeLayout()
	if g.selected >= len(g.layout.Elements) {
		g.selected = 0
	}
}

// SetOnSelect sets the callback fired when an element is activated (Enter).
func (g *WorkflowGraph) SetOnSelect(fn func(GraphElement)) {
	g.onSelect = fn
}

// SetOnChange sets the callback fired when the selection changes.
func (g *WorkflowGraph) SetOnChange(fn func()) {
	g.onChange = fn
}

// SelectedElement returns the currently selected graph element, or nil.
func (g *WorkflowGraph) SelectedElement() *GraphElement {
	if g.layout == nil || len(g.layout.Elements) == 0 {
		return nil
	}
	if g.selected < 0 || g.selected >= len(g.layout.Elements) {
		return nil
	}
	elem := g.layout.Elements[g.selected]
	return &elem
}

// recomputeLayout rebuilds the graph layout from the current workflow.
func (g *WorkflowGraph) recomputeLayout() {
	if g.workflow == nil {
		g.layout = &GraphLayout{}
		return
	}
	g.layout = ComputeLayout(g.workflow)
}

// Draw renders the workflow graph.
func (g *WorkflowGraph) Draw(screen tcell.Screen) {
	g.DrawForSubclass(screen, g)
	x, y, w, h := g.GetInnerRect()

	if g.layout == nil || len(g.layout.Elements) == 0 {
		drawTextAt(screen, x+2, y+1, "No workflow defined", theme.StyleMuted)
		return
	}

	// Adjust scroll to keep selected element visible.
	g.adjustScroll(w, h)

	// Draw edges first (behind nodes).
	for _, edge := range g.layout.Edges {
		for _, pt := range edge.Points {
			px := pt.X - g.scrollX + x
			py := pt.Y - g.scrollY + y
			if px >= x && px < x+w && py >= y && py < y+h {
				screen.SetContent(px, py, '─', nil, theme.StyleMuted)
			}
		}
		// Arrow head at the end.
		if len(edge.Points) > 0 {
			last := edge.Points[len(edge.Points)-1]
			px := last.X - g.scrollX + x + 1
			py := last.Y - g.scrollY + y
			if px >= x && px < x+w && py >= y && py < y+h {
				screen.SetContent(px, py, '▶', nil, theme.StyleMuted)
			}
		}
	}

	// Draw separator line for independent agents.
	if g.layout.IndependentY > 0 {
		sepY := g.layout.IndependentY - g.scrollY + y - 1
		if sepY >= y && sepY < y+h {
			for sx := x; sx < x+w; sx++ {
				screen.SetContent(sx, sepY, '─', nil,
					tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.FgMuted))
			}
			drawTextAt(screen, x+1, sepY, " Indépendants ", tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.FgMuted))
		}
	}

	// Draw nodes.
	for i, node := range g.layout.Nodes {
		isSelected := g.isNodeSelected(node.ID)
		g.drawNode(screen, x, y, w, h, &node, isSelected, i)
	}

	// Highlight selected edge.
	if sel := g.SelectedElement(); sel != nil && sel.Type == ElementEdge {
		for _, edge := range g.layout.Edges {
			edgeID := edge.FromID + "→" + edge.ToID
			if edgeID == sel.ID {
				for _, pt := range edge.Points {
					px := pt.X - g.scrollX + x
					py := pt.Y - g.scrollY + y
					if px >= x && px < x+w && py >= y && py < y+h {
						screen.SetContent(px, py, '━', nil, theme.StyleAccent)
					}
				}
			}
		}
	}
}

func (g *WorkflowGraph) drawNode(screen tcell.Screen, ox, oy, ow, oh int, node *NodeLayout, selected bool, _ int) {
	nx := node.X - g.scrollX + ox
	ny := node.Y - g.scrollY + oy

	// Skip if completely out of view.
	if nx+node.W < ox || nx >= ox+ow || ny+node.H < oy || ny >= oy+oh {
		return
	}

	// Determine styles.
	borderStyle := tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.BorderCard)
	textStyle := theme.StyleDefault
	labelStyle := tcell.StyleDefault.Background(theme.BgCard).Foreground(theme.FgPrimary).Bold(true)

	if selected {
		borderStyle = tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.Accent)
		labelStyle = tcell.StyleDefault.Background(theme.BgCard).Foreground(theme.Accent).Bold(true)
	}

	if node.Locked {
		// Mandatory items get a distinct indicator.
		labelStyle = labelStyle.Foreground(theme.Warning)
	}

	// Background fill.
	for dy := 1; dy < node.H-1; dy++ {
		for dx := 1; dx < node.W-1; dx++ {
			px, py := nx+dx, ny+dy
			if px >= ox && px < ox+ow && py >= oy && py < oy+oh {
				screen.SetContent(px, py, ' ', nil, tcell.StyleDefault.Background(theme.BgCard))
			}
		}
	}

	// Border — rounded corners.
	setCell(screen, nx, ny, '╭', borderStyle, ox, oy, ow, oh)
	setCell(screen, nx+node.W-1, ny, '╮', borderStyle, ox, oy, ow, oh)
	setCell(screen, nx, ny+node.H-1, '╰', borderStyle, ox, oy, ow, oh)
	setCell(screen, nx+node.W-1, ny+node.H-1, '╯', borderStyle, ox, oy, ow, oh)

	for dx := 1; dx < node.W-1; dx++ {
		setCell(screen, nx+dx, ny, '─', borderStyle, ox, oy, ow, oh)
		setCell(screen, nx+dx, ny+node.H-1, '─', borderStyle, ox, oy, ow, oh)
	}
	for dy := 1; dy < node.H-1; dy++ {
		setCell(screen, nx, ny+dy, '│', borderStyle, ox, oy, ow, oh)
		setCell(screen, nx+node.W-1, ny+dy, '│', borderStyle, ox, oy, ow, oh)
	}

	// Content.
	prefix := ""
	if node.Locked {
		prefix = "🔒"
	}

	drawTextClipped(screen, nx+1, ny+1, node.W-2, prefix+node.Label, labelStyle, ox, oy, ow, oh)
	if node.Line2 != "" {
		drawTextClipped(screen, nx+1, ny+2, node.W-2, node.Line2, textStyle, ox, oy, ow, oh)
	}
	if node.Line3 != "" && node.H > 3 {
		drawTextClipped(screen, nx+1, ny+3, node.W-2, node.Line3, textStyle, ox, oy, ow, oh)
	}
}

func (g *WorkflowGraph) isNodeSelected(id string) bool {
	if sel := g.SelectedElement(); sel != nil {
		return sel.ID == id
	}
	return false
}

func (g *WorkflowGraph) adjustScroll(viewW, viewH int) {
	sel := g.SelectedElement()
	if sel == nil {
		return
	}

	// Ensure selected element is visible with 2-cell margin.
	margin := 2
	if sel.X-g.scrollX < margin {
		g.scrollX = sel.X - margin
	}
	if sel.X+sel.W-g.scrollX > viewW-margin {
		g.scrollX = sel.X + sel.W - viewW + margin
	}
	if sel.Y-g.scrollY < margin {
		g.scrollY = sel.Y - margin
	}
	if sel.Y+sel.H-g.scrollY > viewH-margin {
		g.scrollY = sel.Y + sel.H - viewH + margin
	}

	if g.scrollX < 0 {
		g.scrollX = 0
	}
	if g.scrollY < 0 {
		g.scrollY = 0
	}
}

// InputHandler handles keyboard navigation.
func (g *WorkflowGraph) InputHandler() func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
	return g.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p tview.Primitive)) {
		if g.layout == nil || len(g.layout.Elements) == 0 {
			return
		}

		prevSelected := g.selected

		switch event.Key() {
		case tcell.KeyLeft:
			g.navigateLeft()
		case tcell.KeyRight:
			g.navigateRight()
		case tcell.KeyUp:
			g.navigateUp()
		case tcell.KeyDown:
			g.navigateDown()
		case tcell.KeyEnter:
			if g.onSelect != nil && g.selected < len(g.layout.Elements) {
				g.onSelect(g.layout.Elements[g.selected])
			}
			return
		case tcell.KeyRune:
			switch event.Rune() {
			case 'h':
				g.navigateLeft()
			case 'l':
				g.navigateRight()
			case 'k':
				g.navigateUp()
			case 'j':
				g.navigateDown()
			}
		}

		if g.selected != prevSelected && g.onChange != nil {
			g.onChange()
		}
	})
}

// Navigation finds the nearest element in the requested direction.

func (g *WorkflowGraph) navigateLeft() {
	g.navigateToNearest(-1, 0)
}

func (g *WorkflowGraph) navigateRight() {
	g.navigateToNearest(1, 0)
}

func (g *WorkflowGraph) navigateUp() {
	g.navigateToNearest(0, -1)
}

func (g *WorkflowGraph) navigateDown() {
	g.navigateToNearest(0, 1)
}

func (g *WorkflowGraph) navigateToNearest(dx, dy int) {
	if len(g.layout.Elements) <= 1 {
		return
	}

	current := g.layout.Elements[g.selected]
	cx := current.X + current.W/2
	cy := current.Y + current.H/2

	bestIdx := -1
	bestDist := int(^uint(0) >> 1) // max int

	for i, elem := range g.layout.Elements {
		if i == g.selected {
			continue
		}
		ex := elem.X + elem.W/2
		ey := elem.Y + elem.H/2

		// Filter by direction.
		if dx > 0 && ex <= cx {
			continue
		}
		if dx < 0 && ex >= cx {
			continue
		}
		if dy > 0 && ey <= cy {
			continue
		}
		if dy < 0 && ey >= cy {
			continue
		}

		// Manhattan distance with direction bias.
		dist := abs(ex-cx) + abs(ey-cy)
		if dx != 0 {
			dist += abs(ey-cy) * 3 // penalize off-axis
		}
		if dy != 0 {
			dist += abs(ex-cx) * 3
		}

		if dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}

	if bestIdx >= 0 {
		g.selected = bestIdx
	}
}

// ---------------------------------------------------------------------------
// Drawing helpers
// ---------------------------------------------------------------------------

func setCell(screen tcell.Screen, x, y int, ch rune, style tcell.Style, ox, oy, ow, oh int) {
	if x >= ox && x < ox+ow && y >= oy && y < oy+oh {
		screen.SetContent(x, y, ch, nil, style)
	}
}

func drawTextAt(screen tcell.Screen, x, y int, text string, style tcell.Style) {
	for i, ch := range text {
		screen.SetContent(x+i, y, ch, nil, style)
	}
}

func drawTextClipped(screen tcell.Screen, x, y, maxW int, text string, style tcell.Style, ox, oy, ow, oh int) {
	runes := []rune(text)
	if len(runes) > maxW {
		runes = runes[:maxW]
	}
	for i, ch := range runes {
		px := x + i
		if px >= ox && px < ox+ow && y >= oy && y < oy+oh {
			screen.SetContent(px, y, ch, nil, style)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Focus returns true — this widget is focusable.
func (g *WorkflowGraph) Focus(delegate func(p tview.Primitive)) {
	g.Box.Focus(delegate)
}

// HasFocus returns whether this widget has focus.
func (g *WorkflowGraph) HasFocus() bool {
	return g.Box.HasFocus()
}

// GetElementInfo returns a display string for the currently selected element.
func (g *WorkflowGraph) GetElementInfo() string {
	sel := g.SelectedElement()
	if sel == nil {
		return ""
	}
	switch sel.Type {
	case ElementCheckpoint:
		cp := g.workflow.FindCheckpoint(sel.ID)
		if cp == nil {
			return sel.ID
		}
		info := fmt.Sprintf("Checkpoint: %s — %s", cp.ID, cp.Label)
		if cp.Mandatory {
			info += " [mandatory]"
		}
		return info
	case ElementAgent, ElementIndependentAgent:
		a := g.workflow.FindAgent(sel.ID)
		if a == nil {
			return sel.ID
		}
		info := fmt.Sprintf("Agent: %s — %s / %s", a.AgentID, a.Role, a.Mode)
		if a.Mandatory {
			info += " [mandatory]"
		}
		return info
	case ElementEdge:
		return fmt.Sprintf("Edge: %s (press 'a' to add checkpoint)", sel.ID)
	default:
		return sel.ID
	}
}
