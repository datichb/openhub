// Package menu provides a hierarchical navigable sidebar menu
// for the unified TUI shell, built on tview.TreeView.
package menu

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/datichb/openhub/cli/internal/tui/theme"
)

// MenuItem represents a single entry in the menu tree.
type MenuItem struct {
	// ID is the unique identifier (e.g., "sessions.start").
	ID string
	// Label is the display text.
	Label string
	// ViewID is the view to navigate to when selected (empty for category headers).
	ViewID string
	// Action is an optional callback for non-view actions (e.g., suspend+exec).
	Action func()
	// Children are sub-items (nil = leaf node).
	Children []*MenuItem
	// Expanded is the initial expand state for categories.
	Expanded bool
	// Enabled returns whether this item is selectable. Nil means always enabled.
	Enabled func() bool
}

// IsCategory returns true if this item has children (is a group header).
func (mi *MenuItem) IsCategory() bool {
	return len(mi.Children) > 0
}

// IsEnabled returns whether this item is currently selectable.
func (mi *MenuItem) IsEnabled() bool {
	if mi.Enabled == nil {
		return true
	}
	return mi.Enabled()
}

// OnSelect is called when a menu item is activated.
type OnSelect func(item *MenuItem)

// Menu is the navigable sidebar menu widget.
type Menu struct {
	tree        *tview.TreeView
	root        *tview.TreeNode
	items       []*MenuItem
	active      string // ID of the currently active view
	highlighted *tview.TreeNode // node currently under cursor
	onSelect    OnSelect
	nodeMap     map[string]*tview.TreeNode
}

// New creates a themed menu with the given items and selection callback.
func New(items []*MenuItem, onSelect OnSelect) *Menu {
	m := &Menu{
		items:    items,
		onSelect: onSelect,
		nodeMap:  make(map[string]*tview.TreeNode),
	}

	m.root = tview.NewTreeNode("")
	m.tree = tview.NewTreeView().
		SetRoot(m.root).
		SetTopLevel(1).
		SetGraphics(false)

	m.tree.SetBackgroundColor(theme.BgPanel)
	m.tree.SetBorderPadding(1, 0, 1, 1)
	m.tree.SetSelectedFunc(m.handleSelect)
	m.tree.SetChangedFunc(m.handleChanged)

	m.buildTree()
	m.setupKeys()

	return m
}

// Primitive returns the underlying tview primitive for layout integration.
func (m *Menu) Primitive() tview.Primitive {
	return m.tree
}

// SetActive highlights the given view ID as the current active view.
func (m *Menu) SetActive(viewID string) {
	m.active = viewID
	m.refreshNodeStyles()
}

// SelectedID returns the ID of the currently highlighted menu item.
func (m *Menu) SelectedID() string {
	node := m.tree.GetCurrentNode()
	if node == nil {
		return ""
	}
	ref, ok := node.GetReference().(*MenuItem)
	if !ok {
		return ""
	}
	return ref.ID
}

func (m *Menu) buildTree() {
	for _, item := range m.items {
		m.addNode(m.root, item, 0)
	}

	// Select first selectable node
	children := m.root.GetChildren()
	if len(children) > 0 {
		m.tree.SetCurrentNode(children[0])
	}
}

func (m *Menu) addNode(parent *tview.TreeNode, item *MenuItem, depth int) {
	var text string
	if item.IsCategory() {
		text = categoryText(item.Label)
	} else if depth > 0 {
		text = "    " + theme.IconArrow + " " + item.Label
	} else {
		text = item.Label
	}

	node := tview.NewTreeNode(text).
		SetReference(item).
		SetSelectable(!item.IsCategory() || item.IsCategory())

	if item.IsCategory() {
		node.SetExpanded(item.Expanded)
		node.SetTextStyle(tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.FgPrimary))
	} else {
		node.SetTextStyle(tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.FgPrimary).Bold(true))
	}

	if !item.IsEnabled() {
		node.SetColor(theme.FgMuted)
		node.SetSelectable(false)
	}

	m.nodeMap[item.ID] = node
	parent.AddChild(node)

	for _, child := range item.Children {
		m.addNode(node, child, depth+1)
	}
}

