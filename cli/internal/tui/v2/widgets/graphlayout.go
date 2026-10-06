package widgets

import (
	"sort"

	"github.com/datichb/openhub/cli/internal/workflow"
)

// ---------------------------------------------------------------------------
// Graph layout types
// ---------------------------------------------------------------------------

// ElementType classifies what a navigable graph element is.
type ElementType int

const (
	ElementCheckpoint ElementType = iota
	ElementAgent
	ElementEdge
	ElementIndependentAgent
	// ElementStart is the column of the agents run before the first
	// checkpoint (entry agent, agents without `after`).
	ElementStart
)

// GraphElement is a single navigable item in the workflow graph.
type GraphElement struct {
	Type ElementType
	ID   string // checkpoint ID, agent ID, or edge "from→to"
	// Screen position and dimensions (cell coordinates).
	X, Y, W, H int
}

// NodeLayout holds the computed position of a single node (checkpoint or agent).
type NodeLayout struct {
	ID     string
	Label  string
	Line2  string // second line (e.g., behavior, role)
	Line3  string // third line (e.g., mode)
	X, Y   int
	W, H   int
	Type   ElementType
	Locked bool // mandatory / enforced
}

// EdgeLayout holds the path of a connection between two nodes.
type EdgeLayout struct {
	FromID string
	ToID   string
	Points []Point // sequence of cells to draw
}

// Point is a cell coordinate.
type Point struct {
	X, Y int
}

// GraphLayout is the fully computed layout for rendering the workflow graph.
type GraphLayout struct {
	Nodes    []NodeLayout
	Edges    []EdgeLayout
	Elements []GraphElement // ordered for navigation (tab order)
	// IndependentSection marks where the independent agents zone starts.
	IndependentY int
	TotalW       int
	TotalH       int
}

// ---------------------------------------------------------------------------
// Graph model (neutral: legacy definition or oh/v1 workflow)
// ---------------------------------------------------------------------------

// GraphCheckpoint is a checkpoint of the graph, in pass order.
type GraphCheckpoint struct {
	ID       string
	Label    string
	Behavior string // behavior in the shown mode
	Extra    string // third line (remote policy, origin…)
	Locked   bool   // mandatory
	Info     string // detail line (GetElementInfo)
}

// GraphAgent is an agent of the graph.
type GraphAgent struct {
	ID string
	// After is the checkpoint the agent is placed under ("" = start column).
	After       string
	Line2       string // mode, role…
	Extra       string // third line (origin…)
	Independent bool
	Locked      bool
	Info        string
}

// GraphModel is what the graph draws.
type GraphModel struct {
	Checkpoints []GraphCheckpoint
	Agents      []GraphAgent
	// StartLabel names the start column ("" = no start column: agents
	// without checkpoint are not shown, as in the legacy graph).
	StartLabel string
}

