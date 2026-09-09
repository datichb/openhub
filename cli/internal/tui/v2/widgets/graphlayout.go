package widgets

import (
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

// ComputeLayout builds a GraphLayout from a workflow definition.
//
// The layout strategy is:
//  1. Checkpoints are placed left-to-right in a single row.
//  2. Workflow agents are grouped below the checkpoint they are attached to.
//  3. Independent agents are listed in a separate zone below the graph.
//  4. Edges connect consecutive checkpoints with horizontal lines.
func ComputeLayout(wf *workflow.WorkflowDefinition) *GraphLayout {
	layout := &GraphLayout{}

	if len(wf.Checkpoints) == 0 {
		return layout
	}

	// --- Pass 1: Place checkpoints in a row ---
	x := 1
	cpPositions := make(map[string]Point) // checkpoint ID → center position

	for _, cp := range wf.Checkpoints {
		node := NodeLayout{
			ID:     cp.ID,
			Label:  truncate(cp.Label, nodeWidth-2),
			Line2:  formatBehaviorSummary(cp, wf.Modes.Default),
			X:      x,
			Y:      1,
			W:      nodeWidth,
			H:      nodeHeight,
			Type:   ElementCheckpoint,
			Locked: cp.Mandatory,
		}
		layout.Nodes = append(layout.Nodes, node)
		layout.Elements = append(layout.Elements, GraphElement{
			Type: ElementCheckpoint,
			ID:   cp.ID,
			X:    x, Y: 1, W: nodeWidth, H: nodeHeight,
		})
		cpPositions[cp.ID] = Point{X: x + nodeWidth/2, Y: 1 + nodeHeight}
		x += nodeWidth + nodeHSpacing
	}

	// --- Pass 2: Edges between consecutive checkpoints ---
	for i := 0; i < len(wf.Checkpoints)-1; i++ {
		fromCP := wf.Checkpoints[i]
		toCP := wf.Checkpoints[i+1]
		fromX := cpPositions[fromCP.ID].X + nodeWidth/2
		toX := cpPositions[toCP.ID].X - nodeWidth/2 - nodeHSpacing
		edgeY := 1 + nodeHeight/2 // middle of the checkpoint row

		edge := EdgeLayout{
			FromID: fromCP.ID,
			ToID:   toCP.ID,
		}
		// Simple horizontal line.
		for ex := fromX + 1; ex <= toX+nodeHSpacing-1; ex++ {
			edge.Points = append(edge.Points, Point{X: ex, Y: edgeY})
		}
		layout.Edges = append(layout.Edges, edge)

		// Edge is navigable (for adding checkpoints between two).
		edgeMidX := (fromX + toX + nodeHSpacing) / 2
		layout.Elements = append(layout.Elements, GraphElement{
			Type: ElementEdge,
			ID:   fromCP.ID + "→" + toCP.ID,
			X:    edgeMidX, Y: edgeY, W: 3, H: 1,
		})
	}

	// --- Pass 3: Workflow agents below their checkpoints ---
	agentY := 1 + nodeHeight + nodeVSpacing
	agentsPerCP := make(map[string][]workflow.AgentSlot)

	for _, agent := range wf.WorkflowAgents() {
		if agent.Position != nil {
			cpID := agent.Position.AfterCheckpoint
			agentsPerCP[cpID] = append(agentsPerCP[cpID], agent)
		}
	}

	for cpID, agents := range agentsPerCP {
		cpPos, ok := cpPositions[cpID]
		if !ok {
			continue
		}
		// Center agents below the checkpoint.
		totalAgentWidth := len(agents)*agentNodeWidth + (len(agents)-1)*2
		agentStartX := cpPos.X - totalAgentWidth/2
		if agentStartX < 1 {
			agentStartX = 1
		}

		for i, agent := range agents {
			ax := agentStartX + i*(agentNodeWidth+2)
			node := NodeLayout{
				ID:     agent.AgentID,
				Label:  truncate(agent.AgentID, agentNodeWidth-2),
				Line2:  string(agent.Mode),
				X:      ax,
				Y:      agentY,
				W:      agentNodeWidth,
				H:      agentNodeH,
				Type:   ElementAgent,
				Locked: agent.Mandatory,
			}
			layout.Nodes = append(layout.Nodes, node)
			layout.Elements = append(layout.Elements, GraphElement{
				Type: ElementAgent,
				ID:   agent.AgentID,
				X:    ax, Y: agentY, W: agentNodeWidth, H: agentNodeH,
			})
		}
	}

	// --- Pass 4: Independent agents zone ---
	independentAgents := wf.IndependentAgents()
	indepY := agentY + agentNodeH + indepMarginTop + 1
	layout.IndependentY = indepY

	if len(independentAgents) > 0 {
		indepX := 1
		for _, agent := range independentAgents {
			node := NodeLayout{
				ID:     agent.AgentID,
				Label:  truncate(agent.AgentID, agentNodeWidth-2),
				Line2:  invokedByLabel(agent),
				X:      indepX,
				Y:      indepY + 1,
				W:      agentNodeWidth,
				H:      agentNodeH,
				Type:   ElementIndependentAgent,
				Locked: agent.Mandatory,
			}
			layout.Nodes = append(layout.Nodes, node)
			layout.Elements = append(layout.Elements, GraphElement{
				Type: ElementIndependentAgent,
				ID:   agent.AgentID,
				X:    indepX, Y: indepY + 1, W: agentNodeWidth, H: agentNodeH,
			})
			indepX += agentNodeWidth + 2
		}
	}

	// Compute total dimensions.
	layout.TotalW = x
	layout.TotalH = indepY + agentNodeH + 3

	return layout
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func formatBehaviorSummary(cp workflow.Checkpoint, defaultMode string) string {
	if b, ok := cp.Behavior[defaultMode]; ok {
		return string(b)
	}
	return ""
}

func invokedByLabel(a workflow.AgentSlot) string {
	if a.TaskPermissions == nil || len(a.TaskPermissions.CanBeInvokedBy) == 0 {
		return "—"
	}
	result := "← "
	for i, id := range a.TaskPermissions.CanBeInvokedBy {
		if i > 0 {
			result += ", "
		}
		if i >= 2 {
			result += "..."
			break
		}
		result += truncate(id, 8)
	}
	return result
}

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