// categoryText formats a category label as "── LABEL ──────"
func categoryText(label string) string {
	upper := strings.ToUpper(label)
	// Pad with ── to fill ~20 chars
	suffix := strings.Repeat("─", max(2, 18-len(upper)))
	return "── " + upper + " " + suffix
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *Menu) refreshNodeStyles() {
	for id, node := range m.nodeMap {
		ref, ok := node.GetReference().(*MenuItem)
		if !ok {
			continue
		}

		if id == m.active {
			node.SetTextStyle(tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.Action).Bold(true))
			node.SetText("  " + theme.IconActive + " " + ref.Label)
		} else if ref.IsCategory() {
			text := categoryText(ref.Label)
			if m.hasActiveChild(ref) {
				node.SetTextStyle(tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.Action))
			} else {
				node.SetTextStyle(tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.FgPrimary))
			}
			node.SetText(text)
		} else if !ref.IsEnabled() {
			node.SetColor(theme.FgMuted)
		} else {
			node.SetTextStyle(tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.FgPrimary).Bold(true))
			node.SetText("    " + theme.IconArrow + " " + ref.Label)
		}
	}
}

// hasActiveChild returns true if any descendant of item is the active view.
func (m *Menu) hasActiveChild(item *MenuItem) bool {
	for _, child := range item.Children {
		if child.ID == m.active {
			return true
		}
		if m.hasActiveChild(child) {
			return true
		}
	}
	return false
}

func (m *Menu) handleSelect(node *tview.TreeNode) {
	ref, ok := node.GetReference().(*MenuItem)
	if !ok {
		return
	}

	if ref.IsCategory() {
		node.SetExpanded(!node.IsExpanded())
		return
	}

	if !ref.IsEnabled() {
		return
	}

	if m.onSelect != nil {
		m.onSelect(ref)
	}
}

func (m *Menu) handleChanged(node *tview.TreeNode) {
	// Remove gutter from previous highlighted node
	if m.highlighted != nil && m.highlighted != node {
		m.restoreNodeText(m.highlighted)
	}

	// Add gutter to current node (if it's a leaf item, not a category)
	if node != nil {
		ref, ok := node.GetReference().(*MenuItem)
		if ok && !ref.IsCategory() && ref.ID != m.active {
			node.SetText("  " + theme.IconGutter + " " + ref.Label)
			node.SetTextStyle(tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.FgPrimary).Bold(true))
		}
	}
	m.highlighted = node
}

// restoreNodeText resets a node's text to its default (without gutter).
func (m *Menu) restoreNodeText(node *tview.TreeNode) {
	ref, ok := node.GetReference().(*MenuItem)
	if !ok {
		return
	}
	if ref.ID == m.active {
		// Active node keeps its special styling
		return
	}
	if ref.IsCategory() {
		node.SetText(categoryText(ref.Label))
		node.SetTextStyle(tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.FgPrimary))
	} else if !ref.IsEnabled() {
		node.SetColor(theme.FgMuted)
	} else {
		node.SetText("    " + theme.IconArrow + " " + ref.Label)
		node.SetTextStyle(tcell.StyleDefault.Background(theme.BgPanel).Foreground(theme.FgPrimary).Bold(true))
	}
}

func (m *Menu) setupKeys() {
	m.tree.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case 'l':
			node := m.tree.GetCurrentNode()
			if node != nil && !node.IsExpanded() && len(node.GetChildren()) > 0 {
				node.SetExpanded(true)
			}
			return nil
		case 'h':
			node := m.tree.GetCurrentNode()
			if node != nil {
				if node.IsExpanded() && len(node.GetChildren()) > 0 {
					node.SetExpanded(false)
				}
			}
			return nil
		case 'g':
			children := m.root.GetChildren()
			if len(children) > 0 {
				m.tree.SetCurrentNode(children[0])
			}
			return nil
		case 'G':
			children := m.root.GetChildren()
			if len(children) > 0 {
				m.tree.SetCurrentNode(children[len(children)-1])
			}
			return nil
		case ' ':
			node := m.tree.GetCurrentNode()
			if node != nil && len(node.GetChildren()) > 0 {
				node.SetExpanded(!node.IsExpanded())
			}
			return nil
		}
		return event
	})
}
