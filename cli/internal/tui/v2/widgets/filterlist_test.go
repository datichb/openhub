package widgets

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterableList_New(t *testing.T) {
	items := []FilterItem{
		{MainText: "Alpha", SecondaryText: "first"},
		{MainText: "Beta", SecondaryText: "second"},
		{MainText: "Gamma", SecondaryText: "third"},
	}

	var selected FilterItem
	fl := NewFilterableList(items, func(item FilterItem) {
		selected = item
	})

	assert.NotNil(t, fl)
	assert.Equal(t, 3, fl.list.GetItemCount())
	_ = selected // used by callback
}

func TestFilterableList_Filter(t *testing.T) {
	items := []FilterItem{
		{MainText: "Start session", SecondaryText: "launch opencode"},
		{MainText: "Board view", SecondaryText: "kanban"},
		{MainText: "Status check", SecondaryText: "system health"},
	}

	fl := NewFilterableList(items, nil)

	// Filter by "board"
	fl.filter("board")
	assert.Equal(t, 1, len(fl.visible))
	assert.Equal(t, "Board view", fl.visible[0].MainText)

	// Filter by "s" — should match Start and Status
	fl.filter("s")
	assert.Equal(t, 2, len(fl.visible))

	// Empty filter — show all
	fl.filter("")
	assert.Equal(t, 3, len(fl.visible))
}

func TestFilterableList_CaseInsensitive(t *testing.T) {
	items := []FilterItem{
		{MainText: "OpenCode Start"},
		{MainText: "Config Edit"},
	}

	fl := NewFilterableList(items, nil)
	fl.filter("OPENCODE")
	assert.Equal(t, 1, len(fl.visible))
	assert.Equal(t, "OpenCode Start", fl.visible[0].MainText)
}