// SpecGraphModel is the graph of a resolved oh/v1 workflow in mode: the
// checkpoints in pass order, each workflow agent under the checkpoint it
// waits for (directly, or through the agent it follows), the others in the
// start column; independent agents apart. layer names the document layer
// that set each element (B10: the whole resolved workflow is shown, with
// the origin of each element).
func SpecGraphModel(sp *workflow.Spec, mode, lang, startLabel string, layer func(path string) string) GraphModel {
	m := GraphModel{StartLabel: startLabel}
	if sp == nil {
		return m
	}
	if layer == nil {
		layer = func(string) string { return "" }
	}
	isCP := map[string]bool{}
	for _, id := range sp.Checkpoints.Keys() {
		cp, _ := sp.Checkpoints.Get(id)
		if cp.Disabled {
			continue
		}
		isCP[id] = true
		b := string(cp.Mode[mode])
		if b == "" {
			b = "pause"
		}
		mandatory := cp.Mandatory != nil && *cp.Mandatory
		extra := layer("checkpoints." + id)
		if cp.Remote != "" {
			extra = "remote: " + string(cp.Remote)
		}
		m.Checkpoints = append(m.Checkpoints, GraphCheckpoint{ID: id, Label: cp.Label.Text(lang), Behavior: b,
			Extra: extra, Locked: mandatory, Info: id})
	}
	agents := map[string]workflow.AgentRef{}
	for _, id := range sp.Agents.Keys() {
		a, _ := sp.Agents.Get(id)
		agents[id] = a
	}
	// placeOf follows `after` through agents to a checkpoint.
	var placeOf func(id string, seen map[string]bool) string
	placeOf = func(id string, seen map[string]bool) string {
		a := agents[id]
		switch {
		case a.After == "" || seen[id]:
			return ""
		case isCP[a.After]:
			return a.After
		}
		seen[id] = true
		return placeOf(a.After, seen)
	}
	entry := sp.EntryAgent()
	ids := sp.Agents.Keys()
	if _, declared := agents[entry]; !declared && entry != "" {
		ids = append([]string{entry}, ids...)
		agents[entry] = workflow.AgentRef{Role: workflow.RoleWorkflow}
	}
	for _, id := range ids {
		a := agents[id]
		if a.Role == workflow.RoleDisabled {
			continue
		}
		line2 := string(a.Mode)
		if id == entry {
			line2 = "▶ " + line2
		}
		if a.After != "" && !isCP[a.After] {
			line2 += " ← " + a.After
		}
		g := GraphAgent{ID: id, Line2: line2, Extra: layer("agents." + id), Info: id,
			Independent: a.Role == workflow.RoleIndependent}
		if !g.Independent {
			g.After = placeOf(id, map[string]bool{})
		}
		m.Agents = append(m.Agents, g)
	}
	return m
}

// ---------------------------------------------------------------------------
// Layout computation
// ---------------------------------------------------------------------------

const (
	nodeWidth      = 16
	nodeHeight     = 4
	nodeHSpacing   = 4 // horizontal gap between checkpoint nodes
	nodeVSpacing   = 2 // vertical gap between checkpoint row and agent row
	agentNodeWidth = 14
	agentNodeH     = 3
	indepMarginTop = 2
)

// startID is the element id of the start column.
const startID = "▶"

