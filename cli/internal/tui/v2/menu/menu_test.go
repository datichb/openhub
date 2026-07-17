package menu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testItems() []*MenuItem {
	return []*MenuItem{
		{ID: "home", Label: "Home", ViewID: "home"},
		{
			ID: "sessions", Label: "Sessions", Expanded: true,
			Children: []*MenuItem{
				{ID: "sessions.start", Label: "Start", ViewID: "sessions.start"},
				{ID: "sessions.quick", Label: "Quick", ViewID: "sessions.quick"},
			},
		},
		{
			ID: "projects", Label: "Projets", Expanded: false,
			Children: []*MenuItem{
				{ID: "projects.list", Label: "Liste", ViewID: "projects.list"},
				{ID: "projects.add", Label: "Ajouter", ViewID: "projects.add"},
			},
		},
	}
}

func TestMenu_New(t *testing.T) {
	m := New(testItems(), nil)
	assert.NotNil(t, m)
	assert.NotNil(t, m.Primitive())
}

func TestMenu_SetActive(t *testing.T) {
	m := New(testItems(), nil)
	m.SetActive("home")
	assert.Equal(t, "home", m.active)
}

func TestMenu_SelectedID(t *testing.T) {
	m := New(testItems(), nil)
	// First node should be selected by default
	id := m.SelectedID()
	assert.Equal(t, "home", id)
}

func TestMenuItem_IsCategory(t *testing.T) {
	tests := []struct {
		name     string
		item     MenuItem
		expected bool
	}{
		{"leaf", MenuItem{ID: "home", Label: "Home"}, false},
		{"category", MenuItem{ID: "sessions", Label: "Sessions", Children: []*MenuItem{{ID: "x"}}}, true},
		{"empty children", MenuItem{ID: "y", Label: "Y", Children: nil}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.item.IsCategory())
		})
	}
}

func TestMenuItem_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		item     MenuItem
		expected bool
	}{
		{"nil enabled", MenuItem{ID: "x"}, true},
		{"enabled true", MenuItem{ID: "x", Enabled: func() bool { return true }}, true},
		{"enabled false", MenuItem{ID: "x", Enabled: func() bool { return false }}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.item.IsEnabled())
		})
	}
}

func TestMenu_OnSelect(t *testing.T) {
	var selected *MenuItem
	m := New(testItems(), func(item *MenuItem) {
		selected = item
	})

	// Simulate select on the current node
	node := m.tree.GetCurrentNode()
	assert.NotNil(t, node)
	m.handleSelect(node)

	// "home" is a leaf, should trigger onSelect
	assert.NotNil(t, selected)
	assert.Equal(t, "home", selected.ID)
}

func TestMenu_HandleSelect_Category(t *testing.T) {
	m := New(testItems(), nil)

	// Get "sessions" node (a category)
	sessionsNode := m.nodeMap["sessions"]
	assert.NotNil(t, sessionsNode)

	initialExpanded := sessionsNode.IsExpanded()
	m.handleSelect(sessionsNode)
	assert.NotEqual(t, initialExpanded, sessionsNode.IsExpanded())
}
