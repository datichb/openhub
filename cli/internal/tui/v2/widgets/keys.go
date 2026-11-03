package widgets

import "github.com/datichb/openhub/cli/internal/tui/v2/keys"

// The keys of the widgets that the convention does not have: the rest (j/k,
// g/G, Home/End, Enter, Space) are the actions of the keys package.
var (
	// sectionPrevKey and sectionNextKey jump to the previous and the next
	// section of a SectionedList (Tab and Shift+Tab do the same).
	sectionPrevKey = keys.Own("list.section_prev", '{', "a12.keys.section_prev")
	sectionNextKey = keys.Own("list.section_next", '}', "a12.keys.section_next")
	// graphLeftKey and graphRightKey move the selection of a workflow graph
	// to the left and to the right (the arrows do the same).
	graphLeftKey  = keys.Own("graph.left", 'h', "a12.keys.graph_left")
	graphRightKey = keys.Own("graph.right", 'l', "a12.keys.graph_right")
)