// ComputeModelLayout builds a GraphLayout from a graph model.
//
// The layout strategy is:
//  1. Checkpoints are placed left-to-right in a single row (after the start
//     column when the model has one).
//  2. Workflow agents are grouped below the checkpoint they are attached to.
//  3. Independent agents are listed in a separate zone below the graph.
//  4. Edges connect consecutive checkpoints with horizontal lines.
func ComputeModelLayout(m GraphModel) *GraphLayout {
	layout := &GraphLayout{}
	cols := append([]GraphCheckpoint(nil), m.Checkpoints...)
	hasStart := m.StartLabel != ""
	// oh/v1 graph: one more line per node (behavior and origin/remote for
	// checkpoints, mode for agents); legacy graph: historical sizes.
	cpH, agH := nodeHeight, agentNodeH
	if hasStart {
		cpH, agH = nodeHeight+1, agentNodeH+1
	}
	if hasStart {
		cols = append([]GraphCheckpoint{{ID: startID, Label: m.StartLabel}}, cols...)
	}
	if len(cols) == 0 && len(m.Agents) == 0 {
		return layout
	}

	// --- Pass 1: Place checkpoints in a row ---
	x := 1
	cpPositions := make(map[string]Point) // checkpoint ID → center position
	for i, cp := range cols {
		typ := ElementCheckpoint
		if hasStart && i == 0 {
			typ = ElementStart
		}
		layout.Nodes = append(layout.Nodes, NodeLayout{ID: cp.ID, Label: truncate(cp.Label, nodeWidth-2),
			Line2: cp.Behavior, Line3: truncate(cp.Extra, nodeWidth-2), X: x, Y: 1, W: nodeWidth, H: cpH,
			Type: typ, Locked: cp.Locked})
		layout.Elements = append(layout.Elements, GraphElement{Type: typ, ID: cp.ID, X: x, Y: 1, W: nodeWidth, H: cpH})
		cpPositions[cp.ID] = Point{X: x + nodeWidth/2, Y: 1 + cpH}
		x += nodeWidth + nodeHSpacing
	}

	// --- Pass 2: Edges between consecutive checkpoints ---
	for i := 0; i < len(cols)-1; i++ {
		fromCP, toCP := cols[i], cols[i+1]
		fromX := cpPositions[fromCP.ID].X + nodeWidth/2
		toX := cpPositions[toCP.ID].X - nodeWidth/2 - nodeHSpacing
		edgeY := 1 + cpH/2 // middle of the checkpoint row
		edge := EdgeLayout{FromID: fromCP.ID, ToID: toCP.ID}
		for ex := fromX + 1; ex <= toX+nodeHSpacing-1; ex++ {
			edge.Points = append(edge.Points, Point{X: ex, Y: edgeY})
		}
		layout.Edges = append(layout.Edges, edge)
		if hasStart && i == 0 {
			continue // no checkpoint is inserted before the start column
		}
		// Edge is navigable (for adding checkpoints between two).
		edgeMidX := (fromX + toX + nodeHSpacing) / 2
		layout.Elements = append(layout.Elements, GraphElement{Type: ElementEdge, ID: fromCP.ID + "→" + toCP.ID,
			X: edgeMidX, Y: edgeY, W: 3, H: 1})
	}

	// --- Pass 3: Workflow agents below their checkpoints ---
	agentY := 1 + cpH + nodeVSpacing
	agentsPerCP := make(map[string][]GraphAgent)
	var order []string
	for _, a := range m.Agents {
		if a.Independent {
			continue
		}
		col := a.After
		if col == "" {
			if !hasStart {
				continue
			}
			col = startID
		}
		if _, ok := agentsPerCP[col]; !ok {
			order = append(order, col)
		}
		agentsPerCP[col] = append(agentsPerCP[col], a)
	}
	sort.SliceStable(order, func(i, j int) bool { return cpPositions[order[i]].X < cpPositions[order[j]].X })
	maxRows := 0
	for _, cpID := range order {
		agents := agentsPerCP[cpID]
		cpPos, ok := cpPositions[cpID]
		if !ok {
			continue
		}
		// oh/v1 graph (start column): agents stacked below their checkpoint;
		// legacy graph: side by side, centered below it.
		stack := hasStart
		startX := cpPos.X - agentNodeWidth/2
		if !stack {
			startX = cpPos.X - (len(agents)*agentNodeWidth+(len(agents)-1)*2)/2
			maxRows = max(maxRows, 1)
		} else {
			maxRows = max(maxRows, len(agents))
		}
		if startX < 1 {
			startX = 1
		}
		for i, a := range agents {
			ax, ay := startX, agentY+i*(agH+1)
			if !stack {
				ax, ay = startX+i*(agentNodeWidth+2), agentY
			}
			layout.Nodes = append(layout.Nodes, NodeLayout{ID: a.ID, Label: truncate(a.ID, agentNodeWidth-2), Line2: a.Line2,
				X: ax, Y: ay, W: agentNodeWidth, H: agH, Type: ElementAgent, Locked: a.Locked})
			layout.Elements = append(layout.Elements, GraphElement{Type: ElementAgent, ID: a.ID, X: ax, Y: ay, W: agentNodeWidth, H: agH})
		}
	}

	// --- Pass 4: Independent agents zone ---
	rowsH := max(1, maxRows)*(agH+1) - 1 // legacy: one row of agentNodeH
	indepY := agentY + rowsH + indepMarginTop + 1
	layout.IndependentY = indepY
	indepX := 1
	for _, a := range m.Agents {
		if !a.Independent {
			continue
		}
		layout.Nodes = append(layout.Nodes, NodeLayout{ID: a.ID, Label: truncate(a.ID, agentNodeWidth-2), Line2: a.Line2,
			X: indepX, Y: indepY + 1, W: agentNodeWidth, H: agH, Type: ElementIndependentAgent, Locked: a.Locked})
		layout.Elements = append(layout.Elements, GraphElement{Type: ElementIndependentAgent, ID: a.ID,
			X: indepX, Y: indepY + 1, W: agentNodeWidth, H: agH})
		indepX += agentNodeWidth + 2
	}

	layout.TotalW = max(x, indepX)
	layout.TotalH = indepY + agH + 3
	return layout
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}
